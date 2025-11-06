package catalog

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

// Manager handles downloading, caching, and indexing the k0rdent catalog.
type Manager struct {
	db         *DB
	httpClient *http.Client
	cacheDir   string
	cacheTTL   time.Duration
	archiveURL string
	logger     *slog.Logger
}

// NewManager constructs a Manager with the provided options. If options are incomplete,
// sensible defaults are applied.
func NewManager(opts Options) (*Manager, error) {
	if opts.CacheDir == "" {
		opts.CacheDir = DefaultCacheDir
	}
	if opts.ArchiveURL == "" {
		opts.ArchiveURL = DefaultArchiveURL
	}
	if opts.CacheTTL == 0 {
		opts.CacheTTL = DefaultCacheTTL
	}
	if opts.DownloadTimeout == 0 {
		opts.DownloadTimeout = DefaultDownloadTimeout
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	// Create HTTP client with timeout if not provided
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: opts.DownloadTimeout,
		}
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(opts.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	// Open or create database at {cacheDir}/catalog.db
	dbPath := filepath.Join(opts.CacheDir, "catalog.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open catalog database: %w", err)
	}

	m := &Manager{
		db:         db,
		httpClient: client,
		cacheDir:   opts.CacheDir,
		cacheTTL:   opts.CacheTTL,
		archiveURL: opts.ArchiveURL,
		logger:     logging.WithComponent(opts.Logger, "catalog.manager"),
	}

	return m, nil
}

// List returns catalog entries, optionally filtered by app slug. If refresh is true,
// or if the cache is stale/missing, a fresh download and index build occurs.
func (m *Manager) List(ctx context.Context, appFilter string, refresh bool) ([]CatalogEntry, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("list catalog entries", "app_filter", appFilter, "refresh", refresh)

	// Ensure index is built
	if err := m.loadOrRefreshIndex(ctx, refresh); err != nil {
		logger.Error("failed to load or refresh catalog index", "error", err)
		return nil, err
	}

	// Query database
	appsWithTemplates, err := m.db.ListApps(appFilter)
	if err != nil {
		logger.Error("failed to query apps from database", "error", err)
		return nil, fmt.Errorf("query apps: %w", err)
	}

	// Convert to CatalogEntry (keep existing type for compatibility)
	results := make([]CatalogEntry, 0, len(appsWithTemplates))
	for _, awt := range appsWithTemplates {
		versions := make([]ServiceTemplateVersion, 0, len(awt.Templates))
		for _, tmpl := range awt.Templates {
			versions = append(versions, ServiceTemplateVersion{
				Name:                tmpl.ChartName,
				Version:             tmpl.Version,
				Repository:          "", // Will be populated from manifest if needed
				ServiceTemplatePath: tmpl.ServiceTemplatePath,
				HelmRepositoryPath:  tmpl.HelmRepositoryPath,
			})
		}
		results = append(results, CatalogEntry{
			Slug:               awt.App.Slug,
			Title:              awt.App.Title,
			Summary:            awt.App.Summary,
			Tags:               awt.App.Tags,
			ValidatedPlatforms: awt.App.ValidatedPlatforms,
			Versions:           versions,
		})
	}

	logger.Info("catalog entries listed", "count", len(results))
	return results, nil
}

// GetManifests retrieves the ServiceTemplate and optional HelmRepository manifests
// for a specific app, template name, and version. Returns the manifests as byte slices.
func (m *Manager) GetManifests(ctx context.Context, app, template, version string) ([][]byte, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("get manifests", "app", app, "template", template, "version", version)

	if err := m.loadOrRefreshIndex(ctx, false); err != nil {
		logger.Error("failed to load catalog index", "error", err)
		return nil, err
	}

	// Query database for paths
	st, err := m.db.GetServiceTemplate(app, template, version)
	if err != nil {
		logger.Error("service template not found", "app", app, "template", template, "version", version, "error", err)
		return nil, fmt.Errorf("app %q template %q version %q not found", app, template, version)
	}

	// Read manifests from disk using paths from DB
	cacheDir, err := m.getCurrentCacheDir()
	if err != nil {
		logger.Error("failed to determine current cache directory", "error", err)
		return nil, err
	}

	manifests := [][]byte{}

	// Read ServiceTemplate manifest (required)
	stPath := filepath.Join(cacheDir, st.ServiceTemplatePath)
	stData, err := os.ReadFile(stPath)
	if err != nil {
		err = fmt.Errorf("read service template manifest: %w", err)
		logger.Error("failed to read service template manifest", "path", stPath, "error", err)
		return nil, err
	}
	manifests = append(manifests, stData)

	// Read HelmRepository manifest (optional)
	if st.HelmRepositoryPath != "" {
		hrPath := filepath.Join(cacheDir, st.HelmRepositoryPath)
		hrData, err := os.ReadFile(hrPath)
		if err != nil {
			logger.Warn("failed to read helm repository manifest", "path", hrPath, "error", err)
		} else {
			manifests = append(manifests, hrData)
		}
	}

	logger.Info("manifests retrieved", "app", app, "template", template, "version", version, "manifest_count", len(manifests))
	return manifests, nil
}

// loadOrRefreshIndex ensures the database index is populated. If refresh is true,
// or the cache is stale, a new download and indexing pass occurs.
func (m *Manager) loadOrRefreshIndex(ctx context.Context, refresh bool) error {
	logger := logging.WithContext(ctx, m.logger)

	// Check if DB needs rebuild
	currentSHA, err := m.db.GetMetadata("catalog_sha")
	if err != nil {
		logger.Error("failed to get catalog SHA from database", "error", err)
		return fmt.Errorf("get catalog SHA: %w", err)
	}

	// Check if cache is still valid and we don't need to refresh
	if currentSHA != "" && !refresh {
		if valid, err := m.isCacheValid(); err == nil && valid {
			logger.Debug("using existing catalog index")
			return nil
		}
	}

	// Download and extract catalog
	logger.Info("downloading catalog archive", "url", m.archiveURL)
	start := time.Now()

	actualSHA, cacheDir, err := m.downloadAndExtract(ctx)
	if err != nil {
		logger.Error("failed to download and extract catalog", "error", err, "duration_ms", time.Since(start).Milliseconds())
		return err
	}

	logger.Info("catalog archive downloaded", "sha", actualSHA, "duration_ms", time.Since(start).Milliseconds())

	// If SHA changed or refresh requested, rebuild DB
	if actualSHA != currentSHA || refresh {
		logger.Debug("rebuilding catalog database", "old_sha", currentSHA, "new_sha", actualSHA)
		indexStart := time.Now()

		if err := m.db.ClearAll(); err != nil {
			logger.Error("failed to clear database", "error", err)
			return fmt.Errorf("clear database: %w", err)
		}

		if err := buildDatabaseIndex(m.db, cacheDir); err != nil {
			logger.Error("failed to build catalog index", "error", err)
			return fmt.Errorf("build database index: %w", err)
		}

		if err := m.db.SetMetadata("catalog_sha", actualSHA); err != nil {
			logger.Error("failed to set catalog SHA", "error", err)
			return fmt.Errorf("set catalog SHA: %w", err)
		}

		if err := m.db.SetMetadata("indexed_at", time.Now().Format(time.RFC3339)); err != nil {
			logger.Error("failed to set indexed_at", "error", err)
			return fmt.Errorf("set indexed_at: %w", err)
		}

		logger.Info("catalog index built", "duration_ms", time.Since(indexStart).Milliseconds())
	} else {
		logger.Debug("catalog SHA unchanged, skipping index rebuild")
	}

	return nil
}

// downloadAndExtract fetches the catalog tarball, computes its SHA, extracts it
// to a cache directory, and stores metadata. Returns SHA and extraction directory path.
func (m *Manager) downloadAndExtract(ctx context.Context) (string, string, error) {
	logger := logging.WithContext(ctx, m.logger)

	// Download tarball
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.archiveURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download catalog archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Read response into memory and compute SHA
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read archive data: %w", err)
	}

	hash := sha256.Sum256(data)
	sha := hex.EncodeToString(hash[:])
	logger.Debug("catalog archive downloaded", "sha", sha, "size_bytes", len(data))

	// Check if this SHA is already cached
	cacheSubdir := fmt.Sprintf("catalog-%s", sha)
	extractDir := filepath.Join(m.cacheDir, cacheSubdir)

	if _, err := os.Stat(extractDir); err == nil {
		logger.Info("catalog already cached", "sha", sha)
		return sha, extractDir, nil
	}

	// Extract tarball
	logger.Debug("extracting catalog archive", "dest", extractDir)
	if err := m.extractTarball(data, extractDir); err != nil {
		return "", "", fmt.Errorf("extract archive: %w", err)
	}

	// Write cache metadata
	metadata := CacheMetadata{
		SHA:       sha,
		Timestamp: time.Now(),
		URL:       m.archiveURL,
	}

	metadataPath := filepath.Join(m.cacheDir, "metadata.json")
	metadataData, err := json.Marshal(metadata)
	if err != nil {
		logger.Warn("failed to marshal cache metadata", "error", err)
	} else {
		if err := os.WriteFile(metadataPath, metadataData, 0644); err != nil {
			logger.Warn("failed to write cache metadata", "error", err)
		}
	}

	logger.Info("catalog extracted successfully", "sha", sha)
	return sha, extractDir, nil
}

// extractTarball unpacks a gzipped tar archive into the destination directory.
func (m *Manager) extractTarball(data []byte, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	gzReader, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		// Security: validate path to prevent directory traversal
		if strings.Contains(header.Name, "..") {
			m.logger.Warn("skipping suspicious tar entry", "name", header.Name)
			continue
		}

		// Strip the top-level directory (catalog-main/)
		parts := strings.SplitN(header.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		targetPath := filepath.Join(destDir, parts[1])

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("create parent directory for %s: %w", targetPath, err)
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", targetPath, err)
			}
			outFile.Close()
		}
	}

	return nil
}

// isCacheValid checks if the current cache is still within its TTL period.
func (m *Manager) isCacheValid() (bool, error) {
	metadataPath := filepath.Join(m.cacheDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read cache metadata: %w", err)
	}

	var metadata CacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return false, fmt.Errorf("unmarshal cache metadata: %w", err)
	}

	age := time.Since(metadata.Timestamp)
	return age < m.cacheTTL, nil
}

// getCurrentCacheDir returns the path to the most recently cached catalog extraction.
func (m *Manager) getCurrentCacheDir() (string, error) {
	metadataPath := filepath.Join(m.cacheDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", fmt.Errorf("read cache metadata: %w", err)
	}

	var metadata CacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("unmarshal cache metadata: %w", err)
	}

	cacheSubdir := fmt.Sprintf("catalog-%s", metadata.SHA)
	return filepath.Join(m.cacheDir, cacheSubdir), nil
}

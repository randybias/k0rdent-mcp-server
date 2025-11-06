package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	// Query database to verify template exists
	_, err := m.db.GetServiceTemplate(app, template, version)
	if err != nil {
		logger.Error("service template not found", "app", app, "template", template, "version", version, "error", err)
		return nil, fmt.Errorf("app %q template %q version %q not found", app, template, version)
	}

	manifests := [][]byte{}

	// Fetch ServiceTemplate manifest from GitHub (required)
	stURL := m.constructManifestURL(app, template, version)
	logger.Debug("fetching service template manifest", "url", stURL)

	stData, err := m.fetchManifestWithRetry(ctx, stURL)
	if err != nil {
		logger.Error("failed to fetch service template manifest", "url", stURL, "error", err)
		return nil, fmt.Errorf("fetch service template manifest: %w", err)
	}
	manifests = append(manifests, stData)

	// Fetch HelmRepository manifest (optional)
	// Note: HelmRepository is shared across all templates
	hrURL := m.constructHelmRepoURL()
	logger.Debug("fetching helm repository manifest", "url", hrURL)

	hrData, err := m.fetchManifestWithRetry(ctx, hrURL)
	if err != nil {
		logger.Warn("failed to fetch helm repository manifest", "url", hrURL, "error", err)
	} else {
		manifests = append(manifests, hrData)
	}

	logger.Info("manifests retrieved", "app", app, "template", template, "version", version, "manifest_count", len(manifests))
	return manifests, nil
}

// loadOrRefreshIndex ensures the database index is populated. If refresh is true,
// or the cache is stale, a new download and indexing pass occurs.
func (m *Manager) loadOrRefreshIndex(ctx context.Context, refresh bool) error {
	logger := logging.WithContext(ctx, m.logger)

	// Get currently cached index timestamp from database
	currentIndexTimestamp, err := m.db.GetMetadata("index_timestamp")
	if err != nil {
		logger.Error("failed to get index timestamp from database", "error", err)
		return fmt.Errorf("get index timestamp: %w", err)
	}

	// Check if cache is still valid (TTL not expired) and we don't need to refresh
	if currentIndexTimestamp != "" && !refresh {
		if valid, err := m.isCacheValid(); err == nil && valid {
			logger.Debug("using existing catalog index (cache TTL valid)", "timestamp", currentIndexTimestamp)
			return nil
		}
	}

	// Fetch JSON catalog index to check if it has changed
	logger.Debug("fetching JSON index to check for updates", "url", m.archiveURL)
	start := time.Now()

	index, actualSHA, err := m.fetchJSONIndex(ctx)
	if err != nil {
		logger.Error("failed to fetch JSON catalog index", "error", err, "duration_ms", time.Since(start).Milliseconds())
		return err
	}

	newIndexTimestamp := index.Metadata.Generated
	logger.Debug("JSON index fetched", "sha", actualSHA, "timestamp", newIndexTimestamp, "duration_ms", time.Since(start).Milliseconds())

	// Compare timestamps to determine if index needs rebuilding
	if newIndexTimestamp != currentIndexTimestamp || currentIndexTimestamp == "" || refresh {
		logger.Info("catalog index changed or missing, rebuilding database",
			"old_timestamp", currentIndexTimestamp,
			"new_timestamp", newIndexTimestamp,
			"refresh_requested", refresh)
		indexStart := time.Now()

		if err := m.db.ClearAll(); err != nil {
			logger.Error("failed to clear database", "error", err)
			return fmt.Errorf("clear database: %w", err)
		}

		// Parse JSON index into database rows
		apps, templates, err := m.parseJSONIndex(index)
		if err != nil {
			logger.Error("failed to parse JSON index", "error", err)
			return fmt.Errorf("parse JSON index: %w", err)
		}

		// Insert apps and templates into database
		for _, app := range apps {
			if err := m.db.UpsertApp(app); err != nil {
				logger.Error("failed to insert app", "slug", app.Slug, "error", err)
				return fmt.Errorf("insert app %s: %w", app.Slug, err)
			}
		}

		for _, tmpl := range templates {
			if err := m.db.UpsertServiceTemplate(tmpl); err != nil {
				logger.Error("failed to insert template", "app", tmpl.AppSlug, "chart", tmpl.ChartName, "version", tmpl.Version, "error", err)
				return fmt.Errorf("insert template %s/%s/%s: %w", tmpl.AppSlug, tmpl.ChartName, tmpl.Version, err)
			}
		}

		// Store the index timestamp from metadata.generated
		if err := m.db.SetMetadata("index_timestamp", newIndexTimestamp); err != nil {
			logger.Error("failed to set index timestamp", "error", err)
			return fmt.Errorf("set index timestamp: %w", err)
		}

		// Store SHA for reference (keeping for backward compatibility)
		if err := m.db.SetMetadata("catalog_sha", actualSHA); err != nil {
			logger.Error("failed to set catalog SHA", "error", err)
			return fmt.Errorf("set catalog SHA: %w", err)
		}

		if err := m.db.SetMetadata("indexed_at", time.Now().Format(time.RFC3339)); err != nil {
			logger.Error("failed to set indexed_at", "error", err)
			return fmt.Errorf("set indexed_at: %w", err)
		}

		// Write cache metadata with index timestamp
		metadata := CacheMetadata{
			SHA:            actualSHA,
			Timestamp:      time.Now(),
			URL:            m.archiveURL,
			IndexTimestamp: newIndexTimestamp,
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

		logger.Info("catalog index rebuilt successfully",
			"app_count", len(apps),
			"template_count", len(templates),
			"timestamp", newIndexTimestamp,
			"duration_ms", time.Since(indexStart).Milliseconds())
	} else {
		logger.Debug("catalog index timestamp unchanged, skipping rebuild", "timestamp", currentIndexTimestamp)
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

// fetchJSONIndex downloads the JSON catalog index from the configured URL,
// computes its SHA256 hash, and returns both the parsed index and the hash.
func (m *Manager) fetchJSONIndex(ctx context.Context) (*JSONIndex, string, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("fetching JSON index", "url", m.archiveURL)

	// Download JSON index
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.archiveURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download catalog index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Read response into memory and compute SHA
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read index data: %w", err)
	}

	hash := sha256.Sum256(data)
	sha := hex.EncodeToString(hash[:])
	logger.Debug("JSON index downloaded", "sha", sha, "size_bytes", len(data))

	// Parse JSON
	var index JSONIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, "", fmt.Errorf("parse JSON index: %w", err)
	}

	logger.Info("JSON index fetched successfully", "sha", sha, "addon_count", len(index.Addons))
	return &index, sha, nil
}

// parseJSONIndex converts a JSONIndex structure into database rows for storage.
// Returns slices of AppRow and ServiceTemplateRow entries ready to be inserted into SQLite.
func (m *Manager) parseJSONIndex(index *JSONIndex) ([]AppRow, []ServiceTemplateRow, error) {
	if index == nil {
		return nil, nil, fmt.Errorf("index is nil")
	}

	apps := make([]AppRow, 0, len(index.Addons))
	templates := []ServiceTemplateRow{}

	for _, addon := range index.Addons {
		// Map JSONAddon to AppRow
		app := AppRow{
			Slug:    addon.Name,
			Title:   addon.Name, // JSON index doesn't have a separate title field
			Summary: addon.Description,
			Tags:    addon.Metadata.Tags,
			// ValidatedPlatforms not available in JSON index yet
			ValidatedPlatforms: []string{},
		}
		apps = append(apps, app)

		// Map charts and versions to ServiceTemplateRow
		for _, chart := range addon.Charts {
			for _, version := range chart.Versions {
				// Note: ServiceTemplatePath and HelmRepositoryPath will need to be
				// populated separately when we implement manifest downloading from OCI
				tmpl := ServiceTemplateRow{
					AppSlug:             addon.Name,
					ChartName:           chart.Name,
					Version:             version,
					ServiceTemplatePath: "", // To be populated when downloading manifests
					HelmRepositoryPath:  "", // To be populated when downloading manifests
				}
				templates = append(templates, tmpl)
			}
		}
	}

	m.logger.Debug("parsed JSON index", "app_count", len(apps), "template_count", len(templates))
	return apps, templates, nil
}

// constructManifestURL builds the GitHub raw URL for a ServiceTemplate manifest.
// Pattern: https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/{slug}/charts/{name}-service-template-{version}/templates/service-template.yaml
func (m *Manager) constructManifestURL(slug, name, version string) string {
	return fmt.Sprintf(
		"https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/%s/charts/%s-service-template-%s/templates/service-template.yaml",
		slug, name, version,
	)
}

// constructHelmRepoURL builds the GitHub raw URL for the HelmRepository manifest.
// This is a constant path as the HelmRepository is shared across all templates.
func (m *Manager) constructHelmRepoURL() string {
	return "https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/k0rdent-utils/charts/k0rdent-catalog-1.0.0/templates/helm-repository.yaml"
}

// fetchManifestWithRetry fetches a manifest from a URL with retry logic and timeout.
// It will retry up to 3 times with exponential backoff on transient errors.
func (m *Manager) fetchManifestWithRetry(ctx context.Context, url string) ([]byte, error) {
	const maxRetries = 3
	const initialBackoff = 500 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff * (1 << uint(attempt-1))
			m.logger.Debug("retrying manifest fetch", "url", url, "attempt", attempt+1, "backoff_ms", backoff.Milliseconds())

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		data, err := m.fetchManifest(ctx, url)
		if err == nil {
			return data, nil
		}

		lastErr = err

		// Check if we should retry based on error type
		if !m.shouldRetry(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// fetchManifest performs a single HTTP GET request to fetch a manifest.
func (m *Manager) fetchManifest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return data, nil
}

// shouldRetry determines if an error is transient and should trigger a retry.
func (m *Manager) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retry on network errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") {
		return true
	}

	// Retry on 5xx server errors
	if strings.Contains(errStr, "status code 5") {
		return true
	}

	// Retry on 429 (Too Many Requests)
	if strings.Contains(errStr, "status code 429") {
		return true
	}

	// Don't retry on 4xx client errors (except 429)
	if strings.Contains(errStr, "status code 4") {
		return false
	}

	// For other errors, be conservative and don't retry
	return false
}

package catalog

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain creates the test tarball if it doesn't exist
func TestMain(m *testing.M) {
	if err := createTestTarball(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test tarball: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// createTestTarball creates a tarball from the testdata directory structure
func createTestTarball() error {
	testdataDir := filepath.Join("testdata", "catalog-fake-sha", "catalog-main")
	tarballPath := filepath.Join("testdata", "test-archive.tar.gz")

	// Check if tarball already exists and is newer than testdata
	if info, err := os.Stat(tarballPath); err == nil {
		testdataInfo, err := os.Stat(testdataDir)
		if err == nil && info.ModTime().After(testdataInfo.ModTime()) {
			return nil // Tarball is up to date
		}
	}

	// Create the tarball
	file, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("create tarball file: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Walk the testdata directory and add files to the tarball
	err = filepath.Walk(testdataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("create tar header: %w", err)
		}

		// Set name relative to testdata dir, adding catalog-main/ prefix
		relPath, err := filepath.Rel(testdataDir, path)
		if err != nil {
			return fmt.Errorf("get relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		header.Name = filepath.Join("catalog-main", relPath)

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header: %w", err)
		}

		// Write file content if it's a regular file
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			if _, err := tarWriter.Write(data); err != nil {
				return fmt.Errorf("write file to tar: %w", err)
			}
		}

		return nil
	})

	return err
}

func TestNewManager(t *testing.T) {
	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: "http://example.com/catalog.tar.gz",
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	if manager.cacheDir != cacheDir {
		t.Errorf("expected cache dir %s, got %s", cacheDir, manager.cacheDir)
	}

	if manager.archiveURL != "http://example.com/catalog.tar.gz" {
		t.Errorf("unexpected archive URL: %s", manager.archiveURL)
	}

	// Verify cache directory was created
	if _, err := os.Stat(cacheDir); err != nil {
		t.Errorf("cache directory was not created: %v", err)
	}
}

func TestNewManagerDefaults(t *testing.T) {
	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		CacheDir: cacheDir,
	})

	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager.archiveURL != DefaultArchiveURL {
		t.Errorf("expected default archive URL %s, got %s", DefaultArchiveURL, manager.archiveURL)
	}

	if manager.cacheTTL != DefaultCacheTTL {
		t.Errorf("expected default cache TTL %v, got %v", DefaultCacheTTL, manager.cacheTTL)
	}

	if manager.httpClient.Timeout != DefaultDownloadTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultDownloadTimeout, manager.httpClient.Timeout)
	}
}

func TestListWithoutCache(t *testing.T) {
	// Create test server serving tarball
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			t.Logf("failed to read test tarball: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	entries, err := manager.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) == 0 {
		t.Error("expected entries, got none")
	}

	// Verify minio entry exists
	var minioEntry *CatalogEntry
	for i := range entries {
		if entries[i].Slug == "minio" {
			minioEntry = &entries[i]
			break
		}
	}

	if minioEntry == nil {
		t.Fatal("minio entry not found")
	}

	if minioEntry.Title != "MinIO Object Storage" {
		t.Errorf("unexpected title: %s", minioEntry.Title)
	}

	if len(minioEntry.Versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(minioEntry.Versions))
	}
}

func TestListWithCache(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// First call - should download
	_, err = manager.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("first List failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 request after first List, got %d", requestCount)
	}

	// Second call - should use cache
	_, err = manager.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("second List failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 request after second List (cached), got %d", requestCount)
	}
}

func TestListWithRefresh(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// First call
	_, err = manager.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("first List failed: %v", err)
	}

	// Second call with refresh=true - should download again
	_, err = manager.List(context.Background(), "", true)
	if err != nil {
		t.Fatalf("second List with refresh failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests after refresh, got %d", requestCount)
	}
}

func TestListWithAppFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// List with filter
	entries, err := manager.List(context.Background(), "minio", false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Slug != "minio" {
		t.Errorf("expected slug 'minio', got '%s'", entries[0].Slug)
	}

	// Filter for non-existent app
	entries, err = manager.List(context.Background(), "nonexistent", false)
	if err != nil {
		t.Fatalf("List with nonexistent filter failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for nonexistent app, got %d", len(entries))
	}
}

func TestGetManifests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// First list to populate cache
	_, err = manager.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Get manifests for minio
	manifests, err := manager.GetManifests(context.Background(), "minio", "minio", "14.1.2")
	if err != nil {
		t.Fatalf("GetManifests failed: %v", err)
	}

	if len(manifests) < 1 {
		t.Fatal("expected at least 1 manifest (ServiceTemplate)")
	}

	// Verify ServiceTemplate content
	stContent := string(manifests[0])
	if !strings.Contains(stContent, "kind: ServiceTemplate") {
		t.Error("expected ServiceTemplate manifest to contain 'kind: ServiceTemplate'")
	}

	if !strings.Contains(stContent, "minio") {
		t.Error("expected ServiceTemplate manifest to contain 'minio'")
	}

	// If HelmRepository exists, verify it
	if len(manifests) > 1 {
		hrContent := string(manifests[1])
		if !strings.Contains(hrContent, "kind: HelmRepository") {
			t.Error("expected HelmRepository manifest to contain 'kind: HelmRepository'")
		}
	}
}

func TestGetManifestsNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// First list to populate cache
	_, err = manager.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Try to get manifests for non-existent app
	_, err = manager.GetManifests(context.Background(), "nonexistent", "test", "1.0.0")
	if err == nil {
		t.Fatal("expected error for non-existent app")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestGetManifestsVersionNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// First list to populate cache
	_, err = manager.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Try to get manifests for wrong version
	_, err = manager.GetManifests(context.Background(), "minio", "minio", "99.99.99")
	if err == nil {
		t.Fatal("expected error for non-existent version")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestCacheValidation(t *testing.T) {
	cacheDir := t.TempDir()

	// Create a valid cache metadata
	metadata := CacheMetadata{
		SHA:       "test-sha",
		Timestamp: time.Now().Add(-30 * time.Minute),
		URL:       "http://example.com/catalog.tar.gz",
	}

	metadataPath := filepath.Join(cacheDir, "metadata.json")
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	manager := &Manager{
		cacheDir: cacheDir,
		cacheTTL: time.Hour,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	valid, err := manager.isCacheValid()
	if err != nil {
		t.Fatalf("isCacheValid failed: %v", err)
	}

	if !valid {
		t.Error("expected cache to be valid")
	}
}

func TestCacheValidationExpired(t *testing.T) {
	cacheDir := t.TempDir()

	// Create an expired cache metadata
	metadata := CacheMetadata{
		SHA:       "test-sha",
		Timestamp: time.Now().Add(-2 * time.Hour),
		URL:       "http://example.com/catalog.tar.gz",
	}

	metadataPath := filepath.Join(cacheDir, "metadata.json")
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	manager := &Manager{
		cacheDir: cacheDir,
		cacheTTL: time.Hour,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	valid, err := manager.isCacheValid()
	if err != nil {
		t.Fatalf("isCacheValid failed: %v", err)
	}

	if valid {
		t.Error("expected cache to be invalid (expired)")
	}
}

func TestCacheValidationMissing(t *testing.T) {
	cacheDir := t.TempDir()

	manager := &Manager{
		cacheDir: cacheDir,
		cacheTTL: time.Hour,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	valid, err := manager.isCacheValid()
	if err != nil {
		t.Fatalf("isCacheValid failed: %v", err)
	}

	if valid {
		t.Error("expected cache to be invalid (missing metadata)")
	}
}

func TestCacheInvalidationCorruptMetadata(t *testing.T) {
	cacheDir := t.TempDir()

	// Write corrupt metadata
	metadataPath := filepath.Join(cacheDir, "metadata.json")
	if err := os.WriteFile(metadataPath, []byte("invalid json {"), 0644); err != nil {
		t.Fatalf("failed to write corrupt metadata: %v", err)
	}

	manager := &Manager{
		cacheDir: cacheDir,
		cacheTTL: time.Hour,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := manager.isCacheValid()
	if err == nil {
		t.Fatal("expected error for corrupt metadata")
	}
}

func TestDownloadAndExtractNetworkError(t *testing.T) {
	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL:      "http://invalid-host-that-does-not-exist.example.com/catalog.tar.gz",
		CacheDir:        cacheDir,
		CacheTTL:        time.Hour,
		DownloadTimeout: 1 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = manager.List(context.Background(), "", false)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestDownloadAndExtractHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = manager.List(context.Background(), "", false)
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to mention 404, got: %v", err)
	}
}

func TestDownloadAndExtractCorruptArchive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not a valid gzip archive"))
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = manager.List(context.Background(), "", false)
	if err == nil {
		t.Fatal("expected error for corrupt archive")
	}
}

func TestContextCancellation(t *testing.T) {
	// Create a slow server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		data, _ := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL:      ts.URL,
		CacheDir:        cacheDir,
		CacheTTL:        time.Hour,
		DownloadTimeout: 10 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = manager.List(ctx, "", false)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "timeout") {
		t.Logf("error: %v", err)
	}
}

func TestExtractTarballSecurityPathTraversal(t *testing.T) {
	// Create a malicious tarball with path traversal
	cacheDir := t.TempDir()
	manager := &Manager{
		cacheDir: cacheDir,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Create tarball with dangerous path
	var buf strings.Builder
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add entry with .. in path
	header := &tar.Header{
		Name:     "catalog-main/../../../etc/passwd",
		Mode:     0644,
		Size:     4,
		Typeflag: tar.TypeReg,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tarWriter.Write([]byte("test")); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()

	extractDir := filepath.Join(cacheDir, "test-extract")
	err := manager.extractTarball([]byte(buf.String()), extractDir)

	// Should not error, but should skip the malicious entry
	if err != nil {
		t.Fatalf("extractTarball failed: %v", err)
	}

	// Verify the dangerous file was not created
	dangerousPath := filepath.Join(extractDir, "..", "..", "..", "etc", "passwd")
	if _, err := os.Stat(dangerousPath); !os.IsNotExist(err) {
		t.Error("dangerous file should not have been extracted")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "test-archive.tar.gz"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		ArchiveURL: ts.URL,
		CacheDir:   cacheDir,
		CacheTTL:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Make concurrent List calls
	const numGoroutines = 10
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := manager.List(context.Background(), "", false)
			errChan <- err
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent List call failed: %v", err)
		}
	}
}

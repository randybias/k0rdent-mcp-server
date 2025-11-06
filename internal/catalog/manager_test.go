package catalog

import (
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
	// Create test server serving JSON index
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			t.Logf("failed to read test JSON index: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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

	if minioEntry.Title != "minio" {
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
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
		data, _ := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
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

func TestConcurrentAccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join("testdata", "valid-index.json"))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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

// TestConstructManifestURL verifies the URL construction for ServiceTemplate manifests.
func TestConstructManifestURL(t *testing.T) {
	manager := &Manager{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	tests := []struct {
		name     string
		slug     string
		tmplName string
		version  string
		expected string
	}{
		{
			name:     "minio template",
			slug:     "minio",
			tmplName: "minio",
			version:  "14.1.2",
			expected: "https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/minio/charts/minio-service-template-14.1.2/templates/service-template.yaml",
		},
		{
			name:     "postgresql template",
			slug:     "postgresql",
			tmplName: "postgresql",
			version:  "12.3.4",
			expected: "https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/postgresql/charts/postgresql-service-template-12.3.4/templates/service-template.yaml",
		},
		{
			name:     "template with hyphens",
			slug:     "cert-manager",
			tmplName: "cert-manager",
			version:  "1.0.0",
			expected: "https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/cert-manager/charts/cert-manager-service-template-1.0.0/templates/service-template.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := manager.constructManifestURL(tt.slug, tt.tmplName, tt.version)
			if url != tt.expected {
				t.Errorf("expected URL %q, got %q", tt.expected, url)
			}
		})
	}
}

// TestConstructHelmRepoURL verifies the URL construction for HelmRepository manifest.
func TestConstructHelmRepoURL(t *testing.T) {
	manager := &Manager{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	expected := "https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/k0rdent-utils/charts/k0rdent-catalog-1.0.0/templates/helm-repository.yaml"
	url := manager.constructHelmRepoURL()

	if url != expected {
		t.Errorf("expected URL %q, got %q", expected, url)
	}
}

// TestFetchManifestSuccess tests successful manifest fetch.
func TestFetchManifestSuccess(t *testing.T) {
	expectedContent := "apiVersion: v1\nkind: ServiceTemplate\nmetadata:\n  name: test"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedContent))
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	data, err := manager.fetchManifest(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchManifest failed: %v", err)
	}

	if string(data) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(data))
	}
}

// TestFetchManifest404 tests manifest fetch returning 404.
func TestFetchManifest404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := manager.fetchManifest(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to mention 404, got: %v", err)
	}
}

// TestFetchManifest500 tests manifest fetch returning 500.
func TestFetchManifest500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := manager.fetchManifest(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention 500, got: %v", err)
	}
}

// TestFetchManifestNetworkError tests manifest fetch with network error.
func TestFetchManifestNetworkError(t *testing.T) {
	manager := &Manager{
		httpClient: &http.Client{Timeout: 1 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Try to connect to a non-routable address
	_, err := manager.fetchManifest(context.Background(), "http://192.0.2.1:9999/manifest.yaml")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

// TestFetchManifestTimeout tests manifest fetch with timeout.
func TestFetchManifestTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.Write([]byte("too slow"))
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 100 * time.Millisecond},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	ctx := context.Background()
	_, err := manager.fetchManifest(ctx, ts.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// TestFetchManifestContextCancellation tests manifest fetch with cancelled context.
func TestFetchManifestContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("delayed"))
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := manager.fetchManifest(ctx, ts.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// TestFetchManifestWithRetrySuccess tests successful fetch with retry logic.
func TestFetchManifestWithRetrySuccess(t *testing.T) {
	expectedContent := "manifest data"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedContent))
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	data, err := manager.fetchManifestWithRetry(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchManifestWithRetry failed: %v", err)
	}

	if string(data) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(data))
	}
}

// TestFetchManifestWithRetryTransientError tests retry on transient errors.
func TestFetchManifestWithRetryTransientError(t *testing.T) {
	attemptCount := 0
	expectedContent := "success on third try"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			// Return 500 on first two attempts (transient error)
			http.Error(w, "temporary error", http.StatusInternalServerError)
			return
		}
		// Succeed on third attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedContent))
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	data, err := manager.fetchManifestWithRetry(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchManifestWithRetry failed: %v", err)
	}

	if string(data) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(data))
	}

	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount)
	}
}

// TestFetchManifestWithRetryPermanentError tests no retry on permanent errors.
func TestFetchManifestWithRetryPermanentError(t *testing.T) {
	attemptCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Return 404 (permanent error)
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := manager.fetchManifestWithRetry(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	// Should only attempt once for 404 (permanent error)
	if attemptCount != 1 {
		t.Errorf("expected 1 attempt for permanent error, got %d", attemptCount)
	}
}

// TestFetchManifestWithRetryMaxAttempts tests max retry attempts.
func TestFetchManifestWithRetryMaxAttempts(t *testing.T) {
	attemptCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Always return 500 (transient error)
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	manager := &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := manager.fetchManifestWithRetry(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error after max retries")
	}

	// Should attempt 3 times (initial + 2 retries)
	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount)
	}

	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("expected error message to mention retry count, got: %v", err)
	}
}

// TestShouldRetry tests the retry logic decision making.
func TestShouldRetry(t *testing.T) {
	manager := &Manager{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "nil error",
			err:         nil,
			shouldRetry: false,
		},
		{
			name:        "500 error",
			err:         fmt.Errorf("unexpected status code 500"),
			shouldRetry: true,
		},
		{
			name:        "503 error",
			err:         fmt.Errorf("unexpected status code 503"),
			shouldRetry: true,
		},
		{
			name:        "429 error",
			err:         fmt.Errorf("unexpected status code 429"),
			shouldRetry: true,
		},
		{
			name:        "404 error",
			err:         fmt.Errorf("unexpected status code 404"),
			shouldRetry: false,
		},
		{
			name:        "400 error",
			err:         fmt.Errorf("unexpected status code 400"),
			shouldRetry: false,
		},
		{
			name:        "connection refused",
			err:         fmt.Errorf("dial tcp: connection refused"),
			shouldRetry: true,
		},
		{
			name:        "connection reset",
			err:         fmt.Errorf("read tcp: connection reset by peer"),
			shouldRetry: true,
		},
		{
			name:        "timeout",
			err:         fmt.Errorf("context deadline exceeded (timeout)"),
			shouldRetry: true,
		},
		{
			name:        "temporary failure",
			err:         fmt.Errorf("temporary failure in name resolution"),
			shouldRetry: true,
		},
		{
			name:        "other error",
			err:         fmt.Errorf("some other error"),
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.shouldRetry(tt.err)
			if result != tt.shouldRetry {
				t.Errorf("shouldRetry(%v) = %v, want %v", tt.err, result, tt.shouldRetry)
			}
		})
	}
}

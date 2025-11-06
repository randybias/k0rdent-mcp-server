package catalog

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCacheInvalidation_FirstLoad tests cache behavior on first load (no cache exists)
func TestCacheInvalidation_FirstLoad(t *testing.T) {
	// Read test fixture
	fixturePath := filepath.Join("testdata", "valid-index.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// Create test HTTP server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData)
	}))
	defer server.Close()

	// Create temporary cache directory
	tmpDir := t.TempDir()

	// Create manager
	mgr, err := NewManager(Options{
		CacheDir:   tmpDir,
		ArchiveURL: server.URL,
		CacheTTL:   1 * time.Hour,
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.db.Close()

	// First call should fetch and build index
	ctx := context.Background()
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("first loadOrRefreshIndex failed: %v", err)
	}

	// Verify index was built
	timestamp, err := mgr.db.GetMetadata("index_timestamp")
	if err != nil {
		t.Fatalf("failed to get index timestamp: %v", err)
	}
	if timestamp != "2025-11-06T15:02:01.226674" {
		t.Errorf("expected timestamp '2025-11-06T15:02:01.226674', got %q", timestamp)
	}

	// Verify apps were inserted
	apps, err := mgr.db.ListApps("")
	if err != nil {
		t.Fatalf("failed to list apps: %v", err)
	}
	if len(apps) != 3 {
		t.Errorf("expected 3 apps, got %d", len(apps))
	}

	// Verify server was called once
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount)
	}
}

// TestCacheInvalidation_CacheHit tests cache behavior when timestamp matches (cache hit)
func TestCacheInvalidation_CacheHit(t *testing.T) {
	// Read test fixture
	fixturePath := filepath.Join("testdata", "valid-index.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// Create test HTTP server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData)
	}))
	defer server.Close()

	// Create temporary cache directory
	tmpDir := t.TempDir()

	// Create manager
	mgr, err := NewManager(Options{
		CacheDir:   tmpDir,
		ArchiveURL: server.URL,
		CacheTTL:   1 * time.Hour,
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.db.Close()

	ctx := context.Background()

	// First call - builds cache
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("first loadOrRefreshIndex failed: %v", err)
	}

	initialCallCount := callCount

	// Second call - should use cache (no rebuild)
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("second loadOrRefreshIndex failed: %v", err)
	}

	// Verify cache was used (HTTP call count should stay the same because TTL is valid)
	if callCount != initialCallCount {
		t.Errorf("expected cache hit (same call count %d), but got %d calls", initialCallCount, callCount)
	}

	// Verify timestamp is still the same
	timestamp, err := mgr.db.GetMetadata("index_timestamp")
	if err != nil {
		t.Fatalf("failed to get index timestamp: %v", err)
	}
	if timestamp != "2025-11-06T15:02:01.226674" {
		t.Errorf("expected timestamp '2025-11-06T15:02:01.226674', got %q", timestamp)
	}
}

// TestCacheInvalidation_CacheMiss tests cache behavior when timestamp changes (cache miss)
func TestCacheInvalidation_CacheMiss(t *testing.T) {
	// Read test fixture
	fixturePath := filepath.Join("testdata", "valid-index.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// Parse initial fixture
	var initialIndex JSONIndex
	if err := json.Unmarshal(fixtureData, &initialIndex); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	// Create modified fixture with new timestamp
	updatedIndex := initialIndex
	updatedIndex.Metadata.Generated = "2025-11-07T10:30:00.123456"
	updatedData, err := json.Marshal(updatedIndex)
	if err != nil {
		t.Fatalf("failed to marshal updated index: %v", err)
	}

	// Create test HTTP server that returns different timestamps
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			w.Write(fixtureData) // First call: original timestamp
		} else {
			w.Write(updatedData) // Second call: new timestamp
		}
	}))
	defer server.Close()

	// Create temporary cache directory
	tmpDir := t.TempDir()

	// Create manager with short TTL to force re-fetch
	mgr, err := NewManager(Options{
		CacheDir:   tmpDir,
		ArchiveURL: server.URL,
		CacheTTL:   1 * time.Millisecond, // Very short TTL
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.db.Close()

	ctx := context.Background()

	// First call - builds cache with original timestamp
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("first loadOrRefreshIndex failed: %v", err)
	}

	// Verify initial timestamp
	timestamp, err := mgr.db.GetMetadata("index_timestamp")
	if err != nil {
		t.Fatalf("failed to get initial index timestamp: %v", err)
	}
	if timestamp != "2025-11-06T15:02:01.226674" {
		t.Errorf("expected initial timestamp '2025-11-06T15:02:01.226674', got %q", timestamp)
	}

	// Wait for TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Second call - should detect new timestamp and rebuild
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("second loadOrRefreshIndex failed: %v", err)
	}

	// Verify timestamp was updated
	newTimestamp, err := mgr.db.GetMetadata("index_timestamp")
	if err != nil {
		t.Fatalf("failed to get updated index timestamp: %v", err)
	}
	if newTimestamp != "2025-11-07T10:30:00.123456" {
		t.Errorf("expected updated timestamp '2025-11-07T10:30:00.123456', got %q", newTimestamp)
	}

	// Verify server was called twice (once for initial, once after TTL expiry)
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}

	// Verify cache metadata file was updated with new timestamp
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	metadataData, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read cache metadata: %v", err)
	}

	var metadata CacheMetadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatalf("failed to unmarshal cache metadata: %v", err)
	}

	if metadata.IndexTimestamp != "2025-11-07T10:30:00.123456" {
		t.Errorf("expected metadata.IndexTimestamp '2025-11-07T10:30:00.123456', got %q", metadata.IndexTimestamp)
	}
}

// TestCacheInvalidation_TTLExpiry tests cache behavior when TTL expires
func TestCacheInvalidation_TTLExpiry(t *testing.T) {
	// Read test fixture
	fixturePath := filepath.Join("testdata", "valid-index.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// Create test HTTP server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData)
	}))
	defer server.Close()

	// Create temporary cache directory
	tmpDir := t.TempDir()

	// Create manager with very short TTL
	mgr, err := NewManager(Options{
		CacheDir:   tmpDir,
		ArchiveURL: server.URL,
		CacheTTL:   10 * time.Millisecond,
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.db.Close()

	ctx := context.Background()

	// First call - builds cache
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("first loadOrRefreshIndex failed: %v", err)
	}

	initialCallCount := callCount

	// Immediate second call - should use cache
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("second loadOrRefreshIndex failed: %v", err)
	}

	if callCount != initialCallCount {
		t.Errorf("expected cache hit, but got additional HTTP call")
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Third call after TTL expiry - should re-fetch
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("third loadOrRefreshIndex after TTL expiry failed: %v", err)
	}

	// Verify server was called again after TTL expiry
	if callCount <= initialCallCount {
		t.Errorf("expected HTTP call after TTL expiry, but call count stayed at %d", callCount)
	}
}

// TestCacheInvalidation_ForceRefresh tests cache behavior with force refresh flag
func TestCacheInvalidation_ForceRefresh(t *testing.T) {
	// Read test fixture
	fixturePath := filepath.Join("testdata", "valid-index.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// Create test HTTP server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData)
	}))
	defer server.Close()

	// Create temporary cache directory
	tmpDir := t.TempDir()

	// Create manager
	mgr, err := NewManager(Options{
		CacheDir:   tmpDir,
		ArchiveURL: server.URL,
		CacheTTL:   1 * time.Hour,
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.db.Close()

	ctx := context.Background()

	// First call - builds cache
	err = mgr.loadOrRefreshIndex(ctx, false)
	if err != nil {
		t.Fatalf("first loadOrRefreshIndex failed: %v", err)
	}

	initialCallCount := callCount

	// Second call with refresh=true - should force rebuild even with valid cache
	err = mgr.loadOrRefreshIndex(ctx, true)
	if err != nil {
		t.Fatalf("loadOrRefreshIndex with refresh=true failed: %v", err)
	}

	// Verify server was called again despite valid cache
	if callCount <= initialCallCount {
		t.Errorf("expected HTTP call with refresh=true, but call count stayed at %d", callCount)
	}
}

//go:build integration

package catalog

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSQLiteIntegration tests the full SQLite-backed catalog with real GitHub data
func TestSQLiteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp cache dir
	cacheDir := t.TempDir()

	// Create manager
	opts := LoadConfig()
	opts.CacheDir = cacheDir
	opts.CacheTTL = time.Hour

	mgr, err := NewManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Test List - will download and index catalog
	t.Log("Fetching catalog from GitHub...")
	entries, err := mgr.List(ctx, "", false)
	if err != nil {
		t.Fatalf("Failed to list catalog: %v", err)
	}

	t.Logf("Found %d apps with complete ServiceTemplates", len(entries))

	if len(entries) == 0 {
		t.Fatal("Expected at least some apps with ServiceTemplates")
	}

	// Verify database was populated
	dbPath := cacheDir + "/catalog.db"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("catalog.db was not created")
	}

	// Verify some known apps exist
	foundMinio := false
	for _, entry := range entries {
		if entry.Slug == "minio" {
			foundMinio = true
			t.Logf("Found minio with %d versions", len(entry.Versions))
			if len(entry.Versions) == 0 {
				t.Error("minio should have at least one version")
			}
			break
		}
	}

	if !foundMinio {
		t.Error("Expected to find minio in catalog")
	}

	// Test filtered list
	minioEntries, err := mgr.List(ctx, "minio", false)
	if err != nil {
		t.Fatalf("Failed to list minio: %v", err)
	}
	if len(minioEntries) != 1 {
		t.Errorf("Expected 1 minio entry, got %d", len(minioEntries))
	}

	// Test GetManifests
	if foundMinio {
		var minioEntry CatalogEntry
		for _, e := range entries {
			if e.Slug == "minio" {
				minioEntry = e
				break
			}
		}

		if len(minioEntry.Versions) > 0 {
			v := minioEntry.Versions[0]
			manifests, err := mgr.GetManifests(ctx, "minio", v.Name, v.Version)
			if err != nil {
				t.Fatalf("Failed to get minio manifests: %v", err)
			}

			if len(manifests) == 0 {
				t.Fatal("Expected at least ServiceTemplate manifest")
			}

			t.Logf("Got %d manifests for minio %s %s", len(manifests), v.Name, v.Version)
			t.Logf("ServiceTemplate size: %d bytes", len(manifests[0]))
			if len(manifests) > 1 {
				t.Logf("HelmRepository size: %d bytes", len(manifests[1]))
			}

			// Verify manifest content
			if len(manifests[0]) < 100 {
				t.Error("ServiceTemplate manifest seems too small")
			}
		}
	}

	// Test cache hit (second call should be fast)
	start := time.Now()
	entries2, err := mgr.List(ctx, "", false)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Second list failed: %v", err)
	}

	if len(entries2) != len(entries) {
		t.Errorf("Second list returned different count: %d vs %d", len(entries2), len(entries))
	}

	t.Logf("Cache hit took %v (should be < 100ms)", duration)
	if duration > 500*time.Millisecond {
		t.Logf("Warning: Cache hit seems slow (%v)", duration)
	}
}

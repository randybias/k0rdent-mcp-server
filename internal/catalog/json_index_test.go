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
)

// TestFetchJSONIndex tests the fetchJSONIndex method with various scenarios
func TestFetchJSONIndex(t *testing.T) {
	tests := []struct {
		name           string
		fixture        string
		expectError    bool
		expectedAddons int
	}{
		{
			name:           "valid JSON index",
			fixture:        "valid-index.json",
			expectError:    false,
			expectedAddons: 3,
		},
		{
			name:        "invalid JSON",
			fixture:     "invalid-index.json",
			expectError: true,
		},
		{
			name:           "empty addons list",
			fixture:        "empty-index.json",
			expectError:    false,
			expectedAddons: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read test fixture
			fixturePath := filepath.Join("testdata", tt.fixture)
			fixtureData, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			// Create test HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(fixtureData)
			}))
			defer server.Close()

			// Create temporary cache directory
			tmpDir := t.TempDir()

			// Create manager with test server URL
			mgr, err := NewManager(Options{
				CacheDir:   tmpDir,
				ArchiveURL: server.URL,
				Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
			})
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			// Test fetchJSONIndex
			ctx := context.Background()
			index, sha, err := mgr.fetchJSONIndex(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if sha == "" {
				t.Error("expected non-empty SHA")
			}

			if index == nil {
				t.Fatal("expected non-nil index")
			}

			if len(index.Addons) != tt.expectedAddons {
				t.Errorf("expected %d addons, got %d", tt.expectedAddons, len(index.Addons))
			}
		})
	}
}

// TestFetchJSONIndexHTTPErrors tests fetchJSONIndex with HTTP error scenarios
func TestFetchJSONIndexHTTPErrors(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{
			name:        "404 Not Found",
			statusCode:  http.StatusNotFound,
			expectError: true,
		},
		{
			name:        "500 Internal Server Error",
			statusCode:  http.StatusInternalServerError,
			expectError: true,
		},
		{
			name:        "200 OK",
			statusCode:  http.StatusOK,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.statusCode == http.StatusOK {
					// Return valid JSON for 200 OK
					fixturePath := filepath.Join("testdata", "empty-index.json")
					fixtureData, _ := os.ReadFile(fixturePath)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					w.Write(fixtureData)
				} else {
					w.WriteHeader(tt.statusCode)
				}
			}))
			defer server.Close()

			// Create temporary cache directory
			tmpDir := t.TempDir()

			// Create manager with test server URL
			mgr, err := NewManager(Options{
				CacheDir:   tmpDir,
				ArchiveURL: server.URL,
				Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
			})
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			// Test fetchJSONIndex
			ctx := context.Background()
			_, _, err = mgr.fetchJSONIndex(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for status code %d but got none", tt.statusCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for status code %d: %v", tt.statusCode, err)
				}
			}
		})
	}
}

// TestParseJSONIndex tests the parseJSONIndex method
func TestParseJSONIndex(t *testing.T) {
	tests := []struct {
		name              string
		index             *JSONIndex
		expectError       bool
		expectedApps      int
		expectedTemplates int
	}{
		{
			name: "valid index with multiple addons",
			index: &JSONIndex{
				Metadata: JSONMetadata{
					Generated: "2025-11-06T15:02:01.226674",
					Version:   "1.0.0",
				},
				Addons: []JSONAddon{
					{
						Name:          "minio",
						Description:   "High Performance Object Storage",
						LatestVersion: "14.1.2",
						Versions:      []string{"14.1.2"},
						Charts: []JSONChart{
							{
								Name:     "minio",
								Versions: []string{"14.1.2"},
							},
						},
						Metadata: JSONAddonMetadata{
							Tags:  []string{"Storage"},
							Owner: "k0rdent-team",
						},
					},
					{
						Name:          "redis",
						Description:   "In-memory data structure store",
						LatestVersion: "17.11.3",
						Versions:      []string{"17.11.3"},
						Charts: []JSONChart{
							{
								Name:     "redis",
								Versions: []string{"17.11.3"},
							},
							{
								Name:     "redis-cluster",
								Versions: []string{"8.6.2"},
							},
						},
						Metadata: JSONAddonMetadata{
							Tags:  []string{"Cache", "Database"},
							Owner: "k0rdent-team",
						},
					},
				},
			},
			expectError:       false,
			expectedApps:      2,
			expectedTemplates: 3, // minio (1) + redis (2 charts)
		},
		{
			name:        "nil index",
			index:       nil,
			expectError: true,
		},
		{
			name: "empty addons",
			index: &JSONIndex{
				Metadata: JSONMetadata{
					Generated: "2025-11-06T15:02:01.226674",
					Version:   "1.0.0",
				},
				Addons: []JSONAddon{},
			},
			expectError:       false,
			expectedApps:      0,
			expectedTemplates: 0,
		},
		{
			name: "addon with multiple chart versions",
			index: &JSONIndex{
				Metadata: JSONMetadata{
					Generated: "2025-11-06T15:02:01.226674",
					Version:   "1.0.0",
				},
				Addons: []JSONAddon{
					{
						Name:          "postgresql",
						Description:   "PostgreSQL Database",
						LatestVersion: "12.5.8",
						Versions:      []string{"12.5.8", "12.5.7"},
						Charts: []JSONChart{
							{
								Name:     "postgresql",
								Versions: []string{"12.5.8", "12.5.7"},
							},
						},
						Metadata: JSONAddonMetadata{
							Tags:  []string{"Database"},
							Owner: "k0rdent-team",
						},
					},
				},
			},
			expectError:       false,
			expectedApps:      1,
			expectedTemplates: 2, // 2 versions of postgresql
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary cache directory
			tmpDir := t.TempDir()

			// Create manager
			mgr, err := NewManager(Options{
				CacheDir: tmpDir,
				Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
			})
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			// Test parseJSONIndex
			apps, templates, err := mgr.parseJSONIndex(tt.index)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(apps) != tt.expectedApps {
				t.Errorf("expected %d apps, got %d", tt.expectedApps, len(apps))
			}

			if len(templates) != tt.expectedTemplates {
				t.Errorf("expected %d templates, got %d", tt.expectedTemplates, len(templates))
			}

			// Validate app structure
			for i, app := range apps {
				if app.Slug == "" {
					t.Errorf("app %d: slug is empty", i)
				}
				if app.Title == "" {
					t.Errorf("app %d: title is empty", i)
				}
				// Verify slug matches the corresponding addon name
				if tt.index != nil && i < len(tt.index.Addons) {
					if app.Slug != tt.index.Addons[i].Name {
						t.Errorf("app %d: expected slug %q, got %q", i, tt.index.Addons[i].Name, app.Slug)
					}
				}
			}

			// Validate template structure
			for i, tmpl := range templates {
				if tmpl.AppSlug == "" {
					t.Errorf("template %d: app_slug is empty", i)
				}
				if tmpl.ChartName == "" {
					t.Errorf("template %d: chart_name is empty", i)
				}
				if tmpl.Version == "" {
					t.Errorf("template %d: version is empty", i)
				}
			}
		})
	}
}

// TestParseJSONIndexFromFixture tests parsing a real JSON fixture file
func TestParseJSONIndexFromFixture(t *testing.T) {
	// Read valid fixture
	fixturePath := filepath.Join("testdata", "valid-index.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// Parse JSON
	var index JSONIndex
	if err := json.Unmarshal(fixtureData, &index); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	// Create temporary cache directory
	tmpDir := t.TempDir()

	// Create manager
	mgr, err := NewManager(Options{
		CacheDir: tmpDir,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Test parseJSONIndex
	apps, templates, err := mgr.parseJSONIndex(&index)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify results match fixture expectations
	expectedApps := 3      // minio, postgresql, redis
	expectedTemplates := 5 // minio(1) + postgresql(2) + redis(1) + redis-cluster(1)

	if len(apps) != expectedApps {
		t.Errorf("expected %d apps from fixture, got %d", expectedApps, len(apps))
	}

	if len(templates) != expectedTemplates {
		t.Errorf("expected %d templates from fixture, got %d", expectedTemplates, len(templates))
	}

	// Verify specific addon details
	foundMinio := false
	foundPostgresql := false
	foundRedis := false

	for _, app := range apps {
		switch app.Slug {
		case "minio":
			foundMinio = true
			if app.Summary != "High Performance Object Storage" {
				t.Errorf("minio: expected summary %q, got %q", "High Performance Object Storage", app.Summary)
			}
			if len(app.Tags) != 1 || app.Tags[0] != "Storage" {
				t.Errorf("minio: expected tags [Storage], got %v", app.Tags)
			}
		case "postgresql":
			foundPostgresql = true
			if app.Summary != "PostgreSQL Database" {
				t.Errorf("postgresql: expected summary %q, got %q", "PostgreSQL Database", app.Summary)
			}
		case "redis":
			foundRedis = true
			if len(app.Tags) != 2 {
				t.Errorf("redis: expected 2 tags, got %d", len(app.Tags))
			}
		}
	}

	if !foundMinio {
		t.Error("minio addon not found in parsed apps")
	}
	if !foundPostgresql {
		t.Error("postgresql addon not found in parsed apps")
	}
	if !foundRedis {
		t.Error("redis addon not found in parsed apps")
	}
}

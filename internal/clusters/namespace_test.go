package clusters

import (
	"context"
	"log/slog"
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

// TestResolveTargetNamespace_DevMode tests namespace resolution in development mode (no filter)
func TestResolveTargetNamespace_DevMode(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	tests := []struct {
		name              string
		inputNamespace    string
		globalNamespace   string
		expectedNamespace string
		expectError       bool
	}{
		{
			name:              "empty namespace defaults to global",
			inputNamespace:    "",
			globalNamespace:   "kcm-system",
			expectedNamespace: "kcm-system",
			expectError:       false,
		},
		{
			name:              "explicit namespace is used",
			inputNamespace:    "team-alpha",
			globalNamespace:   "kcm-system",
			expectedNamespace: "team-alpha",
			expectError:       false,
		},
		{
			name:              "any namespace is allowed in dev mode",
			inputNamespace:    "random-namespace",
			globalNamespace:   "kcm-system",
			expectedNamespace: "random-namespace",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: tt.globalNamespace,
				namespaceFilter: nil, // Dev mode: no filter
				logger:          slog.Default(),
			}

			namespace, err := manager.ResolveTargetNamespace(context.Background(), tt.inputNamespace)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if namespace != tt.expectedNamespace {
					t.Errorf("expected namespace %q, got %q", tt.expectedNamespace, namespace)
				}
			}
		})
	}
}

// TestResolveTargetNamespace_ProductionMode tests namespace resolution in production mode (with filter)
func TestResolveTargetNamespace_ProductionMode(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	tests := []struct {
		name              string
		inputNamespace    string
		namespaceFilter   *regexp.Regexp
		expectedNamespace string
		expectError       bool
	}{
		{
			name:            "empty namespace requires explicit value",
			inputNamespace:  "",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectError:     true,
		},
		{
			name:              "allowed namespace by filter",
			inputNamespace:    "team-alpha",
			namespaceFilter:   regexp.MustCompile("^team-"),
			expectedNamespace: "team-alpha",
			expectError:       false,
		},
		{
			name:            "forbidden namespace by filter",
			inputNamespace:  "forbidden-namespace",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectError:     true,
		},
		{
			name:              "exact match filter",
			inputNamespace:    "team-alpha",
			namespaceFilter:   regexp.MustCompile("^team-alpha$"),
			expectedNamespace: "team-alpha",
			expectError:       false,
		},
		{
			name:            "exact match filter rejects other",
			inputNamespace:  "team-beta",
			namespaceFilter: regexp.MustCompile("^team-alpha$"),
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				namespaceFilter: tt.namespaceFilter,
				logger:          slog.Default(),
			}

			namespace, err := manager.ResolveTargetNamespace(context.Background(), tt.inputNamespace)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if namespace != tt.expectedNamespace {
					t.Errorf("expected namespace %q, got %q", tt.expectedNamespace, namespace)
				}
			}
		})
	}
}

// TestResolveTargetNamespace_GlobalNamespaceAccess tests access to global namespace
func TestResolveTargetNamespace_GlobalNamespaceAccess(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	tests := []struct {
		name            string
		hasFilter       bool
		inputNamespace  string
		namespaceFilter *regexp.Regexp
		expectError     bool
	}{
		{
			name:            "dev mode allows global namespace",
			hasFilter:       false,
			inputNamespace:  "kcm-system",
			namespaceFilter: nil,
			expectError:     false,
		},
		{
			name:            "production mode blocks global namespace without filter match",
			hasFilter:       true,
			inputNamespace:  "kcm-system",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectError:     true,
		},
		{
			name:            "production mode allows global namespace with explicit filter",
			hasFilter:       true,
			inputNamespace:  "kcm-system",
			namespaceFilter: regexp.MustCompile("^(kcm-system|team-.*)$"),
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				namespaceFilter: tt.namespaceFilter,
				logger:          slog.Default(),
			}

			_, err := manager.ResolveTargetNamespace(context.Background(), tt.inputNamespace)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestResolveResourceNamespace tests resource reference resolution
func TestResolveResourceNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	tests := []struct {
		name              string
		reference         string
		targetNamespace   string
		namespaceFilter   *regexp.Regexp
		expectedNamespace string
		expectedName      string
		expectError       bool
	}{
		{
			name:              "simple name uses target namespace",
			reference:         "azure-cred",
			targetNamespace:   "kcm-system",
			namespaceFilter:   nil,
			expectedNamespace: "kcm-system",
			expectedName:      "azure-cred",
			expectError:       false,
		},
		{
			name:              "namespaced reference",
			reference:         "kcm-system/azure-cred",
			targetNamespace:   "team-alpha",
			namespaceFilter:   nil,
			expectedNamespace: "kcm-system",
			expectedName:      "azure-cred",
			expectError:       false,
		},
		{
			name:            "namespaced reference blocked by filter",
			reference:       "forbidden-ns/cred",
			targetNamespace: "team-alpha",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectError:     true,
		},
		{
			name:              "namespaced reference allowed by filter",
			reference:         "team-alpha/cred",
			targetNamespace:   "team-beta",
			namespaceFilter:   regexp.MustCompile("^team-"),
			expectedNamespace: "team-alpha",
			expectedName:      "cred",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				namespaceFilter: tt.namespaceFilter,
				logger:          slog.Default(),
			}

			namespace, name, err := manager.ResolveResourceNamespace(context.Background(), tt.reference, tt.targetNamespace)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if namespace != tt.expectedNamespace {
					t.Errorf("expected namespace %q, got %q", tt.expectedNamespace, namespace)
				}
				if name != tt.expectedName {
					t.Errorf("expected name %q, got %q", tt.expectedName, name)
				}
			}
		})
	}
}

// TestResolveTargetNamespace_ComplexFilters tests complex regex patterns
func TestResolveTargetNamespace_ComplexFilters(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	tests := []struct {
		name            string
		inputNamespace  string
		namespaceFilter *regexp.Regexp
		expectError     bool
	}{
		{
			name:            "prefix match",
			inputNamespace:  "team-alpha",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectError:     false,
		},
		{
			name:            "suffix match",
			inputNamespace:  "namespace-prod",
			namespaceFilter: regexp.MustCompile("-prod$"),
			expectError:     false,
		},
		{
			name:            "contains match",
			inputNamespace:  "pre-team-post",
			namespaceFilter: regexp.MustCompile("team"),
			expectError:     false,
		},
		{
			name:            "alternation pattern",
			inputNamespace:  "dev-namespace",
			namespaceFilter: regexp.MustCompile("^(dev|test|staging)-"),
			expectError:     false,
		},
		{
			name:            "alternation pattern no match",
			inputNamespace:  "prod-namespace",
			namespaceFilter: regexp.MustCompile("^(dev|test|staging)-"),
			expectError:     true,
		},
		{
			name:            "character class",
			inputNamespace:  "team-1",
			namespaceFilter: regexp.MustCompile("^team-[0-9]$"),
			expectError:     false,
		},
		{
			name:            "character class no match",
			inputNamespace:  "team-alpha",
			namespaceFilter: regexp.MustCompile("^team-[0-9]$"),
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				namespaceFilter: tt.namespaceFilter,
				logger:          slog.Default(),
			}

			_, err := manager.ResolveTargetNamespace(context.Background(), tt.inputNamespace)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestResolveTargetNamespace_NilFilter tests behavior with nil filter
func TestResolveTargetNamespace_NilFilter(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	tests := []struct {
		name              string
		inputNamespace    string
		expectedNamespace string
		expectError       bool
	}{
		{
			name:              "nil filter allows any namespace",
			inputNamespace:    "any-namespace",
			expectedNamespace: "any-namespace",
			expectError:       false,
		},
		{
			name:              "nil filter with empty namespace defaults to global",
			inputNamespace:    "",
			expectedNamespace: "kcm-system",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				namespaceFilter: nil,
				logger:          slog.Default(),
			}

			namespace, err := manager.ResolveTargetNamespace(context.Background(), tt.inputNamespace)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if namespace != tt.expectedNamespace {
					t.Errorf("expected namespace %q, got %q", tt.expectedNamespace, namespace)
				}
			}
		})
	}
}

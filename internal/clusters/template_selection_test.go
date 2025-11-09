package clusters

import (
	"context"
	"log/slog"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

// TestSelectLatestTemplate_Success tests selecting the latest template from multiple versions
func TestSelectLatestTemplate_Success(t *testing.T) {
	tests := []struct {
		name             string
		templates        []*unstructured.Unstructured
		provider         string
		namespace        string
		expectedTemplate string
		expectedError    bool
	}{
		{
			name: "select latest from multiple AWS templates",
			templates: []*unstructured.Unstructured{
				createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-14", "kcm-system", "1.0.14", nil),
				createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-15", "kcm-system", "1.0.15", nil),
				createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-13", "kcm-system", "1.0.13", nil),
			},
			provider:         "aws",
			namespace:        "kcm-system",
			expectedTemplate: "aws-standalone-cp-1-0-15",
			expectedError:    false,
		},
		{
			name: "select latest with mixed versions (minor)",
			templates: []*unstructured.Unstructured{
				createTestClusterTemplateWithVersion("azure-standalone-cp-1-0-99", "kcm-system", "1.0.99", nil),
				createTestClusterTemplateWithVersion("azure-standalone-cp-1-1-0", "kcm-system", "1.1.0", nil),
				createTestClusterTemplateWithVersion("azure-standalone-cp-1-0-50", "kcm-system", "1.0.50", nil),
			},
			provider:         "azure",
			namespace:        "kcm-system",
			expectedTemplate: "azure-standalone-cp-1-1-0",
			expectedError:    false,
		},
		{
			name: "select latest with mixed versions (major)",
			templates: []*unstructured.Unstructured{
				createTestClusterTemplateWithVersion("gcp-standalone-cp-1-9-99", "kcm-system", "1.9.99", nil),
				createTestClusterTemplateWithVersion("gcp-standalone-cp-2-0-0", "kcm-system", "2.0.0", nil),
				createTestClusterTemplateWithVersion("gcp-standalone-cp-1-10-5", "kcm-system", "1.10.5", nil),
			},
			provider:         "gcp",
			namespace:        "kcm-system",
			expectedTemplate: "gcp-standalone-cp-2-0-0",
			expectedError:    false,
		},
		{
			name: "select from single template",
			templates: []*unstructured.Unstructured{
				createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-14", "kcm-system", "1.0.14", nil),
			},
			provider:         "aws",
			namespace:        "kcm-system",
			expectedTemplate: "aws-standalone-cp-1-0-14",
			expectedError:    false,
		},
		{
			name: "no matching templates returns error",
			templates: []*unstructured.Unstructured{
				createTestClusterTemplateWithVersion("azure-standalone-cp-1-0-15", "kcm-system", "1.0.15", nil),
			},
			provider:      "aws",
			namespace:     "kcm-system",
			expectedError: true,
		},
		{
			name: "nonexistent namespace returns error",
			templates: []*unstructured.Unstructured{
				createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-14", "kcm-system", "1.0.14", nil),
			},
			provider:      "aws",
			namespace:     "nonexistent",
			expectedError: true,
		},
		{
			name: "filters by provider prefix correctly",
			templates: []*unstructured.Unstructured{
				createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-14", "kcm-system", "1.0.14", nil),
				createTestClusterTemplateWithVersion("azure-standalone-cp-1-0-15", "kcm-system", "1.0.15", nil),
				createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-16", "kcm-system", "1.0.16", nil),
				createTestClusterTemplateWithVersion("gcp-standalone-cp-2-0-0", "kcm-system", "2.0.0", nil),
			},
			provider:         "aws",
			namespace:        "kcm-system",
			expectedTemplate: "aws-standalone-cp-1-0-16",
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			var objs []runtime.Object
			for _, tmpl := range tt.templates {
				objs = append(objs, tmpl)
			}
			client := fake.NewSimpleDynamicClient(scheme, objs...)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
			}

			templateName, err := manager.SelectLatestTemplate(context.Background(), tt.provider, tt.namespace)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if templateName != tt.expectedTemplate {
				t.Errorf("expected template %q, got %q", tt.expectedTemplate, templateName)
			}
		})
	}
}

// TestCompareVersions tests semantic version comparison
func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Patch version comparisons
		{
			name:     "v1 greater patch",
			v1:       "1.0.15",
			v2:       "1.0.14",
			expected: 1,
		},
		{
			name:     "v2 greater patch",
			v1:       "1.0.14",
			v2:       "1.0.15",
			expected: -1,
		},
		{
			name:     "equal versions",
			v1:       "1.0.14",
			v2:       "1.0.14",
			expected: 0,
		},

		// Minor version comparisons
		{
			name:     "v1 greater minor",
			v1:       "1.1.0",
			v2:       "1.0.99",
			expected: 1,
		},
		{
			name:     "v2 greater minor",
			v1:       "1.0.99",
			v2:       "1.1.0",
			expected: -1,
		},

		// Major version comparisons
		{
			name:     "v1 greater major",
			v1:       "2.0.0",
			v2:       "1.9.99",
			expected: 1,
		},
		{
			name:     "v2 greater major",
			v1:       "1.9.99",
			v2:       "2.0.0",
			expected: -1,
		},

		// Edge cases
		{
			name:     "v1 empty string",
			v1:       "",
			v2:       "1.0.0",
			expected: -1,
		},
		{
			name:     "v2 empty string",
			v1:       "1.0.0",
			v2:       "",
			expected: 1,
		},
		{
			name:     "both empty strings",
			v1:       "",
			v2:       "",
			expected: 0,
		},
		{
			name:     "version with spaces",
			v1:       " 1.0.14 ",
			v2:       "1.0.13",
			expected: 1,
		},

		// Double-digit versions
		{
			name:     "double-digit minor",
			v1:       "1.10.0",
			v2:       "1.9.0",
			expected: 1,
		},
		{
			name:     "double-digit patch",
			v1:       "1.0.100",
			v2:       "1.0.99",
			expected: 1,
		},

		// Complex comparisons
		{
			name:     "complex v1 greater",
			v1:       "2.10.99",
			v2:       "2.9.100",
			expected: 1,
		},
		{
			name:     "complex equal",
			v1:       "10.20.30",
			v2:       "10.20.30",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, expected %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

// TestParseVersion tests version string parsing
func TestParseVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected [3]int
	}{
		{
			name:     "standard version",
			version:  "1.0.14",
			expected: [3]int{1, 0, 14},
		},
		{
			name:     "double-digit version",
			version:  "10.20.30",
			expected: [3]int{10, 20, 30},
		},
		{
			name:     "empty string",
			version:  "",
			expected: [3]int{0, 0, 0},
		},
		{
			name:     "version with spaces",
			version:  " 1 . 0 . 14 ",
			expected: [3]int{1, 0, 14},
		},
		{
			name:     "incomplete version (two parts)",
			version:  "1.0",
			expected: [3]int{1, 0, 0},
		},
		{
			name:     "incomplete version (one part)",
			version:  "1",
			expected: [3]int{1, 0, 0},
		},
		{
			name:     "invalid characters",
			version:  "1.x.14",
			expected: [3]int{1, 0, 14},
		},
		{
			name:     "very large numbers",
			version:  "999.888.777",
			expected: [3]int{999, 888, 777},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVersion(tt.version)
			if result != tt.expected {
				t.Errorf("parseVersion(%q) = %v, expected %v", tt.version, result, tt.expected)
			}
		})
	}
}

// TestSelectLatestTemplate_FiltersByNamespace tests that selection is namespace-scoped
func TestSelectLatestTemplate_FiltersByNamespace(t *testing.T) {
	templates := []*unstructured.Unstructured{
		createTestClusterTemplateWithVersion("aws-standalone-cp-1-0-14", "kcm-system", "1.0.14", nil),
		createTestClusterTemplateWithVersion("aws-standalone-cp-2-0-0", "team-alpha", "2.0.0", nil),
	}

	scheme := runtime.NewScheme()
	var objs []runtime.Object
	for _, tmpl := range templates {
		objs = append(objs, tmpl)
	}
	client := fake.NewSimpleDynamicClient(scheme, objs...)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	// Search in kcm-system should find v1.0.14 (not v2.0.0 from team-alpha)
	templateName, err := manager.SelectLatestTemplate(context.Background(), "aws", "kcm-system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if templateName != "aws-standalone-cp-1-0-14" {
		t.Errorf("expected template from kcm-system namespace, got %q", templateName)
	}

	// Search in team-alpha should find v2.0.0
	templateName, err = manager.SelectLatestTemplate(context.Background(), "aws", "team-alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if templateName != "aws-standalone-cp-2-0-0" {
		t.Errorf("expected template from team-alpha namespace, got %q", templateName)
	}
}

// createTestClusterTemplateWithVersion creates a test ClusterTemplate with explicit version
func createTestClusterTemplateWithVersion(name, namespace, version string, labels map[string]string) *unstructured.Unstructured {
	template := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterTemplate",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": "2025-01-01T00:00:00Z",
			},
			"spec": map[string]interface{}{
				"version": version,
			},
		},
	}

	if labels != nil {
		template.SetLabels(labels)
	}

	return template
}

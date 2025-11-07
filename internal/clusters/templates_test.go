package clusters

import (
	"log/slog"
	"context"
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

// TestListTemplates_Success tests successful template listing
func TestListTemplates_Success(t *testing.T) {
	template1 := createTestClusterTemplate("azure-standalone-cp-1-0-15", "kcm-system", map[string]string{
		"k0rdent.mirantis.com/provider": "azure",
		"k0rdent.mirantis.com/scope":    "global",
	})
	template2 := createTestClusterTemplate("aws-eks-template", "team-alpha", map[string]string{
		"k0rdent.mirantis.com/provider": "aws",
		"k0rdent.mirantis.com/scope":    "local",
	})

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, template1, template2)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	tests := []struct {
		name          string
		namespaces    []string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "list from kcm-system only",
			namespaces:    []string{"kcm-system"},
			expectedCount: 1,
			expectedNames: []string{"azure-standalone-cp-1-0-15"},
		},
		{
			name:          "list from team-alpha only",
			namespaces:    []string{"team-alpha"},
			expectedCount: 1,
			expectedNames: []string{"aws-eks-template"},
		},
		{
			name:          "list from multiple namespaces",
			namespaces:    []string{"kcm-system", "team-alpha"},
			expectedCount: 2,
			expectedNames: []string{"azure-standalone-cp-1-0-15", "aws-eks-template"},
		},
		{
			name:          "list from empty namespace list",
			namespaces:    []string{},
			expectedCount: 0,
		},
		{
			name:          "list from non-existent namespace",
			namespaces:    []string{"nonexistent"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templates, err := manager.ListTemplates(context.Background(), tt.namespaces)
			if err != nil {
				t.Fatalf("ListTemplates returned error: %v", err)
			}

			if len(templates) != tt.expectedCount {
				t.Errorf("expected %d templates, got %d", tt.expectedCount, len(templates))
			}

			for i, tmpl := range templates {
				if i < len(tt.expectedNames) && tmpl.Name != tt.expectedNames[i] {
					t.Errorf("template %d: expected name %q, got %q", i, tt.expectedNames[i], tmpl.Name)
				}
			}
		})
	}
}

// TestListTemplates_ScopeFiltering tests filtering by scope (global/local)
func TestListTemplates_ScopeFiltering(t *testing.T) {
	globalTemplate1 := createTestClusterTemplate("azure-global", "kcm-system", map[string]string{
		"k0rdent.mirantis.com/provider": "azure",
		"k0rdent.mirantis.com/scope":    "global",
	})
	globalTemplate2 := createTestClusterTemplate("aws-global", "kcm-system", map[string]string{
		"k0rdent.mirantis.com/provider": "aws",
		"k0rdent.mirantis.com/scope":    "global",
	})
	localTemplate1 := createTestClusterTemplate("team-template", "team-alpha", map[string]string{
		"k0rdent.mirantis.com/provider": "azure",
		"k0rdent.mirantis.com/scope":    "local",
	})
	localTemplate2 := createTestClusterTemplate("team-template-2", "team-beta", map[string]string{
		"k0rdent.mirantis.com/provider": "gcp",
		"k0rdent.mirantis.com/scope":    "local",
	})

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, globalTemplate1, globalTemplate2, localTemplate1, localTemplate2)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	tests := []struct {
		name           string
		scope          string
		namespaceFilter *regexp.Regexp
		expectedCount  int
		expectedScopes []string
	}{
		{
			name:           "global scope only",
			scope:          "global",
			namespaceFilter: nil,
			expectedCount:  2,
			expectedScopes: []string{"global", "global"},
		},
		{
			name:           "local scope with team- filter",
			scope:          "local",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectedCount:  2,
			expectedScopes: []string{"local", "local"},
		},
		{
			name:           "all scopes with team-alpha filter",
			scope:          "all",
			namespaceFilter: regexp.MustCompile("^team-alpha$"),
			expectedCount:  3, // 2 global + 1 local from team-alpha
			expectedScopes: []string{"global", "global", "local"},
		},
		{
			name:           "local scope with no matching filter",
			scope:          "local",
			namespaceFilter: regexp.MustCompile("^prod-"),
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compute allowed namespaces based on scope and filter
			var allowedNamespaces []string

			if tt.scope == "global" || tt.scope == "all" {
				allowedNamespaces = append(allowedNamespaces, "kcm-system")
			}

			if tt.scope == "local" || tt.scope == "all" {
				if tt.namespaceFilter != nil {
					allNamespaces := []string{"team-alpha", "team-beta"}
					for _, ns := range allNamespaces {
						if tt.namespaceFilter.MatchString(ns) {
							allowedNamespaces = append(allowedNamespaces, ns)
						}
					}
				}
			}

			templates, err := manager.ListTemplates(context.Background(), allowedNamespaces)
			if err != nil {
				t.Fatalf("ListTemplates returned error: %v", err)
			}

			if len(templates) != tt.expectedCount {
				t.Errorf("expected %d templates, got %d", tt.expectedCount, len(templates))
			}
		})
	}
}

// TestListTemplates_ProviderLabels tests provider extraction from labels
func TestListTemplates_ProviderLabels(t *testing.T) {
	tests := []struct {
		name             string
		labels           map[string]string
		expectedProvider string
	}{
		{
			name: "azure provider",
			labels: map[string]string{
				"k0rdent.mirantis.com/provider": "azure",
			},
			expectedProvider: "azure",
		},
		{
			name: "aws provider",
			labels: map[string]string{
				"k0rdent.mirantis.com/provider": "aws",
			},
			expectedProvider: "aws",
		},
		{
			name: "gcp provider",
			labels: map[string]string{
				"k0rdent.mirantis.com/provider": "gcp",
			},
			expectedProvider: "gcp",
		},
		{
			name:             "no provider label",
			labels:           map[string]string{},
			expectedProvider: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := createTestClusterTemplate("test-template", "kcm-system", tt.labels)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, template)

			manager := &Manager{
				dynamicClient:          client,
				globalNamespace: "kcm-system",
		logger:          slog.Default(),
			}

			templates, err := manager.ListTemplates(context.Background(), []string{"kcm-system"})
			if err != nil {
				t.Fatalf("ListTemplates returned error: %v", err)
			}

			if len(templates) != 1 {
				t.Fatalf("expected 1 template, got %d", len(templates))
			}

			if templates[0].Provider != tt.expectedProvider {
				t.Errorf("expected provider %q, got %q", tt.expectedProvider, templates[0].Provider)
			}
		})
	}
}

// TestListTemplates_VersionAndDescription tests metadata extraction
func TestListTemplates_VersionAndDescription(t *testing.T) {
	template := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterTemplate",
			"metadata": map[string]interface{}{
				"name":              "detailed-template",
				"namespace":         "kcm-system",
				"creationTimestamp": "2025-01-01T00:00:00Z",
				"labels": map[string]interface{}{
					"k0rdent.mirantis.com/provider": "azure",
				},
				"annotations": map[string]interface{}{
					"k0rdent.mirantis.com/description": "Azure standalone cluster template",
				},
			},
			"spec": map[string]interface{}{
				"version": "1.0.15",
				"config": map[string]interface{}{
					"clusterIdentity": map[string]interface{}{
						"name":      "azure-cluster-identity",
						"namespace": "kcm-system",
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, template)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	templates, err := manager.ListTemplates(context.Background(), []string{"kcm-system"})
	if err != nil {
		t.Fatalf("ListTemplates returned error: %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]

	if tmpl.Name != "detailed-template" {
		t.Errorf("expected name 'detailed-template', got %q", tmpl.Name)
	}

	if tmpl.Version != "1.0.15" {
		t.Errorf("expected version '1.0.15', got %q", tmpl.Version)
	}

	if tmpl.Description != "Azure standalone cluster template" {
		t.Errorf("expected description 'Azure standalone cluster template', got %q", tmpl.Description)
	}
}

// TestListTemplates_ConfigSchema tests config schema extraction
func TestListTemplates_ConfigSchema(t *testing.T) {
	t.Skip("ConfigSchema field not yet implemented in ClusterTemplateSummary - future enhancement")
	// Note: This test documents the expected behavior for config schema extraction.
	// When implemented, the ClusterTemplateSummary should include a ConfigSchema field
	// that extracts the spec.schema from ClusterTemplate resources to help users
	// understand required configuration parameters.
}

// createTestClusterTemplate creates a test ClusterTemplate unstructured object
func createTestClusterTemplate(name, namespace string, labels map[string]string) *unstructured.Unstructured {
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
				"version": "1.0.0",
			},
		},
	}

	if labels != nil {
		template.SetLabels(labels)
	}

	return template
}

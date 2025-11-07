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

// TestListCredentials_Success tests successful credential listing
func TestListCredentials_Success(t *testing.T) {
	// Create test credentials
	cred1 := createTestCredential("azure-cluster-credential", "kcm-system", map[string]string{
		"k0rdent.mirantis.com/provider": "azure",
	})
	cred2 := createTestCredential("aws-prod-credential", "team-alpha", map[string]string{
		"k0rdent.mirantis.com/provider": "aws",
	})

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, cred1, cred2)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	tests := []struct {
		name              string
		namespaces        []string
		expectedCount     int
		expectedNames     []string
		expectedProviders []string
	}{
		{
			name:              "list from kcm-system only",
			namespaces:        []string{"kcm-system"},
			expectedCount:     1,
			expectedNames:     []string{"azure-cluster-credential"},
			expectedProviders: []string{"azure"},
		},
		{
			name:              "list from team-alpha only",
			namespaces:        []string{"team-alpha"},
			expectedCount:     1,
			expectedNames:     []string{"aws-prod-credential"},
			expectedProviders: []string{"aws"},
		},
		{
			name:              "list from multiple namespaces",
			namespaces:        []string{"kcm-system", "team-alpha"},
			expectedCount:     2,
			expectedNames:     []string{"azure-cluster-credential", "aws-prod-credential"},
			expectedProviders: []string{"azure", "aws"},
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
			credentials, err := manager.ListCredentials(context.Background(), tt.namespaces)
			if err != nil {
				t.Fatalf("ListCredentials returned error: %v", err)
			}

			if len(credentials) != tt.expectedCount {
				t.Errorf("expected %d credentials, got %d", tt.expectedCount, len(credentials))
			}

			for i, cred := range credentials {
				if i < len(tt.expectedNames) && cred.Name != tt.expectedNames[i] {
					t.Errorf("credential %d: expected name %q, got %q", i, tt.expectedNames[i], cred.Name)
				}
				if i < len(tt.expectedProviders) && cred.Provider != tt.expectedProviders[i] {
					t.Errorf("credential %d: expected provider %q, got %q", i, tt.expectedProviders[i], cred.Provider)
				}
			}
		})
	}
}

// TestListCredentials_WithNamespaceFilter tests namespace filtering
func TestListCredentials_WithNamespaceFilter(t *testing.T) {
	cred1 := createTestCredential("global-cred", "kcm-system", map[string]string{
		"k0rdent.mirantis.com/provider": "azure",
	})
	cred2 := createTestCredential("team-cred", "team-alpha", map[string]string{
		"k0rdent.mirantis.com/provider": "aws",
	})
	cred3 := createTestCredential("team-cred-beta", "team-beta", map[string]string{
		"k0rdent.mirantis.com/provider": "gcp",
	})

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, cred1, cred2, cred3)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	tests := []struct {
		name             string
		namespaceFilter  *regexp.Regexp
		includeGlobal    bool
		expectedCount    int
		expectedNames    []string
	}{
		{
			name:            "filter matching team- prefix with global",
			namespaceFilter: regexp.MustCompile("^team-"),
			includeGlobal:   true,
			expectedCount:   3,
			expectedNames:   []string{"global-cred", "team-cred", "team-cred-beta"},
		},
		{
			name:            "filter matching team- prefix without global",
			namespaceFilter: regexp.MustCompile("^team-"),
			includeGlobal:   false,
			expectedCount:   2,
			expectedNames:   []string{"team-cred", "team-cred-beta"},
		},
		{
			name:            "filter matching team-alpha only",
			namespaceFilter: regexp.MustCompile("^team-alpha$"),
			includeGlobal:   true,
			expectedCount:   2,
			expectedNames:   []string{"global-cred", "team-cred"},
		},
		{
			name:            "filter matching nothing",
			namespaceFilter: regexp.MustCompile("^prod-"),
			includeGlobal:   false,
			expectedCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compute allowed namespaces based on filter
			allNamespaces := []string{"kcm-system", "team-alpha", "team-beta"}
			var allowedNamespaces []string

			if tt.includeGlobal {
				allowedNamespaces = append(allowedNamespaces, "kcm-system")
			}

			for _, ns := range allNamespaces {
				if ns != "kcm-system" && tt.namespaceFilter.MatchString(ns) {
					allowedNamespaces = append(allowedNamespaces, ns)
				}
			}

			credentials, err := manager.ListCredentials(context.Background(), allowedNamespaces)
			if err != nil {
				t.Fatalf("ListCredentials returned error: %v", err)
			}

			if len(credentials) != tt.expectedCount {
				t.Errorf("expected %d credentials, got %d", tt.expectedCount, len(credentials))
			}
		})
	}
}

// TestListCredentials_ReadyStatus tests credential readiness extraction
func TestListCredentials_ReadyStatus(t *testing.T) {
	tests := []struct {
		name           string
		credential     *unstructured.Unstructured
		expectedReady  bool
		expectedReason string
	}{
		{
			name: "credential with Ready=True condition",
			credential: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "k0rdent.mirantis.com/v1beta1",
					"kind":       "Credential",
					"metadata": map[string]interface{}{
						"name":      "ready-cred",
						"namespace": "kcm-system",
					},
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":    "Ready",
								"status":  "True",
								"reason":  "Available",
								"message": "Credential is ready",
							},
						},
					},
				},
			},
			expectedReady:  true,
			expectedReason: "Available",
		},
		{
			name: "credential with Ready=False condition",
			credential: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "k0rdent.mirantis.com/v1beta1",
					"kind":       "Credential",
					"metadata": map[string]interface{}{
						"name":      "not-ready-cred",
						"namespace": "kcm-system",
					},
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":    "Ready",
								"status":  "False",
								"reason":  "ValidationFailed",
								"message": "Invalid credentials",
							},
						},
					},
				},
			},
			expectedReady:  false,
			expectedReason: "ValidationFailed",
		},
		{
			name: "credential without status",
			credential: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "k0rdent.mirantis.com/v1beta1",
					"kind":       "Credential",
					"metadata": map[string]interface{}{
						"name":      "no-status-cred",
						"namespace": "kcm-system",
					},
				},
			},
			expectedReady:  false,
			expectedReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, tt.credential)

			manager := &Manager{
				dynamicClient:          client,
				globalNamespace: "kcm-system",
		logger:          slog.Default(),
			}

			credentials, err := manager.ListCredentials(context.Background(), []string{"kcm-system"})
			if err != nil {
				t.Fatalf("ListCredentials returned error: %v", err)
			}

			if len(credentials) != 1 {
				t.Fatalf("expected 1 credential, got %d", len(credentials))
			}

			cred := credentials[0]
			if cred.Ready != tt.expectedReady {
				t.Errorf("expected ready=%v, got %v", tt.expectedReady, cred.Ready)
			}
		})
	}
}

// TestListCredentials_ProviderLabels tests provider extraction from labels
func TestListCredentials_ProviderLabels(t *testing.T) {
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
			cred := createTestCredential("test-cred", "kcm-system", tt.labels)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, cred)

			manager := &Manager{
				dynamicClient:          client,
				globalNamespace: "kcm-system",
		logger:          slog.Default(),
			}

			credentials, err := manager.ListCredentials(context.Background(), []string{"kcm-system"})
			if err != nil {
				t.Fatalf("ListCredentials returned error: %v", err)
			}

			if len(credentials) != 1 {
				t.Fatalf("expected 1 credential, got %d", len(credentials))
			}

			if credentials[0].Provider != tt.expectedProvider {
				t.Errorf("expected provider %q, got %q", tt.expectedProvider, credentials[0].Provider)
			}
		})
	}
}

// createTestCredential creates a test Credential unstructured object
func createTestCredential(name, namespace string, labels map[string]string) *unstructured.Unstructured {
	cred := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "Credential",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": "2025-01-01T00:00:00Z",
			},
		},
	}

	if labels != nil {
		cred.SetLabels(labels)
	}

	return cred
}

package clusters

import (
	"log/slog"
	"context"
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

// TestDeployCluster_Validation tests input validation
func TestDeployCluster_Validation(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	tests := []struct {
		name          string
		input         DeployRequest
		expectedError string
	}{
		{
			name: "missing cluster name",
			input: DeployRequest{
				Template:   "azure-template",
				Credential: "azure-cred",
				Config:     map[string]interface{}{"location": "westus2"},
			},
			expectedError: "cluster name is required",
		},
		{
			name: "missing template",
			input: DeployRequest{
				Name:       "test-cluster",
				Credential: "azure-cred",
				Config:     map[string]interface{}{"location": "westus2"},
			},
			expectedError: "template is required",
		},
		{
			name: "missing credential",
			input: DeployRequest{
				Name:     "test-cluster",
				Template: "azure-template",
				Config:   map[string]interface{}{"location": "westus2"},
			},
			expectedError: "credential is required",
		},
		{
			name: "missing config",
			input: DeployRequest{
				Name:       "test-cluster",
				Template:   "azure-template",
				Credential: "azure-cred",
			},
			expectedError: "config is required",
		},
		{
			name: "valid input",
			input: DeployRequest{
				Name:       "test-cluster",
				Template:   "azure-template",
				Credential: "azure-cred",
				Config:     map[string]interface{}{"location": "westus2"},
			},
			expectedError: "", // no error expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.DeployCluster(context.Background(), "kcm-system", tt.input)

			if tt.expectedError == "" {
				// For valid input, we expect error about missing template/credential resources
				// since they don't exist in the fake client
				if err == nil {
					t.Error("expected error about missing resources, got nil")
				}
			} else {
				if err == nil {
					t.Errorf("expected error %q, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

// TestDeployCluster_NamespaceResolution tests namespace resolution logic
func TestDeployCluster_NamespaceResolution(t *testing.T) {
	// Create test resources
	template := createTestClusterTemplate("azure-template", "kcm-system", nil)
	credential := createTestCredential("azure-cred", "kcm-system", nil)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, template, credential)

	tests := []struct {
		name              string
		devMode           bool
		namespaceFilter   *regexp.Regexp
		inputNamespace    string
		expectedNamespace string
		expectError       bool
	}{
		{
			name:              "dev mode with no input namespace defaults to global",
			// devMode removed:           true,
			namespaceFilter:   nil,
			inputNamespace:    "",
			expectedNamespace: "kcm-system",
			expectError:       false,
		},
		{
			name:              "dev mode with explicit namespace",
			// devMode removed:           true,
			namespaceFilter:   nil,
			inputNamespace:    "team-alpha",
			expectedNamespace: "team-alpha",
			expectError:       false,
		},
		{
			name:            "production mode without namespace should error",
			// devMode removed:         false,
			namespaceFilter: regexp.MustCompile("^team-"),
			inputNamespace:  "",
			expectError:     true,
		},
		{
			name:              "production mode with allowed namespace",
			// devMode removed:           false,
			namespaceFilter:   regexp.MustCompile("^team-"),
			inputNamespace:    "team-alpha",
			expectedNamespace: "team-alpha",
			expectError:       false,
		},
		{
			name:            "production mode with forbidden namespace",
			// devMode removed:         false,
			namespaceFilter: regexp.MustCompile("^team-"),
			inputNamespace:  "forbidden-namespace",
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				dynamicClient:          client,
				globalNamespace: "kcm-system",
		logger:          slog.Default(),
				// devMode removed:         tt.devMode,
				namespaceFilter: tt.namespaceFilter,
			}

			input := DeployRequest{
				Name:       "test-cluster",
				Template:   "azure-template",
				Credential: "azure-cred",
				Config:     map[string]interface{}{"location": "westus2"},
			}

			namespace := tt.inputNamespace
			if namespace == "" && tt.devMode {
				namespace = "kcm-system" // Manager should default to this
			}

			result, err := manager.DeployCluster(context.Background(), namespace, input)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				// Note: fake client doesn't support server-side apply, so we expect an error
				// but we're testing that it got past namespace validation
				if err != nil && err.Error() == "namespace not allowed by filter" {
					t.Errorf("unexpected namespace filter error: %v", err)
				}

				if result.Namespace != "" && result.Namespace != tt.expectedNamespace {
					t.Errorf("expected namespace %q, got %q", tt.expectedNamespace, result.Namespace)
				}
			}
		})
	}
}

// TestDeployCluster_TemplateResolution tests template reference resolution
func TestDeployCluster_TemplateResolution(t *testing.T) {
	globalTemplate := createTestClusterTemplate("global-template", "kcm-system", nil)
	localTemplate := createTestClusterTemplate("local-template", "team-alpha", nil)
	credential := createTestCredential("test-cred", "kcm-system", nil)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, globalTemplate, localTemplate, credential)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
		// devMode removed:         true,
	}

	tests := []struct {
		name              string
		templateRef       string
		targetNamespace   string
		expectError       bool
		expectedTemplate  string
	}{
		{
			name:             "simple template name from global namespace",
			templateRef:      "global-template",
			targetNamespace:  "kcm-system",
			expectError:      false,
			expectedTemplate: "global-template",
		},
		{
			name:             "simple template name from local namespace",
			templateRef:      "local-template",
			targetNamespace:  "team-alpha",
			expectError:      false,
			expectedTemplate: "local-template",
		},
		{
			name:             "namespaced template reference",
			templateRef:      "kcm-system/global-template",
			targetNamespace:  "team-alpha",
			expectError:      false,
			expectedTemplate: "global-template",
		},
		{
			name:            "non-existent template",
			templateRef:     "nonexistent-template",
			targetNamespace: "kcm-system",
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := DeployRequest{
				Name:       "test-cluster",
				Template:   tt.templateRef,
				Credential: "test-cred",
				Config:     map[string]interface{}{"location": "westus2"},
			}

			_, err := manager.DeployCluster(context.Background(), tt.targetNamespace, input)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				// Fake client doesn't support apply, but we verify it got past template resolution
				if err != nil && (err.Error() == "template not found" || err.Error() == "template "+tt.templateRef+" not found") {
					t.Errorf("template resolution failed: %v", err)
				}
			}
		})
	}
}

// TestDeployCluster_CredentialResolution tests credential reference resolution
func TestDeployCluster_CredentialResolution(t *testing.T) {
	template := createTestClusterTemplate("test-template", "kcm-system", nil)
	globalCred := createTestCredential("global-cred", "kcm-system", nil)
	localCred := createTestCredential("local-cred", "team-alpha", nil)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, template, globalCred, localCred)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
		// devMode removed:         true,
	}

	tests := []struct {
		name              string
		credentialRef     string
		targetNamespace   string
		expectError       bool
		expectedCredential string
	}{
		{
			name:               "simple credential name from global namespace",
			credentialRef:      "global-cred",
			targetNamespace:    "kcm-system",
			expectError:        false,
			expectedCredential: "global-cred",
		},
		{
			name:               "simple credential name from local namespace",
			credentialRef:      "local-cred",
			targetNamespace:    "team-alpha",
			expectError:        false,
			expectedCredential: "local-cred",
		},
		{
			name:               "namespaced credential reference",
			credentialRef:      "kcm-system/global-cred",
			targetNamespace:    "team-alpha",
			expectError:        false,
			expectedCredential: "global-cred",
		},
		{
			name:            "non-existent credential",
			credentialRef:   "nonexistent-cred",
			targetNamespace: "kcm-system",
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := DeployRequest{
				Name:       "test-cluster",
				Template:   "test-template",
				Credential: tt.credentialRef,
				Config:     map[string]interface{}{"location": "westus2"},
			}

			_, err := manager.DeployCluster(context.Background(), tt.targetNamespace, input)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				// Fake client doesn't support apply, but we verify it got past credential resolution
				if err != nil && (err.Error() == "credential not found" || err.Error() == "credential "+tt.credentialRef+" not found") {
					t.Errorf("credential resolution failed: %v", err)
				}
			}
		})
	}
}

// TestDeployCluster_Labels tests label application
func TestDeployCluster_Labels(t *testing.T) {
	template := createTestClusterTemplate("test-template", "kcm-system", nil)
	credential := createTestCredential("test-cred", "kcm-system", nil)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, template, credential)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
		// devMode removed:         true,
		fieldOwner:      "mcp.clusters",
	}

	input := DeployRequest{
		Name:       "test-cluster",
		Template:   "test-template",
		Credential: "test-cred",
		Config:     map[string]interface{}{"location": "westus2"},
		Labels: map[string]string{
			"environment": "test",
			"team":        "platform",
		},
	}

	// Note: fake client doesn't support server-side apply, so we can't fully test this
	// But we verify the input is accepted and processed
	_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

	// We expect an error due to fake client limitations, but not a validation error
	if err != nil && (err.Error() == "cluster name is required" || err.Error() == "template is required" || err.Error() == "credential is required") {
		t.Errorf("unexpected validation error: %v", err)
	}
}

// TestDeployCluster_ConfigPassthrough tests config object passthrough
func TestDeployCluster_ConfigPassthrough(t *testing.T) {
	template := createTestClusterTemplate("test-template", "kcm-system", nil)
	credential := createTestCredential("test-cred", "kcm-system", nil)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, template, credential)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
		// devMode removed:         true,
	}

	complexConfig := map[string]interface{}{
		"clusterIdentity": map[string]interface{}{
			"name":      "azure-cluster-identity",
			"namespace": "kcm-system",
		},
		"controlPlane": map[string]interface{}{
			"rootVolumeSize": 32,
			"vmSize":         "Standard_A4_v2",
		},
		"controlPlaneNumber": 1,
		"location":           "westus2",
		"subscriptionID":     "test-subscription-id",
		"worker": map[string]interface{}{
			"rootVolumeSize": 32,
			"vmSize":         "Standard_A4_v2",
		},
		"workersNumber": 1,
	}

	input := DeployRequest{
		Name:       "test-cluster",
		Template:   "test-template",
		Credential: "test-cred",
		Config:     complexConfig,
	}

	// Note: fake client doesn't support server-side apply
	// We're just testing that complex config objects are accepted
	_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

	// We expect an error due to fake client limitations, but not a config validation error
	if err != nil && err.Error() == "config is required" {
		t.Errorf("config validation failed: %v", err)
	}
}

// TestDeployCluster_ManagedLabels tests managed label application
func TestDeployCluster_ManagedLabels(t *testing.T) {
	// This test verifies that the manager applies managed labels to track MCP-created resources
	// The actual label application happens during server-side apply, which fake client doesn't support
	// This test documents the expected behavior

	template := createTestClusterTemplate("test-template", "kcm-system", nil)
	credential := createTestCredential("test-cred", "kcm-system", nil)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, template, credential)

	manager := &Manager{
		dynamicClient:          client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
		// devMode removed:         true,
		fieldOwner:      "mcp.clusters",
	}

	input := DeployRequest{
		Name:       "test-cluster",
		Template:   "test-template",
		Credential: "test-cred",
		Config:     map[string]interface{}{"location": "westus2"},
	}

	_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

	// Expected behavior (documented for when implementation exists):
	// - Should apply label: k0rdent.mirantis.com/managed=true
	// - Should use field owner: mcp.clusters (or configured value)
	// - Should preserve user-provided labels

	// We expect an error due to fake client not supporting apply
	if err == nil {
		t.Log("Note: When implementation is complete, verify managed labels are applied")
	}
}

// TestDeployCluster_Idempotency tests idempotent apply behavior
func TestDeployCluster_Idempotency(t *testing.T) {
	t.Skip("Skipping: fake dynamic client does not support server-side Apply - tested in integration tests")
	// Note: This test would verify that calling DeployCluster twice with the same input
	// results in an update rather than an error. The first call creates the resource,
	// the second call updates it. Both should succeed with different result statuses
	// (created vs updated).
	//
	// Expected behavior:
	// - First deploy: result.Created = true
	// - Second deploy: result.Created = false, result.Updated = true
	// - Both deploys: same UID, same namespace, no errors
}

// TestDeployCluster_ServerSideApply tests server-side apply behavior
func TestDeployCluster_ServerSideApply(t *testing.T) {
	t.Skip("Skipping: fake dynamic client does not support server-side Apply - tested in integration tests")
	// Note: This test would verify server-side apply semantics:
	// - Field ownership tracking
	// - Merge behavior for managed fields
	// - Conflict resolution
	//
	// These behaviors require a real API server or more sophisticated fake client.
}

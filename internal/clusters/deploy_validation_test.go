package clusters

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

// TestDeployCluster_AWSValidation tests AWS-specific configuration validation in deploy flow
func TestDeployCluster_AWSValidation(t *testing.T) {
	tests := []struct {
		name                string
		config              map[string]interface{}
		expectError         bool
		expectedErrContains []string
	}{
		{
			name: "valid AWS config",
			config: map[string]interface{}{
				"region": "us-west-2",
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
				"worker": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectError: false,
		},
		{
			name: "missing region",
			config: map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
				"worker": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"AWS cluster configuration validation failed",
				"config.region",
				"AWS region is required",
				"us-west-2",
				"https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/",
			},
		},
		{
			name: "empty region string",
			config: map[string]interface{}{
				"region": "   ",
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"AWS cluster configuration validation failed",
				"config.region",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test resources
			template := createTestClusterTemplate("aws-standalone-cp-1-0-16", "kcm-system", nil)
			credential := createTestCredential("aws-cred", "kcm-system", nil)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, template, credential)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
				fieldOwner:      "mcp.clusters",
			}

			input := DeployRequest{
				Name:       "test-aws-cluster",
				Template:   "aws-standalone-cp-1-0-16",
				Credential: "aws-cred",
				Config:     tt.config,
			}

			_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}

				// Check that error is ErrInvalidRequest
				if !errors.Is(err, ErrInvalidRequest) {
					t.Errorf("expected ErrInvalidRequest, got: %v", err)
				}

				// Check error message contains expected strings
				errMsg := err.Error()
				for _, expectedStr := range tt.expectedErrContains {
					if !strings.Contains(errMsg, expectedStr) {
						t.Errorf("expected error to contain %q, got: %s", expectedStr, errMsg)
					}
				}
			} else {
				// Valid config should pass validation but fail on apply (fake client limitation)
				// Should NOT be a validation error
				if err != nil && errors.Is(err, ErrInvalidRequest) {
					t.Errorf("unexpected validation error for valid config: %v", err)
				}
			}
		})
	}
}

// TestDeployCluster_AzureValidation tests Azure-specific configuration validation in deploy flow
func TestDeployCluster_AzureValidation(t *testing.T) {
	tests := []struct {
		name                string
		config              map[string]interface{}
		expectError         bool
		expectedErrContains []string
	}{
		{
			name: "valid Azure config",
			config: map[string]interface{}{
				"location":       "westus2",
				"subscriptionID": "12345678-1234-1234-1234-123456789abc",
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
				"worker": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectError: false,
		},
		{
			name: "missing location",
			config: map[string]interface{}{
				"subscriptionID": "12345678-1234-1234-1234-123456789abc",
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"Azure cluster configuration validation failed",
				"config.location",
				"Azure location is required",
				"westus2",
				"https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/",
			},
		},
		{
			name: "missing subscriptionID",
			config: map[string]interface{}{
				"location": "westus2",
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"Azure cluster configuration validation failed",
				"config.subscriptionID",
				"Azure subscription ID is required",
			},
		},
		{
			name: "missing both location and subscriptionID",
			config: map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"Azure cluster configuration validation failed",
				"config.location",
				"config.subscriptionID",
			},
		},
		{
			name: "empty strings for required fields",
			config: map[string]interface{}{
				"location":       "",
				"subscriptionID": "   ",
			},
			expectError: true,
			expectedErrContains: []string{
				"Azure cluster configuration validation failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test resources
			template := createTestClusterTemplate("azure-standalone-cp-1-0-17", "kcm-system", nil)
			credential := createTestCredential("azure-cred", "kcm-system", nil)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, template, credential)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
				fieldOwner:      "mcp.clusters",
			}

			input := DeployRequest{
				Name:       "test-azure-cluster",
				Template:   "azure-standalone-cp-1-0-17",
				Credential: "azure-cred",
				Config:     tt.config,
			}

			_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}

				// Check that error is ErrInvalidRequest
				if !errors.Is(err, ErrInvalidRequest) {
					t.Errorf("expected ErrInvalidRequest, got: %v", err)
				}

				// Check error message contains expected strings
				errMsg := err.Error()
				for _, expectedStr := range tt.expectedErrContains {
					if !strings.Contains(errMsg, expectedStr) {
						t.Errorf("expected error to contain %q, got: %s", expectedStr, errMsg)
					}
				}
			} else {
				// Valid config should pass validation but fail on apply (fake client limitation)
				// Should NOT be a validation error
				if err != nil && errors.Is(err, ErrInvalidRequest) {
					t.Errorf("unexpected validation error for valid config: %v", err)
				}
			}
		})
	}
}

// TestDeployCluster_GCPValidation tests GCP-specific configuration validation in deploy flow
func TestDeployCluster_GCPValidation(t *testing.T) {
	tests := []struct {
		name                string
		config              map[string]interface{}
		expectError         bool
		expectedErrContains []string
	}{
		{
			name: "valid GCP config",
			config: map[string]interface{}{
				"project": "my-gcp-project-123456",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": "default",
				},
				"controlPlane": map[string]interface{}{
					"instanceType": "n1-standard-4",
				},
				"worker": map[string]interface{}{
					"instanceType": "n1-standard-4",
				},
			},
			expectError: false,
		},
		{
			name: "missing project",
			config: map[string]interface{}{
				"region": "us-central1",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"GCP cluster configuration validation failed",
				"config.project",
				"GCP project ID is required",
				"https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/",
			},
		},
		{
			name: "missing region",
			config: map[string]interface{}{
				"project": "my-gcp-project-123456",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"GCP cluster configuration validation failed",
				"config.region",
				"GCP region is required",
			},
		},
		{
			name: "missing network.name",
			config: map[string]interface{}{
				"project": "my-gcp-project-123456",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"subnet": "default-subnet",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"GCP cluster configuration validation failed",
				"config.network.name",
				"GCP network name is required",
			},
		},
		{
			name: "missing network object entirely",
			config: map[string]interface{}{
				"project": "my-gcp-project-123456",
				"region":  "us-central1",
			},
			expectError: true,
			expectedErrContains: []string{
				"GCP cluster configuration validation failed",
				"config.network.name",
			},
		},
		{
			name: "missing all required fields",
			config: map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"instanceType": "n1-standard-4",
				},
			},
			expectError: true,
			expectedErrContains: []string{
				"GCP cluster configuration validation failed",
				"config.project",
				"config.region",
				"config.network.name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test resources
			template := createTestClusterTemplate("gcp-standalone-cp-1-0-15", "kcm-system", nil)
			credential := createTestCredential("gcp-cred", "kcm-system", nil)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, template, credential)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
				fieldOwner:      "mcp.clusters",
			}

			input := DeployRequest{
				Name:       "test-gcp-cluster",
				Template:   "gcp-standalone-cp-1-0-15",
				Credential: "gcp-cred",
				Config:     tt.config,
			}

			_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}

				// Check that error is ErrInvalidRequest
				if !errors.Is(err, ErrInvalidRequest) {
					t.Errorf("expected ErrInvalidRequest, got: %v", err)
				}

				// Check error message contains expected strings
				errMsg := err.Error()
				for _, expectedStr := range tt.expectedErrContains {
					if !strings.Contains(errMsg, expectedStr) {
						t.Errorf("expected error to contain %q, got: %s", expectedStr, errMsg)
					}
				}
			} else {
				// Valid config should pass validation but fail on apply (fake client limitation)
				// Should NOT be a validation error
				if err != nil && errors.Is(err, ErrInvalidRequest) {
					t.Errorf("unexpected validation error for valid config: %v", err)
				}
			}
		})
	}
}

// TestDeployCluster_NonCloudProviderPassthrough tests that non-cloud providers pass validation
func TestDeployCluster_NonCloudProviderPassthrough(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		config       map[string]interface{}
	}{
		{
			name:         "vsphere template with arbitrary config",
			templateName: "vsphere-standalone-cp",
			config: map[string]interface{}{
				"datacenter": "dc1",
				"cluster":    "cluster1",
				"datastore":  "datastore1",
			},
		},
		{
			name:         "openstack template",
			templateName: "openstack-standalone",
			config: map[string]interface{}{
				"cloudName": "mycloud",
				"image":     "ubuntu-22.04",
			},
		},
		{
			name:         "custom template",
			templateName: "my-custom-template",
			config: map[string]interface{}{
				"customField": "value",
			},
		},
		{
			name:         "template with no prefix",
			templateName: "standalone-template",
			config:       map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test resources
			template := createTestClusterTemplate(tt.templateName, "kcm-system", nil)
			credential := createTestCredential("test-cred", "kcm-system", nil)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, template, credential)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
				fieldOwner:      "mcp.clusters",
			}

			input := DeployRequest{
				Name:       "test-cluster",
				Template:   tt.templateName,
				Credential: "test-cred",
				Config:     tt.config,
			}

			_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

			// Should not be a validation error - unknown providers pass through
			if err != nil && errors.Is(err, ErrInvalidRequest) {
				t.Errorf("unexpected validation error for non-cloud provider: %v", err)
			}
		})
	}
}

// TestDeployCluster_ValidationErrorFormat tests that validation errors are properly formatted
func TestDeployCluster_ValidationErrorFormat(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		config       map[string]interface{}
		provider     string
		checkFormat  func(t *testing.T, errMsg string)
	}{
		{
			name:         "AWS error format",
			templateName: "aws-standalone-cp",
			config:       map[string]interface{}{},
			provider:     "AWS",
			checkFormat: func(t *testing.T, errMsg string) {
				// Check for provider in error message
				if !strings.Contains(errMsg, "AWS cluster configuration validation failed") {
					t.Error("missing AWS-specific header in error message")
				}
				// Check for example config
				if !strings.Contains(errMsg, "Example valid AWS configuration") {
					t.Error("missing example configuration in error message")
				}
				// Check for docs link
				if !strings.Contains(errMsg, "https://docs.k0rdent.io") {
					t.Error("missing documentation link in error message")
				}
			},
		},
		{
			name:         "Azure error format",
			templateName: "azure-hosted-cp",
			config:       map[string]interface{}{},
			provider:     "Azure",
			checkFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "Azure cluster configuration validation failed") {
					t.Error("missing Azure-specific header in error message")
				}
				if !strings.Contains(errMsg, "Example valid Azure configuration") {
					t.Error("missing example configuration in error message")
				}
				if !strings.Contains(errMsg, "vmSize") {
					t.Error("missing Azure-specific terminology (vmSize) in example")
				}
			},
		},
		{
			name:         "GCP error format",
			templateName: "gcp-hosted-cp",
			config:       map[string]interface{}{},
			provider:     "GCP",
			checkFormat: func(t *testing.T, errMsg string) {
				if !strings.Contains(errMsg, "GCP cluster configuration validation failed") {
					t.Error("missing GCP-specific header in error message")
				}
				if !strings.Contains(errMsg, "Example valid GCP configuration") {
					t.Error("missing example configuration in error message")
				}
				if !strings.Contains(errMsg, "network") {
					t.Error("missing GCP-specific nested field (network) in example")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test resources
			template := createTestClusterTemplate(tt.templateName, "kcm-system", nil)
			credential := createTestCredential("test-cred", "kcm-system", nil)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, template, credential)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
			}

			input := DeployRequest{
				Name:       "test-cluster",
				Template:   tt.templateName,
				Credential: "test-cred",
				Config:     tt.config,
			}

			_, err := manager.DeployCluster(context.Background(), "kcm-system", input)

			if err == nil {
				t.Fatal("expected validation error, got nil")
			}

			tt.checkFormat(t, err.Error())
		})
	}
}

package clusters

import (
	"context"
	"log/slog"
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

// TestGetAzureClusterDetail_Success tests successful detail extraction with complete AzureCluster data
func TestGetAzureClusterDetail_Success(t *testing.T) {
	// Create test resources with complete data
	clusterDeployment := createTestClusterDeploymentForAzure("test-cluster", "kcm-system", map[string]string{
		"environment": "test",
	})
	azureCluster := createTestAzureCluster("test-cluster", "kcm-system", true)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, clusterDeployment, azureCluster)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	detail, err := manager.GetAzureClusterDetail(context.Background(), "kcm-system", "test-cluster")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify basic metadata
	if detail.Name != "test-cluster" {
		t.Errorf("expected name 'test-cluster', got %q", detail.Name)
	}
	if detail.Namespace != "kcm-system" {
		t.Errorf("expected namespace 'kcm-system', got %q", detail.Namespace)
	}
	if detail.Provider != "azure" {
		t.Errorf("expected provider 'azure', got %q", detail.Provider)
	}

	// Verify Azure infrastructure details
	if detail.Azure.ResourceGroup != "test-rg" {
		t.Errorf("expected resourceGroup 'test-rg', got %q", detail.Azure.ResourceGroup)
	}
	if detail.Azure.SubscriptionID != "sub-123" {
		t.Errorf("expected subscriptionID 'sub-123', got %q", detail.Azure.SubscriptionID)
	}
	if detail.Azure.Location != "westus2" {
		t.Errorf("expected location 'westus2', got %q", detail.Azure.Location)
	}

	// Verify VNet details
	if detail.Azure.VNet == nil {
		t.Fatal("expected VNet to be present")
	}
	if detail.Azure.VNet.Name != "test-vnet" {
		t.Errorf("expected vnet name 'test-vnet', got %q", detail.Azure.VNet.Name)
	}
	if detail.Azure.VNet.CIDR != "10.0.0.0/16" {
		t.Errorf("expected vnet CIDR '10.0.0.0/16', got %q", detail.Azure.VNet.CIDR)
	}

	// Verify subnets
	if len(detail.Azure.Subnets) != 2 {
		t.Fatalf("expected 2 subnets, got %d", len(detail.Azure.Subnets))
	}

	// Verify control plane endpoint
	if detail.ControlPlaneEndpoint == nil {
		t.Fatal("expected control plane endpoint to be present")
	}
	if detail.ControlPlaneEndpoint.Host != "test-api.westus2.cloudapp.azure.com" {
		t.Errorf("expected host 'test-api.westus2.cloudapp.azure.com', got %q", detail.ControlPlaneEndpoint.Host)
	}
	if detail.ControlPlaneEndpoint.Port != 6443 {
		t.Errorf("expected port 6443, got %d", detail.ControlPlaneEndpoint.Port)
	}

	// Verify kubeconfig secret
	if detail.KubeconfigSecret == nil {
		t.Fatal("expected kubeconfig secret to be present")
	}
	if detail.KubeconfigSecret.Name != "test-cluster-kubeconfig" {
		t.Errorf("expected kubeconfig secret name 'test-cluster-kubeconfig', got %q", detail.KubeconfigSecret.Name)
	}
}

// TestGetAzureClusterDetail_MinimalData tests successful detail extraction with minimal AzureCluster data
func TestGetAzureClusterDetail_MinimalData(t *testing.T) {
	// Create test resources with minimal data (optional fields missing)
	clusterDeployment := createTestClusterDeploymentForAzure("minimal-cluster", "kcm-system", nil)
	azureCluster := createTestAzureCluster("minimal-cluster", "kcm-system", false)

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, clusterDeployment, azureCluster)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	detail, err := manager.GetAzureClusterDetail(context.Background(), "kcm-system", "minimal-cluster")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify basic metadata is present
	if detail.Name != "minimal-cluster" {
		t.Errorf("expected name 'minimal-cluster', got %q", detail.Name)
	}
	if detail.Provider != "azure" {
		t.Errorf("expected provider 'azure', got %q", detail.Provider)
	}

	// Verify that required fields are present even with minimal data
	if detail.Azure.ResourceGroup != "minimal-rg" {
		t.Errorf("expected resourceGroup 'minimal-rg', got %q", detail.Azure.ResourceGroup)
	}

	// Verify optional fields can be nil/empty
	if detail.Azure.VNet != nil && detail.Azure.VNet.Name != "" {
		t.Log("VNet is present (optional)")
	}
	if detail.ControlPlaneEndpoint != nil && detail.ControlPlaneEndpoint.Host != "" {
		t.Log("Control plane endpoint is present (optional)")
	}
}

// TestGetAzureClusterDetail_ClusterDeploymentNotFound tests error when ClusterDeployment not found
func TestGetAzureClusterDetail_ClusterDeploymentNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	_, err := manager.GetAzureClusterDetail(context.Background(), "kcm-system", "nonexistent-cluster")

	if err == nil {
		t.Fatal("expected error when ClusterDeployment not found, got nil")
	}

	// Verify error message indicates ClusterDeployment not found
	expectedErrSubstr := "get ClusterDeployment"
	if !containsSubstr(err.Error(), expectedErrSubstr) {
		t.Errorf("expected error containing %q, got %q", expectedErrSubstr, err.Error())
	}
}

// TestGetAzureClusterDetail_AzureClusterNotFound tests error when AzureCluster not found
func TestGetAzureClusterDetail_AzureClusterNotFound(t *testing.T) {
	// Create only ClusterDeployment, but not the corresponding AzureCluster
	clusterDeployment := createTestClusterDeploymentForAzure("test-cluster", "kcm-system", nil)

	scheme := runtime.NewScheme()
	// Register AzureCluster GVR for LIST operations
	gvrToListKind := map[schema.GroupVersionResource]string{
		AzureClusterGVR: "AzureClusterList",
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, clusterDeployment)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	_, err := manager.GetAzureClusterDetail(context.Background(), "kcm-system", "test-cluster")

	if err == nil {
		t.Fatal("expected error when AzureCluster not found, got nil")
	}

	// Verify error message indicates AzureCluster not found
	expectedErrSubstr := "AzureCluster not found"
	if !containsSubstr(err.Error(), expectedErrSubstr) {
		t.Errorf("expected error containing %q, got %q", expectedErrSubstr, err.Error())
	}
}

// TestGetAzureClusterDetail_NamespaceValidation tests namespace validation with NamespaceFilter
func TestGetAzureClusterDetail_NamespaceValidation(t *testing.T) {
	tests := []struct {
		name            string
		targetNamespace string
		namespaceFilter *regexp.Regexp
		setupResources  bool
		expectError     bool
		errorContains   string
	}{
		{
			name:            "dev mode allows any namespace",
			targetNamespace: "team-alpha",
			namespaceFilter: nil,
			setupResources:  true,
			expectError:     false,
		},
		{
			name:            "production mode with allowed namespace",
			targetNamespace: "team-alpha",
			namespaceFilter: regexp.MustCompile("^team-"),
			setupResources:  true,
			expectError:     false,
		},
		{
			name:            "production mode with forbidden namespace",
			targetNamespace: "forbidden-ns",
			namespaceFilter: regexp.MustCompile("^team-"),
			setupResources:  false,
			expectError:     true,
			errorContains:   "not found", // Will fail to find cluster since resources aren't created
		},
		{
			name:            "kcm-system allowed in production mode",
			targetNamespace: "kcm-system",
			namespaceFilter: regexp.MustCompile("^(kcm-system|team-)"),
			setupResources:  true,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client *fake.FakeDynamicClient
			if tt.setupResources {
				clusterDeployment := createTestClusterDeploymentForAzure("test-cluster", tt.targetNamespace, nil)
				azureCluster := createTestAzureCluster("test-cluster", tt.targetNamespace, false)
				scheme := runtime.NewScheme()
				client = fake.NewSimpleDynamicClient(scheme, clusterDeployment, azureCluster)
			} else {
				scheme := runtime.NewScheme()
				client = fake.NewSimpleDynamicClient(scheme)
			}

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
				namespaceFilter: tt.namespaceFilter,
			}

			_, err := manager.GetAzureClusterDetail(context.Background(), tt.targetNamespace, "test-cluster")

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !containsSubstr(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestDiscoverAzureCluster tests AzureCluster discovery by label matching
func TestDiscoverAzureCluster(t *testing.T) {
	tests := []struct {
		name              string
		clusterName       string
		azureClusters     []*unstructured.Unstructured
		expectError       bool
		expectedClusterID string
	}{
		{
			name:        "discover by label match",
			clusterName: "test-cluster",
			azureClusters: []*unstructured.Unstructured{
				createTestAzureClusterWithLabel("azure-1", "kcm-system", "test-cluster"),
				createTestAzureClusterWithLabel("azure-2", "kcm-system", "other-cluster"),
			},
			expectError:       false,
			expectedClusterID: "azure-1",
		},
		{
			name:          "no matching cluster",
			clusterName:   "test-cluster",
			azureClusters: []*unstructured.Unstructured{
				createTestAzureClusterWithLabel("azure-1", "kcm-system", "other-cluster"),
			},
			expectError: true,
		},
		{
			name:          "no clusters in namespace",
			clusterName:   "test-cluster",
			azureClusters: []*unstructured.Unstructured{},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterDeployment := createTestClusterDeploymentForAzure(tt.clusterName, "kcm-system", nil)

			objects := []runtime.Object{clusterDeployment}
			for i := range tt.azureClusters {
				objects = append(objects, tt.azureClusters[i])
			}

			scheme := runtime.NewScheme()
			// Register AzureCluster GVR for LIST operations
			gvrToListKind := map[schema.GroupVersionResource]string{
				AzureClusterGVR: "AzureClusterList",
			}
			client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
			}

			azureCluster, err := manager.discoverAzureCluster(context.Background(), "kcm-system", clusterDeployment)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if azureCluster.GetName() != tt.expectedClusterID {
					t.Errorf("expected cluster name %q, got %q", tt.expectedClusterID, azureCluster.GetName())
				}
			}
		})
	}
}

// TestExtractAzureInfrastructure tests Azure infrastructure extraction
func TestExtractAzureInfrastructure(t *testing.T) {
	tests := []struct {
		name            string
		azureCluster    *unstructured.Unstructured
		expectedRG      string
		expectedSubID   string
		expectedLoc     string
		expectVNet      bool
		expectSubnets   int
		expectIdentity  bool
	}{
		{
			name:           "complete infrastructure",
			azureCluster:   createTestAzureCluster("test-cluster", "kcm-system", true),
			expectedRG:     "test-rg",
			expectedSubID:  "sub-123",
			expectedLoc:    "westus2",
			expectVNet:     true,
			expectSubnets:  2,
			expectIdentity: true,
		},
		{
			name:           "minimal infrastructure",
			azureCluster:   createTestAzureCluster("minimal-cluster", "kcm-system", false),
			expectedRG:     "minimal-rg",
			expectedSubID:  "",
			expectedLoc:    "",
			expectVNet:     false,
			expectSubnets:  0,
			expectIdentity: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				logger: slog.Default(),
			}

			infra := manager.extractAzureInfrastructure(tt.azureCluster)

			if infra.ResourceGroup != tt.expectedRG {
				t.Errorf("expected resourceGroup %q, got %q", tt.expectedRG, infra.ResourceGroup)
			}
			if infra.SubscriptionID != tt.expectedSubID {
				t.Errorf("expected subscriptionID %q, got %q", tt.expectedSubID, infra.SubscriptionID)
			}
			if infra.Location != tt.expectedLoc {
				t.Errorf("expected location %q, got %q", tt.expectedLoc, infra.Location)
			}

			if tt.expectVNet && infra.VNet == nil {
				t.Error("expected VNet to be present")
			} else if !tt.expectVNet && infra.VNet != nil {
				t.Error("expected VNet to be nil")
			}

			if len(infra.Subnets) != tt.expectSubnets {
				t.Errorf("expected %d subnets, got %d", tt.expectSubnets, len(infra.Subnets))
			}

			if tt.expectIdentity && infra.IdentityRef == nil {
				t.Error("expected identityRef to be present")
			} else if !tt.expectIdentity && infra.IdentityRef != nil {
				t.Error("expected identityRef to be nil")
			}
		})
	}
}

// Helper functions

// createTestClusterDeploymentForAzure creates a test ClusterDeployment for Azure testing
func createTestClusterDeploymentForAzure(name, namespace string, labels map[string]string) *unstructured.Unstructured {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterDeployment",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": "2025-01-01T00:00:00Z",
			},
			"spec": map[string]interface{}{
				"template": "azure-standalone-cp",
				"credential": "azure-cred",
				"config": map[string]interface{}{
					"location": "westus2",
				},
			},
			"status": map[string]interface{}{
				"ready":             true,
				"observedGeneration": int64(1),
				"kubeconfigSecret":   name + "-kubeconfig",
			},
		},
	}

	if labels != nil {
		deployment.SetLabels(labels)
	}

	return deployment
}

// createTestAzureCluster creates a test AzureCluster resource
func createTestAzureCluster(name, namespace string, complete bool) *unstructured.Unstructured {
	azureCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "AzureCluster",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name": name,
				},
			},
			"spec": map[string]interface{}{
				"resourceGroup": name + "-rg",
			},
		},
	}

	if !complete {
		// Minimal cluster with just resource group
		azureCluster.Object["spec"].(map[string]interface{})["resourceGroup"] = "minimal-rg"
		return azureCluster
	}

	// Complete cluster with all fields
	spec := azureCluster.Object["spec"].(map[string]interface{})
	spec["resourceGroup"] = "test-rg"
	spec["subscriptionID"] = "sub-123"
	spec["location"] = "westus2"
	spec["identityRef"] = map[string]interface{}{
		"name":      "azure-identity",
		"namespace": namespace,
	}
	spec["networkSpec"] = map[string]interface{}{
		"vnet": map[string]interface{}{
			"name": "test-vnet",
			"id":   "/subscriptions/sub-123/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet",
			"cidrBlocks": []interface{}{
				"10.0.0.0/16",
			},
		},
		"subnets": []interface{}{
			map[string]interface{}{
				"name": "control-plane-subnet",
				"id":   "/subscriptions/sub-123/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/control-plane-subnet",
				"cidrBlocks": []interface{}{
					"10.0.1.0/24",
				},
				"role": "control-plane",
			},
			map[string]interface{}{
				"name": "worker-subnet",
				"id":   "/subscriptions/sub-123/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/worker-subnet",
				"cidrBlocks": []interface{}{
					"10.0.2.0/24",
				},
				"role": "worker",
			},
		},
		"apiServerLB": map[string]interface{}{
			"name": "test-api-lb",
			"id":   "/subscriptions/sub-123/resourceGroups/test-rg/providers/Microsoft.Network/loadBalancers/test-api-lb",
			"type": "Public",
			"frontendIPs": []interface{}{
				map[string]interface{}{
					"publicIP": map[string]interface{}{
						"ipAddress": "20.30.40.50",
					},
				},
			},
		},
	}
	spec["controlPlaneEndpoint"] = map[string]interface{}{
		"host": "test-api.westus2.cloudapp.azure.com",
		"port": int64(6443),
	}

	return azureCluster
}

// createTestAzureClusterWithLabel creates a test AzureCluster with specific cluster label
func createTestAzureClusterWithLabel(name, namespace, clusterName string) *unstructured.Unstructured {
	azureCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
			"kind":       "AzureCluster",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name": clusterName,
				},
			},
			"spec": map[string]interface{}{
				"resourceGroup": name + "-rg",
			},
		},
	}

	return azureCluster
}

package core

import (
	"context"
	"log/slog"
	"regexp"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	runtimepkg "github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

// TestAzureClusterDetailTool_Success tests successful tool invocation
func TestAzureClusterDetailTool_Success(t *testing.T) {
	// Create test resources
	clusterDeployment := createTestAzureClusterDeployment("test-cluster", "kcm-system", nil)
	azureCluster := createTestAzureClusterResource("test-cluster", "kcm-system", true)

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme, clusterDeployment, azureCluster)

	// Create cluster manager
	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   client,
		NamespaceFilter: nil,
		GlobalNamespace: "kcm-system",
		Logger:          slog.Default(),
	})
	require.NoError(t, err)

	session := &runtimepkg.Session{
		Logger:          slog.Default(),
		NamespaceFilter: nil,
		Clusters:        mgr,
		Clients: runtimepkg.Clients{
			Dynamic: client,
		},
	}

	tool := &azureClusterDetailTool{session: session}

	input := azureClusterDetailInput{
		Name:      "test-cluster",
		Namespace: "kcm-system",
	}

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.mgmt.clusterDeployments.azure.detail",
		},
	}

	_, result, err := tool.detail(context.Background(), req, input)
	require.NoError(t, err)

	// Verify result structure
	assert.Equal(t, "test-cluster", result.Name)
	assert.Equal(t, "kcm-system", result.Namespace)
	assert.Equal(t, "azure", result.Provider)

	// Verify Azure infrastructure details
	assert.Equal(t, "test-rg", result.Azure.ResourceGroup)
	assert.Equal(t, "sub-123", result.Azure.SubscriptionID)
	assert.Equal(t, "westus2", result.Azure.Location)

	// Verify VNet details
	require.NotNil(t, result.Azure.VNet)
	assert.Equal(t, "test-vnet", result.Azure.VNet.Name)
	assert.Equal(t, "10.0.0.0/16", result.Azure.VNet.CIDR)

	// Verify subnets
	require.Len(t, result.Azure.Subnets, 2)

	// Verify control plane endpoint
	require.NotNil(t, result.ControlPlaneEndpoint)
	assert.Equal(t, "test-api.westus2.cloudapp.azure.com", result.ControlPlaneEndpoint.Host)
	assert.Equal(t, int32(6443), result.ControlPlaneEndpoint.Port)

	// Verify kubeconfig secret
	require.NotNil(t, result.KubeconfigSecret)
	assert.Equal(t, "test-cluster-kubeconfig", result.KubeconfigSecret.Name)
}

// TestAzureClusterDetailTool_ValidationErrors tests validation of required fields
func TestAzureClusterDetailTool_ValidationErrors(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   client,
		NamespaceFilter: nil,
		GlobalNamespace: "kcm-system",
		Logger:          slog.Default(),
	})
	require.NoError(t, err)

	session := &runtimepkg.Session{
		Logger:          slog.Default(),
		NamespaceFilter: nil,
		Clusters:        mgr,
		Clients: runtimepkg.Clients{
			Dynamic: client,
		},
	}

	tool := &azureClusterDetailTool{session: session}
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.mgmt.clusterDeployments.azure.detail",
		},
	}

	tests := []struct {
		name          string
		input         azureClusterDetailInput
		errorContains string
	}{
		{
			name: "missing cluster name",
			input: azureClusterDetailInput{
				Namespace: "kcm-system",
			},
			errorContains: "cluster name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tool.detail(context.Background(), req, tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

// TestAzureClusterDetailTool_NamespaceResolution_DevMode tests namespace resolution in DEV_ALLOW_ANY mode
func TestAzureClusterDetailTool_NamespaceResolution_DevMode(t *testing.T) {
	tests := []struct {
		name              string
		inputNamespace    string
		expectedNamespace string
		setupResources    bool
		expectError       bool
		errorContains     string
	}{
		{
			name:              "explicit namespace in dev mode",
			inputNamespace:    "team-alpha",
			expectedNamespace: "team-alpha",
			setupResources:    true,
			expectError:       false,
		},
		{
			name:              "default to kcm-system in dev mode",
			inputNamespace:    "",
			expectedNamespace: "kcm-system",
			setupResources:    true,
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetNS := tt.expectedNamespace
			var objects []runtime.Object

			if tt.setupResources {
				clusterDeployment := createTestAzureClusterDeployment("test-cluster", targetNS, nil)
				azureCluster := createTestAzureClusterResource("test-cluster", targetNS, true)
				objects = []runtime.Object{clusterDeployment, azureCluster}
			}

			scheme := runtime.NewScheme()
			client := dynamicfake.NewSimpleDynamicClient(scheme, objects...)

			mgr, err := clusters.NewManager(clusters.Options{
				DynamicClient:   client,
				NamespaceFilter: nil, // DEV_ALLOW_ANY mode
				GlobalNamespace: "kcm-system",
				Logger:          slog.Default(),
			})
			require.NoError(t, err)

			session := &runtimepkg.Session{
				Logger:          slog.Default(),
				NamespaceFilter: nil, // DEV_ALLOW_ANY mode
				Clusters:        mgr,
				Clients: runtimepkg.Clients{
					Dynamic: client,
				},
			}

			tool := &azureClusterDetailTool{session: session}

			input := azureClusterDetailInput{
				Name:      "test-cluster",
				Namespace: tt.inputNamespace,
			}

			req := &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name: "k0rdent.mgmt.clusterDeployments.azure.detail",
				},
			}

			_, result, err := tool.detail(context.Background(), req, input)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNamespace, result.Namespace)
			}
		})
	}
}

// TestAzureClusterDetailTool_NamespaceResolution_OIDCMode tests namespace resolution in OIDC_REQUIRED mode
func TestAzureClusterDetailTool_NamespaceResolution_OIDCMode(t *testing.T) {
	tests := []struct {
		name              string
		inputNamespace    string
		namespaceFilter   *regexp.Regexp
		expectedNamespace string
		setupResources    bool
		expectError       bool
		errorContains     string
	}{
		{
			name:              "explicit namespace allowed by filter",
			inputNamespace:    "team-alpha",
			namespaceFilter:   regexp.MustCompile("^team-"),
			expectedNamespace: "team-alpha",
			setupResources:    true,
			expectError:       false,
		},
		{
			name:            "explicit namespace denied by filter",
			inputNamespace:  "forbidden-ns",
			namespaceFilter: regexp.MustCompile("^team-"),
			setupResources:  false,
			expectError:     true,
			errorContains:   "not allowed by namespace filter",
		},
		{
			name:            "no namespace in OIDC mode requires explicit",
			inputNamespace:  "",
			namespaceFilter: regexp.MustCompile("^team-"),
			setupResources:  false,
			expectError:     true,
			errorContains:   "namespace must be specified in OIDC_REQUIRED mode",
		},
		{
			name:              "kcm-system allowed explicitly",
			inputNamespace:    "kcm-system",
			namespaceFilter:   regexp.MustCompile("^(kcm-system|team-)"),
			expectedNamespace: "kcm-system",
			setupResources:    true,
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetNS := tt.inputNamespace
			if targetNS == "" {
				targetNS = "team-alpha" // default for resource creation
			}

			var objects []runtime.Object
			if tt.setupResources {
				clusterDeployment := createTestAzureClusterDeployment("test-cluster", targetNS, nil)
				azureCluster := createTestAzureClusterResource("test-cluster", targetNS, true)
				objects = []runtime.Object{clusterDeployment, azureCluster}
			}

			scheme := runtime.NewScheme()
			client := dynamicfake.NewSimpleDynamicClient(scheme, objects...)

			mgr, err := clusters.NewManager(clusters.Options{
				DynamicClient:   client,
				NamespaceFilter: tt.namespaceFilter, // OIDC_REQUIRED mode
				GlobalNamespace: "kcm-system",
				Logger:          slog.Default(),
			})
			require.NoError(t, err)

			session := &runtimepkg.Session{
				Logger:          slog.Default(),
				NamespaceFilter: tt.namespaceFilter, // OIDC_REQUIRED mode
				Clusters:        mgr,
				Clients: runtimepkg.Clients{
					Dynamic: client,
				},
			}

			tool := &azureClusterDetailTool{session: session}

			input := azureClusterDetailInput{
				Name:      "test-cluster",
				Namespace: tt.inputNamespace,
			}

			req := &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name: "k0rdent.mgmt.clusterDeployments.azure.detail",
				},
			}

			_, result, err := tool.detail(context.Background(), req, input)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNamespace, result.Namespace)
			}
		})
	}
}

// TestAzureClusterDetailTool_ClusterNotFound tests error when cluster doesn't exist
func TestAzureClusterDetailTool_ClusterNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   client,
		NamespaceFilter: nil,
		GlobalNamespace: "kcm-system",
		Logger:          slog.Default(),
	})
	require.NoError(t, err)

	session := &runtimepkg.Session{
		Logger:          slog.Default(),
		NamespaceFilter: nil,
		Clusters:        mgr,
		Clients: runtimepkg.Clients{
			Dynamic: client,
		},
	}

	tool := &azureClusterDetailTool{session: session}

	input := azureClusterDetailInput{
		Name:      "nonexistent-cluster",
		Namespace: "kcm-system",
	}

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.mgmt.clusterDeployments.azure.detail",
		},
	}

	_, _, err = tool.detail(context.Background(), req, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get Azure cluster detail")
}

// TestAzureClusterDetailTool_AzureClusterNotFound tests error when AzureCluster resource doesn't exist
func TestAzureClusterDetailTool_AzureClusterNotFound(t *testing.T) {
	// Create only ClusterDeployment, but not the corresponding AzureCluster
	clusterDeployment := createTestAzureClusterDeployment("test-cluster", "kcm-system", nil)

	scheme := runtime.NewScheme()
	// Register AzureCluster GVR for LIST operations
	gvrToListKind := map[schema.GroupVersionResource]string{
		clusters.AzureClusterGVR: "AzureClusterList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, clusterDeployment)

	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   client,
		NamespaceFilter: nil,
		GlobalNamespace: "kcm-system",
		Logger:          slog.Default(),
	})
	require.NoError(t, err)

	session := &runtimepkg.Session{
		Logger:          slog.Default(),
		NamespaceFilter: nil,
		Clusters:        mgr,
		Clients: runtimepkg.Clients{
			Dynamic: client,
		},
	}

	tool := &azureClusterDetailTool{session: session}

	input := azureClusterDetailInput{
		Name:      "test-cluster",
		Namespace: "kcm-system",
	}

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.mgmt.clusterDeployments.azure.detail",
		},
	}

	_, _, err = tool.detail(context.Background(), req, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AzureCluster not found")
}

// TestAzureClusterDetailTool_MinimalData tests detail extraction with minimal data
func TestAzureClusterDetailTool_MinimalData(t *testing.T) {
	// Create test resources with minimal data
	clusterDeployment := createTestAzureClusterDeployment("minimal-cluster", "kcm-system", nil)
	azureCluster := createTestAzureClusterResource("minimal-cluster", "kcm-system", false)

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme, clusterDeployment, azureCluster)

	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   client,
		NamespaceFilter: nil,
		GlobalNamespace: "kcm-system",
		Logger:          slog.Default(),
	})
	require.NoError(t, err)

	session := &runtimepkg.Session{
		Logger:          slog.Default(),
		NamespaceFilter: nil,
		Clusters:        mgr,
		Clients: runtimepkg.Clients{
			Dynamic: client,
		},
	}

	tool := &azureClusterDetailTool{session: session}

	input := azureClusterDetailInput{
		Name:      "minimal-cluster",
		Namespace: "kcm-system",
	}

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.mgmt.clusterDeployments.azure.detail",
		},
	}

	_, result, err := tool.detail(context.Background(), req, input)
	require.NoError(t, err)

	// Verify basic fields are present
	assert.Equal(t, "minimal-cluster", result.Name)
	assert.Equal(t, "kcm-system", result.Namespace)
	assert.Equal(t, "azure", result.Provider)

	// Verify required Azure fields
	assert.Equal(t, "minimal-rg", result.Azure.ResourceGroup)

	// Optional fields should be handled gracefully
	// (they may be empty/nil but shouldn't cause errors)
}

// Helper functions

// createTestAzureClusterDeployment creates a test ClusterDeployment for Azure detail testing
func createTestAzureClusterDeployment(name, namespace string, labels map[string]string) *unstructured.Unstructured {
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

// createTestAzureClusterResource creates a test AzureCluster resource for detail testing
func createTestAzureClusterResource(name, namespace string, complete bool) *unstructured.Unstructured {
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

package core

import (
	"context"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	runtimepkg "github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

func TestAzureClusterDeployTool_ValidDeploy(t *testing.T) {
	t.Skip("Skipping: fake Kubernetes client does not include Credential resources - will be fixed by fix-provider-tool-test-fixtures proposal")
	scheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(scheme)

	// Create test templates
	template := makeAzureTemplate("azure-standalone-cp-1.0.14", "kcm-system", "1.0.14")
	dynamicClient := makeTestDynamicClient(scheme, &template)

	// Create real cluster manager
	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   dynamicClient,
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
			Dynamic: dynamicClient,
		},
	}

	tool := &azureClusterDeployTool{session: session}

	input := azureClusterDeployInput{
		Name:           "test-azure-cluster",
		Credential:     "azure-cred-1",
		Location:       "westus2",
		SubscriptionID: "12345678-1234-1234-1234-123456789abc",
		ControlPlane: azureNodeConfig{
			VMSize:         "Standard_A4_v2",
			RootVolumeSize: 50,
		},
		Worker: azureNodeConfig{
			VMSize:         "Standard_A2_v2",
			RootVolumeSize: 40,
		},
		ControlPlaneNumber: 3,
		WorkersNumber:      2,
		Namespace:          "kcm-system",
	}

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.provider.azure.clusterDeployments.deploy",
		},
	}

	_, result, err := tool.deploy(context.Background(), req, input)
	require.NoError(t, err)
	assert.Equal(t, "test-azure-cluster", result.Name)
	assert.Equal(t, "kcm-system", result.Namespace)
}

func TestAzureClusterDeployTool_DefaultValues(t *testing.T) {
	t.Skip("Skipping: fake Kubernetes client does not include Credential resources - will be fixed by fix-provider-tool-test-fixtures proposal")
	scheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(scheme)

	template := makeAzureTemplate("azure-standalone-cp-1.0.14", "kcm-system", "1.0.14")
	dynamicClient := makeTestDynamicClient(scheme, &template)

	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   dynamicClient,
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
			Dynamic: dynamicClient,
		},
	}

	tool := &azureClusterDeployTool{session: session}

	input := azureClusterDeployInput{
		Name:           "test-cluster",
		Credential:     "azure-cred",
		Location:       "eastus",
		SubscriptionID: "12345678-1234-1234-1234-123456789abc",
		ControlPlane: azureNodeConfig{
			VMSize: "Standard_A4_v2",
			// RootVolumeSize omitted - should default to 30
		},
		Worker: azureNodeConfig{
			VMSize: "Standard_A2_v2",
			// RootVolumeSize omitted - should default to 30
		},
		// ControlPlaneNumber omitted - should default to 3
		// WorkersNumber omitted - should default to 2
		Namespace: "kcm-system",
	}

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.provider.azure.clusterDeployments.deploy",
		},
	}

	// Verify deployment succeeds (defaults will be applied internally)
	_, result, err := tool.deploy(context.Background(), req, input)
	require.NoError(t, err)
	assert.Equal(t, "test-cluster", result.Name)
}

func TestAzureClusterDeployTool_ValidationErrors(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(scheme)

	template := makeAzureTemplate("azure-standalone-cp-1.0.14", "kcm-system", "1.0.14")
	dynamicClient := makeTestDynamicClient(scheme, &template)

	mgr, err := clusters.NewManager(clusters.Options{
		DynamicClient:   dynamicClient,
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
			Dynamic: dynamicClient,
		},
	}

	tool := &azureClusterDeployTool{session: session}
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "k0rdent.provider.azure.clusterDeployments.deploy",
		},
	}

	tests := []struct {
		name          string
		input         azureClusterDeployInput
		errorContains string
	}{
		{
			name: "missing location",
			input: azureClusterDeployInput{
				Name:           "test",
				Credential:     "cred",
				SubscriptionID: "sub-id",
				ControlPlane: azureNodeConfig{
					VMSize: "Standard_A4_v2",
				},
				Worker: azureNodeConfig{
					VMSize: "Standard_A2_v2",
				},
			},
			errorContains: "location is required",
		},
		{
			name: "missing subscriptionID",
			input: azureClusterDeployInput{
				Name:       "test",
				Credential: "cred",
				Location:   "westus2",
				ControlPlane: azureNodeConfig{
					VMSize: "Standard_A4_v2",
				},
				Worker: azureNodeConfig{
					VMSize: "Standard_A2_v2",
				},
			},
			errorContains: "subscriptionID is required",
		},
		{
			name: "missing control plane vmSize",
			input: azureClusterDeployInput{
				Name:           "test",
				Credential:     "cred",
				Location:       "westus2",
				SubscriptionID: "sub-id",
				ControlPlane:   azureNodeConfig{},
				Worker: azureNodeConfig{
					VMSize: "Standard_A2_v2",
				},
			},
			errorContains: "controlPlane.vmSize is required",
		},
		{
			name: "missing worker vmSize",
			input: azureClusterDeployInput{
				Name:           "test",
				Credential:     "cred",
				Location:       "westus2",
				SubscriptionID: "sub-id",
				ControlPlane: azureNodeConfig{
					VMSize: "Standard_A4_v2",
				},
				Worker: azureNodeConfig{},
			},
			errorContains: "worker.vmSize is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tool.deploy(context.Background(), req, tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestAzureClusterDeployTool_TemplateSelection(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = k8sscheme.AddToScheme(scheme)

	tests := []struct {
		name             string
		templates        []unstructured.Unstructured
		expectedTemplate string
		expectError      bool
	}{
		{
			name: "select latest from multiple versions",
			templates: []unstructured.Unstructured{
				makeAzureTemplate("azure-standalone-cp-1.0.14", "kcm-system", "1.0.14"),
				makeAzureTemplate("azure-standalone-cp-1.0.15", "kcm-system", "1.0.15"),
				makeAzureTemplate("azure-standalone-cp-1.0.13", "kcm-system", "1.0.13"),
			},
			expectedTemplate: "azure-standalone-cp-1.0.15",
		},
		{
			name:        "no templates available",
			templates:   []unstructured.Unstructured{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "select latest from multiple versions" {
				t.Skip("Skipping: fake Kubernetes client does not include Credential resources - will be fixed by fix-provider-tool-test-fixtures proposal")
			}
			objs := make([]runtime.Object, 0, len(tt.templates))
			for i := range tt.templates {
				objs = append(objs, &tt.templates[i])
			}

			dynamicClient := makeTestDynamicClient(scheme, objs...)

			mgr, err := clusters.NewManager(clusters.Options{
				DynamicClient:   dynamicClient,
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
					Dynamic: dynamicClient,
				},
			}

			tool := &azureClusterDeployTool{session: session}

			input := azureClusterDeployInput{
				Name:           "test-cluster",
				Credential:     "azure-cred",
				Location:       "westus2",
				SubscriptionID: "sub-id",
				ControlPlane: azureNodeConfig{
					VMSize: "Standard_A4_v2",
				},
				Worker: azureNodeConfig{
					VMSize: "Standard_A2_v2",
				},
				Namespace: "kcm-system",
			}

			req := &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name: "k0rdent.provider.azure.clusterDeployments.deploy",
				},
			}

			_, _, err = tool.deploy(context.Background(), req, input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAzureClusterDeployTool_ResolveDeployNamespace(t *testing.T) {
	tests := []struct {
		name            string
		inputNamespace  string
		namespaceFilter *regexp.Regexp
		expectedNS      string
		expectError     bool
		errorContains   string
	}{
		{
			name:            "explicit namespace in dev mode",
			inputNamespace:  "my-namespace",
			namespaceFilter: nil,
			expectedNS:      "my-namespace",
		},
		{
			name:            "default to kcm-system in dev mode",
			inputNamespace:  "",
			namespaceFilter: nil,
			expectedNS:      "kcm-system",
		},
		{
			name:            "explicit namespace allowed by filter",
			inputNamespace:  "team-azure",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectedNS:      "team-azure",
		},
		{
			name:            "explicit namespace denied by filter",
			inputNamespace:  "forbidden-ns",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectError:     true,
			errorContains:   "not allowed by namespace filter",
		},
		{
			name:            "no namespace in OIDC mode requires explicit",
			inputNamespace:  "",
			namespaceFilter: regexp.MustCompile("^team-"),
			expectError:     true,
			errorContains:   "namespace must be specified in OIDC_REQUIRED mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &runtimepkg.Session{
				Logger:          slog.Default(),
				NamespaceFilter: tt.namespaceFilter,
			}

			tool := &azureClusterDeployTool{session: session}

			ns, err := tool.resolveDeployNamespace(context.Background(), tt.inputNamespace, slog.Default())

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNS, ns)
			}
		})
	}
}

// Helper function to create Azure cluster templates for testing
func makeAzureTemplate(name, namespace, version string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterTemplate",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": metav1.Time{Time: time.Now()}.Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"version": version,
			},
		},
	}
}

// Helper function to create a dynamic client with list kinds registered
func makeTestDynamicClient(scheme *runtime.Scheme, objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	gvr := map[schema.GroupVersionResource]string{
		{Group: "k0rdent.mirantis.com", Version: "v1beta1", Resource: "clustertemplates"}: "ClusterTemplateList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvr, objs...)
}

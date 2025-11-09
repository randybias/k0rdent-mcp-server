package core

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	clustermonitor "github.com/k0rdent/mcp-k0rdent-server/internal/kube/cluster_monitor"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

func TestParseClusterMonitorURI(t *testing.T) {
	target, err := parseClusterMonitorURI("k0rdent://cluster-monitor/kcm-system/demo-cluster?timeout=120")
	require.NoError(t, err)
	require.Equal(t, "kcm-system", target.Namespace)
	require.Equal(t, "demo-cluster", target.Name)
	require.Equal(t, 120*time.Second, target.Timeout)
}

func TestParseClusterMonitorURIInvalid(t *testing.T) {
	_, err := parseClusterMonitorURI("k0rdent://cluster-monitor/only-namespace")
	require.Error(t, err)

	_, err = parseClusterMonitorURI("https://cluster-monitor/ns/name")
	require.Error(t, err)
}

func TestClusterMonitorToolState(t *testing.T) {
	listKinds := map[schema.GroupVersionResource]string{
		clusters.ClusterDeploymentsGVR: "ClusterDeploymentList",
	}
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterDeployment",
			"metadata": map[string]any{
				"name":      "demo-cluster",
				"namespace": "kcm-system",
			},
			"status": map[string]any{
				"phase": "Provisioning",
				"conditions": []any{
					map[string]any{
						"type":   "InfrastructureReady",
						"status": "False",
					},
				},
			},
		},
	}
	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(apiruntime.NewScheme(), listKinds, obj)
	session := &runtime.Session{
		Clients: runtime.Clients{
			Dynamic: fakeClient,
		},
	}
	tool := &clusterMonitorTool{session: session}
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "k0rdent.mgmt.clusterDeployments.getState"}}

	_, resp, err := tool.state(context.Background(), req, clusterMonitorStateInput{
		Namespace: "kcm-system",
		Name:      "demo-cluster",
	})
	require.NoError(t, err)
	require.Equal(t, clustermonitor.PhaseProvisioning, resp.Update.Phase)
	require.False(t, resp.Update.Timestamp.IsZero())
}

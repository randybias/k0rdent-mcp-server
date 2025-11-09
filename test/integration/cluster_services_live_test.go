//go:build live

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0rdent/mcp-k0rdent-server/internal/k0rdent/api"
)

const (
	envLiveServiceClusterNamespace  = "K0RDENT_LIVE_SERVICE_CLUSTER_NAMESPACE"
	envLiveServiceClusterName       = "K0RDENT_LIVE_SERVICE_CLUSTER_NAME"
	envLiveServiceTemplateNamespace = "K0RDENT_LIVE_SERVICE_TEMPLATE_NAMESPACE"
	envLiveServiceTemplateName      = "K0RDENT_LIVE_SERVICE_TEMPLATE_NAME"
)

type clusterServiceApplyLiveResult struct {
	Service map[string]any `json:"service"`
	Status  map[string]any `json:"status"`
}

func TestClusterServiceApplyLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	clusterNS := os.Getenv(envLiveServiceClusterNamespace)
	clusterName := os.Getenv(envLiveServiceClusterName)
	templateNS := os.Getenv(envLiveServiceTemplateNamespace)
	templateName := os.Getenv(envLiveServiceTemplateName)
	if clusterNS == "" || clusterName == "" || templateNS == "" || templateName == "" {
		t.Skipf("set %s, %s, %s, and %s to run this test",
			envLiveServiceClusterNamespace, envLiveServiceClusterName,
			envLiveServiceTemplateNamespace, envLiveServiceTemplateName)
	}

	kubeconfig, _ := requireLiveEnv(t)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("load kubeconfig: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("create dynamic client: %v", err)
	}

	client := newLiveClient(t)
	serviceName := fmt.Sprintf("svc-%d", time.Now().UnixNano())

	args := map[string]any{
		"clusterNamespace":  clusterNS,
		"clusterName":       clusterName,
		"templateNamespace": templateNS,
		"templateName":      templateName,
		"serviceName":       serviceName,
		"values": map[string]any{
			"replicaCount": 1,
		},
		"dryRun": true,
	}

	t.Log("Performing dry-run preview...")
	client.CallTool(t, "k0rdent.mgmt.clusterDeployments.services.apply", args)

	args["dryRun"] = false

	t.Logf("Applying service template %s/%s as %s", templateNS, templateName, serviceName)
	raw := client.CallTool(t, "k0rdent.mgmt.clusterDeployments.services.apply", args)

	var applyResult clusterServiceApplyLiveResult
	if err := json.Unmarshal(raw, &applyResult); err != nil {
		t.Fatalf("decode apply result: %v", err)
	}
	if applyResult.Service["name"] != serviceName {
		t.Fatalf("unexpected service in response: %#v", applyResult.Service["name"])
	}

	t.Log("Waiting for service to reach Deployed state...")
	waitForServiceState(t, dynamicClient, clusterNS, clusterName, serviceName, "Deployed", 10*time.Minute)
}

func waitForServiceState(t *testing.T, client dynamic.Interface, namespace, clusterName, serviceName, desiredState string, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 15*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		obj, err := client.
			Resource(api.ClusterDeploymentGVR()).
			Namespace(namespace).
			Get(ctx, clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		entries, found, err := unstructured.NestedSlice(obj.Object, "status", "services")
		if err != nil || !found {
			return false, nil
		}
		for _, entry := range entries {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			if name != serviceName {
				continue
			}
			state, _ := m["state"].(string)
			t.Logf("service %s current state=%s", serviceName, state)
			if state == desiredState {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("service %s did not reach %s: %v", serviceName, desiredState, err)
	}
}

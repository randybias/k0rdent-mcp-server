package core

import (
	"context"
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k0rdent/mcp-k0rdent-server/internal/k0rdent/api"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
	testdynamic "github.com/k0rdent/mcp-k0rdent-server/internal/testutil/dynamic"
)

func TestClusterServiceApplyAddsService(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", nil, nil))
	client.Add(api.ServiceTemplateGVR(), newServiceTemplateObject("kcm-system", "minio-1-0-0"))

	tool := &clusterServiceApplyTool{
		session: &runtime.Session{
			Clients: runtime.Clients{Dynamic: client},
		},
	}

	input := clusterServiceApplyInput{
		ClusterNamespace:  "tenant-a",
		ClusterName:       "dev-cluster",
		TemplateNamespace: "kcm-system",
		TemplateName:      "minio-1-0-0",
		ServiceName:       "minio",
		Values: map[string]any{
			"replicaCount": 1,
		},
	}

	_, result, err := tool.apply(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	if result.Service["name"] != "minio" {
		t.Fatalf("expected service name minio, got %#v", result.Service["name"])
	}

	obj, ok := client.GetObject(api.ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found after apply")
	}
	list, _, _ := unstructured.NestedSlice(obj.Object, "spec", "serviceSpec", "services")
	if len(list) != 1 {
		t.Fatalf("expected 1 service entry, got %d", len(list))
	}
	entry, _ := list[0].(map[string]any)
	if entry["template"] != "minio-1-0-0" {
		t.Fatalf("template mismatch: %#v", entry["template"])
	}
	if entry["values"] != "replicaCount: 1\n" {
		t.Fatalf("values not encoded as yaml: %#v", entry["values"])
	}
}

func TestClusterServiceApplyDryRun(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", nil, nil))
	client.Add(api.ServiceTemplateGVR(), newServiceTemplateObject("kcm-system", "logging-1-0-0"))

	tool := &clusterServiceApplyTool{
		session: &runtime.Session{
			Clients: runtime.Clients{Dynamic: client},
		},
	}

	input := clusterServiceApplyInput{
		ClusterNamespace:  "tenant-a",
		ClusterName:       "dev-cluster",
		TemplateNamespace: "kcm-system",
		TemplateName:      "logging-1-0-0",
		ServiceName:       "logging",
		DryRun:            true,
	}

	if _, _, err := tool.apply(context.Background(), nil, input); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	obj, _ := client.GetObject(api.ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	list, _, _ := unstructured.NestedSlice(obj.Object, "spec", "serviceSpec", "services")
	if len(list) != 0 {
		t.Fatalf("expected no services persisted during dry-run, got %d", len(list))
	}
}

func TestClusterServiceApplyNamespaceFilter(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", nil, nil))
	client.Add(api.ServiceTemplateGVR(), newServiceTemplateObject("kcm-system", "logging-1-0-0"))

	tool := &clusterServiceApplyTool{
		session: &runtime.Session{
			NamespaceFilter: regexp.MustCompile("^team-"),
			Clients:         runtime.Clients{Dynamic: client},
		},
	}

	input := clusterServiceApplyInput{
		ClusterNamespace:  "tenant-a",
		ClusterName:       "dev-cluster",
		TemplateNamespace: "kcm-system",
		TemplateName:      "logging-1-0-0",
		ServiceName:       "logging",
	}

	if _, _, err := tool.apply(context.Background(), nil, input); err == nil {
		t.Fatalf("expected namespace filter error")
	}
}

func TestClusterServiceApplyTemplateNotFound(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", nil, nil))

	tool := &clusterServiceApplyTool{
		session: &runtime.Session{
			Clients: runtime.Clients{Dynamic: client},
		},
	}

	input := clusterServiceApplyInput{
		ClusterNamespace:  "tenant-a",
		ClusterName:       "dev-cluster",
		TemplateNamespace: "kcm-system",
		TemplateName:      "missing-template",
		ServiceName:       "missing",
	}

	if _, _, err := tool.apply(context.Background(), nil, input); err == nil {
		t.Fatalf("expected error for missing template")
	}
}

func newClusterObject(namespace, name string, services []map[string]any, status []map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterDeployment",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
	if services != nil {
		slice := make([]any, len(services))
		for i, svc := range services {
			slice[i] = svc
		}
		_ = unstructured.SetNestedField(obj.Object, map[string]any{
			"services": slice,
		}, "spec", "serviceSpec")
	}
	if status != nil {
		slice := make([]any, len(status))
		for i, svc := range status {
			slice[i] = svc
		}
		_ = unstructured.SetNestedSlice(obj.Object, slice, "status", "services")
	}
	return obj
}

func newServiceTemplateObject(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ServiceTemplate",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

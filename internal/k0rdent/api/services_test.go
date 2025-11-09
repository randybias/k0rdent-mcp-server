package api

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	testdynamic "github.com/k0rdent/mcp-k0rdent-server/internal/testutil/dynamic"
)

func TestApplyClusterServiceAddsEntry(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	client.Add(ClusterDeploymentGVR(), newClusterDeployment("tenant-a", "dev-cluster", nil, nil))

	values := "replicaCount: 2\n"
	opts := ApplyClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		Service: ClusterServiceApplySpec{
			TemplateNamespace: "kcm-system",
			TemplateName:      "minio-1-0-0",
			ServiceName:       "minio",
		},
	}
	opts.Service.Values = &values

	result, err := ApplyClusterService(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("ApplyClusterService returned error: %v", err)
	}

	if result.Service["name"] != "minio" {
		t.Fatalf("expected service name minio, got %#v", result.Service["name"])
	}
	if result.Service["template"] != "minio-1-0-0" {
		t.Fatalf("expected template reference, got %#v", result.Service["template"])
	}

	obj, ok := client.GetObject(ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found after apply")
	}
	list := extractServiceEntries(obj)
	if len(list) != 1 {
		t.Fatalf("expected 1 service after apply, got %d", len(list))
	}
	entry := list[0]
	if entry["name"] != "minio" {
		t.Fatalf("unexpected service entry %#v", entry)
	}
	if entry["values"] != values {
		t.Fatalf("values not applied: %#v", entry["values"])
	}
}

func TestApplyClusterServiceUpdatesExistingEntry(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	existing := map[string]any{
		"name":      "ingress",
		"namespace": "tenant-a",
		"template":  "ingress-1-0-0",
		"values": map[string]any{
			"replicaCount": 1,
		},
	}
	client.Add(ClusterDeploymentGVR(), newClusterDeployment("tenant-a", "dev-cluster", []map[string]any{existing}, nil))

	values := "replicaCount: 3\n"
	opts := ApplyClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		Service: ClusterServiceApplySpec{
			TemplateNamespace: "kcm-system",
			TemplateName:      "ingress-1-1-0",
			ServiceName:       "ingress",
		},
	}
	opts.Service.Values = &values

	result, err := ApplyClusterService(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("ApplyClusterService returned error: %v", err)
	}
	if result.Service["template"] != "ingress-1-1-0" {
		t.Fatalf("expected template update, got %#v", result.Service["template"])
	}

	obj, _ := client.GetObject(ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	list := extractServiceEntries(obj)
	entry := list[0]
	if entry["template"] != "ingress-1-1-0" {
		t.Fatalf("template not updated: %#v", entry["template"])
	}
	if entry["values"] != values {
		t.Fatalf("values not updated: %#v", entry["values"])
	}
}

func TestApplyClusterServiceDryRunDoesNotPersist(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	client.Add(ClusterDeploymentGVR(), newClusterDeployment("tenant-a", "dev-cluster", nil, nil))

	opts := ApplyClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		DryRun:           true,
		Service: ClusterServiceApplySpec{
			TemplateNamespace: "kcm-system",
			TemplateName:      "logging-1-0-0",
			ServiceName:       "logging",
		},
	}

	if _, err := ApplyClusterService(context.Background(), client, opts); err != nil {
		t.Fatalf("ApplyClusterService returned error: %v", err)
	}

	obj, _ := client.GetObject(ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if services := extractServiceEntries(obj); len(services) != 0 {
		t.Fatalf("expected no services persisted during dry-run, got %d", len(services))
	}
}

func newClusterDeployment(namespace, name string, services []map[string]any, status []map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterDeployment",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec":   map[string]any{},
			"status": map[string]any{},
		},
	}
	if len(services) > 0 {
		slice := make([]any, len(services))
		for i, svc := range services {
			slice[i] = svc
		}
		obj.Object["spec"].(map[string]any)["serviceSpec"] = map[string]any{
			"services": slice,
		}
	}
	if len(status) > 0 {
		slice := make([]any, len(status))
		for i, svc := range status {
			slice[i] = svc
		}
		obj.Object["status"].(map[string]any)["services"] = slice
	}
	return obj
}

func extractServiceEntries(obj *unstructured.Unstructured) []map[string]any {
	spec, _ := obj.Object["spec"].(map[string]any)
	serviceSpec, _ := spec["serviceSpec"].(map[string]any)
	raw, _ := serviceSpec["services"].([]any)
	entries := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		if m, ok := entry.(map[string]any); ok {
			entries = append(entries, m)
		}
	}
	return entries
}

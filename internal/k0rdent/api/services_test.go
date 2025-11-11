package api

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

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

func TestRemoveClusterServiceFromMultiServiceCluster(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":      "minio",
			"namespace": "tenant-a",
			"template":  "minio-1-0-0",
		},
		{
			"name":      "ingress",
			"namespace": "tenant-a",
			"template":  "ingress-1-0-0",
		},
		{
			"name":      "logging",
			"namespace": "tenant-a",
			"template":  "logging-1-0-0",
		},
	}
	client.Add(ClusterDeploymentGVR(), newClusterDeployment("tenant-a", "dev-cluster", services, nil))

	opts := RemoveClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "ingress",
	}

	result, err := RemoveClusterService(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("RemoveClusterService returned error: %v", err)
	}

	if result.RemovedService == nil {
		t.Fatalf("expected removed service to be returned, got nil")
	}
	if result.RemovedService["name"] != "ingress" {
		t.Fatalf("expected removed service name to be 'ingress', got %#v", result.RemovedService["name"])
	}
	if result.Message != "service removed successfully" {
		t.Fatalf("unexpected message: %s", result.Message)
	}

	obj, ok := client.GetObject(ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found after removal")
	}
	list := extractServiceEntries(obj)
	if len(list) != 2 {
		t.Fatalf("expected 2 services after removal, got %d", len(list))
	}

	foundMinio := false
	foundLogging := false
	for _, entry := range list {
		name := entry["name"].(string)
		if name == "minio" {
			foundMinio = true
		} else if name == "logging" {
			foundLogging = true
		} else if name == "ingress" {
			t.Fatalf("ingress should have been removed but was found in services list")
		}
	}
	if !foundMinio || !foundLogging {
		t.Fatalf("expected minio and logging to remain, got services: %v", list)
	}
}

func TestRemoveOnlyService(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":      "minio",
			"namespace": "tenant-a",
			"template":  "minio-1-0-0",
		},
	}
	client.Add(ClusterDeploymentGVR(), newClusterDeployment("tenant-a", "dev-cluster", services, nil))

	opts := RemoveClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "minio",
	}

	result, err := RemoveClusterService(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("RemoveClusterService returned error: %v", err)
	}

	if result.RemovedService == nil {
		t.Fatalf("expected removed service to be returned, got nil")
	}
	if result.RemovedService["name"] != "minio" {
		t.Fatalf("expected removed service name to be 'minio', got %#v", result.RemovedService["name"])
	}
	if result.Message != "service removed successfully" {
		t.Fatalf("unexpected message: %s", result.Message)
	}

	obj, ok := client.GetObject(ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found after removal")
	}
	list := extractServiceEntries(obj)
	if len(list) != 0 {
		t.Fatalf("expected empty services array after removing only service, got %d services", len(list))
	}
}

func TestRemoveNonexistentService(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":      "minio",
			"namespace": "tenant-a",
			"template":  "minio-1-0-0",
		},
	}
	client.Add(ClusterDeploymentGVR(), newClusterDeployment("tenant-a", "dev-cluster", services, nil))

	opts := RemoveClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "nonexistent",
	}

	result, err := RemoveClusterService(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("RemoveClusterService returned error: %v", err)
	}

	if result.RemovedService != nil {
		t.Fatalf("expected removed service to be nil for nonexistent service, got %#v", result.RemovedService)
	}
	if result.Message != "service not found (already removed)" {
		t.Fatalf("unexpected message: %s", result.Message)
	}

	obj, ok := client.GetObject(ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found after removal attempt")
	}
	list := extractServiceEntries(obj)
	if len(list) != 1 {
		t.Fatalf("expected original service to remain, got %d services", len(list))
	}
	if list[0]["name"] != "minio" {
		t.Fatalf("expected minio to remain, got %#v", list[0]["name"])
	}
}

func TestRemoveDryRun(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":      "minio",
			"namespace": "tenant-a",
			"template":  "minio-1-0-0",
		},
		{
			"name":      "ingress",
			"namespace": "tenant-a",
			"template":  "ingress-1-0-0",
		},
	}
	client.Add(ClusterDeploymentGVR(), newClusterDeployment("tenant-a", "dev-cluster", services, nil))

	opts := RemoveClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "ingress",
		DryRun:           true,
	}

	result, err := RemoveClusterService(context.Background(), client, opts)
	if err != nil {
		t.Fatalf("RemoveClusterService returned error: %v", err)
	}

	if result.RemovedService == nil {
		t.Fatalf("expected removed service to be returned in dry-run, got nil")
	}
	if result.RemovedService["name"] != "ingress" {
		t.Fatalf("expected removed service name to be 'ingress', got %#v", result.RemovedService["name"])
	}
	if result.Message != "service removed successfully" {
		t.Fatalf("unexpected message: %s", result.Message)
	}

	obj, ok := client.GetObject(ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found after dry-run removal")
	}
	list := extractServiceEntries(obj)
	if len(list) != 2 {
		t.Fatalf("expected services unchanged during dry-run, got %d services", len(list))
	}

	foundMinio := false
	foundIngress := false
	for _, entry := range list {
		name := entry["name"].(string)
		if name == "minio" {
			foundMinio = true
		} else if name == "ingress" {
			foundIngress = true
		}
	}
	if !foundMinio || !foundIngress {
		t.Fatalf("expected both services to remain unchanged during dry-run, got services: %v", list)
	}
}

func TestRemoveClusterDeploymentNotFound(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()

	opts := RemoveClusterServiceOptions{
		ClusterNamespace: "tenant-a",
		ClusterName:      "nonexistent-cluster",
		ServiceName:      "minio",
	}

	_, err := RemoveClusterService(context.Background(), client, opts)
	if err == nil {
		t.Fatalf("expected error for nonexistent ClusterDeployment, got nil")
	}

	expectedError := "get cluster deployment"
	if !contains(err.Error(), expectedError) {
		t.Fatalf("expected error to contain '%s', got: %v", expectedError, err)
	}
}

func TestRemoveRequiredFields(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()

	tests := []struct {
		name        string
		opts        RemoveClusterServiceOptions
		expectedErr string
	}{
		{
			name: "nil client",
			opts: RemoveClusterServiceOptions{
				ClusterNamespace: "tenant-a",
				ClusterName:      "dev-cluster",
				ServiceName:      "minio",
			},
			expectedErr: "dynamic client is required",
		},
		{
			name: "missing cluster namespace",
			opts: RemoveClusterServiceOptions{
				ClusterNamespace: "",
				ClusterName:      "dev-cluster",
				ServiceName:      "minio",
			},
			expectedErr: "cluster namespace is required",
		},
		{
			name: "missing cluster name",
			opts: RemoveClusterServiceOptions{
				ClusterNamespace: "tenant-a",
				ClusterName:      "",
				ServiceName:      "minio",
			},
			expectedErr: "cluster name is required",
		},
		{
			name: "missing service name",
			opts: RemoveClusterServiceOptions{
				ClusterNamespace: "tenant-a",
				ClusterName:      "dev-cluster",
				ServiceName:      "",
			},
			expectedErr: "service name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testClient dynamic.Interface
			if tt.expectedErr == "dynamic client is required" {
				testClient = nil
			} else {
				testClient = client
			}

			_, err := RemoveClusterService(context.Background(), testClient, tt.opts)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
			if !contains(err.Error(), tt.expectedErr) {
				t.Fatalf("expected error to contain '%s', got: %v", tt.expectedErr, err)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

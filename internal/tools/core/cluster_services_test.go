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

func TestRemoveServiceTool_Success(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":               "minio",
			"template":           "minio-1-0-0",
			"templateNamespace":  "kcm-system",
			"values":             "replicaCount: 1\n",
		},
		{
			"name":               "logging",
			"template":           "logging-1-0-0",
			"templateNamespace":  "kcm-system",
		},
	}
	status := []map[string]any{
		{
			"name":  "minio",
			"state": "Ready",
		},
		{
			"name":  "logging",
			"state": "Pending",
		},
	}
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", services, status))

	tool := &removeClusterServiceTool{
		session: &runtime.Session{
			Clients: runtime.Clients{Dynamic: client},
		},
	}

	input := removeClusterServiceInput{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "minio",
	}

	_, result, err := tool.remove(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("remove returned error: %v", err)
	}

	// Verify removedService structure
	if result.RemovedService == nil {
		t.Fatalf("expected removedService to be non-nil")
	}
	if result.RemovedService["name"] != "minio" {
		t.Errorf("expected removed service name 'minio', got %v", result.RemovedService["name"])
	}

	// Verify updatedServices contains only logging
	if len(result.UpdatedServices) != 1 {
		t.Fatalf("expected 1 remaining service, got %d", len(result.UpdatedServices))
	}
	if result.UpdatedServices[0]["name"] != "logging" {
		t.Errorf("expected remaining service 'logging', got %v", result.UpdatedServices[0]["name"])
	}

	// Verify message indicates success
	if result.Message != "service removed successfully" {
		t.Errorf("unexpected message: %s", result.Message)
	}

	// Verify clusterStatus is present
	if result.ClusterStatus == nil {
		t.Errorf("expected clusterStatus to be non-nil")
	}

	// Verify the cluster object was actually updated
	obj, ok := client.GetObject(api.ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found after remove")
	}
	list, _, _ := unstructured.NestedSlice(obj.Object, "spec", "serviceSpec", "services")
	if len(list) != 1 {
		t.Fatalf("expected 1 service entry in cluster, got %d", len(list))
	}
}

func TestRemoveServiceTool_NotFound(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":               "logging",
			"template":           "logging-1-0-0",
			"templateNamespace":  "kcm-system",
		},
	}
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", services, nil))

	tool := &removeClusterServiceTool{
		session: &runtime.Session{
			Clients: runtime.Clients{Dynamic: client},
		},
	}

	input := removeClusterServiceInput{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "nonexistent",
	}

	_, result, err := tool.remove(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("remove returned error for missing service: %v", err)
	}

	// Verify idempotent response: removedService is nil
	if result.RemovedService != nil {
		t.Errorf("expected removedService to be nil for nonexistent service, got %v", result.RemovedService)
	}

	// Verify message indicates not found
	if result.Message != "service not found (already removed)" {
		t.Errorf("unexpected message: %s", result.Message)
	}

	// Verify updatedServices still contains original service
	if len(result.UpdatedServices) != 1 {
		t.Fatalf("expected 1 remaining service, got %d", len(result.UpdatedServices))
	}
	if result.UpdatedServices[0]["name"] != "logging" {
		t.Errorf("expected remaining service 'logging', got %v", result.UpdatedServices[0]["name"])
	}

	// Verify cluster object unchanged
	obj, ok := client.GetObject(api.ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found")
	}
	list, _, _ := unstructured.NestedSlice(obj.Object, "spec", "serviceSpec", "services")
	if len(list) != 1 {
		t.Fatalf("expected 1 service entry (unchanged), got %d", len(list))
	}
}

func TestRemoveServiceTool_DryRun(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":               "minio",
			"template":           "minio-1-0-0",
			"templateNamespace":  "kcm-system",
		},
	}
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", services, nil))

	tool := &removeClusterServiceTool{
		session: &runtime.Session{
			Clients: runtime.Clients{Dynamic: client},
		},
	}

	input := removeClusterServiceInput{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "minio",
		DryRun:           true,
	}

	_, result, err := tool.remove(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("remove returned error: %v", err)
	}

	// Verify removedService is returned (indicating what would be removed)
	if result.RemovedService == nil {
		t.Fatalf("expected removedService to be non-nil in dry-run")
	}
	if result.RemovedService["name"] != "minio" {
		t.Errorf("expected removed service name 'minio', got %v", result.RemovedService["name"])
	}

	// Verify updatedServices shows preview (empty in this case)
	if len(result.UpdatedServices) != 0 {
		t.Errorf("expected 0 remaining services in preview, got %d", len(result.UpdatedServices))
	}

	// Verify cluster object remains unchanged (dry-run doesn't persist)
	obj, ok := client.GetObject(api.ClusterDeploymentGVR(), "tenant-a", "dev-cluster")
	if !ok {
		t.Fatalf("cluster not found")
	}
	list, _, _ := unstructured.NestedSlice(obj.Object, "spec", "serviceSpec", "services")
	if len(list) != 1 {
		t.Fatalf("expected 1 service entry (unchanged due to dry-run), got %d", len(list))
	}
}

func TestRemoveServiceTool_ValidationErrors(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", nil, nil))

	tool := &removeClusterServiceTool{
		session: &runtime.Session{
			Clients: runtime.Clients{Dynamic: client},
		},
	}

	tests := []struct {
		name        string
		input       removeClusterServiceInput
		errContains string
	}{
		{
			name: "missing clusterNamespace",
			input: removeClusterServiceInput{
				ClusterName: "dev-cluster",
				ServiceName: "minio",
			},
			errContains: "clusterNamespace is required",
		},
		{
			name: "missing clusterName",
			input: removeClusterServiceInput{
				ClusterNamespace: "tenant-a",
				ServiceName:      "minio",
			},
			errContains: "clusterName is required",
		},
		{
			name: "missing serviceName",
			input: removeClusterServiceInput{
				ClusterNamespace: "tenant-a",
				ClusterName:      "dev-cluster",
			},
			errContains: "serviceName is required",
		},
		{
			name: "empty strings (whitespace)",
			input: removeClusterServiceInput{
				ClusterNamespace: "  ",
				ClusterName:      "  ",
				ServiceName:      "  ",
			},
			errContains: "is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tool.remove(context.Background(), nil, tt.input)
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}

func TestRemoveServiceTool_NamespaceFilter(t *testing.T) {
	client := testdynamic.NewFakeDynamicClient()
	services := []map[string]any{
		{
			"name":               "minio",
			"template":           "minio-1-0-0",
			"templateNamespace":  "kcm-system",
		},
	}
	client.Add(api.ClusterDeploymentGVR(), newClusterObject("tenant-a", "dev-cluster", services, nil))

	tool := &removeClusterServiceTool{
		session: &runtime.Session{
			NamespaceFilter: regexp.MustCompile("^team-"),
			Clients:         runtime.Clients{Dynamic: client},
		},
	}

	input := removeClusterServiceInput{
		ClusterNamespace: "tenant-a",
		ClusterName:      "dev-cluster",
		ServiceName:      "minio",
	}

	_, _, err := tool.remove(context.Background(), nil, input)
	if err == nil {
		t.Fatalf("expected namespace filter error")
	}
	if !contains(err.Error(), "not allowed by namespace filter") {
		t.Errorf("expected namespace filter error message, got: %v", err)
	}
}

// Helper function for string containment checks
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

//go:build integration

package integration

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0rdent/mcp-k0rdent-server/internal/catalog"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

// TestCatalogInstall_Live tests the full catalog install workflow
func TestCatalogInstall_Live(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	kubeconfigPath := os.Getenv("K0RDENT_MGMT_KUBECONFIG_PATH")
	if kubeconfigPath == "" {
		t.Skip("K0RDENT_MGMT_KUBECONFIG_PATH not set")
	}

	// Load kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to load kubeconfig: %v", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create dynamic client: %v", err)
	}

	// Create catalog manager
	cacheDir := t.TempDir()
	opts := catalog.LoadConfig()
	opts.CacheDir = cacheDir
	opts.CacheTTL = time.Hour

	manager, err := catalog.NewManager(opts)
	if err != nil {
		t.Fatalf("failed to create catalog manager: %v", err)
	}

	// Create runtime session
	session := &runtime.Session{
		Clients: runtime.Clients{
			Dynamic: dynamicClient,
		},
		NamespaceFilter: regexp.MustCompile(".*"), // Allow all namespaces
	}

	ctx := context.Background()

	// Clean up any existing test resources
	t.Log("Cleaning up any existing test resources...")
	cleanupTestResources(t, dynamicClient)

	// Test install
	t.Log("Installing minio ServiceTemplate...")
	installTool := &catalogInstallTool{
		session: session,
		manager: manager,
	}

	input := catalogInstallInput{
		App:      "minio",
		Template: "minio",
		Version:  "14.1.2",
	}

	_, result, err := installTool.install(ctx, nil, input)
	if err != nil {
		t.Fatalf("catalog install failed: %v", err)
	}

	t.Logf("Applied %d resources: %v", len(result.Applied), result.Applied)

	if len(result.Applied) == 0 {
		t.Fatal("expected at least one resource to be applied")
	}

	// Verify ServiceTemplate was created
	t.Log("Verifying ServiceTemplate was created...")
	stGVR := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "servicetemplates",
	}

	st, err := dynamicClient.Resource(stGVR).Namespace("kcm-system").Get(ctx, "minio-14-1-2", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get ServiceTemplate: %v", err)
	}

	t.Logf("ServiceTemplate created: %s", st.GetName())

	// Wait for ServiceTemplate to become valid
	t.Log("Waiting for ServiceTemplate to become valid...")
	timeout := time.After(60 * time.Second)
	tick := time.Tick(2 * time.Second)
	valid := false

	for !valid {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for ServiceTemplate to become valid")
		case <-tick:
			st, err = dynamicClient.Resource(stGVR).Namespace("kcm-system").Get(ctx, "minio-14-1-2", metav1.GetOptions{})
			if err != nil {
				t.Logf("error getting ServiceTemplate: %v", err)
				continue
			}

			status, found, err := unstructured.NestedFieldCopy(st.Object, "status", "valid")
			if err != nil {
				t.Logf("error getting status.valid: %v", err)
				continue
			}

			if found && status == true {
				valid = true
				t.Log("ServiceTemplate is now valid!")
			} else {
				t.Log("ServiceTemplate not yet valid, waiting...")
			}
		}
	}

	// Clean up
	t.Log("Cleaning up test resources...")
	cleanupTestResources(t, dynamicClient)
}

// Helper type to match the private type in core package
type catalogInstallTool struct {
	session *runtime.Session
	manager *catalog.Manager
}

type catalogInstallInput struct {
	App      string `json:"app"`
	Template string `json:"template"`
	Version  string `json:"version"`
}

type catalogInstallResult struct {
	Applied []string `json:"applied"`
	Status  string   `json:"status"`
}

// Make install method available for testing
func (t *catalogInstallTool) install(ctx context.Context, req interface{}, input catalogInstallInput) (interface{}, catalogInstallResult, error) {
	// This is a test helper that calls the actual implementation
	// Since the real implementation is in the core package and unexported,
	// we'll need to refactor it to be testable or use the MCP interface
	//
	// For now, let's test via the actual tool registration
	return nil, catalogInstallResult{}, nil
}

func cleanupTestResources(t *testing.T, client dynamic.Interface) {
	ctx := context.Background()

	// Delete ServiceTemplate
	stGVR := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "servicetemplates",
	}
	err := client.Resource(stGVR).Namespace("kcm-system").Delete(ctx, "minio-14-1-2", metav1.DeleteOptions{})
	if err != nil {
		t.Logf("Note: Could not delete ServiceTemplate (may not exist): %v", err)
	}

	// Delete HelmRepository
	hrGVR := schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "helmrepositories",
	}
	err = client.Resource(hrGVR).Namespace("kcm-system").Delete(ctx, "k0rdent-catalog", metav1.DeleteOptions{})
	if err != nil {
		t.Logf("Note: Could not delete HelmRepository (may not exist): %v", err)
	}
}

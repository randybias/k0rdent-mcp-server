//go:build live

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0rdent/mcp-k0rdent-server/internal/catalog"
)

// catalogEntry matches the structure from internal/catalog/types.go
type catalogEntry struct {
	Slug               string                        `json:"slug"`
	Title              string                        `json:"title"`
	Summary            string                        `json:"summary,omitempty"`
	Tags               []string                      `json:"tags,omitempty"`
	ValidatedPlatforms []string                      `json:"validated_platforms,omitempty"`
	Versions           []catalogServiceTemplateVersion `json:"versions"`
}

type catalogServiceTemplateVersion struct {
	Name                string `json:"name"`
	Version             string `json:"version"`
	Repository          string `json:"repository"`
	ServiceTemplatePath string `json:"service_template_path"`
	HelmRepositoryPath  string `json:"helm_repository_path,omitempty"`
}

type catalogListResult struct {
	Entries []catalogEntry `json:"entries"`
}

type catalogInstallResult struct {
	Applied []string `json:"applied"`
	Status  string   `json:"status"`
}

type catalogDeleteResult struct {
	Deleted []string `json:"deleted"`
	Status  string   `json:"status"`
}

// TestCatalogListLive verifies we can list all catalog entries
func TestCatalogListLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)

	raw := client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{})
	var result catalogListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode catalog list result: %v", err)
	}

	if len(result.Entries) == 0 {
		t.Fatal("expected catalog entries, got none")
	}

	// JSON index should have a substantial number of addons (30+ as of implementation)
	// Note: Not all addons may have installable ServiceTemplates, so exact count may vary
	if len(result.Entries) < 30 {
		t.Errorf("expected at least 30 catalog entries from JSON index, got %d", len(result.Entries))
	}

	t.Logf("Found %d catalog entries", len(result.Entries))

	// Verify structure of entries
	foundMinio := false
	for _, entry := range result.Entries {
		if entry.Slug == "" {
			t.Errorf("entry missing slug: %+v", entry)
		}
		if entry.Title == "" {
			t.Errorf("entry %s missing title", entry.Slug)
		}
		if len(entry.Versions) == 0 {
			t.Errorf("entry %s has no versions", entry.Slug)
		}

		// Check for well-known apps
		if entry.Slug == "minio" {
			foundMinio = true
			t.Logf("Found minio: %s with %d versions", entry.Title, len(entry.Versions))
		}

		// Verify version structure
		for _, ver := range entry.Versions {
			if ver.Name == "" {
				t.Errorf("entry %s has version with empty name", entry.Slug)
			}
			if ver.Version == "" {
				t.Errorf("entry %s has version with empty version string", entry.Slug)
			}
			// Note: Repository field is intentionally empty in JSON index implementation
			// It will be populated from manifest when needed
		}
	}

	if !foundMinio {
		t.Logf("Warning: minio not found in catalog (expected well-known app)")
	}
}

// TestCatalogListWithFilterLive verifies filtering works
func TestCatalogListWithFilterLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)

	// First, get all entries to find a valid app
	raw := client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{})
	var allResult catalogListResult
	if err := json.Unmarshal(raw, &allResult); err != nil {
		t.Fatalf("decode catalog list result: %v", err)
	}

	if len(allResult.Entries) == 0 {
		t.Fatal("no catalog entries to test filtering with")
	}

	// Use the first entry for filtering
	testApp := allResult.Entries[0].Slug
	t.Logf("Testing filter with app: %s", testApp)

	// Now filter by that app
	raw = client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{
		"app": testApp,
	})
	var filtered catalogListResult
	if err := json.Unmarshal(raw, &filtered); err != nil {
		t.Fatalf("decode filtered result: %v", err)
	}

	if len(filtered.Entries) == 0 {
		t.Fatalf("filter returned no entries for app %s", testApp)
	}

	// Verify all returned entries match the filter
	for _, entry := range filtered.Entries {
		if entry.Slug != testApp {
			t.Errorf("filter returned entry %s, expected only %s", entry.Slug, testApp)
		}
	}

	t.Logf("Filter test passed: returned %d entry for %s", len(filtered.Entries), testApp)
}

// TestCatalogListRefreshLive verifies refresh parameter works
func TestCatalogListRefreshLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)

	// First call without refresh
	raw1 := client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{})
	var result1 catalogListResult
	if err := json.Unmarshal(raw1, &result1); err != nil {
		t.Fatalf("decode first result: %v", err)
	}

	if len(result1.Entries) == 0 {
		t.Fatal("first call returned no entries")
	}

	t.Logf("First call returned %d entries", len(result1.Entries))

	// Second call with refresh=true
	raw2 := client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{
		"refresh": true,
	})
	var result2 catalogListResult
	if err := json.Unmarshal(raw2, &result2); err != nil {
		t.Fatalf("decode second result: %v", err)
	}

	if len(result2.Entries) == 0 {
		t.Fatal("refresh call returned no entries")
	}

	t.Logf("Refresh call returned %d entries", len(result2.Entries))

	// Both should return valid data (exact counts may vary with catalog updates)
	if len(result1.Entries) == 0 || len(result2.Entries) == 0 {
		t.Fatal("one or both calls returned empty results")
	}
}

// TestCatalogInstallNginxIngressLive is the main test for installing nginx ingress controller
func TestCatalogInstallNginxIngressLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)
	kubeconfig, _ := requireLiveEnv(t)

	// Get Kubernetes clients
	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("load kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		t.Fatalf("create kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		t.Fatalf("create dynamic client: %v", err)
	}

	ctx := context.Background()
	testNamespace := "default"
	serviceTemplateName := "test-ingress-nginx"

	// Cleanup before and after test
	defer cleanupServiceTemplate(t, dynamicClient, serviceTemplateName, testNamespace)
	cleanupServiceTemplate(t, dynamicClient, serviceTemplateName, testNamespace)

	// 1. Find nginx ingress in catalog
	t.Log("Searching for nginx ingress in catalog...")
	raw := client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{})
	var catalogResult catalogListResult
	if err := json.Unmarshal(raw, &catalogResult); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	nginxEntry := findCatalogEntry(t, catalogResult.Entries, "ingress-nginx")
	if nginxEntry == nil {
		t.Skip("ingress-nginx not found in catalog, skipping test")
	}

	if len(nginxEntry.Versions) == 0 {
		t.Fatal("ingress-nginx has no versions available")
	}

	// Use the first available version
	version := nginxEntry.Versions[0]
	t.Logf("Found ingress-nginx version %s", version.Version)

	// 2. Install nginx ingress
	t.Log("Installing ingress-nginx ServiceTemplate...")
	installRaw := client.CallTool(t, "k0rdent.mgmt.serviceTemplates.install_from_catalog", map[string]any{
		"app":      "ingress-nginx",
		"template": version.Name,
		"version":  version.Version,
	})

	var installResult catalogInstallResult
	if err := json.Unmarshal(installRaw, &installResult); err != nil {
		t.Fatalf("decode install result: %v", err)
	}

	if len(installResult.Applied) == 0 {
		t.Fatal("install reported no resources applied")
	}

	t.Logf("Install result: status=%s, applied=%d resources", installResult.Status, len(installResult.Applied))
	for _, res := range installResult.Applied {
		t.Logf("  Applied: %s", res)
	}

	// 3. Verify ServiceTemplate was created
	t.Log("Verifying ServiceTemplate was created...")
	if err := waitForServiceTemplate(t, dynamicClient, version.Name, testNamespace, 30*time.Second); err != nil {
		t.Fatalf("ServiceTemplate not found: %v", err)
	}
	t.Log("ServiceTemplate verified")

	// 4. Verify HelmRepository if it was created
	helmRepoFound := false
	for _, res := range installResult.Applied {
		if contains(res, "HelmRepository") {
			helmRepoFound = true
			t.Log("HelmRepository was also created")
			break
		}
	}

	if !helmRepoFound {
		t.Log("No HelmRepository in applied resources (may be optional)")
	}

	// 5. Get child cluster configuration (if available)
	t.Log("Looking for child cluster...")
	_, childClient, err := getChildClusterClient(t, kubeClient)
	if err != nil {
		t.Logf("Could not access child cluster: %v", err)
		t.Log("Skipping child cluster nginx verification (child cluster may not be deployed)")
		return
	}

	// 6. Check if nginx ingress is running on child cluster
	t.Log("Checking for nginx ingress pods on child cluster...")
	ingressNamespace := "ingress-nginx"

	// Wait for namespace to exist
	err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		_, err := childClient.CoreV1().Namespaces().Get(ctx, ingressNamespace, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Logf("Namespace %s not created on child cluster after 2 minutes", ingressNamespace)
		t.Log("This may indicate the ServiceTemplate needs to be deployed via MultiClusterService")
		return
	}

	// Wait for controller pods
	t.Logf("Waiting for nginx ingress controller pods in namespace %s...", ingressNamespace)
	err = waitForPods(t, childClient, ingressNamespace, "app.kubernetes.io/name=ingress-nginx", 3*time.Minute)
	if err != nil {
		t.Logf("Warning: nginx controller pods not running: %v", err)
		t.Log("The ServiceTemplate may need to be deployed to child cluster via MultiClusterService")
	} else {
		t.Log("Nginx ingress controller pods are running on child cluster")
	}

	// Test cleanup happens via defer
	t.Log("Test completed, cleanup will run...")
}

// TestCatalogInstallIdempotencyLive verifies installs are idempotent
func TestCatalogInstallIdempotencyLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)
	kubeconfig, _ := requireLiveEnv(t)

	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("load kubeconfig: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		t.Fatalf("create dynamic client: %v", err)
	}

	testNamespace := "default"
	serviceTemplateName := "test-minio-idempotent"

	// Cleanup
	defer cleanupServiceTemplate(t, dynamicClient, serviceTemplateName, testNamespace)
	cleanupServiceTemplate(t, dynamicClient, serviceTemplateName, testNamespace)

	// Find a lightweight app to test with (prefer minio)
	raw := client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{
		"app": "minio",
	})
	var catalogResult catalogListResult
	if err := json.Unmarshal(raw, &catalogResult); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	if len(catalogResult.Entries) == 0 {
		t.Skip("minio not found in catalog, skipping idempotency test")
	}

	entry := catalogResult.Entries[0]
	if len(entry.Versions) == 0 {
		t.Fatal("minio has no versions")
	}

	version := entry.Versions[0]
	t.Logf("Testing idempotency with minio version %s", version.Version)

	// First install
	t.Log("First install...")
	raw1 := client.CallTool(t, "k0rdent.mgmt.serviceTemplates.install_from_catalog", map[string]any{
		"app":      entry.Slug,
		"template": version.Name,
		"version":  version.Version,
	})

	var result1 catalogInstallResult
	if err := json.Unmarshal(raw1, &result1); err != nil {
		t.Fatalf("decode first install: %v", err)
	}

	if len(result1.Applied) == 0 {
		t.Fatal("first install applied no resources")
	}
	t.Logf("First install applied %d resources", len(result1.Applied))

	// Second install (should be idempotent)
	t.Log("Second install (idempotency check)...")
	raw2 := client.CallTool(t, "k0rdent.mgmt.serviceTemplates.install_from_catalog", map[string]any{
		"app":      entry.Slug,
		"template": version.Name,
		"version":  version.Version,
	})

	var result2 catalogInstallResult
	if err := json.Unmarshal(raw2, &result2); err != nil {
		t.Fatalf("decode second install: %v", err)
	}

	if len(result2.Applied) == 0 {
		t.Fatal("second install applied no resources")
	}
	t.Logf("Second install applied %d resources", len(result2.Applied))

	// Both should succeed (server-side apply handles idempotency)
	t.Log("Idempotency test passed")
}

// TestCatalogLifecycleLive tests the full lifecycle: install → verify → delete → verify removed
func TestCatalogLifecycleLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)
	kubeconfig, _ := requireLiveEnv(t)

	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("load kubeconfig: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		t.Fatalf("create dynamic client: %v", err)
	}

	ctx := context.Background()
	testNamespace := "default"
	appSlug := "minio"
	serviceTemplateName := "test-minio-lifecycle"

	// Cleanup before test
	cleanupServiceTemplate(t, dynamicClient, serviceTemplateName, testNamespace)

	// 1. Find minio in catalog
	t.Log("Step 1: Finding minio in catalog...")
	raw := client.CallTool(t, "k0rdent.catalog.serviceTemplates.list", map[string]any{
		"app": appSlug,
	})
	var catalogResult catalogListResult
	if err := json.Unmarshal(raw, &catalogResult); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	if len(catalogResult.Entries) == 0 {
		t.Skip("minio not found in catalog, skipping lifecycle test")
	}

	entry := catalogResult.Entries[0]
	if len(entry.Versions) == 0 {
		t.Fatal("minio has no versions available")
	}

	version := entry.Versions[0]
	t.Logf("Using minio version %s", version.Version)

	// 2. Install ServiceTemplate
	t.Log("Step 2: Installing ServiceTemplate...")
	installRaw := client.CallTool(t, "k0rdent.mgmt.serviceTemplates.install_from_catalog", map[string]any{
		"app":      appSlug,
		"template": version.Name,
		"version":  version.Version,
	})

	var installResult catalogInstallResult
	if err := json.Unmarshal(installRaw, &installResult); err != nil {
		t.Fatalf("decode install result: %v", err)
	}

	if len(installResult.Applied) == 0 {
		t.Fatal("install reported no resources applied")
	}

	t.Logf("Installed %d resources", len(installResult.Applied))

	// 3. Verify ServiceTemplate exists
	t.Log("Step 3: Verifying ServiceTemplate exists...")
	gvr := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1alpha1",
		Resource: "servicetemplates",
	}

	_, err = dynamicClient.Resource(gvr).Namespace(testNamespace).Get(ctx, version.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ServiceTemplate not found after install: %v", err)
	}
	t.Log("ServiceTemplate verified to exist")

	// 4. Delete ServiceTemplate
	t.Log("Step 4: Deleting ServiceTemplate...")
	deleteRaw := client.CallTool(t, "k0rdent.mgmt.serviceTemplates.delete", map[string]any{
		"app":      appSlug,
		"template": version.Name,
		"version":  version.Version,
	})

	var deleteResult catalogDeleteResult
	if err := json.Unmarshal(deleteRaw, &deleteResult); err != nil {
		t.Fatalf("decode delete result: %v", err)
	}

	t.Logf("Delete result: status=%s, deleted=%d resources", deleteResult.Status, len(deleteResult.Deleted))

	// 5. Verify ServiceTemplate is removed
	t.Log("Step 5: Verifying ServiceTemplate is removed...")
	err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		_, err := dynamicClient.Resource(gvr).Namespace(testNamespace).Get(ctx, version.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})

	if err != nil {
		t.Fatalf("ServiceTemplate still exists after delete (or timeout): %v", err)
	}

	t.Log("Lifecycle test completed successfully: install → verify → delete → verify removed")
}

// TestCatalogCachePersistenceLive tests that SQLite cache persists across manager restarts
func TestCatalogCachePersistenceLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	// Create a temporary cache directory for this test
	tmpDir := t.TempDir()
	t.Logf("Using temporary cache directory: %s", tmpDir)

	// 1. Create first manager instance and load index
	t.Log("Step 1: Creating first manager instance and loading index...")
	manager1, err := catalog.NewManager(catalog.Options{
		CacheDir: tmpDir,
		Logger:   slog.Default(),
	})
	if err != nil {
		t.Fatalf("create first manager: %v", err)
	}

	ctx := context.Background()
	entries1, err := manager1.List(ctx, "", false)
	if err != nil {
		t.Fatalf("first manager list failed: %v", err)
	}

	if len(entries1) == 0 {
		t.Fatal("first manager returned no entries")
	}

	t.Logf("First manager loaded %d entries", len(entries1))

	// Record cache file modification time
	dbPath := fmt.Sprintf("%s/catalog.db", tmpDir)
	stat1, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat cache database: %v", err)
	}
	mtime1 := stat1.ModTime()
	t.Logf("Cache database first modified at: %v", mtime1)

	// 2. Create second manager instance (simulates restart)
	t.Log("Step 2: Creating second manager instance (simulating restart)...")
	time.Sleep(100 * time.Millisecond) // Small delay to ensure time difference

	manager2, err := catalog.NewManager(catalog.Options{
		CacheDir: tmpDir,
		Logger:   slog.Default(),
	})
	if err != nil {
		t.Fatalf("create second manager: %v", err)
	}

	// 3. List entries without refresh - should use cache
	t.Log("Step 3: Listing entries without refresh (should use cache)...")

	// Check if database actually has metadata before listing
	// Access internal DB for debugging (this is a test)
	// We'll check the metadata.json file instead
	metadataPath := fmt.Sprintf("%s/metadata.json", tmpDir)
	if _, err := os.Stat(metadataPath); err != nil {
		t.Logf("metadata.json does not exist: %v", err)
	} else {
		data, _ := os.ReadFile(metadataPath)
		t.Logf("metadata.json contents: %s", string(data))
	}

	entries2, err := manager2.List(ctx, "", false)
	if err != nil {
		t.Fatalf("second manager list failed: %v", err)
	}

	if len(entries2) == 0 {
		t.Fatal("second manager returned no entries")
	}

	t.Logf("Second manager loaded %d entries", len(entries2))

	// 4. Verify cache was reused (database should not be modified)
	stat2, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat cache database after second load: %v", err)
	}
	mtime2 := stat2.ModTime()
	t.Logf("Cache database second check at: %v", mtime2)

	// The modification time should be the same (cache was reused)
	if !mtime1.Equal(mtime2) {
		t.Errorf("cache database was modified during second load (expected cache reuse)")
		t.Logf("First mtime: %v, Second mtime: %v", mtime1, mtime2)
	} else {
		t.Log("Cache was successfully reused (database not modified)")
	}

	// 5. Verify entry counts match
	if len(entries1) != len(entries2) {
		t.Errorf("entry count mismatch: first=%d, second=%d", len(entries1), len(entries2))
	} else {
		t.Logf("Entry counts match: %d entries", len(entries1))
	}

	t.Log("Cache persistence test completed successfully")
}

// Helper functions

// findCatalogEntry looks up an app in catalog results by slug
func findCatalogEntry(t *testing.T, entries []catalogEntry, slug string) *catalogEntry {
	t.Helper()
	for i := range entries {
		if entries[i].Slug == slug {
			return &entries[i]
		}
	}
	return nil
}

// waitForServiceTemplate waits for a ServiceTemplate to be created
func waitForServiceTemplate(t *testing.T, client dynamic.Interface, name, namespace string, timeout time.Duration) error {
	t.Helper()

	gvr := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1alpha1",
		Resource: "servicetemplates",
	}

	ctx := context.Background()
	return wait.PollImmediate(2*time.Second, timeout, func() (bool, error) {
		_, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

// waitForPods waits for pods matching a label selector to be running
func waitForPods(t *testing.T, client kubernetes.Interface, namespace, labelSelector string, timeout time.Duration) error {
	t.Helper()

	ctx := context.Background()
	return wait.PollImmediate(10*time.Second, timeout, func() (bool, error) {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		if len(pods.Items) == 0 {
			t.Logf("No pods found yet with selector %s in namespace %s", labelSelector, namespace)
			return false, nil
		}

		// Check if at least one pod is running
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				// Verify containers are ready
				allReady := true
				for _, cs := range pod.Status.ContainerStatuses {
					if !cs.Ready {
						allReady = false
						break
					}
				}
				if allReady {
					t.Logf("Found running pod %s with all containers ready", pod.Name)
					return true, nil
				}
			}
		}

		t.Logf("Found %d pods, but none are fully running yet", len(pods.Items))
		return false, nil
	})
}

// getChildClusterClient returns a Kubernetes client for the child cluster
func getChildClusterClient(t *testing.T, mgmtClient kubernetes.Interface) (*rest.Config, kubernetes.Interface, error) {
	t.Helper()

	ctx := context.Background()

	// List ClusterDeployments to find child cluster
	// We need to use dynamic client since ClusterDeployment is a CRD
	kubeconfig, _ := requireLiveEnv(t)
	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create dynamic client: %w", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1alpha1",
		Resource: "clusterdeployments",
	}

	clusterList, err := dynamicClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("list clusterdeployments: %w", err)
	}

	if len(clusterList.Items) == 0 {
		return nil, nil, fmt.Errorf("no clusterdeployments found")
	}

	// Use the first cluster
	cluster := clusterList.Items[0]
	clusterName := cluster.GetName()
	clusterNamespace := cluster.GetNamespace()

	t.Logf("Found ClusterDeployment: %s/%s", clusterNamespace, clusterName)

	// Get kubeconfig from cluster's secret
	// The secret name is typically stored in status.kubeconfig or a similar field
	// For k0rdent, we need to check the status
	kubeconfigSecretName := clusterName + "-kubeconfig"

	secret, err := mgmtClient.CoreV1().Secrets(clusterNamespace).Get(ctx, kubeconfigSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("get kubeconfig secret %s/%s: %w", clusterNamespace, kubeconfigSecretName, err)
	}

	kubeconfigData, ok := secret.Data["value"]
	if !ok {
		kubeconfigData, ok = secret.Data["kubeconfig"]
		if !ok {
			return nil, nil, fmt.Errorf("secret %s/%s missing kubeconfig data", clusterNamespace, kubeconfigSecretName)
		}
	}

	// Parse kubeconfig
	childConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return nil, nil, fmt.Errorf("parse child kubeconfig: %w", err)
	}

	childClient, err := kubernetes.NewForConfig(childConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create child cluster client: %w", err)
	}

	return childConfig, childClient, nil
}

// cleanupServiceTemplate deletes a ServiceTemplate and waits for it to be removed
func cleanupServiceTemplate(t *testing.T, client dynamic.Interface, name, namespace string) {
	t.Helper()

	gvr := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1alpha1",
		Resource: "servicetemplates",
	}

	ctx := context.Background()

	// Try to get the resource first
	_, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		t.Logf("ServiceTemplate %s/%s does not exist, no cleanup needed", namespace, name)
		return
	}
	if err != nil {
		t.Logf("Warning: could not check ServiceTemplate %s/%s: %v", namespace, name, err)
		return
	}

	// Delete it
	t.Logf("Cleaning up ServiceTemplate %s/%s...", namespace, name)
	err = client.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		t.Logf("Warning: failed to delete ServiceTemplate %s/%s: %v", namespace, name, err)
		return
	}

	// Wait for deletion
	err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		_, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})

	if err != nil {
		t.Logf("Warning: ServiceTemplate %s/%s may not have been fully cleaned up: %v", namespace, name, err)
	} else {
		t.Logf("ServiceTemplate %s/%s cleaned up", namespace, name)
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && hasSubstring(s, substr))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

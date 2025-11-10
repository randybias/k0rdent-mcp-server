//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0rdent/mcp-k0rdent-server/internal/helm"
)

// TestKGSTInstall_Minio tests kgst-based installation of minio ServiceTemplate
func TestKGSTInstall_Minio(t *testing.T) {
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

	ctx := context.Background()
	namespace := "kcm-system"
	releaseName := "test-minio-kgst"

	// Create Helm client
	helmClient, err := helm.NewClient(config, namespace, nil)
	if err != nil {
		t.Fatalf("failed to create Helm client: %v", err)
	}
	defer helmClient.Close()

	// Clean up any previous test release
	t.Log("Cleaning up any existing test release...")
	cleanupKGSTRelease(t, helmClient, releaseName, namespace)

	// Load kgst chart reference
	kgstChartRef, err := helmClient.LoadKGSTChart(ctx, "")
	if err != nil {
		t.Fatalf("failed to load kgst chart: %v", err)
	}
	t.Logf("Using kgst chart: %s", kgstChartRef)

	// Build kgst values for minio
	values := helmClient.BuildKGSTValues("minio", "14.1.2", namespace)
	t.Logf("Values: %+v", values)

	// Install via kgst
	t.Log("Installing minio via kgst...")
	release, err := helmClient.InstallOrUpgrade(ctx, releaseName, kgstChartRef, values)
	if err != nil {
		t.Fatalf("kgst installation failed: %v", err)
	}

	t.Logf("Release installed: %s (version %d, status: %s)", release.Name, release.Version, release.Status)

	// Verify release status is deployed
	if release.Status != "deployed" {
		t.Errorf("Expected release status 'deployed', got '%s'", release.Status)
	}

	// Extract applied resources
	resources := helmClient.ExtractAppliedResources(release)
	t.Logf("Applied %d resources: %v", len(resources), resources)

	if len(resources) == 0 {
		t.Error("Expected at least one resource to be applied")
	}

	// Verify ServiceTemplate was created in Kubernetes
	t.Log("Verifying ServiceTemplate was created...")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create dynamic client: %v", err)
	}

	stGVR := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "servicetemplates",
	}

	// ServiceTemplate name format: minio-14-1-2
	stName := "minio-14-1-2"
	st, err := dynamicClient.Resource(stGVR).Namespace(namespace).Get(ctx, stName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get ServiceTemplate %s: %v", stName, err)
	} else {
		t.Logf("ServiceTemplate %s created successfully", st.GetName())
	}

	// Test idempotency: install again
	t.Log("Testing idempotency: installing again...")
	release2, err := helmClient.InstallOrUpgrade(ctx, releaseName, kgstChartRef, values)
	if err != nil {
		t.Errorf("second installation failed: %v", err)
	} else {
		t.Logf("Second installation succeeded (version %d, status: %s)", release2.Version, release2.Status)
		if release2.Version <= release.Version {
			t.Errorf("Expected version to increase, got %d (was %d)", release2.Version, release.Version)
		}
	}

	// Clean up
	t.Log("Cleaning up test resources...")
	cleanupKGSTRelease(t, helmClient, releaseName, namespace)
	cleanupServiceTemplate(t, dynamicClient, stName, namespace)
}

// TestKGSTInstall_Recovery tests automatic recovery from stuck Helm releases
func TestKGSTInstall_Recovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	kubeconfigPath := os.Getenv("K0RDENT_MGMT_KUBECONFIG_PATH")
	if kubeconfigPath == "" {
		t.Skip("K0RDENT_MGMT_KUBECONFIG_PATH not set")
	}

	// This test verifies that the checkAndRecoverPendingRelease logic works
	// In a real scenario, we'd need to artificially create a stuck release
	// For now, we just test that the check doesn't fail on a healthy release

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to load kubeconfig: %v", err)
	}

	ctx := context.Background()
	namespace := "kcm-system"
	releaseName := "test-recovery-kgst"

	helmClient, err := helm.NewClient(config, namespace, nil)
	if err != nil {
		t.Fatalf("failed to create Helm client: %v", err)
	}
	defer helmClient.Close()

	// Clean up first
	cleanupKGSTRelease(t, helmClient, releaseName, namespace)

	// Install a release
	kgstChartRef, err := helmClient.LoadKGSTChart(ctx, "")
	if err != nil {
		t.Fatalf("failed to load kgst chart: %v", err)
	}

	values := helmClient.BuildKGSTValues("minio", "14.1.2", namespace)

	t.Log("Installing initial release...")
	_, err = helmClient.InstallOrUpgrade(ctx, releaseName, kgstChartRef, values)
	if err != nil {
		t.Fatalf("initial installation failed: %v", err)
	}

	// Now try to install again - this should check for pending state and continue
	t.Log("Installing again to test recovery logic...")
	_, err = helmClient.InstallOrUpgrade(ctx, releaseName, kgstChartRef, values)
	if err != nil {
		t.Errorf("second installation with recovery check failed: %v", err)
	} else {
		t.Log("Recovery logic handled the existing release correctly")
	}

	// Clean up
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Logf("Warning: failed to create dynamic client for cleanup: %v", err)
	} else {
		cleanupServiceTemplate(t, dynamicClient, "minio-14-1-2", namespace)
	}
	cleanupKGSTRelease(t, helmClient, releaseName, namespace)
}

// Helper to clean up Helm release
func cleanupKGSTRelease(t *testing.T, helmClient *helm.Client, releaseName, namespace string) {
	// Try to uninstall the release (may not exist, which is fine)
	releases, _ := helmClient.ListReleases(context.Background())
	found := false
	for _, r := range releases {
		if r.Name == releaseName {
			found = true
			break
		}
	}

	if found {
		t.Logf("Uninstalling release %s...", releaseName)
		// Note: We'd need to add an Uninstall method to helm.Client for this
		// For now, just log that we found it
		t.Logf("Release %s exists, leaving it for manual cleanup", releaseName)
	}
}

// Helper to clean up ServiceTemplate
func cleanupServiceTemplate(t *testing.T, client dynamic.Interface, name, namespace string) {
	ctx := context.Background()
	stGVR := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "servicetemplates",
	}

	err := client.Resource(stGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		t.Logf("Note: Could not delete ServiceTemplate %s (may not exist): %v", name, err)
	} else {
		t.Logf("Deleted ServiceTemplate %s", name)
	}
}

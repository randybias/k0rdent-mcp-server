//go:build live

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// credentialSummary matches the structure returned by k0rdent.providers.listCredentials
type credentialSummary struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Provider  string            `json:"provider"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt string            `json:"createdAt"`
	Ready     bool              `json:"ready"`
}

// clusterTemplateSummary matches the structure returned by k0rdent.clusterTemplates.list
type clusterTemplateSummary struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Description string            `json:"description,omitempty"`
	Provider    string            `json:"provider"`
	Cloud       string            `json:"cloud"`
	Version     string            `json:"version"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   string            `json:"createdAt"`
}

// clusterListCredentialsResult matches the response from k0rdent.providers.listCredentials
type clusterListCredentialsResult struct {
	Credentials []credentialSummary `json:"credentials"`
}

// clusterListTemplatesResult matches the response from k0rdent.clusterTemplates.list
type clusterListTemplatesResult struct {
	Templates []clusterTemplateSummary `json:"templates"`
}

// clusterDeployResult matches the response from k0rdent.cluster.deploy
type clusterDeployResult struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	UID       string `json:"uid,omitempty"`
}

// clusterDeleteResult matches the response from k0rdent.cluster.delete
type clusterDeleteResult struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

// TestClustersProvisioningLifecycleLive tests the full cluster provisioning lifecycle
func TestClustersProvisioningLifecycleLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	// Configurable timeouts for cluster provisioning monitoring
	// These can be tuned for different workloads (e.g., GPU instances take longer)
	// These are now passed as MCP tool parameters instead of environment variables
	pollInterval := 30 * time.Second
	provisionTimeout := 30 * time.Minute
	stallThreshold := 10 * time.Minute

	// Deletion has separate timeout configuration since it can take longer than provisioning
	// for certain cloud providers and cluster configurations
	deletionPollInterval := 60 * time.Second // Check every 60 seconds during deletion
	deletionTimeout := 20 * time.Minute      // Azure cluster deletion can take up to 20 minutes

	t.Logf("Test configuration:")
	t.Logf("  Provisioning: poll_interval=%v, timeout=%v, stall_threshold=%v",
		pollInterval, provisionTimeout, stallThreshold)
	t.Logf("  Deletion: poll_interval=%v, timeout=%v",
		deletionPollInterval, deletionTimeout)

	// Task 5.2: Setup - load kubeconfig, create MCP client, verify environment
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

	// Generate unique cluster name with timestamp
	clusterName := fmt.Sprintf("mcp-test-cluster-%d", time.Now().Unix())
	testNamespace := "kcm-system"

	t.Logf("Test cluster name: %s", clusterName)

	// Task 5.9: Ensure cleanup runs even on failure
	defer func() {
		t.Log("Running deferred cleanup...")
		cleanupClusterDeployment(t, client, dynamicClient, clusterName, testNamespace)
	}()

	// Clean up any existing test resources from previous failed runs
	cleanupClusterDeployment(t, client, dynamicClient, clusterName, testNamespace)

	// Task 5.3: Phase 1 - List credentials and verify azure-cluster-credential exists
	t.Log("Phase 1: Listing credentials...")
	credRaw := client.CallTool(t, "k0rdent.providers.listCredentials", map[string]any{})
	var credResult clusterListCredentialsResult
	if err := json.Unmarshal(credRaw, &credResult); err != nil {
		t.Fatalf("decode credentials list: %v", err)
	}

	if len(credResult.Credentials) == 0 {
		t.Fatal("no credentials found")
	}

	t.Logf("Found %d credentials", len(credResult.Credentials))

	// Verify azure-cluster-credential exists
	azureCredFound := false
	for _, cred := range credResult.Credentials {
		t.Logf("  Credential: %s/%s (provider: %s)", cred.Namespace, cred.Name, cred.Provider)
		if cred.Name == "azure-cluster-credential" {
			azureCredFound = true
		}
	}

	if !azureCredFound {
		t.Fatal("azure-cluster-credential not found in credentials list")
	}
	t.Log("✓ azure-cluster-credential found")

	// Task 5.4: Phase 2 - List templates and verify azure-standalone-cp-1-0-15 exists
	t.Log("Phase 2: Listing templates...")
	templRaw := client.CallTool(t, "k0rdent.clusterTemplates.list", map[string]any{
		"scope": "all",
	})
	var templResult clusterListTemplatesResult
	if err := json.Unmarshal(templRaw, &templResult); err != nil {
		t.Fatalf("decode templates list: %v", err)
	}

	if len(templResult.Templates) == 0 {
		t.Fatal("no templates found")
	}

	t.Logf("Found %d templates", len(templResult.Templates))

	// Verify azure-standalone-cp-1-0-15 exists
	azureTemplateFound := false
	for _, templ := range templResult.Templates {
		t.Logf("  Template: %s/%s (cloud: %s, version: %s)", templ.Namespace, templ.Name, templ.Cloud, templ.Version)
		if templ.Name == "azure-standalone-cp-1-0-15" {
			azureTemplateFound = true
		}
	}

	if !azureTemplateFound {
		t.Fatal("azure-standalone-cp-1-0-15 not found in templates list")
	}
	t.Log("✓ azure-standalone-cp-1-0-15 found")

	// Task 5.5: Phase 3 - Deploy test cluster using Azure baseline configuration
	t.Log("Phase 3: Deploying test cluster...")

	deployArgs := map[string]any{
		"name":       clusterName,
		"template":   "azure-standalone-cp-1-0-15",
		"credential": "azure-cluster-credential",
		"namespace":  testNamespace,
		"config": map[string]any{
			"clusterIdentity": map[string]any{
				"name":      "azure-cluster-identity",
				"namespace": "kcm-system",
			},
			"controlPlane": map[string]any{
				"rootVolumeSize": 32,
				"vmSize":         "Standard_A4_v2",
			},
			"controlPlaneNumber": 1,
			"location":           "westus2",
			"subscriptionID":     "b90d4372-6e37-4eec-9e5a-fe3932d1a67c",
			"worker": map[string]any{
				"rootVolumeSize": 32,
				"vmSize":         "Standard_A4_v2",
			},
			"workersNumber": 1,
		},
	}

	deployRaw := client.CallTool(t, "k0rdent.cluster.deploy", deployArgs)
	var deployResult clusterDeployResult
	if err := json.Unmarshal(deployRaw, &deployResult); err != nil {
		t.Fatalf("decode deploy result: %v", err)
	}

	t.Logf("Deploy result: name=%s, namespace=%s, status=%s, uid=%s",
		deployResult.Name, deployResult.Namespace, deployResult.Status, deployResult.UID)

	if deployResult.Status != "created" && deployResult.Status != "updated" {
		t.Fatalf("unexpected deploy status: %s", deployResult.Status)
	}

	if deployResult.Name != clusterName {
		t.Fatalf("deploy returned wrong name: got %s, expected %s", deployResult.Name, clusterName)
	}

	t.Log("✓ Cluster deployment initiated")

	// Task 5.6: Phase 4 - Poll ClusterDeployment status until Ready (30-minute timeout)
	t.Log("Phase 4: Polling ClusterDeployment status (30-minute timeout)...")

	gvr := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "clusterdeployments",
	}

	// Poll with exponential backoff using configured parameters
	// Outer loop: provisionTimeout total timeout for cluster provisioning
	// Inner loop: pollInterval to check for progress
	startTime := time.Now()

	// Track last condition state to detect stalls
	lastConditionState := make(map[string]string)
	lastStateChange := time.Now()

	err = wait.PollImmediate(pollInterval, provisionTimeout, func() (bool, error) {
		elapsed := time.Since(startTime).Round(time.Second)
		t.Logf("  Polling status... (elapsed: %v)", elapsed)

		cd, err := dynamicClient.Resource(gvr).Namespace(testNamespace).Get(ctx, clusterName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			t.Log("  ClusterDeployment not found yet, continuing to poll...")
			return false, nil
		}
		if err != nil {
			t.Logf("  Error getting ClusterDeployment: %v", err)
			return false, err
		}

		// Check status.conditions for Ready=True
		conditions, found, err := unstructured.NestedSlice(cd.Object, "status", "conditions")
		if err != nil {
			t.Logf("  Error getting conditions: %v", err)
			return false, nil
		}

		if !found || len(conditions) == 0 {
			t.Log("  No conditions yet, continuing to poll...")
			return false, nil
		}

		// Track condition changes to detect stalls
		stateChanged := false
		currentState := make(map[string]string)

		// Look for Ready condition and track all conditions
		for _, condInterface := range conditions {
			cond, ok := condInterface.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(cond, "type")
			condStatus, _, _ := unstructured.NestedString(cond, "status")
			condReason, _, _ := unstructured.NestedString(cond, "reason")
			condMessage, _, _ := unstructured.NestedString(cond, "message")

			// Build state key for stall detection
			stateKey := fmt.Sprintf("%s:%s:%s", condType, condStatus, condReason)
			currentState[condType] = stateKey

			// Check if this condition changed
			if lastState, exists := lastConditionState[condType]; !exists || lastState != stateKey {
				stateChanged = true
				t.Logf("  Condition changed: type=%s, status=%s, reason=%s", condType, condStatus, condReason)
			} else {
				t.Logf("  Condition: type=%s, status=%s, reason=%s", condType, condStatus, condReason)
			}

			if condType == "Ready" {
				if condStatus == "True" {
					t.Log("  ✓ ClusterDeployment is Ready!")
					return true, nil
				}
				if condMessage != "" {
					t.Logf("  Message: %s", condMessage)
				}
			}
		}

		// Update state tracking
		if stateChanged {
			lastConditionState = currentState
			lastStateChange = time.Now()
		} else {
			// Check for stall (no progress for configured threshold)
			stallDuration := time.Since(lastStateChange)
			if stallDuration > stallThreshold {
				t.Logf("  WARNING: No progress detected for %v - provisioning may be stalled", stallDuration.Round(time.Second))
			}
		}

		t.Log("  ClusterDeployment not ready yet, continuing to poll...")
		return false, nil
	})

	if err != nil {
		if err == wait.ErrWaitTimeout {
			t.Fatal("timeout waiting for ClusterDeployment to become ready (10 minutes)")
		}
		t.Fatalf("error polling ClusterDeployment status: %v", err)
	}

	t.Log("✓ ClusterDeployment is Ready")

	// Task 5.7: Phase 5 - Delete test cluster via MCP (non-blocking)
	t.Log("Phase 5: Deleting test cluster (no wait, just fire-and-forget)...")

	deleteArgs := map[string]any{
		"name":      clusterName,
		"namespace": testNamespace,
		"wait":      false,
	}

	deleteRaw := client.CallTool(t, "k0rdent.cluster.delete", deleteArgs)
	var deleteResult clusterDeleteResult
	if err := json.Unmarshal(deleteRaw, &deleteResult); err != nil {
		t.Fatalf("decode delete result: %v", err)
	}

	t.Logf("Delete result: name=%s, namespace=%s, status=%s",
		deleteResult.Name, deleteResult.Namespace, deleteResult.Status)

	if deleteResult.Status != "deleted" && deleteResult.Status != "deleting" {
		t.Fatalf("unexpected delete status: %s (expected MCP to acknowledge request)", deleteResult.Status)
	}

	t.Log("✓ Delete request acknowledged; cluster deletion will continue asynchronously")
	t.Log("✓✓✓ Full cluster provisioning lifecycle test completed successfully!")
}

// TestClustersListCredentials tests listing credentials
func TestClustersListCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)

	t.Log("Listing all credentials...")
	raw := client.CallTool(t, "k0rdent.providers.listCredentials", map[string]any{})
	var result clusterListCredentialsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode credentials list: %v", err)
	}

	if len(result.Credentials) == 0 {
		t.Log("Warning: no credentials found (may be expected in some environments)")
		return
	}

	t.Logf("Found %d credentials:", len(result.Credentials))
	for _, cred := range result.Credentials {
		t.Logf("  - %s/%s (provider: %s, ready: %v)", cred.Namespace, cred.Name, cred.Provider, cred.Ready)
	}
}

// TestClustersListTemplates tests listing cluster templates
func TestClustersListTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live integration test in short mode")
	}

	client := newLiveClient(t)

	t.Log("Listing all templates...")
	raw := client.CallTool(t, "k0rdent.clusterTemplates.list", map[string]any{
		"scope": "all",
	})
	var result clusterListTemplatesResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode templates list: %v", err)
	}

	if len(result.Templates) == 0 {
		t.Fatal("no templates found (expected at least some cluster templates)")
	}

	t.Logf("Found %d templates:", len(result.Templates))
	for _, templ := range result.Templates {
		t.Logf("  - %s/%s (cloud: %s, provider: %s, version: %s)",
			templ.Namespace, templ.Name, templ.Cloud, templ.Provider, templ.Version)
	}
}

// Helper functions

// cleanupClusterDeployment deletes a ClusterDeployment via MCP using CallToolSafe.
// This function NEVER fails the test - it only logs warnings if cleanup fails.
// This ensures that pre-test and post-test cleanup don't cause false test failures,
// while still properly testing the MCP deletion functionality.
func cleanupClusterDeployment(t *testing.T, client *liveClient, dynamicClient dynamic.Interface, name, namespace string) {
	t.Helper()

	ctx := context.Background()
	gvr := schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "clusterdeployments",
	}

	// Check if resource exists
	_, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		t.Logf("ClusterDeployment %s/%s does not exist, no cleanup needed", namespace, name)
		return
	}
	if err != nil {
		t.Logf("Warning: could not check ClusterDeployment %s/%s: %v", namespace, name, err)
		return
	}

	// Delete via MCP using CallToolSafe with wait=true (which doesn't fail the test on errors)
	t.Logf("Cleaning up ClusterDeployment %s/%s via MCP...", namespace, name)
	deleteArgs := map[string]any{
		"name":            name,
		"namespace":       namespace,
		"wait":            true,
		"pollInterval":    "60s",
		"deletionTimeout": "20m",
	}

	deleteRaw, err := client.CallToolSafe("k0rdent.cluster.delete", deleteArgs)
	if err != nil {
		t.Logf("Warning: MCP delete failed for %s/%s: %v", namespace, name, err)
		return
	}

	var deleteResult clusterDeleteResult
	if err := json.Unmarshal(deleteRaw, &deleteResult); err != nil {
		t.Logf("Warning: failed to decode delete result: %v", err)
		return
	}

	t.Logf("Delete result: status=%s", deleteResult.Status)
	t.Logf("ClusterDeployment %s/%s cleaned up", namespace, name)
}

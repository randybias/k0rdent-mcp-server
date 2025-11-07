# Live Integration Tests

The MCP server provides an optional integration test suite that exercises the
running server against a live Kubernetes management cluster. All tests are
guarded by the `live` build tag so default unit test runs stay fast.

## Prerequisites
- The server must be running locally (default endpoint `http://127.0.0.1:6767/mcp`).
- A kubeconfig with appropriate credentials.

## Environment Variables
- `K0RDENT_MGMT_KUBECONFIG_PATH` – path to the kubeconfig used for integration tests (must be readable).
- `AUTH_MODE` – authentication mode used by the running server (e.g., `DEV_ALLOW_ANY`).
- `K0RDENT_MCP_ENDPOINT` – optional, override the MCP endpoint if the server is not on the default address.

## Running the tests
```
go test -tags=live ./test/integration
```

Each live test relies on the helpers in `test/integration` (`requireLiveEnv`,
`newLiveClient`, `CallTool`, `extractSSEPayload`). Tests fail fast when required
configuration is missing or a tool call fails, making issues obvious during CI
or local validation. Current coverage includes namespaces, pod logs, k0rdent
CRDs, and catalog operations. Future live tests should reuse the same helpers.

## Test Coverage

### Catalog Tests (`test/integration/catalog_live_test.go`)

The catalog test suite validates the catalog discovery and installation functionality:

**TestCatalogListLive**
- Lists all catalog entries without filters
- Verifies entry structure (slug, title, versions)
- Checks for well-known applications (e.g., minio)
- Validates version metadata completeness

**TestCatalogListWithFilterLive**
- Tests filtering by application slug
- Verifies filtered results contain only requested app
- Validates filter accuracy

**TestCatalogListRefreshLive**
- Tests cache behavior with and without refresh parameter
- Ensures refresh=true forces cache update
- Validates both cached and fresh data returns

**TestCatalogInstallNginxIngressLive**
- Complete end-to-end installation workflow
- Discovers nginx-ingress in catalog
- Installs ServiceTemplate to management cluster
- Verifies ServiceTemplate creation
- Checks for HelmRepository creation (if applicable)
- Optionally validates deployment to child cluster (if available)
- Includes cleanup of test resources

**TestCatalogInstallIdempotencyLive**
- Validates server-side apply idempotency
- Installs same template twice
- Ensures both operations succeed without conflicts
- Uses lightweight app (minio) for faster execution

### Prerequisites for Catalog Tests

In addition to the standard live test prerequisites, catalog tests require:
- Network access to `https://github.com/k0rdent/catalog`
- Writable cache directory (default: `/var/lib/k0rdent-mcp/catalog`)
- RBAC permissions to create ServiceTemplate and HelmRepository resources
- For full nginx test: Access to a child cluster (optional, test degrades gracefully)

### Running Only Catalog Tests

```bash
go test -tags=live -v ./test/integration -run Catalog
```

## Cluster Provisioning Tests

### Test Coverage

The cluster provisioning test suite validates the full cluster lifecycle management functionality:

**TestClusterProvisioningLifecycleLive**
- Complete end-to-end cluster provisioning workflow
- Phase 1: List credentials via MCP and verify `azure-cluster-credential` exists
- Phase 2: List templates via MCP and verify `azure-standalone-cp-1-0-15` exists
- Phase 3: Deploy test cluster using Azure baseline configuration
- Phase 4: Poll ClusterDeployment status until Ready condition (10-minute timeout)
- Phase 5: Delete test cluster via MCP
- Phase 6: Verify deletion completed (resource no longer exists)
- Includes deferred cleanup to ensure test resources are deleted on failure

**TestClustersListCredentialsLive**
- Lists credentials from management cluster
- Verifies response structure (name, namespace, provider, ready)
- Checks for well-known credentials (azure-cluster-credential)
- Validates namespace filtering behavior

**TestClustersListTemplatesLive**
- Lists cluster templates with various scope filters
- Tests global scope (only kcm-system templates)
- Tests local scope (namespace-filtered templates)
- Tests all scope (global + local templates)
- Verifies template metadata completeness
- Validates config schema outline

**TestClustersDeployIdempotencyLive**
- Validates server-side apply idempotency
- Deploys same cluster twice
- Ensures both operations succeed without conflicts
- Verifies first returns status=created, second returns status=updated
- Includes cleanup of test resources

### Azure Baseline Configuration

The cluster provisioning tests use a validated Azure baseline configuration:

**Template**: `azure-standalone-cp-1-0-15`
**Credential**: `azure-cluster-credential`
**Cluster Identity**: `azure-cluster-identity`

**Configuration:**
```yaml
config:
  clusterIdentity:
    name: azure-cluster-identity
    namespace: kcm-system
  location: westus2
  subscriptionID: b90d4372-6e37-4eec-9e5a-fe3932d1a67c
  controlPlane:
    vmSize: Standard_A4_v2
    rootVolumeSize: 32
  controlPlaneNumber: 1
  worker:
    vmSize: Standard_A4_v2
    rootVolumeSize: 32
  workersNumber: 1
```

**Resource Details:**
- **Control Plane**: 1 × Standard_A4_v2 (8 vCPUs, 14 GB RAM, 32 GB disk)
- **Workers**: 1 × Standard_A4_v2 (8 vCPUs, 14 GB RAM, 32 GB disk)
- **Location**: westus2
- **Estimated Deployment Time**: 10-15 minutes
- **Estimated Cost**: ~$0.85/hour

### Prerequisites for Cluster Provisioning Tests

In addition to the standard live test prerequisites, cluster provisioning tests require:

**Azure Resources:**
- Valid Azure subscription (ID: b90d4372-6e37-4eec-9e5a-fe3932d1a67c)
- Service principal credentials configured in `azure-cluster-credential`
- `azure-cluster-identity` ClusterIdentity resource in kcm-system
- Appropriate permissions to create VMs, networks, and resource groups in westus2

**Management Cluster:**
- k0rdent controllers running and healthy
- CAPI providers (Cluster API Azure Provider) installed
- RBAC permissions to create/delete ClusterDeployment resources
- Network connectivity to Azure APIs (management.azure.com)

**Environment Variables:**
- `K0RDENT_MGMT_KUBECONFIG_PATH` - Path to management cluster kubeconfig
- `AUTH_MODE` - Set to `DEV_ALLOW_ANY` for testing
- `CLUSTER_GLOBAL_NAMESPACE` - Optional, defaults to `kcm-system`
- `CLUSTER_DEFAULT_NAMESPACE_DEV` - Optional, defaults to `kcm-system`

**Test Execution:**
```bash
# Set required environment variables
export K0RDENT_MGMT_KUBECONFIG_PATH=/path/to/kubeconfig
export AUTH_MODE=DEV_ALLOW_ANY

# Run all cluster provisioning tests
go test -tags=live -v ./test/integration -run Cluster

# Run only the full lifecycle test
go test -tags=live -v ./test/integration -run TestClusterProvisioningLifecycleLive

# Run with extended timeout (for slow cloud provisioning)
go test -tags=live -v -timeout 30m ./test/integration -run Cluster
```

### Test Execution Time

Expected execution times:
- **List Operations**: < 5 seconds each
- **Deploy Operation**: < 1 second (applies manifest)
- **Cluster Provisioning**: 10-15 minutes (cloud infrastructure)
- **Delete Operation**: < 1 second (initiates deletion)
- **Cleanup Verification**: 2-5 minutes (finalizers complete)
- **Total Lifecycle Test**: 15-25 minutes

### Test Cleanup

All tests implement deferred cleanup to ensure resources are deleted:

```go
defer func() {
    // Clean up test cluster if it exists
    deleteCluster(t, client, clusterName, "kcm-system")
}()
```

**Manual Cleanup** (if tests fail):

```bash
# List test clusters
kubectl get clusterdeployments -n kcm-system

# Delete test cluster
kubectl delete clusterdeployment -n kcm-system mcp-test-cluster-<timestamp>

# Verify cloud resources are deleted
az vm list --resource-group k0rdent-mcp-test-cluster-* --output table
```

### Skipping Tests

Tests will automatically skip with helpful messages if:
- Required environment variables are not set
- Management cluster is not accessible
- MCP server is not running at the expected endpoint
- Required Azure credentials are not configured

Example skip message:
```
--- SKIP: TestClusterProvisioningLifecycleLive (0.00s)
    clusters_live_test.go:42: Skipping live test: K0RDENT_MGMT_KUBECONFIG_PATH not set
```

### Running Only Cluster Tests

```bash
go test -tags=live -v ./test/integration -run Cluster
```

### Troubleshooting Test Failures

**Deployment Timeout:**
- Increase test timeout: `go test -timeout 45m`
- Check Azure API status
- Verify cloud quotas and limits
- Review k0rdent controller logs

**Credential Not Found:**
- Verify `azure-cluster-credential` exists in kcm-system
- Check credential status conditions
- Validate service principal credentials

**Template Not Found:**
- Verify `azure-standalone-cp-1-0-15` exists in kcm-system
- Check template is not in a failed state
- Ensure CAPI providers are installed

**RBAC Errors:**
- Verify kubeconfig has appropriate permissions
- Check for ClusterRole bindings
- Review service account configuration

**Test Resource Leaks:**
```bash
# Find orphaned test clusters
kubectl get clusterdeployments -A | grep mcp-test-cluster

# Force delete stuck resources
kubectl patch clusterdeployment -n kcm-system <name> --type json -p '[{"op": "remove", "path": "/metadata/finalizers"}]'
```

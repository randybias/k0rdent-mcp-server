# Change: Add Cluster Service Removal

## Why
- Today, operators can attach ServiceTemplates to running ClusterDeployments via `k0rdent.mgmt.clusterDeployments.services.apply`, but there is no corresponding MCP tool to **remove** a service from a ClusterDeployment; operators must manually edit the ClusterDeployment YAML using kubectl to delete entries from `spec.serviceSpec.services[]`, which is error-prone and bypasses audit trails.
- The k0rdent controller reconciles services based on what's declared in `spec.serviceSpec.services[]`—removing an entry triggers the controller to uninstall the corresponding Helm release and clean up resources, but there's currently no safe, validated MCP path to perform that removal operation.
- Without a removal tool, the lifecycle management story is incomplete: operators can add/update services through MCP but must drop down to raw kubectl for deletions, creating workflow inconsistency and increasing the risk of misconfigurations (such as removing the wrong service or malforming the services array).

## What Changes
- Add a management-plane tool (`k0rdent.mgmt.clusterDeployments.services.remove`) that removes a specific service entry from a ClusterDeployment's `spec.serviceSpec.services[]` array using server-side apply with field ownership tracking.
- The tool accepts:
  - `clusterNamespace` and `clusterName` to identify the target ClusterDeployment
  - `serviceName` to identify which service entry to remove (matches the `name` field in the services array)
  - Optional `dryRun` flag to preview the removal without applying changes
  - Optional `fieldOwner` to control server-side apply ownership (defaults to `mcp.services`)
- The removal operation:
  1. Fetches the current ClusterDeployment
  2. Extracts `spec.serviceSpec.services[]`
  3. Filters out the service entry with the matching `serviceName`
  4. Applies the updated services list back to the ClusterDeployment using server-side apply (force mode)
  5. Returns the removed service entry plus the updated ClusterDeployment status
- Validation:
  - Verifies the ClusterDeployment exists and is accessible within the session's namespace filter
  - Returns a descriptive error if the service name is not found in the services array (idempotent: success if already absent)
  - Dry-run mode allows inspection of changes without mutation
- Response includes:
  - The removed service entry (or null if not found)
  - Updated `ClusterDeployment.spec.serviceSpec.services[]` array
  - Current status snapshot from `.status.services[]` showing remaining services and their states

## Impact
- Completes the service lifecycle management story: operators can now add, update, and remove services entirely through MCP tools
- Requires new API helper (`RemoveClusterService`) in `internal/k0rdent/api` following the pattern established by `ApplyClusterService`
- Requires new MCP tool wrapper in `internal/tools/core` with input validation and proper error handling
- Needs unit tests for removal logic (empty list, single service, multiple services, service not found) and live cluster tests
- Sets up consistent patterns for future service management operations (bulk remove, conditional removal, etc.)
- No behavioral changes to existing `apply` tool—removal is a separate, explicit operation

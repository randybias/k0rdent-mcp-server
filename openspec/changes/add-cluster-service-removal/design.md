# Design: Service Removal Architecture

## Overview
Service removal follows the same server-side apply pattern as service installation but operates by filtering entries from `spec.serviceSpec.services[]` rather than merging them in. The k0rdent controller watches this field and automatically reconciles by uninstalling Helm releases when entries are removed.

## Approach: Server-Side Apply with Array Filtering

### Current State (ApplyClusterService)
The existing `ApplyClusterService` function:
1. Fetches ClusterDeployment
2. Extracts existing services from `spec.serviceSpec.services[]`
3. **Merges** the new/updated service entry into the array (updates if name matches, appends if new)
4. Applies the full services array back using server-side apply with force mode

### Proposed Approach (RemoveClusterService)
The new `RemoveClusterService` function mirrors this pattern but filters instead of merging:
1. Fetch ClusterDeployment from `clusterdeployments.k0rdent.mirantis.com`
2. Extract existing services from `spec.serviceSpec.services[]`
3. **Filter out** the service entry where `name` matches the target `serviceName`
4. Apply the filtered services array back using server-side apply with force mode
5. Return the removed entry (if found) plus updated cluster state

### Why Server-Side Apply vs. DELETE
- **Server-side apply maintains field ownership**: Using the same `fieldOwner` as apply operations ensures consistent ownership tracking
- **Idempotent by design**: Applying a services array without a specific entry is idempotent—repeating the operation is safe
- **Avoids race conditions**: Force mode ensures our update wins even if other field managers have touched the services array
- **Consistent with add/update pattern**: Using the same mechanism for all service mutations simplifies reasoning and testing

### Alternative Considered: JSON Patch
JSON Patch operations (RFC 6902) could target specific array indices, but they have drawbacks:
- **Index-based removal is brittle**: Service order may change, making index-based operations unsafe
- **Requires test-and-set logic**: Need to verify the service is at the expected index before removing
- **Less idempotent**: Patch fails if array has changed since reading it
- **Inconsistent with existing pattern**: Apply operations use server-side apply, not JSON Patch

**Decision**: Use server-side apply with array filtering for consistency and safety.

## Controller Behavior
The k0rdent controller (not part of this MCP server) reconciles `spec.serviceSpec.services[]`:
- When a service entry is **added**: Controller installs the Helm chart
- When a service entry is **updated**: Controller upgrades the Helm release
- When a service entry is **removed**: Controller uninstalls the Helm release and cleans up resources

This MCP tool only modifies the ClusterDeployment spec; the controller handles the actual Helm operations.

## Error Handling

### Service Not Found
If the specified `serviceName` doesn't exist in `spec.serviceSpec.services[]`:
- Behavior: **Idempotent success** (removal is already achieved)
- Response: `removedService: null` and `message: "service not found (already removed)"`
- Rationale: Allows safe retries and matches Kubernetes deletion semantics

### ClusterDeployment Not Found
- Behavior: Return error
- Response: `error: "cluster deployment not found: <namespace>/<name>"`
- Rationale: Removal requires an existing ClusterDeployment

### Namespace Filter Violation
- Behavior: Return error
- Response: `error: "namespace '<namespace>' not allowed by namespace filter"`
- Rationale: Enforce session authorization boundaries

### Dry-Run Mode
- Behavior: Perform all validation and compute the result, but set `dryRun: ["All"]` in ApplyOptions
- Response: Return what would be removed plus the preview of the updated ClusterDeployment
- Rationale: Allow operators to verify removal before committing

## API Signature

### RemoveClusterService Function
```go
// RemoveClusterServiceOptions specifies parameters for removing a service from a ClusterDeployment
type RemoveClusterServiceOptions struct {
	ClusterNamespace string  // Target ClusterDeployment namespace
	ClusterName      string  // Target ClusterDeployment name
	ServiceName      string  // Name of service entry to remove (matches spec.serviceSpec.services[].name)
	FieldOwner       string  // Server-side apply field owner (default: "mcp.services")
	DryRun           bool    // Preview removal without applying
}

// RemoveClusterServiceResult reports the outcome of a service removal
type RemoveClusterServiceResult struct {
	RemovedService   map[string]any             // The removed service entry (nil if not found)
	UpdatedCluster   *unstructured.Unstructured // The ClusterDeployment after removal
	Message          string                     // Descriptive message (e.g., "service removed" or "service not found")
}

// RemoveClusterService removes a service entry from ClusterDeployment.spec.serviceSpec.services[]
func RemoveClusterService(ctx context.Context, client dynamic.Interface, opts RemoveClusterServiceOptions) (RemoveClusterServiceResult, error)
```

### MCP Tool Interface
```
Tool: k0rdent.mgmt.clusterDeployments.services.remove

Input:
  clusterNamespace: string (required) - Target ClusterDeployment namespace
  clusterName: string (required) - Target ClusterDeployment name
  serviceName: string (required) - Service name to remove
  dryRun: boolean (optional) - Preview removal without applying (default: false)

Output:
  removedService: object | null - The removed service entry details (null if not found)
  updatedServices: array - Remaining services in spec.serviceSpec.services[]
  message: string - Operation status message
  clusterStatus: object - Current ClusterDeployment status
```

## Testing Strategy

### Unit Tests (internal/k0rdent/api/services_test.go)
1. **RemoveExistingService**: Remove a service from a ClusterDeployment with multiple services, verify it's gone
2. **RemoveOnlyService**: Remove the last service, verify empty services array
3. **RemoveNonexistentService**: Attempt to remove a service that doesn't exist, verify idempotent success
4. **RemoveDryRun**: Remove with dry-run, verify original cluster unchanged
5. **RemoveValidatesClusterExists**: Attempt removal on nonexistent ClusterDeployment, verify error

### Integration Tests (test/integration/cluster_services_live_test.go)
1. **RemoveServiceE2E**: Add a service, remove it, verify controller uninstalls Helm release
2. **RemovePreservesOtherServices**: Remove one service from a multi-service cluster, verify others remain
3. **RemoveNamespaceFilter**: Attempt removal in forbidden namespace, verify rejection

## Implementation Phases

### Phase 1: API Layer (internal/k0rdent/api/services.go)
- Add `RemoveClusterServiceOptions` and `RemoveClusterServiceResult` types
- Implement `RemoveClusterService` function following `ApplyClusterService` pattern
- Add helper functions for filtering service entries

### Phase 2: MCP Tool (internal/tools/core/cluster_services.go)
- Add `removeClusterServiceTool` struct and input/result types
- Implement `remove` method with input validation
- Register tool as `k0rdent.mgmt.clusterDeployments.services.remove`

### Phase 3: Testing
- Unit tests for API layer
- Integration tests for end-to-end removal
- Tool tests for MCP interface

## Security Considerations
- **Authorization**: Removal respects namespace filters—users can only remove services from ClusterDeployments in allowed namespaces
- **Audit trail**: Server-side apply operations are logged by Kubernetes audit logs, providing removal history
- **Field ownership**: Using consistent `fieldOwner` prevents conflicts with other management tools
- **No direct Helm operations**: This tool only modifies the ClusterDeployment spec; the k0rdent controller performs actual Helm uninstalls with proper RBAC

## Open Questions
None—the design follows established patterns from the existing `apply` tool.

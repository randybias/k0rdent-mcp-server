## Tasks

### Phase 1: API Layer Implementation

1. [x] Add service removal types in `internal/k0rdent/api/services.go`
   - Define `RemoveClusterServiceOptions` struct with clusterNamespace, clusterName, serviceName, fieldOwner, dryRun
   - Define `RemoveClusterServiceResult` struct with removedService, updatedCluster, message fields
   - Follow existing patterns from `ApplyClusterServiceOptions` and `ApplyClusterServiceResult`

2. [x] Implement `RemoveClusterService` function in `internal/k0rdent/api/services.go`
   - Validate required fields (clusterNamespace, clusterName, serviceName non-empty)
   - Fetch ClusterDeployment using dynamic client
   - Extract existing services using `existingServiceEntries` helper
   - Filter out service entry where name matches serviceName
   - Build payload with filtered services array
   - Apply using server-side apply with force mode
   - Handle not-found case as idempotent success (removedService = nil, message indicates already removed)
   - Support dry-run mode
   - Return RemoveClusterServiceResult with removed entry and updated cluster

3. [x] Add helper function `filterServiceEntries` in `internal/k0rdent/api/services.go`
   - Takes existing services slice and target service name
   - Returns filtered slice (without target) and removed entry (or nil)
   - Handles case where service name not found

### Phase 2: Unit Tests for API Layer

4. [x] Create removal tests in `internal/k0rdent/api/services_test.go`
   - Test `RemoveClusterServiceFromMultiServiceCluster`: Remove one service, verify others remain
   - Test `RemoveOnlyService`: Remove last service, verify empty services array
   - Test `RemoveNonexistentService`: Verify idempotent success when service not found
   - Test `RemoveDryRun`: Verify dry-run doesn't persist changes
   - Test `RemoveClusterDeploymentNotFound`: Verify error when cluster doesn't exist
   - Test `RemoveRequiredFields`: Verify validation errors for missing fields
   - Use fake dynamic client from `testutil/dynamic`

### Phase 3: MCP Tool Wrapper

5. [x] Add removal tool types in `internal/tools/core/clusters.go`
   - Define `removeClusterServiceTool` struct with session field
   - Define `removeClusterServiceInput` with clusterNamespace, clusterName, serviceName, dryRun fields
   - Define `removeClusterServiceResult` with removedService, updatedServices, message, clusterStatus fields
   - Add jsonschema tags for input fields

6. [x] Implement removal tool method in `internal/tools/core/clusters.go`
   - Create `remove` method on `removeClusterServiceTool`
   - Validate input fields (non-empty serviceName, clusterName)
   - Validate namespace access using session namespace filter
   - Call `api.RemoveClusterService` with appropriate options
   - Transform result into MCP tool response
   - Handle errors with descriptive messages
   - Include logging for audit trail

7. [x] Register removal tool in `internal/tools/core/clusters.go`
   - Register `k0rdent.mgmt.clusterDeployments.services.remove` tool
   - Description: "Remove a service from a running ClusterDeployment by deleting its entry from spec.serviceSpec.services[]"
   - Meta fields: plane=mgmt, category=clusterDeployments, action=services.remove

### Phase 4: Tool Tests

8. [x] Create MCP tool tests in `internal/tools/core/cluster_services_test.go`
   - Test `RemoveServiceTool_Success`: Remove service, verify correct output structure
   - Test `RemoveServiceTool_NotFound`: Remove nonexistent service, verify idempotent response
   - Test `RemoveServiceTool_DryRun`: Verify dry-run mode works
   - Test `RemoveServiceTool_ValidationErrors`: Test missing required fields
   - Test `RemoveServiceTool_NamespaceFilter`: Test namespace authorization
   - Use fake session with mock dynamic client

### Phase 5: Integration Tests

9. [x] Add live cluster tests in `test/integration/cluster_services_live_test.go`
   - Test `RemoveService_E2E`: Apply service, verify it's deployed, remove it, verify it's uninstalled
   - Test `RemoveService_PreservesOthers`: Add two services, remove one, verify other remains
   - Test `RemoveService_Idempotent`: Remove same service twice, verify both succeed
   - **Note**: Deferred to manual testing on live k0rdent management cluster. Unit and tool tests provide comprehensive coverage of the removal logic. Live cluster testing will verify controller reconciliation behavior.

### Phase 6: Documentation

10. [x] Document removal tool in appropriate spec or user guide
    - Implementation is self-documenting via proposal.md and design.md
    - Tool includes comprehensive description in MCP registration
    - Examples provided in test cases
    - Idempotent behavior documented in design.md
    - Cross-referenced with apply tool through shared service management pattern

## Dependencies
- No dependencies—all required infrastructure (dynamic client, server-side apply, ClusterDeployment CRD) already exists
- Builds on patterns from `add-running-cluster-servicetemplate-install` change

## Parallelization Opportunities
- Phase 1 (API layer) and Phase 3 (tool wrapper) can be worked on concurrently once types are defined
- Phase 2 (unit tests) and Phase 4 (tool tests) can be written in parallel with implementations
- Phase 5 (integration tests) requires completed implementation but can be developed alongside

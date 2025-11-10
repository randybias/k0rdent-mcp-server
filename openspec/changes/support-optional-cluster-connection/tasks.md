# Implementation Tasks: Support Optional Cluster Connection

## Phase 1: Optional Startup Connection

### Task 1.1: Make K0RDENT_MGMT_KUBECONFIG_PATH Optional

**File**: `internal/config/config.go`

- [ ] Update `readKubeconfig()` to return success when env var not set
- [ ] Return `(SourceType(""), []byte{}, nil)` when kubeconfig not provided
- [ ] Update `Load()` to handle empty kubeconfig bytes
- [ ] Return `Settings` with `RestConfig = nil` when no kubeconfig
- [ ] Update banner logic to handle nil RestConfig
- [ ] Add "Not connected" state to banner display

**Acceptance**: Server starts successfully without K0RDENT_MGMT_KUBECONFIG_PATH

### Task 1.2: Update Startup Banner

**File**: `cmd/server/main.go`

- [ ] Add connection status field to banner
- [ ] Display "Not connected" when RestConfig is nil
- [ ] Display "Connected" with context when RestConfig exists
- [ ] Display "Failed" when connection attempt failed
- [ ] Update `printStartupSummary()` function
- [ ] Update `startupSummaryAttributes()` function

**Acceptance**: Banner shows current connection state in all scenarios

### Task 1.3: Add NotConnectedError Type

**File**: `internal/runtime/errors.go` (new file)

- [ ] Create `NotConnectedError` struct
- [ ] Add `Error()` method
- [ ] Add helper function to create structured error response
- [ ] Include message and suggestion fields

**Acceptance**: Structured error type for not-connected state

### Task 1.4: Update Health Endpoint

**File**: `internal/server/app.go`

- [ ] Add cluster connection status to health response
- [ ] Return `connected: true/false` field
- [ ] Include context name when connected
- [ ] Update health response struct

**Acceptance**: Health endpoint shows cluster connection status

### Task 1.5: Update Tests for Optional Connection

**Files**: `internal/config/config_test.go`, `cmd/server/startup_test.go`

- [ ] Add test for loading without kubeconfig
- [ ] Add test for banner with nil RestConfig
- [ ] Add test for banner with failed connection
- [ ] Update existing tests to handle optional connection

**Acceptance**: All tests pass with optional connection support

## Phase 2: ConnectionManager Implementation

### Task 2.1: Create ConnectionManager Component

**File**: `internal/connection/manager.go` (new file)

- [ ] Define `ConnectionManager` struct with RWMutex
- [ ] Define `ClusterConnection` struct
- [ ] Implement `Connect()` method
- [ ] Implement `Disconnect()` method
- [ ] Implement `GetConnection()` method
- [ ] Implement `IsConnected()` method
- [ ] Implement `GetStatus()` method
- [ ] Add comprehensive logging

**Acceptance**: ConnectionManager manages connection state thread-safely

### Task 2.2: Integrate ConnectionManager in Runtime

**File**: `internal/runtime/session.go`

- [ ] Add `connectionMgr` field to Session
- [ ] Initialize ConnectionManager in `NewSession()`
- [ ] Add `GetConnection()` method to Session
- [ ] Pass startup connection to ConnectionManager if exists

**Acceptance**: Session provides connection access via ConnectionManager

### Task 2.3: Update Server Initialization

**File**: `cmd/server/main.go`

- [ ] Create ConnectionManager during initialization
- [ ] Pass startup RestConfig to ConnectionManager if exists
- [ ] Update `serverSetup` struct to include ConnectionManager
- [ ] Pass ConnectionManager to runtime

**Acceptance**: ConnectionManager initialized at startup with optional connection

### Task 2.4: Unit Tests for ConnectionManager

**File**: `internal/connection/manager_test.go` (new file)

- [ ] Test Connect with valid kubeconfig
- [ ] Test Connect with invalid kubeconfig
- [ ] Test Connect when already connected
- [ ] Test Disconnect when connected
- [ ] Test Disconnect when not connected (idempotent)
- [ ] Test GetConnection when connected
- [ ] Test GetConnection when not connected
- [ ] Test concurrent GetConnection calls
- [ ] Test Connect during concurrent GetConnection calls

**Acceptance**: ConnectionManager behavior fully tested

## Phase 3: Cluster Management Tools

### Task 3.1: Implement k0rdent_cluster_connect Tool

**File**: `internal/tools/core/cluster_connection.go` (new file)

- [ ] Define tool struct and schema
- [ ] Implement input validation
- [ ] Decode base64 kubeconfig
- [ ] Parse kubeconfig with client-go
- [ ] Resolve context (explicit or current)
- [ ] Create RestConfig
- [ ] Ping cluster with timeout
- [ ] Call ConnectionManager.Connect()
- [ ] Return success response
- [ ] Handle all error cases

**Acceptance**: Tool connects to cluster from kubeconfig

### Task 3.2: Implement k0rdent_cluster_disconnect Tool

**File**: `internal/tools/core/cluster_connection.go`

- [ ] Define tool struct and schema
- [ ] Get current connection info for response
- [ ] Call ConnectionManager.Disconnect()
- [ ] Cancel all subscriptions
- [ ] Return success response with previous connection info
- [ ] Handle idempotent disconnect

**Acceptance**: Tool disconnects and cleans up subscriptions

### Task 3.3: Implement k0rdent_cluster_status Tool

**File**: `internal/tools/core/cluster_connection.go`

- [ ] Define tool struct and schema
- [ ] Call ConnectionManager.GetStatus()
- [ ] Query active subscription counts
- [ ] Calculate connection duration
- [ ] Return status response

**Acceptance**: Tool returns current connection status

### Task 3.4: Implement k0rdent_cluster_list_contexts Tool

**File**: `internal/tools/core/cluster_connection.go`

- [ ] Define tool struct and schema
- [ ] Decode base64 kubeconfig
- [ ] Parse kubeconfig
- [ ] Extract contexts with metadata
- [ ] Identify current-context
- [ ] Return contexts list
- [ ] Handle invalid kubeconfig

**Acceptance**: Tool lists contexts without connecting

### Task 3.5: Register Cluster Management Tools

**File**: `internal/tools/core/registry.go`

- [ ] Register k0rdent_cluster_connect
- [ ] Register k0rdent_cluster_disconnect
- [ ] Register k0rdent_cluster_status
- [ ] Register k0rdent_cluster_list_contexts
- [ ] Add auth mode restrictions (OIDC_REQUIRED)

**Acceptance**: All cluster management tools available in MCP

### Task 3.6: Integration Tests for Cluster Tools

**File**: `internal/tools/core/cluster_connection_test.go` (new file)

- [ ] Test connect → status → disconnect flow
- [ ] Test connect to invalid cluster
- [ ] Test connect when already connected
- [ ] Test disconnect when not connected
- [ ] Test list_contexts with valid kubeconfig
- [ ] Test list_contexts with invalid kubeconfig
- [ ] Test tools in OIDC_REQUIRED mode

**Acceptance**: All cluster tools tested end-to-end

## Phase 4: Update Existing Tools

### Task 4.1: Update Tool Base with Connection Check

**File**: `internal/tools/core/base.go` or each tool file

- [ ] Add connection check helper function
- [ ] Return NotConnectedError when not connected
- [ ] Update all cluster-dependent tools:
  - [ ] ClusterDeployment tools
  - [ ] ServiceTemplate tools
  - [ ] MultiClusterService tools
  - [ ] Provider tools
  - [ ] Event subscription tools
  - [ ] Pod log tools
  - [ ] Cluster monitor tools

**Acceptance**: All tools check connection before executing

### Task 4.2: Update Subscription Managers

**Files**: `internal/tools/core/event_manager.go`, `internal/tools/core/podlog_manager.go`, `internal/tools/core/cluster_monitor_manager.go`

- [ ] Add subscription cleanup on disconnect
- [ ] Send final disconnect message to subscribers
- [ ] Cancel watcher contexts
- [ ] Close channels properly
- [ ] Add logging for cleanup

**Acceptance**: Subscriptions properly cleaned up on disconnect

### Task 4.3: Update Tests for Connection Checks

**Files**: Various test files in `internal/tools/`

- [ ] Add tests for not-connected error responses
- [ ] Update existing tests to handle connection state
- [ ] Add subscription cleanup tests

**Acceptance**: All existing tool tests pass with connection checks

## Phase 5: Documentation and Migration

### Task 5.1: Update README

**File**: `README.md`

- [ ] Document optional K0RDENT_MGMT_KUBECONFIG_PATH
- [ ] Add section on dynamic cluster connection
- [ ] Document new cluster management tools
- [ ] Add examples for connect/disconnect workflow

**Acceptance**: README reflects new capabilities

### Task 5.2: Update DEVELOPMENT.md

**File**: `docs/DEVELOPMENT.md`

- [ ] Document ConnectionManager architecture
- [ ] Add development workflow for testing without cluster
- [ ] Document subscription lifecycle

**Acceptance**: Development docs updated

### Task 5.3: Update API Documentation

**File**: `docs/API.md`

- [ ] Document all four cluster management tools
- [ ] Add input/output schemas
- [ ] Add error response examples
- [ ] Document connection state behavior

**Acceptance**: API docs complete for cluster management

### Task 5.4: Migration Guide

**File**: `docs/MIGRATION.md` (new file)

- [ ] Document backward compatibility
- [ ] Explain behavior changes
- [ ] Provide migration examples
- [ ] Document new error responses

**Acceptance**: Migration path clearly documented

## Phase 6: Testing and Validation

### Task 6.1: End-to-End Integration Tests

**File**: `test/integration/cluster_connection_test.go` (new file)

- [ ] Start server without cluster
- [ ] Connect dynamically
- [ ] Execute various tools
- [ ] Disconnect
- [ ] Verify errors after disconnect
- [ ] Reconnect to different cluster
- [ ] Test subscription cleanup

**Acceptance**: Complete workflow tested end-to-end

### Task 6.2: Manual Testing Checklist

- [ ] Start server without K0RDENT_MGMT_KUBECONFIG_PATH
- [ ] Verify banner shows "Not connected"
- [ ] Call cluster_status (not connected)
- [ ] List contexts from kubeconfig file
- [ ] Connect to cluster
- [ ] Verify banner could show connected state on restart
- [ ] Execute ClusterDeployment list
- [ ] Start event subscription
- [ ] Disconnect
- [ ] Verify subscription cancelled
- [ ] Verify tools return not-connected errors
- [ ] Reconnect to different cluster
- [ ] Verify tools use new connection

**Acceptance**: Manual testing validates all scenarios

### Task 6.3: Backward Compatibility Verification

- [ ] Test with K0RDENT_MGMT_KUBECONFIG_PATH set at startup
- [ ] Verify immediate connection
- [ ] Verify all tools work as before
- [ ] Verify existing deployments unaffected

**Acceptance**: Existing behavior preserved

## Dependencies

- Task 1.x can be done in parallel
- Task 2.x depends on Task 1.3 (NotConnectedError)
- Task 3.x depends on Task 2.x (ConnectionManager)
- Task 4.x depends on Task 2.x and Task 3.x
- Task 5.x can be done in parallel after Phase 4
- Task 6.x depends on all previous phases

## Estimated Effort

- Phase 1: 1-2 days
- Phase 2: 2-3 days
- Phase 3: 2-3 days
- Phase 4: 2-3 days
- Phase 5: 1 day
- Phase 6: 1-2 days

Total: 9-14 days

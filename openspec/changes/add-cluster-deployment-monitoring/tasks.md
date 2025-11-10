# Tasks: Add Cluster Deployment Monitoring

## Phase 1: Core Infrastructure (Foundation)

### 1. Create cluster monitor package structure
- [x] Completed
Create `internal/kube/cluster_monitor/` package with core types and interfaces.

**Files**:
- `internal/kube/cluster_monitor/types.go`: Progress update types, phase constants
- `internal/kube/cluster_monitor/filter.go`: Event filtering logic (stub)
- `internal/kube/cluster_monitor/phases.go`: Phase detection logic (stub)

**Deliverables**:
- `ProgressUpdate` struct with all fields (timestamp, phase, progress, message, etc.)
- `ProvisioningPhase` enum constants
- Package compiles without errors

**Validation**:
- `go build ./internal/kube/cluster_monitor`
- Unit test: `TestProgressUpdateSerialization` (JSON marshaling)

---

### 2. Implement phase detection logic
- [x] Completed
Build the core phase detection engine that maps ClusterDeployment conditions and events to logical phases.

**Files**:
- `internal/kube/cluster_monitor/phases.go`

**Logic**:
- `DetectPhase(cd *unstructured.Unstructured) ProvisioningPhase`
- Condition-based detection (priority 1): Ready, InfrastructureReady, ControlPlaneInitialized, etc.
- Fallback to Initializing if no clear signals

**Deliverables**:
- Phase detection function with 90%+ accuracy on test data

**Validation**:
- Unit tests with mock ClusterDeployment conditions:
  - `TestDetectPhase_Initializing`
  - `TestDetectPhase_Provisioning`
  - `TestDetectPhase_Bootstrapping`
  - `TestDetectPhase_Scaling`
  - `TestDetectPhase_Ready`
  - `TestDetectPhase_Failed`
- Test with real YAML fixtures from `test/fixtures/cluster_deployments/`

---

### 3. Implement event filtering logic
- [x] Completed
Build the significance filter that reduces 100+ raw events to 5-10 high-value updates.

**Files**:
- `internal/kube/cluster_monitor/filter.go`

**Logic**:
- `IsSignificantEvent(event corev1.Event, clusterName string) bool`
- Keyword-based filtering (infrastructure, control plane, node joined, etc.)
- Frequency limiting (dedup within 30s window)
- Scope filtering (involvedObject matching cluster resources)

**Deliverables**:
- Filtering function achieving 10:1 signal-to-noise ratio

**Validation**:
- Unit tests:
  - `TestFilterSignificantEvents_Include` (major checkpoints pass)
  - `TestFilterSignificantEvents_Exclude` (noisy events blocked)
  - `TestDeduplication` (suppress duplicates within 30s)
- Integration fixture: 150-event test dataset with expected 10-15 outputs

---

### 4. Add progress percentage estimation
- [x] Completed
Optional enhancement to estimate % completion based on phase and conditions.

**Files**:
- `internal/kube/cluster_monitor/phases.go`

**Logic**:
- `EstimateProgress(phase ProvisioningPhase, conditions []ConditionSummary) *int`
- Provider-agnostic estimates: Initializing=5%, Provisioning=25%, Bootstrapping=50%, Scaling=75%, Installing=90%, Ready=100%
- Fine-tuning based on condition details

**Deliverables**:
- Progress estimation within ±10% of actual completion (observational target)

**Validation**:
- Unit tests with known phase states
- Manual validation: Deploy test cluster, compare estimates to actual timing

---

## Phase 2: Subscription Manager

### 5. Create ClusterMonitorManager skeleton
- [x] Completed
Scaffold the manager that coordinates subscriptions and watches.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Structure**:
```go
type ClusterMonitorManager struct {
    mu            sync.Mutex
    server        *mcp.Server
    session       *runtime.Session
    subscriptions map[string]*clusterSubscription
}

type clusterSubscription struct {
    namespace    string
    name         string
    cancel       context.CancelFunc
    done         chan struct{}
    // ... watch interfaces, state tracking
}
```

**Deliverables**:
- Manager with `Subscribe()` and `Unsubscribe()` methods (stub implementations)
- Subscription tracking map

**Validation**:
- Compiles without errors
- Unit test: `TestClusterMonitorManager_SubscribeUnsubscribe` (lifecycle)

---

### 6. Implement ClusterDeployment watch
- [x] Completed
Add watch for the specific ClusterDeployment resource being monitored.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Use `session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).Namespace(ns).Watch()`
- Watch single resource by name (field selector)
- Handle watch events in goroutine: `streamProgress()`

**Deliverables**:
- ClusterDeployment condition changes trigger progress updates

**Validation**:
- Integration test: Watch test ClusterDeployment, verify condition updates received
- Test watch reconnect after connection loss

---

### 7. Implement namespace event watch
- [x] Completed
Add watch for Kubernetes events in the cluster's namespace, filtered by involved object.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Use existing `session.Events.WatchNamespace()` with field selector
- Filter by `involvedObject.name` matching cluster name or related resources
- Pass events through significance filter

**Deliverables**:
- Filtered events appear in progress stream

**Validation**:
- Integration test: Generate test events, verify filtering
- Test event watch with 100+ events, confirm only 10-15 published

---

### 8. Implement subscription lifecycle management
- [x] Completed
Add subscribe/unsubscribe flows with proper cleanup.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Subscribe: Parse URI, validate namespace, create watches, send initial snapshot
- Unsubscribe: Cancel context, close channels, cleanup watches, remove from map
- Auto-unsubscribe: Detect terminal states (Ready, Failed), trigger cleanup
- Timeout: Start timer on subscribe, cleanup at timeout

**Deliverables**:
- Full subscription lifecycle with automatic cleanup

**Validation**:
- Integration test: Subscribe → receive updates → auto-unsubscribe on Ready
- Test explicit unsubscribe cleans up resources
- Test timeout triggers cleanup
- Test concurrent subscriptions don't interfere

---

### 9. Add initial snapshot on subscribe
- [x] Completed
Send current cluster state + recent events immediately after subscribing.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Get ClusterDeployment current status
- List recent events (last 5 minutes)
- Detect current phase
- Send initial `ProgressUpdate` with current state

**Deliverables**:
- Clients receive immediate context on subscribe

**Validation**:
- Integration test: Subscribe to in-progress cluster, verify initial snapshot received
- Test snapshot includes phase, recent events, progress estimate

---

## Phase 3: MCP Integration

### 10. Register resource template and subscription handler
- [x] Completed
Integrate with MCP Server's subscription router.

**Files**:
- `internal/tools/core/cluster_monitor.go`
- `internal/tools/core/register.go`

**Logic**:
- Register `mcp.ResourceTemplate` with URI template `k0rdent://cluster-monitor/{namespace}/{name}`
- Register ClusterMonitorManager with SubscriptionRouter (host="cluster-monitor")
- Add manager initialization in `Register()` function

**Deliverables**:
- Resource template discoverable via `resources/list`
- Subscriptions routed to ClusterMonitorManager

**Validation**:
- Start server, list resources, verify template present
- Subscribe via MCP client, verify routing works

---

### 10a. Expose cluster state via tool call
- [x] Completed
Add a standard MCP tool so clients can fetch the latest monitoring snapshot without opening a subscription.

**Files**:
- `internal/tools/core/cluster_monitor.go`
- `docs/features/cluster-monitoring.md`

**Deliverables**:
- Tool `k0rdent.mgmt.clusterDeployments.getState` accepts namespace/name and returns a `ProgressUpdate`
- Validates namespace filters and handles not-found errors cleanly
- Documented usage example in the cluster monitoring guide

**Validation**:
- Unit tests covering success and not-found paths
- Manual `tools/call` verifies response format

---

### 10b. Add service state extraction to ProgressUpdate
- [ ] Todo
Enhance the ProgressUpdate structure and getState tool to include detailed per-service state information from `.status.services`.

**Files**:
- `internal/kube/cluster_monitor/types.go`: Add ServiceStatus type
- `internal/clusters/summary.go`: Add ExtractServiceStatuses function
- `internal/tools/core/cluster_monitor.go`: Update buildClusterProgress to include services

**New Types**:
```go
type ServiceStatus struct {
    Name              string              `json:"name"`
    Namespace         string              `json:"namespace"`
    Template          string              `json:"template"`
    State             string              `json:"state"`             // Ready, Pending, Failed, Upgrading, etc.
    Type              string              `json:"type,omitempty"`   // Helm, etc.
    Version           string              `json:"version,omitempty"`
    Conditions        []ConditionSummary  `json:"conditions,omitempty"`
    LastTransitionTime *time.Time         `json:"lastTransitionTime,omitempty"`
}

type ProgressUpdate struct {
    // ... existing fields ...
    Services []ServiceStatus `json:"services,omitempty"`  // NEW
}
```

**Logic**:
- `ExtractServiceStatuses(obj *unstructured.Unstructured) []ServiceStatus`
  - Iterate through `.status.services` array
  - Extract name, namespace, template, state, type, version from each entry
  - Extract conditions array (type, status, reason, message, lastTransitionTime)
  - Handle missing/empty status.services gracefully (return empty slice)
- Update `buildClusterProgress()` to call ExtractServiceStatuses and populate Services field

**Deliverables**:
- ServiceStatus type with all required fields
- Service extraction function returning detailed per-service state
- getState tool response includes services array with full details
- Empty clusters (no services) return empty services array without errors

**Validation**:
- Unit tests:
  - `TestExtractServiceStatuses_MultipleServices` (3 services with different states)
  - `TestExtractServiceStatuses_EmptyServices` (no services deployed)
  - `TestExtractServiceStatuses_MissingStatusServices` (.status.services absent)
  - `TestServiceStatusWithConditions` (service with failure conditions)
- Integration test: Deploy cluster with 2 services, verify getState returns both with correct states
- Manual verification: Call getState on demo-cluster, inspect services array in response

---

### 11. Implement progress update publishing
Publish `ResourceUpdated` notifications with progress deltas.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Convert `ProgressUpdate` to JSON
- Wrap in `ResourceUpdatedNotificationParams` with `Meta.delta`
- Call `server.ResourceUpdated()`
- Rate-limit: max 1 update per 10 seconds (except phase transitions)

**Deliverables**:
- Progress updates delivered to subscribed clients

**Validation**:
- Integration test: Subscribe, verify `ResourceUpdated` notifications received
- Test rate limiting: rapid events don't flood client
- Test phase transitions always published

---

### 12. Add namespace authorization checks
Enforce session namespace filter on subscription requests.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- In `Subscribe()`: Check `session.NamespaceFilter.MatchString(namespace)`
- Reject if namespace not allowed
- Allow if DEV_ALLOW_ANY mode (nil filter or matches all)

**Deliverables**:
- Unauthorized namespace subscriptions rejected

**Validation**:
- Integration test: Session with filter="team-a", attempt subscribe to "team-b", verify rejection
- Test DEV_ALLOW_ANY mode allows all namespaces

---

### 13. Implement resource limits
Enforce per-client and global subscription limits.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Track per-client count (keyed by session ID or connection ID)
- Track global count across all clients
- Reject new subscriptions if limits exceeded

**Configuration**:
- Per-client max: 10
- Global max: 100

**Deliverables**:
- Subscription limits enforced

**Validation**:
- Integration test: Create 11 subscriptions from single client, verify 11th rejected
- Test global limit with multiple mock clients

---

## Phase 4: Error Handling & Resilience

### 14. Add watch reconnection logic
Handle watch connection failures with exponential backoff.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Detect watch error/closure
- Publish warning update to client
- Retry watch creation (3 attempts, exponential backoff: 1s, 2s, 4s)
- If all retries fail, publish error and cleanup subscription

**Deliverables**:
- Subscriptions survive transient watch failures

**Validation**:
- Integration test: Simulate watch connection loss, verify reconnect
- Test exhausted retries trigger cleanup

---

### 15. Handle ClusterDeployment deletion during watch
Gracefully handle cluster being deleted while monitoring.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Detect watch event type `DELETED`
- Publish warning update: "ClusterDeployment deleted"
- Cleanup subscription

**Deliverables**:
- Deletion during monitoring handled gracefully

**Validation**:
- Integration test: Subscribe, delete cluster, verify deletion update + cleanup

---

### 16. Add timeout warning and enforcement
Warn clients before timeout, then enforce cleanup.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Logic**:
- Start timeout timer (default: 60 minutes) on subscribe
- At timeout-5min, publish warning update
- At timeout, publish "timeout exceeded" update, cleanup

**Configuration**:
- Default timeout: 60 minutes
- Warning threshold: 5 minutes before timeout

**Deliverables**:
- Long-running subscriptions automatically terminate

**Validation**:
- Integration test: Mock timeout with 10s limit, verify warning at 5s, cleanup at 10s

---

## Phase 5: Testing & Validation

### 17. Add unit tests for filtering and phase detection
Comprehensive unit tests for core logic.

**Files**:
- `internal/kube/cluster_monitor/filter_test.go`
- `internal/kube/cluster_monitor/phases_test.go`

**Coverage**:
- Event filtering: significant vs. noisy events
- Phase detection: all phase states with various condition combinations
- Progress estimation: verify % ranges

**Target**: 90%+ code coverage for cluster_monitor package

**Validation**:
- `go test ./internal/kube/cluster_monitor -cover`
- All tests pass

---

### 18. Add integration test for full subscription lifecycle
End-to-end test of subscribe → monitor → auto-unsubscribe.

**Files**:
- `test/integration/cluster_monitor_test.go`

**Scenarios**:
- Deploy test cluster with monitoring active
- Verify 5-10 progress updates received
- Verify phase transitions observed
- Verify auto-unsubscribe on Ready

**Requires**: Live test cluster (mock or real)

**Validation**:
- `go test ./test/integration -run TestClusterMonitor_Lifecycle`
- Test passes with real ClusterDeployment

---

### 19. Add cross-provider compatibility tests
Test monitoring across AWS, Azure, GCP providers.

**Files**:
- `test/integration/cluster_monitor_provider_test.go`

**Scenarios**:
- Deploy minimal cluster on each provider
- Monitor with same subscription logic
- Verify phase detection works across all
- Verify filtering tuned appropriately

**Requires**: Provider credentials and quota for test clusters

**Validation**:
- Tests pass on all three providers
- Phase detection accuracy ≥90%

---

### 20. Manual testing with real Azure deployment
Deploy a real Azure cluster and observe monitoring quality.

**Procedure**:
1. Deploy minimal Azure cluster (1 control plane, 1 worker) via MCP tool
2. Subscribe to `k0rdent://cluster-monitor/kcm-system/<cluster-name>`
3. Observe progress updates in real-time
4. Verify 5-10 meaningful updates received
5. Confirm auto-unsubscribe on Ready
6. Review logs for any errors or warnings

**Success Criteria**:
- Monitoring provides clear progress visibility
- No false positives (premature Ready detection)
- No missed terminal states
- Filtering eliminates noise effectively

---

## Phase 6: Documentation & Polish

### 21. Add usage documentation
- [x] Completed
Document the cluster monitoring feature for users.

**Files**:
- `docs/features/cluster-monitoring.md` (new file)

**Content**:
- Feature overview
- Subscription URI format
- Example usage with Claude Code or other MCP clients
- Troubleshooting common issues

**Validation**:
- Doc review by project maintainers

---

### 22. Update README with monitoring feature
- [x] Completed
Add cluster monitoring to main project README.

**Files**:
- `README.md`

**Changes**:
- Add "Cluster Deployment Monitoring" to features list
- Link to detailed documentation
- Include quick example

**Validation**:
- README renders correctly in GitHub

---

### 23. Add metrics and observability (future enhancement)
Optional: Export Prometheus metrics for monitoring system health.

**Files**:
- `internal/tools/core/cluster_monitor.go`

**Metrics**:
- `k0rdent_cluster_monitor_subscriptions_active` (gauge)
- `k0rdent_cluster_monitor_updates_published_total` (counter)
- `k0rdent_cluster_monitor_timeouts_total` (counter)

**Deliverables**:
- Metrics exposed on `/metrics` endpoint

**Validation**:
- Query metrics, verify values accurate

---

## Phase 7: Finalization

### 24. Review and optimize performance
Profile the monitoring system under load.

**Actions**:
- Run 50 concurrent cluster monitoring subscriptions
- Profile memory usage (should be <50MB total)
- Profile CPU usage
- Check for goroutine leaks
- Verify watch connection limits not exceeded

**Validation**:
- No memory leaks detected
- CPU usage reasonable (<5% idle, <30% under load)
- All goroutines cleaned up on unsubscribe

---

### 25. Security review
Review authorization, input validation, and resource limits.

**Checklist**:
- [ ] Namespace filter enforced on all subscriptions
- [ ] URI parsing validates input (no injection risks)
- [ ] Resource limits prevent DoS
- [ ] Watch permissions checked (RBAC)
- [ ] No sensitive data leaked in progress updates

**Validation**:
- Security checklist 100% complete

---

### 26. Update OpenSpec to "complete"
Mark the change as complete in OpenSpec.

**Actions**:
- Run `openspec show add-cluster-deployment-monitoring` to verify all tasks done
- Update proposal status to "Complete"
- Archive proposal with `openspec archive add-cluster-deployment-monitoring`

**Validation**:
- `openspec list` shows change as complete

---

## Summary

**Total Tasks**: 26
**Estimated Effort**: 3-5 days (developer experienced with Go, Kubernetes, MCP)

**Critical Path**:
1. Phase 1 (Core Infrastructure): Tasks 1-4 → Foundation for filtering/detection
2. Phase 2 (Subscription Manager): Tasks 5-9 → Core functionality
3. Phase 3 (MCP Integration): Tasks 10-13 → Expose to clients
4. Phase 4 (Error Handling): Tasks 14-16 → Production-ready resilience
5. Phase 5 (Testing): Tasks 17-20 → Validation
6. Phase 6-7 (Docs/Polish): Tasks 21-26 → Release preparation

**Dependencies**:
- No external dependencies beyond existing codebase
- Builds on established patterns (EventManager, PodLogManager)
- Can be developed incrementally with feature flags if needed

**Parallelization Opportunities**:
- Tasks 1-4 (core logic) can be developed concurrently
- Tasks 5-9 (manager) can overlap with Phase 1 testing
- Tasks 17-20 (testing) can run in parallel once code complete

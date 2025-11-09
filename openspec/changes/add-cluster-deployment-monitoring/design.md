# Design: Cluster Deployment Monitoring

## Architecture Overview

```
┌──────────────┐
│  MCP Client  │
│ (Claude Code)│
└──────┬───────┘
       │ Subscribe(k0rdent://cluster-monitor/ns/name)
       │ ← ResourceUpdated notifications (progress deltas)
       ▼
┌──────────────────────────────────────────────────────────┐
│            MCP Server (k0rdent-mcp-server)               │
│                                                           │
│  ┌────────────────────────────────────────────────────┐  │
│  │         ClusterMonitorManager                      │  │
│  │                                                    │  │
│  │  • Manages active subscriptions                   │  │
│  │  • Coordinates multiple watches                   │  │
│  │  • Filters & enriches events                      │  │
│  │  • Detects progress phases                        │  │
│  │  • Auto-cleanup on completion                     │  │
│  └────────────┬──────────┬──────────┬─────────────────┘  │
│               │          │          │                     │
└───────────────┼──────────┼──────────┼─────────────────────┘
                │          │          │
                ▼          ▼          ▼
        ┌───────────┐ ┌─────────┐ ┌──────────┐
        │ CD Watch  │ │  Event  │ │   Pod    │
        │           │ │  Watch  │ │   Logs   │
        │ (Status)  │ │  (NS)   │ │ (Optional)│
        └─────┬─────┘ └────┬────┘ └────┬─────┘
              │            │           │
              ▼            ▼           ▼
      ┌────────────────────────────────────────┐
      │   Kubernetes API (Management Cluster)   │
      │                                          │
      │  • ClusterDeployment CRD                │
      │  • Namespace Events                      │
      │  • CAPI Controller Pods                  │
      └────────────────────────────────────────┘
```

## Component Design

### 1. ClusterMonitorManager

**Purpose**: Centralized manager for cluster provisioning subscriptions, coordinating multiple Kubernetes watches and publishing filtered progress updates.

**Key Responsibilities**:
- Subscribe/Unsubscribe lifecycle management
- Multi-watch coordination (ClusterDeployment + Events + optional Logs)
- Event filtering and enrichment
- Progress phase detection and synthesis
- Automatic cleanup on terminal states
- Timeout handling

**State Management**:
```go
type ClusterMonitorManager struct {
    mu            sync.Mutex
    server        *mcp.Server
    session       *runtime.Session
    subscriptions map[string]*clusterSubscription  // key: "namespace/name"
}

type clusterSubscription struct {
    namespace      string
    name           string
    cancel         context.CancelFunc
    done           chan struct{}

    // Watches
    cdWatch        watch.Interface      // ClusterDeployment watch
    eventWatch     watch.Interface      // Namespace events watch

    // State tracking
    currentPhase   ProvisioningPhase
    lastUpdate     time.Time
    seenEvents     map[string]bool      // Deduplication

    // Configuration
    timeout        time.Duration
    startTime      time.Time
}
```

### 2. Event Filtering Strategy

**Goal**: Reduce 100+ raw events to 5-10 meaningful progress updates.

**Filtering Layers**:

1. **Scope Filter** (Pre-filter):
   - Events: Only events where `involvedObject.name` contains cluster name OR `involvedObject.namespace` matches cluster namespace
   - ClusterDeployment: Only watch the specific CD resource

2. **Significance Filter** (Content-based):
   ```go
   // High-significance event patterns
   significantPatterns := []string{
       // Infrastructure
       "Creating infrastructure",
       "Infrastructure created",
       "Network configured",

       // Control Plane
       "Bootstrapping control plane",
       "Control plane initialized",
       "Control plane ready",

       // Workers
       "Scaling machine deployment",
       "Machine ready",
       "Node joined cluster",

       // Services
       "Installing service template",
       "Service template ready",

       // Terminal states
       "Cluster ready",
       "Provisioning failed",
   }
   ```

3. **Frequency Filter** (Time-based):
   - Suppress duplicate events within 30s window
   - Rate-limit updates to max 1 per 10 seconds
   - Always publish condition changes (`.status.conditions`)

4. **Phase Transition Filter**:
   - Always publish when phase changes (e.g., Provisioning → Bootstrapping)
   - Optional: Suppress sub-phase events if phase-level updates sufficient

**Event Enrichment**:
```go
type ProgressUpdate struct {
    Timestamp     time.Time          `json:"timestamp"`
    Phase         ProvisioningPhase  `json:"phase"`
    Progress      *int               `json:"progress,omitempty"`  // % estimate
    Message       string             `json:"message"`
    Source        string             `json:"source"`  // "condition", "event", "log"
    Severity      string             `json:"severity"` // "info", "warning", "error"
    RelatedObject *ObjectReference   `json:"relatedObject,omitempty"`
}

type ProvisioningPhase string
const (
    PhaseInitializing   ProvisioningPhase = "Initializing"
    PhaseProvisioning   ProvisioningPhase = "Provisioning"
    PhaseBootstrapping  ProvisioningPhase = "Bootstrapping"
    PhaseScaling        ProvisioningPhase = "Scaling"
    PhaseInstalling     ProvisioningPhase = "Installing"
    PhaseReady          ProvisioningPhase = "Ready"
    PhaseFailed         ProvisioningPhase = "Failed"
)
```

### 3. Progress Phase Detection

**Phase Mapping Logic**:

```go
func detectPhase(cd *unstructured.Unstructured, events []corev1.Event) ProvisioningPhase {
    // Priority 1: ClusterDeployment conditions
    conditions := extractConditions(cd)

    if hasCondition(conditions, "Ready", "True") {
        return PhaseReady
    }
    if hasCondition(conditions, "Ready", "False", "Failed") {
        return PhaseFailed
    }

    // Priority 2: Condition types indicate phase
    if hasCondition(conditions, "InfrastructureReady", "False") {
        return PhaseProvisioning
    }
    if hasCondition(conditions, "ControlPlaneInitialized", "False") {
        return PhaseBootstrapping
    }
    if hasCondition(conditions, "WorkersJoined", "False") {
        return PhaseScaling
    }
    if hasCondition(conditions, "ServicesInstalled", "False") {
        return PhaseInstalling
    }

    // Priority 3: Recent events contain phase keywords
    recentEvents := filterRecent(events, 2*time.Minute)
    if containsAny(recentEvents, []string{"bootstrap", "control plane"}) {
        return PhaseBootstrapping
    }
    if containsAny(recentEvents, []string{"machine ready", "node joined"}) {
        return PhaseScaling
    }

    // Default: initializing
    return PhaseInitializing
}
```

**Progress Percentage Estimation** (Optional):
```go
func estimateProgress(phase ProvisioningPhase, conditions []ConditionSummary) *int {
    // Provider-agnostic estimates based on phase
    progressMap := map[ProvisioningPhase]int{
        PhaseInitializing:  5,
        PhaseProvisioning:  25,
        PhaseBootstrapping: 50,
        PhaseScaling:       75,
        PhaseInstalling:    90,
        PhaseReady:         100,
        PhaseFailed:        0,
    }

    base := progressMap[phase]

    // Refine based on condition details
    if phase == PhaseBootstrapping {
        if hasCondition(conditions, "ControlPlaneInitialized", "True") {
            base = 60  // Bootstrap complete, transitioning
        }
    }

    return &base
}
```

### 4. Subscription Lifecycle

**Subscribe Flow**:
```
1. Client → Subscribe(k0rdent://cluster-monitor/namespace/name)
2. Parse URI, validate cluster exists
3. Check namespace permissions (session.NamespaceFilter)
4. Create clusterSubscription with:
   - ClusterDeployment watch (single resource)
   - Namespace event watch (filtered by involved object)
5. Start goroutine: streamProgress()
6. Send initial snapshot (current state + recent events)
7. Return success
```

**StreamProgress Loop**:
```
for {
    select {
    case cdEvent := <-cdWatchCh:
        // Parse condition changes
        update := processConditionChange(cdEvent)
        if shouldPublish(update) {
            publishUpdate(update)
        }

        // Check terminal state
        if isTerminal(cdEvent) {
            publishFinalUpdate(cdEvent)
            cleanup()
            return
        }

    case k8sEvent := <-eventWatchCh:
        // Filter by significance
        if isSignificant(k8sEvent) {
            update := processEvent(k8sEvent)
            if shouldPublish(update) {
                publishUpdate(update)
            }
        }

    case <-timeoutTimer:
        publishWarning("Provisioning timeout approaching")
        cleanup()
        return

    case <-ctx.Done():
        return
    }
}
```

**Auto-Cleanup Triggers**:
- ClusterDeployment Ready=True
- ClusterDeployment Ready=False with terminal reason (Failed, etc.)
- Timeout exceeded (configurable, default 60 minutes)
- Client unsubscribe
- ClusterDeployment deleted

### 5. Subscription URI Format

**Template**: `k0rdent://cluster-monitor/{namespace}/{name}[?options]`

**Examples**:
```
k0rdent://cluster-monitor/kcm-system/my-azure-cluster
k0rdent://cluster-monitor/team-a/prod-cluster?timeout=3600
```

**Query Parameters** (Future):
- `timeout`: Max subscription duration in seconds (default: 3600)
- `verbosity`: Filter level - `minimal`, `default`, `verbose` (default: `default`)
- `includeLogs`: Include pod logs from provisioning controllers (default: false)

### 6. Error Handling

**Scenarios**:

1. **Cluster Not Found**:
   - Return error immediately on subscribe
   - Do not create subscription

2. **Permission Denied**:
   - Check namespace filter on subscribe
   - Return authorization error if filtered

3. **Watch Connection Lost**:
   - Publish warning notification
   - Attempt reconnect (3 retries with exponential backoff)
   - If all retries fail, publish error and cleanup

4. **ClusterDeployment Deleted During Watch**:
   - Publish "Cluster deleted" update with severity=warning
   - Cleanup subscription

5. **Timeout Exceeded**:
   - Publish warning 5 minutes before timeout
   - Publish "Monitoring timeout exceeded" at timeout
   - Cleanup subscription

### 7. Performance Considerations

**Resource Usage**:
- Each subscription: 2-3 Kubernetes watches (CD + Events + optional Logs)
- Estimated memory: ~1MB per active subscription
- Limit: Max 100 concurrent cluster monitoring subscriptions per server instance

**Watch Efficiency**:
- Use label selectors on event watch to reduce traffic
- Single ClusterDeployment watch (resource version tracking)
- Event watch scoped to namespace with field selector

**Deduplication**:
- Track seen event UIDs in memory (cleared on cleanup)
- Time-based deduplication window (30 seconds)

### 8. Integration Points

**Existing Components**:
- `SubscriptionRouter` (subscriptions.go): Register "cluster-monitor" host
- `runtime.Session`: Reuse Events provider for namespace event access
- `clusters.SummarizeClusterDeployment()`: Leverage existing status parsing

**New Components**:
```
internal/tools/core/cluster_monitor.go      # ClusterMonitorManager + tool registration
internal/kube/cluster_monitor/              # New package
    ├── filter.go                          # Event filtering logic
    ├── phases.go                          # Phase detection & estimation
    └── progress.go                        # Progress update types
```

### Snapshot Tool

In addition to the streaming API, clients often need a quick health check (e.g., after reconnecting or before deciding to subscribe). We expose a minimal tool call:

- **Tool**: `k0rdent.mgmt.clusterDeployments.getState`
- **Inputs**: `namespace`, `name` of the ClusterDeployment (namespace defaults to the session's global namespace if omitted)
- **Output**: The same `ProgressUpdate` structure used for streaming payloads, stamped with the request time
- **Validation**:
  - Enforces namespace filters identical to the subscription path
  - Returns `NotFound` errors verbatim when the ClusterDeployment is missing
  - Reuses `buildClusterProgress` to ensure consistent phase/progress calculations

This tool is wired through `registerClusterMonitor`, making it discoverable alongside the resource template while keeping implementation localized to the cluster monitor package.

## Security & Authorization

**Permissions Required**:
- Watch ClusterDeployment in subscribed namespace
- List/Watch Events in subscribed namespace
- Optional: Get/List Pods + Get Logs (if log streaming enabled)

**Namespace Filtering**:
- Respect `session.NamespaceFilter` from OIDC token claims
- Reject subscriptions for disallowed namespaces
- Same authorization model as existing event subscriptions

**Resource Limits**:
- Max subscriptions per client connection: 10
- Global max subscriptions per server: 100
- Timeout enforcement: All subscriptions auto-cleanup after 60 minutes

## Testing Strategy

### Unit Tests
- Phase detection logic with various condition combinations
- Event filtering heuristics (include/exclude decisions)
- Progress percentage estimation accuracy
- Deduplication with overlapping events

### Integration Tests
1. **Happy Path**:
   - Deploy test cluster
   - Subscribe to monitor
   - Verify 5-10 progress updates received
   - Verify auto-unsubscribe on Ready

2. **Failure Scenarios**:
   - Invalid credential triggers failure phase
   - Quota exceeded triggers failure phase
   - Verify error messages propagated

3. **Timeout Handling**:
   - Mock slow provisioning
   - Verify warning before timeout
   - Verify cleanup at timeout

4. **Concurrent Subscriptions**:
   - Deploy 5 clusters simultaneously
   - Subscribe to all 5
   - Verify no cross-contamination of updates

### Manual Testing with Real Providers
- Deploy minimal clusters on Azure, AWS, GCP
- Observe quality of progress updates
- Tune filtering thresholds based on observed noise

## Future Enhancements

1. **Historical Replay**: Support reading past progress for completed deployments
2. **Multi-Cluster View**: Single subscription monitoring multiple clusters (dashboard use case)
3. **Custom Filters**: Let clients specify event filters via query params
4. **Webhook Integration**: Alternative to MCP subscriptions for external integrations
5. **Metrics Export**: Prometheus metrics for provisioning duration, failure rates
6. **AI-Powered Insights**: LLM analysis of failure patterns ("Your quota is exceeded in region X")

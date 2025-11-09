# Event Filtering Specification

**Based on**: Live observation of Azure cluster provisioning (2025-11-09)
**Validation**: Real deployment generated ~150 events, filtered to 12-15 high-value updates

## Filtering Architecture

```
Raw Kubernetes Events (~150)
         ↓
    Scope Filter (namespace + involvedObject matching)
         ↓
    Significance Filter (pattern matching)
         ↓
    Frequency Filter (time-based deduplication)
         ↓
    Phase Transition Filter (always emit)
         ↓
Progress Updates (12-15 meaningful messages)
```

## 1. Scope Filter

**Purpose**: Pre-filter events to only those related to the monitored cluster.

```go
func isInScope(event corev1.Event, clusterName, namespace string) bool {
    // Must be in the correct namespace
    if event.InvolvedObject.Namespace != namespace {
        return false
    }

    // Event must reference the cluster or its child resources
    return event.InvolvedObject.Name == clusterName ||
           strings.HasPrefix(event.InvolvedObject.Name, clusterName+"-")
}
```

## 2. Significance Filter

**Purpose**: Include only events that represent major provisioning checkpoints.

### High-Value Event Patterns (EMIT)

```go
var significantEventPatterns = []EventPattern{
    // === Phase 1: Initialization ===
    {
        Reason:          "HelmReleaseCreated",
        InvolvedKind:    "ClusterDeployment",
        Phase:           PhaseInitializing,
        Message:         "Helm release created for cluster deployment",
    },
    {
        Reason:          "InstallSucceeded",
        InvolvedKind:    "HelmRelease",
        Phase:           PhaseInitializing,
        Message:         "Helm chart installation succeeded",
    },
    {
        Reason:          "Provisioning",
        InvolvedKind:    "Cluster",
        Phase:           PhaseProvisioning,
        Message:         "Cluster provisioning started",
    },

    // === Phase 2: Infrastructure Provisioning ===
    {
        Reason:          "BeginCreateOrUpdate",
        InvolvedKind:    "ResourceGroup",
        Phase:           PhaseProvisioning,
        MessageTemplate: "Azure resource group creation started: {{.ResourceID}}",
    },
    {
        Reason:          "BeginCreateOrUpdate",
        InvolvedKind:    "VirtualNetwork",
        Phase:           PhaseProvisioning,
        MessageTemplate: "Virtual network creation started: {{.Name}}",
    },
    {
        Reason:          "BeginCreateOrUpdate",
        InvolvedKind:    "Subnet",
        Phase:           PhaseProvisioning,
        MessageTemplate: "Subnet creation started: {{.Name}}",
    },
    {
        Reason:          "SuccessfulCreate",
        InvolvedKind:    "MachineDeployment",
        Phase:           PhaseProvisioning,
        Message:         "Worker node deployment created",
    },
    {
        Reason:          "SuccessfulCreate",
        InvolvedKind:    "MachineSet",
        MessageContains: "Created machine",
        Phase:           PhaseProvisioning,
        Message:         "Machine provisioning started",
    },

    // === Phase 3: Control Plane Bootstrap ===
    {
        Reason:          "ControlPlaneReady",
        InvolvedKind:    "K0sControlPlane",
        Phase:           PhaseBootstrapping,
        Message:         "Control plane nodes operational",
    },
    {
        Reason:          "MachineReady",
        InvolvedKind:    "Machine",
        LabelsContain:   "control-plane",
        Phase:           PhaseBootstrapping,
        MessageTemplate: "Control plane machine ready: {{.Name}}",
    },

    // === Phase 4: Worker Scaling ===
    {
        Reason:          "MachineReady",
        InvolvedKind:    "Machine",
        LabelsContain:   "worker",
        Phase:           PhaseScaling,
        MessageTemplate: "Worker machine ready: {{.Name}}",
    },
    {
        Reason:          "NodeJoined",
        InvolvedKind:    "Machine",
        Phase:           PhaseScaling,
        MessageTemplate: "Node joined cluster: {{.NodeName}}",
    },

    // === Phase 5: Service Installation ===
    {
        Reason:          "ServiceInstalling",
        InvolvedKind:    "ServiceSet",
        Phase:           PhaseInstalling,
        MessageTemplate: "Installing service: {{.ServiceName}}",
    },
    {
        Reason:          "ServiceReady",
        InvolvedKind:    "ServiceSet",
        Phase:           PhaseInstalling,
        MessageTemplate: "Service ready: {{.ServiceName}}",
    },

    // === Phase 6: Ready ===
    {
        Reason:          "CAPIClusterIsReady",
        InvolvedKind:    "ClusterDeployment",
        Phase:           PhaseReady,
        Message:         "Cluster fully provisioned and operational",
        Terminal:        true,
    },
}
```

### Noisy Events (SUPPRESS)

```go
var suppressedEventPatterns = []string{
    // Internal reconciliation noise
    "OwnerRefNotSet",
    "Machine controller dependency not yet met",
    "ServiceSetEnsureProfileFailed",
    "ServiceSetCollectServiceStatusesFailed",

    // Routine controller operations
    "ArtifactUpToDate",

    // Non-blocking warnings
    "VMIdentityNone",

    // Repetitive status messages (captured in conditions instead)
    "WaitingForControlPlaneInitialization",
    "WaitingForClusterInfrastructure",
}

func isSuppressed(event corev1.Event) bool {
    for _, pattern := range suppressedEventPatterns {
        if event.Reason == pattern || strings.Contains(event.Message, pattern) {
            return true
        }
    }

    // Suppress transient reconciliation failures (unless persistent)
    if strings.Contains(event.Reason, "ReconcilerFailed") && event.Type == "Warning" {
        return true  // Will be caught by condition monitoring if critical
    }

    return false
}
```

## 3. Frequency Filter (Time-Based Deduplication)

**Purpose**: Prevent flooding from repeated events.

```go
type DeduplicationRule struct {
    Reason string
    Window time.Duration
}

var deduplicationRules = []DeduplicationRule{
    {Reason: "CAPIClusterIsProvisioning", Window: 60 * time.Second},
    {Reason: "ServiceSetCollectServiceStatusesFailed", Window: 300 * time.Second},
    {Reason: "ClusterReconcilerNormalFailed", Window: 120 * time.Second},
}

type EventDeduplicator struct {
    lastSeen map[string]time.Time
    mu       sync.Mutex
}

func (d *EventDeduplicator) ShouldEmit(event corev1.Event) bool {
    d.mu.Lock()
    defer d.mu.Unlock()

    key := fmt.Sprintf("%s/%s/%s", event.Reason, event.InvolvedObject.Kind, event.InvolvedObject.Name)

    // Check if event is subject to deduplication
    for _, rule := range deduplicationRules {
        if event.Reason == rule.Reason {
            lastTime, exists := d.lastSeen[key]
            if exists && time.Since(lastTime) < rule.Window {
                return false  // Suppress: too soon since last emission
            }
            d.lastSeen[key] = time.Now()
            return true
        }
    }

    // Not subject to deduplication, emit
    return true
}
```

## 4. Phase Transition Filter

**Purpose**: Always emit updates when the cluster transitions between phases, bypassing rate limits.

```go
func isPhaseTransition(currentPhase, previousPhase ProvisioningPhase) bool {
    return currentPhase != previousPhase && currentPhase != ""
}

// Phase transitions ALWAYS bypass rate limiting
if isPhaseTransition(newPhase, sub.currentPhase) {
    sub.currentPhase = newPhase
    publishUpdate(update)  // Immediate emission
    return
}
```

## 5. Rate Limiting

**Purpose**: Prevent update flooding while allowing critical updates.

```go
const (
    minUpdateInterval = 15 * time.Second  // Minimum time between updates
    maxBurstUpdates   = 3                 // Allow burst of 3 updates, then enforce interval
)

type RateLimiter struct {
    lastUpdate  time.Time
    burstCount  int
    mu          sync.Mutex
}

func (r *RateLimiter) AllowUpdate(isPhaseTransition bool) bool {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Phase transitions bypass rate limiting
    if isPhaseTransition {
        r.burstCount = 0
        r.lastUpdate = time.Now()
        return true
    }

    now := time.Now()
    elapsed := now.Sub(r.lastUpdate)

    // Allow burst
    if r.burstCount < maxBurstUpdates {
        r.burstCount++
        r.lastUpdate = now
        return true
    }

    // Enforce minimum interval
    if elapsed >= minUpdateInterval {
        r.burstCount = 0
        r.lastUpdate = now
        return true
    }

    return false  // Rate limited
}
```

## Complete Filtering Pipeline

```go
func (m *ClusterMonitorManager) processEvent(event corev1.Event) {
    // 1. Scope Filter
    if !isInScope(event, m.clusterName, m.namespace) {
        return
    }

    // 2. Significance Filter
    if isSuppressed(event) {
        return
    }

    significantPattern := matchSignificantPattern(event)
    if significantPattern == nil {
        return  // Not a significant event
    }

    // 3. Frequency Filter (Deduplication)
    if !m.deduplicator.ShouldEmit(event) {
        return
    }

    // 4. Phase Detection
    newPhase := detectPhaseFromEvent(event, significantPattern.Phase)
    isTransition := isPhaseTransition(newPhase, m.currentPhase)

    // 5. Rate Limiting
    if !m.rateLimiter.AllowUpdate(isTransition) {
        return
    }

    // 6. Build and Publish Update
    update := buildProgressUpdate(event, significantPattern, newPhase)
    m.publishUpdate(update)

    // Update internal state
    if isTransition {
        m.currentPhase = newPhase
    }
}
```

## Observed Filtering Effectiveness

From live Azure deployment monitoring:

| Stage | Events | Result |
|-------|--------|--------|
| Raw Kubernetes Events | ~150 | Input |
| After Scope Filter | ~80 | 47% reduction |
| After Significance Filter | ~25 | 69% reduction (from step 2) |
| After Frequency Filter | ~15 | 40% reduction (from step 3) |
| **Final Progress Updates** | **12-15** | **90% total reduction** ✅ |

**Target Met**: 10:1 signal-to-noise ratio achieved ✅

## Example: Real Event Processing

### Input: Raw Kubernetes Event
```yaml
kind: Event
metadata:
  name: monitor-test-cluster.18764d8ba3f1030e
  namespace: kcm-system
type: Normal
reason: InstallSucceeded
involvedObject:
  kind: HelmRelease
  name: monitor-test-cluster
  namespace: kcm-system
message: "Helm install succeeded for release kcm-system/monitor-test-cluster.v1 with chart azure-standalone-cp@1.0.15"
lastTimestamp: "2025-11-09T09:46:06Z"
```

### Processing Pipeline
1. ✅ **Scope Filter**: `involvedObject.name` matches cluster name
2. ✅ **Significance Filter**: Matches pattern `InstallSucceeded` / `HelmRelease`
3. ✅ **Frequency Filter**: First occurrence, no deduplication needed
4. ✅ **Phase Detection**: Event indicates `PhaseInitializing`
5. ✅ **Rate Limit**: Within burst allowance
6. ✅ **Emit**: Publish progress update

### Output: Progress Update
```json
{
  "timestamp": "2025-11-09T09:46:06Z",
  "phase": "Initializing",
  "message": "Helm chart installation succeeded",
  "source": "event",
  "severity": "info",
  "relatedObject": {
    "kind": "HelmRelease",
    "name": "monitor-test-cluster",
    "namespace": "kcm-system"
  }
}
```

## Testing & Validation

### Unit Test Coverage
- `TestScopeFilter_MatchesClusterResources`
- `TestSignificanceFilter_IncludesKeyEvents`
- `TestSignificanceFilter_SuppressesNoise`
- `TestFrequencyFilter_Deduplicates`
- `TestRateLimiter_AllowsPhaseTransitions`
- `TestRateLimiter_EnforcesMinInterval`

### Integration Test
- Deploy real cluster, count emitted updates
- Assert: 10-20 updates for full provisioning lifecycle
- Assert: No duplicate events within dedup window
- Assert: Phase transitions always emitted

### Performance Test
- Process 1000 events through pipeline
- Assert: <5ms per event
- Assert: Memory stable (no leaks from dedup cache)

# Capability: Cluster Deployment Monitoring

## ADDED Requirements

### Requirement: ClusterDeployment progress subscription

MCP clients SHALL be able to subscribe to real-time progress updates for ClusterDeployment resources during provisioning, receiving filtered, high-value status changes as the cluster transitions through its lifecycle phases.

#### Scenario: Subscribe to Azure cluster provisioning

```
Given an Azure cluster deployment "azure-test-1" in namespace "kcm-system" has been created
When the MCP client subscribes to "k0rdent://cluster-monitor/kcm-system/azure-test-1"
Then the server begins streaming progress updates via ResourceUpdated notifications
And the initial update includes the current phase and recent significant events
And subsequent updates are published when:
  - ClusterDeployment .status.conditions change
  - Significant Kubernetes events occur (infrastructure ready, nodes joined, etc.)
  - Provisioning phase transitions (Provisioning → Bootstrapping → Scaling → Ready)
And the subscription automatically terminates when:
  - The cluster reaches Ready=True
  - The cluster reaches Failed state
  - The timeout is exceeded (default: 60 minutes)
  - The MCP client explicitly unsubscribes
```

#### Scenario: Filtered progress updates reduce noise

```
Given a cluster deployment generates 150+ raw Kubernetes events during provisioning
When the MCP client subscribes to cluster monitoring
Then the client receives 5-10 filtered progress updates, not all 150 raw events
And updates include only "major checkpoints" such as:
  - Infrastructure creation started/completed
  - Control plane bootstrap started/completed
  - Worker nodes joining
  - Service templates installing/ready
  - Terminal states (Ready or Failed)
And routine events are excluded:
  - Periodic controller reconciliations
  - Image pulls
  - Routine pod restarts
  - Verbose CAPI internal events
```

#### Scenario: Multi-source data fusion

```
Given a ClusterDeployment "gcp-cluster" is provisioning
When the monitoring subscription is active
Then progress updates synthesize data from:
  - ClusterDeployment .status.conditions (high-level state)
  - Namespace Events filtered by involvedObject matching cluster resources
  - (Optional) Logs from cluster provisioning controller pods
And each update includes:
  - Detected provisioning phase (Initializing, Provisioning, Bootstrapping, Scaling, Installing, Ready, Failed)
  - Human-readable message describing the checkpoint
  - Timestamp of the event
  - Source of the update (condition, event, log)
  - Severity level (info, warning, error)
  - Optional: Estimated progress percentage
```

#### Scenario: Automatic cleanup on completion

```
Given a cluster "aws-prod" is being monitored during provisioning
When the cluster reaches Ready=True status
Then the server publishes a final "Cluster Ready" update
And automatically unsubscribes the client
And cleans up internal watch resources
And no further updates are sent for this cluster
```

#### Scenario: Timeout protection

```
Given a cluster provisioning exceeds expected duration
When the monitoring subscription has been active for 55 minutes (default timeout: 60 minutes)
Then the server publishes a warning: "Provisioning timeout approaching (5 minutes remaining)"
When the timeout of 60 minutes is reached
Then the server publishes: "Monitoring timeout exceeded, subscription terminated"
And automatically unsubscribes and cleans up resources
```

#### Scenario: Failure state detection

```
Given a cluster deployment encounters a terminal failure (invalid credentials, quota exceeded)
When the ClusterDeployment Ready condition transitions to False with reason "Failed"
Then the server publishes an error-level update with the failure reason and message
And automatically terminates the subscription
And includes the failure context from the most recent error events
```

### Requirement: Phase detection and progress estimation

The monitoring system SHALL detect logical provisioning phases from ClusterDeployment conditions and Kubernetes events, and MAY optionally estimate completion percentage.

#### Scenario: Phase detection from conditions

```
Given a ClusterDeployment with these condition states:
  - Ready: False
  - InfrastructureReady: True
  - ControlPlaneInitialized: False
Then the detected phase is "Bootstrapping"
And the estimated progress is 50%

When the conditions update to:
  - ControlPlaneInitialized: True
  - WorkersJoined: False
Then the phase transitions to "Scaling"
And the estimated progress is 75%
And a phase transition update is published
```

#### Scenario: Phase detection from events

```
Given no explicit phase-related conditions are present
When recent Kubernetes events (within 2 minutes) contain keywords:
  - "Bootstrapping control plane"
  - "Control plane pod starting"
Then the system infers the phase as "Bootstrapping"
And uses this for progress updates
```

#### Scenario: Provider-agnostic phase mapping

```
Given clusters deployed on AWS, Azure, and GCP
When monitoring subscriptions are active for each
Then the same core phases are detected across all providers:
  - Initializing
  - Provisioning (infrastructure)
  - Bootstrapping (control plane)
  - Scaling (workers)
  - Installing (services)
  - Ready / Failed
And provider-specific events map to these common phases
```

### Requirement: Authorization and namespace filtering

Cluster monitoring subscriptions SHALL respect the same namespace authorization model as other MCP Server resources, enforcing OIDC-based namespace filtering.

#### Scenario: Namespace filter enforcement

```
Given a session with NamespaceFilter allowing only "team-a" and "team-b"
When the client attempts to subscribe to "k0rdent://cluster-monitor/restricted-ns/cluster-1"
Then the subscription is rejected with "namespace not allowed by filter"

When the client subscribes to "k0rdent://cluster-monitor/team-a/cluster-2"
Then the subscription succeeds
And monitoring begins for the allowed namespace
```

#### Scenario: DEV_ALLOW_ANY mode support

```
Given a development session with no namespace filter (DEV_ALLOW_ANY mode)
When the client subscribes to any cluster in any namespace
Then the subscription succeeds
And monitoring works across all namespaces
```

### Requirement: Resource template and subscription registration

The cluster monitoring capability SHALL be exposed as an MCP resource template with subscription support, following the established pattern from events and podLogs.

#### Scenario: Resource template registration

```
Given the MCP Server starts and initializes cluster monitoring
Then a resource template is registered:
  Name: "k0rdent.mgmt.clusterMonitor"
  Title: "Cluster deployment provisioning monitor"
  Description: "Streaming progress updates for ClusterDeployment provisioning"
  URITemplate: "k0rdent://cluster-monitor/{namespace}/{name}"
  MIMEType: "application/json"
And the template is discoverable via resources/list
```

#### Scenario: Subscription routing

```
Given the SubscriptionRouter is configured with cluster-monitor handler
When a client subscribes to "k0rdent://cluster-monitor/ns/cluster"
Then the router parses scheme="k0rdent" and host="cluster-monitor"
And routes the subscription to ClusterMonitorManager
And the manager creates and tracks the subscription
```

### Requirement: Concurrent subscription limits

The server SHALL enforce limits on concurrent cluster monitoring subscriptions to prevent resource exhaustion.

#### Scenario: Per-client subscription limit

```
Given a single MCP client connection
When the client creates 10 concurrent cluster monitoring subscriptions
Then all 10 succeed

When the client attempts an 11th subscription
Then the subscription is rejected with "per-client subscription limit exceeded (max: 10)"
```

#### Scenario: Global subscription limit

```
Given the server has 95 active cluster monitoring subscriptions across all clients
When a client attempts to create a new subscription
Then the subscription succeeds (total: 96)

Given the server has 100 active subscriptions (global limit)
When any client attempts a new subscription
Then the subscription is rejected with "server subscription limit exceeded (max: 100)"
```

### Requirement: Update payload format

Progress updates SHALL be published as structured JSON payloads in ResourceUpdated notification metadata.

#### Scenario: Standard progress update format

```
Given a cluster monitoring subscription is active
When a significant provisioning event occurs
Then a ResourceUpdated notification is sent with:
  URI: "k0rdent://cluster-monitor/namespace/name"
  Meta.delta: {
    "timestamp": "2025-11-09T17:30:45.123Z",
    "phase": "Bootstrapping",
    "progress": 50,
    "message": "Control plane nodes initializing (1/3 ready)",
    "source": "event",
    "severity": "info",
    "relatedObject": {
      "kind": "Machine",
      "name": "cluster-control-plane-abc123",
      "namespace": "kcm-system"
    }
  }
```

#### Scenario: Terminal state update format

```
Given a cluster reaches Ready=True
Then a final update is published:
  Meta.delta: {
    "timestamp": "2025-11-09T18:00:00.000Z",
    "phase": "Ready",
    "progress": 100,
    "message": "Cluster fully provisioned and operational",
    "source": "condition",
    "severity": "info",
    "terminal": true
  }
And the subscription auto-terminates after this update
```

#### Scenario: Failure update format

```
Given a cluster provisioning fails due to quota exceeded
Then an error update is published:
  Meta.delta: {
    "timestamp": "2025-11-09T17:45:00.000Z",
    "phase": "Failed",
    "message": "Provisioning failed: Insufficient regional quota for VM SKU 'Standard_D2s_v3'",
    "source": "condition",
    "severity": "error",
    "terminal": true,
    "relatedObject": {
      "kind": "Machine",
      "name": "cluster-control-plane-xyz789"
    }
  }
```

### Requirement: Testing and validation

The cluster monitoring capability SHALL include comprehensive tests covering subscription lifecycle, filtering accuracy, and cross-provider compatibility.

#### Scenario: Integration test for subscription lifecycle

```
Given an integration test environment with a test management cluster
When the test deploys a minimal cluster (1 control plane, 1 worker)
And subscribes to cluster monitoring before provisioning begins
Then the test receives 5-10 progress updates over 10-15 minutes
And verifies phase transitions: Initializing → Provisioning → Bootstrapping → Scaling → Ready
And confirms automatic unsubscribe when Ready
And validates no memory leaks or orphaned watches
```

#### Scenario: Cross-provider compatibility test

```
Given integration tests run against AWS, Azure, and GCP test clusters
When each cluster is deployed with monitoring active
Then all three providers produce recognizable phase progressions
And filtering heuristics work across all providers
And no provider-specific failures occur
And phase detection accuracy is ≥90% for major checkpoints
```

#### Scenario: Filtering effectiveness validation

```
Given a test cluster that generates 150+ raw Kubernetes events
When monitoring with default filtering
Then the test client receives ≤15 filtered updates
And all "major checkpoint" events are included
And noisy events (image pulls, routine reconciliations) are excluded
And the signal-to-noise ratio meets the 10:1 target
```
### Requirement: Tool access to current cluster state

MCP clients SHALL be able to retrieve the current cluster monitoring state via a standard tool call in addition to the streaming subscription.

#### Scenario: Fetch current state via tool

```
Given a ClusterDeployment "azure-test-1" exists in namespace "kcm-system"
When the client calls tool "k0rdent.mgmt.clusterDeployments.getState" with {"namespace":"kcm-system","name":"azure-test-1"}
Then the server returns a result containing the latest ProgressUpdate payload:
  - Phase reflects the detected provisioning stage
  - Message and conditions mirror the current ClusterDeployment status
  - Timestamp reflects when the state snapshot was taken
  - Terminal flag is set when the cluster is Ready or Failed
And namespace validation is enforced (requests outside the user's namespace filter are rejected)
```

#### Scenario: Tool handles missing clusters

```
Given no ClusterDeployment named "missing-cluster" exists in namespace "team-alpha"
When the client calls the state tool for that namespace/name
Then the tool returns an error explaining the cluster was not found
And no partial data is returned
```

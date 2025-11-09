# Proposal: Add Cluster Deployment Monitoring

## Problem Statement

Currently, when a user deploys a k0rdent child cluster using the MCP Server tools (e.g., `k0rdent.provider.azure.clusterDeployments.deploy`), they receive an immediate response confirming the deployment was initiated. However, cluster provisioning is a long-running process that can take 10-30 minutes depending on the provider and configuration.

**Current limitations:**

1. **No visibility into provisioning progress**: After deployment starts, the MCP client has no way to track the cluster's status through the natural lifecycle phases (e.g., validating credentials, creating infrastructure, bootstrapping control plane, joining workers)
2. **No real-time feedback**: Users must poll the `k0rdent.mgmt.clusterDeployments.list` tool repeatedly to check if the cluster is ready
3. **Lost context during failures**: When provisioning fails, users don't see the sequence of events leading to the failure, making debugging difficult
4. **Inefficient resource usage**: Polling is wasteful and adds latency between state changes and user awareness

**User impact:**

- MCP clients (like Claude Code) cannot provide meaningful progress updates during cluster provisioning
- Users don't know if their deployment is progressing normally or has stalled
- Troubleshooting provisioning issues requires manual kubectl inspection of events and logs

## Proposed Solution

Add a **streaming subscription capability** to monitor ClusterDeployment resources during provisioning, delivering filtered, high-value progress updates to MCP clients as the cluster sequences through its lifecycle.

### Core Capabilities

1. **ClusterDeployment Monitoring Subscription**
   - New subscription URI: `k0rdent://cluster-monitor/{namespace}/{name}`
   - Streams filtered progress updates as JSON deltas
   - Automatically detects completion (Ready=True) or failure states
   - Unsubscribes automatically when terminal state reached

2. **Intelligent Event Filtering**
   - Monitors both ClusterDeployment conditions AND related Kubernetes events
   - Filters for "major checkpoints": credential validation, infrastructure creation, control plane bootstrap, worker joins, service installation
   - Excludes noisy/repetitive events (routine pod restarts, image pulls, etc.)
   - Aggregates related events into logical progress phases

3. **Multi-Source Data Fusion**
   - Watches ClusterDeployment `.status.conditions` for high-level state transitions
   - Monitors namespace events filtered by `involvedObject.name` matching the cluster
   - Optionally streams logs from cluster provisioning pods (e.g., CAPI controllers)
   - Synthesizes a unified progress narrative from all sources

### Architecture Approach

This follows the existing subscription pattern established in `internal/tools/core/events.go` and `podlogs.go`:

1. **ClusterMonitorManager** (similar to EventManager/PodLogManager)
   - Manages active cluster monitoring subscriptions
   - Coordinates multiple Kubernetes watches (ClusterDeployment + Events)
   - Filters and enriches events before publishing
   - Handles automatic cleanup on completion/timeout

2. **Resource Template Registration**
   - Registers `k0rdent://cluster-monitor/{namespace}/{name}` as streamable resource
   - Publishes `ResourceUpdated` notifications with delta payloads
   - Includes metadata: phase, progress percentage (estimated), timestamp

3. **Progress Phase Detection**
   - Maps condition changes and event patterns to logical phases:
     - `Initializing`: Deployment created, validating inputs
     - `Provisioning`: Infrastructure being created (VMs, networks, etc.)
     - `Bootstrapping`: Control plane nodes starting
     - `Scaling`: Worker nodes joining
     - `Installing`: Service templates being deployed
     - `Ready`: Cluster fully operational
     - `Failed`: Terminal error state

### Success Criteria

1. MCP clients can subscribe to cluster progress during deployment
2. Clients receive 5-10 high-value progress updates (not 100+ raw events)
3. Subscription automatically terminates when cluster reaches Ready or Failed state
4. Filtering logic correctly identifies major provisioning milestones
5. Works consistently across AWS, Azure, and GCP providers

### Non-Goals

- Real-time log streaming for all cluster pods (use existing podLogs subscription)
- Historical replay of past deployments (use events list tool)
- Detailed infrastructure metrics (CPU, memory, network traffic)
- Multi-cluster monitoring dashboard (single cluster focus)

## Dependencies

- Requires existing namespace events infrastructure (`internal/kube/events`)
- Builds on subscription router pattern (`internal/tools/core/subscriptions.go`)
- Uses ClusterDeployment status fields (`.status.conditions`, `.status.phase`)
- Needs RBAC permissions to watch ClusterDeployments and related Events

## Alternatives Considered

1. **Polling Enhancement**: Add a "wait" option to deployment tools that polls until ready
   - **Rejected**: Blocks the MCP tool call for 10-30 minutes, poor UX
   - Doesn't leverage MCP's native streaming capabilities

2. **Webhook-Based Notifications**: Add webhook support to post updates to external endpoints
   - **Rejected**: Requires external infrastructure, complex authentication
   - Not aligned with MCP's subscription model

3. **Full Event Stream**: Subscribe to all namespace events without filtering
   - **Rejected**: Too noisy, overwhelming for MCP clients
   - Doesn't provide semantic progress phases

## Open Questions

1. **Filtering Granularity**: Should we expose filtering options (e.g., verbosity level) or use fixed heuristics?
   - **Recommendation**: Start with fixed "major checkpoints" filter, add options in future if needed

2. **Timeout Behavior**: Should subscriptions auto-terminate after a timeout (e.g., 60 minutes)?
   - **Recommendation**: Yes, with configurable timeout and warning notification before termination

3. **Multi-Cluster Subscriptions**: Should a single subscription support monitoring multiple clusters?
   - **Recommendation**: No, keep 1:1 subscription-to-cluster mapping for simplicity

4. **Progress Percentage Estimation**: Should we attempt to estimate % completion?
   - **Recommendation**: Optional, provider-specific estimates based on observed phase transitions

## Validation Plan

1. **Manual Testing**:
   - Deploy minimal Azure cluster (1 control plane, 1 worker)
   - Subscribe to monitoring during deployment
   - Verify 5-10 meaningful progress updates received
   - Confirm automatic unsubscribe on Ready state

2. **Integration Tests**:
   - Test subscription lifecycle (subscribe → updates → auto-unsubscribe)
   - Verify filtering excludes noisy events
   - Test failure scenarios (invalid credentials, quota exceeded)
   - Validate timeout behavior

3. **Cross-Provider Testing**:
   - Run same monitoring test on AWS, Azure, and GCP
   - Ensure phase detection works across different infrastructure patterns

## References

- Existing subscription patterns: `internal/tools/core/events.go`, `podlogs.go`
- ClusterDeployment CRD: k0rdent.mirantis.com/v1beta1
- Kubernetes Events API: corev1.Event with field selectors
- MCP Subscriptions spec: https://spec.modelcontextprotocol.io/specification/server/subscriptions/

# Cluster Deployment Monitoring

Cluster deployments often take several minutes and touch multiple controllers. The cluster monitoring stream turns that noisy lifecycle into a handful of high-value updates any MCP client can display in real time.

## Overview

- **Subscription URI:** `k0rdent://cluster-monitor/{namespace}/{name}` (optional `?timeout=seconds` query)
- **Payloads:** Structured JSON deltas containing phase, severity, reason, message, optional progress %, related object, and recent conditions.
- **Sources:** ClusterDeployment status conditions plus filtered Kubernetes Events from the cluster's namespace.
- **Auto cleanup:** Subscriptions stop automatically when the cluster reaches `Ready`, `Failed`, gets deleted, or the timeout expires.

## Getting Started

1. Deploy a cluster with any of the existing `k0rdent.provider.*.clusterDeployments.deploy` tools.
2. Start a subscription:

```json
{
  "method": "subscriptions/subscribe",
  "params": {
    "uri": "k0rdent://cluster-monitor/kcm-system/azure-test-1"
  }
}
```

3. Watch for `ResourceUpdated` notifications referencing the same URI. Each notification's `meta.delta` field contains a serialized `ProgressUpdate`, for example:

```json
{
  "timestamp": "2025-11-09T09:52:17Z",
  "phase": "Bootstrapping",
  "progress": 55,
  "message": "Control plane machine ready: azure-test-1-cp-7dbs9",
  "source": "event",
  "severity": "info",
  "relatedObject": {
    "kind": "Machine",
    "name": "azure-test-1-cp-7dbs9"
  }
}
```

4. When provisioning completes (or fails), the stream sends a final `terminal: true` update and releases the subscription.

To stop receiving updates earlier, call `subscriptions/unsubscribe` with the same URI.

### One-Off State Check

Need a quick snapshot without subscribing? Call the tool:

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.getState",
    "arguments": {
      "namespace": "kcm-system",
      "name": "azure-test-1"
    }
  }
}
```

The response contains a single `update` object identical to the streaming payload (phase, progress, message, severity, conditions, terminal flag). Namespace filters still apply, and you can omit `namespace` to fall back to the session's global namespace.

## Phases & Progress

The manager maps ClusterDeployment conditions plus recent Events into a coarse-grained lifecycle:

| Phase | Description | Default Progress |
|-------|-------------|------------------|
| `Initializing` | Helm charts / prerequisites staged | 5% |
| `Provisioning` | Cloud infrastructure being created | 25% |
| `Bootstrapping` | Control-plane machines configuring | 50% |
| `Scaling` | Worker machines join the cluster | 75% |
| `Installing` | Service templates roll out | 90% |
| `Ready` | Cluster operational | 100% |
| `Failed` | Terminal error detected | 0% |

Progress is reported as best-effort estimates and updated whenever the underlying conditions advance.

## Event Filtering

Raw namespaces can emit hundreds of events. The monitoring pipeline narrows these down using:

1. **Scope filtering** – only events whose involved object lives in the same namespace and shares the cluster name prefix.
2. **Significance patterns** – emits milestones such as `BeginCreateOrUpdate`, `MachineReady`, `ServiceReady`, `CAPIClusterIsReady`, plus notable warnings (quota issues, reconciliation failures).
3. **Deduplication** – suppresses repeats of the same reason/object pairs within short windows (typically 30–300 seconds).
4. **Phase awareness** – phase transitions always generate updates, even if no event passed the filter, so the client sees at least one update per lifecycle stage.

## Timeouts & Limits

- Default timeout is 60 minutes. Override with `?timeout=1800` (seconds) in the URI.
- Five-minute warning is sent before timeout, followed by a terminal timeout message if provisioning still runs.
- Each MCP session can hold up to 10 cluster-monitor subscriptions; the server enforces a global cap of 100 concurrent streams.

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| `namespace ... not allowed by filter` | Your token's namespace regex does not include the requested namespace | Re-run subscription with an allowed namespace or adjust RBAC/claims |
| No updates arrive | Cluster already finished before you subscribed, or subscription timed out | Call `resources/read` on the URI to fetch the latest snapshot, then re-subscribe with a higher timeout if needed |
| Immediate terminal error | Cluster was deleted or not found | double-check the namespace/name; use `k0rdent.mgmt.clusterDeployments.list` to confirm |
| `per-client/server subscription limit exceeded` | Too many active monitors | Unsubscribe from older streams or wait for running ones to finish |

## Reference

- Resource template: `k0rdent.cluster.monitor`
- Subscribe URI format: `k0rdent://cluster-monitor/{namespace}/{name}[?timeout=<seconds>]`
- Related tooling: namespace events (`k0rdent://events/{namespace}`) and pod log streaming (`k0rdent://podlogs/...`).

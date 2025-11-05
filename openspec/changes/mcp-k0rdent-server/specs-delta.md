## Requirements

### Transport & Auth
- The server SHALL implement an MCP **Streamable HTTP** endpoint.
- When `AUTH_MODE=OIDC_REQUIRED`, the server SHALL return **401** if `Authorization: Bearer <token>` is missing or invalid.
- The server SHALL NOT use Kubernetes impersonation. When a Bearer token is present, the server SHALL pass it to Kubernetes as the user credential.

### Config
- The server SHALL accept exactly one of `K0RDENT_MGMT_KUBECONFIG_{PATH,B64,TEXT}`, with optional `K0RDENT_MGMT_CONTEXT`.
- A namespace allowlist filter MAY be configured via `K0RDENT_NAMESPACE_FILTER`.

### Core Tools
- `k0.namespaces.list()` — returns namespaces (name, labels, status).
- `k0.events.list(ns, sinceSeconds?, limit?, types?, forKind?, forName?)`
  - MUST read Events from **`events.k8s.io/v1`** if available; MUST fall back to core `v1` Events.
  - MUST support **field selectors** for involved/regarding object (kind & name).
- `k0.podLogs.get(ns, pod, container?, tailLines?, sinceSeconds?, previous?, follow?)`
  - MUST stream when `follow=true`; MUST require `container` when multiple containers are present.

### k0rdent Tools
- `k0.k0rdent.serviceTemplates.list()`
- `k0.k0rdent.clusterDeployments.list(selector?)`
- `k0.k0rdent.multiClusterServices.list(selector?)`

### Graph
- `k0.graph.snapshot(ns?, kinds?)` — returns `{ nodes, edges }` where edges derive from:
  - K8s ownerReferences and label selectors (Service→Pod, HPA→scale target, etc.)
  - k0rdent CRD refs (e.g., ClusterDeployment.spec.serviceSpec.services[].template, MultiClusterService selectors/precedence)
- `sub.k0.graph(ns?, kinds?)` — emits `add|update|delete` deltas with affected nodes/edges.

### Events Subscription
- `sub.k0.events(ns)` — pushes new/updated/deleted Events scoped to the namespace.

### Pod Logs Subscription
- `sub.k0.podLogs(ns, pod, container?)` — streams logs as text frames.

### Health & Build
- `/healthz` SHALL be provided for liveness/readiness.
- Binary SHALL embed version/build info via ldflags.

### RBAC (minimum, read-only)
- get/list/watch: core v1 (pods, namespaces, services, configmaps, secrets, endpoints, events, pvcs), apps/v1 (deployments, replicasets, statefulsets, daemonsets), networking.k8s.io/v1 (ingresses), batch/v1 (jobs, cronjobs), discovery.k8s.io/v1 (endpointslices), autoscaling/v2 (hpas), and k0rdent CRDs (`servicetemplates`, `clusterdeployments`, `multiclusterservices`).
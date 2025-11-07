# Change: mcp-k0rdent-server

## Why
- We need an HA **MCP server** in the k0rdent **management cluster** that exposes read-only insights and troubleshooting (namespaces, events, pod logs) and treats **k0rdent CRDs** as first-class (ServiceTemplate, ClusterDeployment, MultiClusterService).
- Kubeconfig-first auth simplifies bring-up; later we’ll require **OIDC Bearer** at the transport and pass it directly to the API server (no k8s impersonation).
- Pattern is proven by **KubeView** (service layer + watches + streaming); we mirror capabilities but expose them via MCP.

## What Changes
- Implement an MCP server (Go, modelcontextprotocol/go-sdk) with tools:
  - `k0rdent.namespaces.list()`
  - `k0rdent.events.list(ns, sinceSeconds?, limit?, types?, forKind?, forName?)`
  - `k0rdent.podLogs.get(ns, pod, container?, tailLines?, sinceSeconds?, previous?, follow?)`
  - `k0rdent.k0rdent.serviceTemplates.list()`
  - `k0rdent.k0rdent.clusterDeployments.list(selector?)`
  - `k0rdent.k0rdent.multiClusterServices.list(selector?)`
  - `k0rdent.graph.snapshot(ns?, kinds?)`
- Subscriptions: `sub.k0rdent.events(ns)`, `sub.k0rdent.graph(ns?, kinds?)`, `sub.k0rdent.podLogs(ns, pod, container?)`.
- Runtime config via env: one of `K0RDENT_MGMT_KUBECONFIG_{PATH|B64|TEXT}` (+ `K0RDENT_MGMT_CONTEXT`), `AUTH_MODE` (`DEV_ALLOW_ANY`|`OIDC_REQUIRED`), optional `K0RDENT_NAMESPACE_FILTER`.
- HA via client-go **leader election**; add `/healthz` and build ldflags.

## Impact
- Enables agents to query k0rdent’s higher-level view (CRDs + relationships) and perform first-line troubleshooting (events/logs) without cluster-admin UIs.
- Read-only RBAC footprint; clean separation between **spec deltas** (in `specs/*/spec.md`) and implementation tasks.
- Production hardening path: move from kubeconfig-only to **OIDC required** without changing the tool surface.

## Out of Scope
- UI and public REST routes (MCP only).
- Kubernetes impersonation.
- KOF support (future change).

## Acceptance
- `openspec validate mcp-k0rdent-server --strict` passes.
- Tools list namespaces, events (with field selectors), and pod logs (snapshot + follow).
- k0rdent CRDs list successfully; `k0rdent.graph.snapshot` includes edges:
  - `ClusterDeployment.spec.serviceSpec.services[].template` → ServiceTemplate
  - `MultiClusterService` → targeted clusters (selector/precedence)
- No request accepted when `AUTH_MODE=OIDC_REQUIRED` and Bearer is missing.
- With leader election enabled, only one replica publishes deltas; failover works.

## Links / References
- OpenSpec workflow (proposal + tasks + spec deltas; validate, show, archive).  [oai_citation:1‡GitHub](https://github.com/Fission-AI/OpenSpec)

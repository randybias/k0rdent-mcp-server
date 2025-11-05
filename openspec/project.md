# Project: k0rdent MCP Server (KubeView-patterned)

We are building a Go MCP server that runs in the k0rdent **management cluster**, exposes **MCP tools + subscriptions** for:
- Core K8s: namespaces, namespace events, pod logs (for troubleshooting), and a minimal set of pods/deployments/services to keep parity with KubeView.  
- **k0rdent CRDs as first-class**: ServiceTemplate, ClusterDeployment, MultiClusterService — plus a live graph of relationships.

Architecture & Prior Art
- Patterned on **KubeView v2** backend: service layer over client-go, caches + watchers, and a streaming channel (KubeView uses SSE; we use MCP subscriptions). KubeView routes include `/api/namespaces`, `/api/logs/{ns}/{pod}`, and `/updates` for SSE; it lists “Display list of events… Drill down and show pod logs…” as core features. We copy that capability surface into MCP.  
  Sources: KubeView README (features, routes, RBAC) and site docs.  
  (Refs: kubeview readme/features, routes, RBAC; SSE architecture)  

Security & Phasing
- **No Kubernetes impersonation** anywhere.
- **Baseline**: kubeconfig-first for connectivity (dev/testing) — supports PATH/B64/TEXT with optional context.
- **Production**: **reject requests without an OIDC Bearer token** at the MCP transport; when present, use that token directly against the apiserver (no impersonation). Kubeconfig may still carry cluster endpoint/TLS, but **credentials come from the inbound OIDC token**.

Availability
- Stateless replicas; leader election for watch publishing (only leader pushes deltas/updates).

Ops Surface
- `/healthz` (HTTP, via chi) for probes; MCP is the primary API.
- Build embeds version/build info; image is multi-stage Go build.

References
- KubeView features & routes, required RBAC (read-only for core kinds).  
- client-go out-of-cluster & kubeconfig loading; Events API with field selectors; Pod logs via `PodLogOptions`; MCP Go SDK (Streamable HTTP) & Authorization.
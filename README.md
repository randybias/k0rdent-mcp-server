# k0rdent MCP Server

The k0rdent MCP server exposes management-plane tooling over the Model Context Protocol so assistants can provision, inspect, and troubleshoot child clusters safely. It wraps Kubernetes resources (ClusterDeployments, credentials, templates, logs, events, etc.) behind curated tools and subscriptions that respect namespace filters and per-session RBAC.

## Features
- **Cluster Deployment Monitoring** – Subscribe to `k0rdent://cluster-monitor/{namespace}/{name}` to follow provisioning progress with filtered, high-signal updates. See [docs/features/cluster-monitoring.md](docs/features/cluster-monitoring.md).
- **Cluster Provisioning Tools** – List templates/credentials and launch or delete ClusterDeployments programmatically.
- **Service Attachments** – Attach installed ServiceTemplates to running clusters with `k0rdent.mgmt.clusterDeployments.services.apply`, including dry-run previews before mutating production clusters. See [docs/cluster-provisioning.md](docs/cluster-provisioning.md#k0rdentmgmtclusterdeploymentsservicesapply).
- **Namespace Event Streaming** – Follow live Kubernetes events via `k0rdent://events/{namespace}`.
- **Pod Log Streaming** – Tail application logs with `k0rdent://podlogs/{namespace}/{pod}/{container}`.
- **Catalog Access** – Browse and install catalog entries exposed by the k0rdent control plane.

## Getting Started
1. Build and run the server: `make run` (requires access to the management cluster and matching kubeconfig settings in `config.yaml`).
2. Connect an MCP-compatible client (e.g., Claude Desktop) using the server URL and an authentication token recognized by the management cluster.
3. List available tools with `tools/list`, then call any tool by name. Subscriptions use `subscriptions/subscribe` with the URIs above.

## Documentation
- [Cluster Provisioning](docs/cluster-provisioning.md)
- [Provider-Specific Deployment Tools](docs/provider-specific-deployment.md)
- [Cluster Deployment Monitoring](docs/features/cluster-monitoring.md)
- [Live Test Playbooks](docs/live-tests.md)

For more specs and change proposals, see the `openspec/` directory or run `openspec list`.

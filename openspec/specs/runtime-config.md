# Runtime Configuration (env-driven)

## Cluster target (mgmt cluster)
One of:
- `K0RDENT_MGMT_KUBECONFIG_PATH` = /path/to/kubeconfig
- `K0RDENT_MGMT_KUBECONFIG_B64`  = <base64 kubeconfig>
- `K0RDENT_MGMT_KUBECONFIG_TEXT` = <raw kubeconfig YAML>

Optional:
- `K0RDENT_MGMT_CONTEXT` = override context name from kubeconfig
- `K0RDENT_NAMESPACE_FILTER` = regex for allowed namespaces (server-side filter)

## Auth (MCP transport)
- `AUTH_MODE` = `DEV_ALLOW_ANY` | `OIDC_REQUIRED`
  - `DEV_ALLOW_ANY`: for local/dev bring-up (requires kubeconfig; rejects only if missing header is disallowed by test mode).
  - `OIDC_REQUIRED`: **reject** any request without `Authorization: Bearer <token>`; token is passed to the Kubernetes API as the **user credential** (no impersonation).
- `DEV_BEARER_TOKEN` (dev only) = opaque string accepted by the MCP server when `AUTH_MODE=DEV_ALLOW_ANY`

## Cluster Provisioning
- `CLUSTER_GLOBAL_NAMESPACE` = namespace for global cluster resources (default: `kcm-system`)
- `CLUSTER_DEFAULT_NAMESPACE_DEV` = default namespace for cluster operations in dev mode (default: `kcm-system`)
- `CLUSTER_DEPLOY_FIELD_OWNER` = field manager name for server-side apply of ClusterDeployment resources (default: `mcp.clusters`)

## HA
- `LEADER_ELECTION_ENABLED` = `true|false` (default true)
- `LEADER_ELECTION_LEASE_NAME` = `k0rdent-mcp-server`
- `LEADER_ELECTION_NS` = namespace for coordination Lease
# Design: MCP cluster provisioning tools

## Overview
We will expose cluster provisioning workflows through the MCP server so Claude users can:
1. Enumerate credentials they are allowed to use.
2. Discover global (`kcm-system`) and namespace-local `ClusterTemplate` definitions.
3. Create `ClusterDeployment` objects that reference those inputs and apply configuration blocks from the user guide (`spec.config`, labels, etc.).

The implementation mirrors the pattern used for catalog installs: a manager component builds typed views over k0rdent CRDs, tool handlers provide thin orchestration, and namespace enforcement relies on the runtime session’s filter/auth mode.

## Resources & APIs
- **Credentials**: `k0rdent.mirantis.com/v1beta1`, resource `credentials`. Namespaced. Global defaults live in `kcm-system`.
- **ClusterTemplates**: `k0rdent.mirantis.com/v1beta1`, resource `clustertemplates`. Global definitions in `kcm-system`; tenants may copy into their namespace.
- **ClusterDeployments**: `k0rdent.mirantis.com/v1beta1`, resource `clusterdeployments`. Creating one triggers child cluster provisioning. Existing manifests (e.g., `scripts/config/aws-cld.yaml`) guide the shape of `spec.config` (controlPlane/worker sizing, cloud region, etc.).

## Manager responsibilities
We will add an `internal/clusters` package that wraps the dynamic client and implements:
- `ListCredentials(ctx, namespaces []string) ([]CredentialSummary, error)` – fetch credentials from requested namespaces, mapping metadata (provider labels, readiness conditions, creation timestamp).
- `ListTemplates(ctx, namespaces []string) ([]ClusterTemplateSummary, error)` – fetch templates, capturing description, provider type, compatible cloud, version, and required config hints (e.g., `spec.schema` if present).
- `DeployCluster(ctx, namespace string, input DeployRequest) (DeployResult, error)` – build/merge the `ClusterDeployment` manifest and perform server-side apply with a dedicated field owner (e.g., `mcp.clusters`).
- `DeleteCluster(ctx, namespace, name string) (DeleteResult, error)` – remove the `ClusterDeployment` using foreground propagation policy to ensure finalizers execute and child resources clean up properly.
- Namespace resolution helpers (global vs local) and dev-mode detection.

The manager reuses `runtime.Session.Clients.Dynamic`. No local cache is required; each call is scoped and quick.

## Namespace resolution & auth
- **Dev mode**: detected when runtime settings report `AuthModeDevAllowAny`. In this mode, the default install namespace is `kcm-system`, mirroring manual workflows.
- **Production mode**: the namespace filter (regex) drives the allowed list. Tool inputs may include an explicit namespace; when omitted, we use the first namespace that matches the filter. If no namespace matches, we return `forbidden`.
- All list operations respect the filter by precomputing allowed namespaces (globally `["kcm-system"]` plus filter-matched ones).

## Tool contracts
### `k0.clusters.listCredentials`
Input: optional `namespace` filter. Output: array with `name`, `namespace`, `provider`, `labels`, `createdAt`, `ready` (derived from status conditions). Hidden credentials (outside allowed namespaces) are filtered out.

### `k0.clusters.listTemplates`
Input: `scope` (`"global"|"local"|"all"`), optional namespace. Output: array with slug, description, cloud tags, version, config schema summary (fields + required keys). For templates referencing sample configs, include link fields gleaned from annotations.

### `k0.clusters.deploy`
Input:
- `name`
- `template` (string, may be `namespace/name` or just name when target namespace is known)
- `credential` (same format)
- `namespace` (optional)
- `labels` (map)
- `config` (arbitrary JSON object matching doc’s cluster config)

Flow:
1. Resolve namespace (per rules above).
2. Resolve template & credential (preferring explicit namespace; fallback to target namespace or `kcm-system` for template when flagged as global).
3. Assemble `ClusterDeployment` manifest (see `scripts/config/aws-cld.yaml` for baseline). Set `metadata.namespace`, `spec.template`, `spec.credential`, and `spec.config` from request.
4. Server-side apply with managed field `mcp.clusters` and label `k0rdent.mirantis.com/managed=true` for traceability.
5. Return result summarising `created` vs `updated`, along with cluster UID and resolved namespace.

## Validation & UX safeguards
- Verify template and credential existence before apply; return `invalidParams` with suggestions (nearest names) if missing.
- Optionally validate `spec.config` types by introspecting template’s `spec.schema` (if present); otherwise, allow pass-through and rely on admission errors.
- Surface Kubernetes errors (RBAC, validation) using MCP error codes (`forbidden`, `invalidParams`, `internal`).
- Log at INFO level for successful deployments and WARN for validation failures.

## Deletion flow
### `k0.clusters.delete`
Input: `name`, optional `namespace`

Flow:
1. Resolve namespace (same rules as deploy).
2. Check if `ClusterDeployment` exists in target namespace.
3. If exists, delete using foreground propagation (`DeletePropagationForeground`) to trigger finalizers and wait for child resource cleanup.
4. If not exists, return success with `status="not_found"` (idempotent).
5. Return result with `name`, `namespace`, and `status`.

Safety considerations:
- Foreground propagation ensures that k0rdent's finalizers run, which triggers proper cleanup of cloud resources and CAPI objects.
- Log deletion attempts at INFO level with cluster name/namespace.
- Surface RBAC errors as `forbidden`.

## Testing

### Unit tests
- Use fake dynamic client fixtures for credentials/templates/deployments/deletions.
- Table-driven tests for namespace resolution covering dev mode, production mode with regex filters, explicit namespace overrides, and forbidden namespaces.
- Deploy tests verifying server-side apply idempotency (apply twice results in update path without error).
- Delete tests verifying idempotent deletion (delete non-existent resource succeeds).

### Live integration tests
- Add `test/integration/clusters_live_test.go` guarded by `//go:build live`.
- Test structure mirrors `catalog_install_live_test.go`:
  - Setup: Load kubeconfig from `K0RDENT_MGMT_KUBECONFIG_PATH`, create MCP client, verify auth mode.
  - Phase 1: List credentials via MCP, verify `azure-cluster-credential` is present.
  - Phase 2: List templates via MCP, verify `azure-standalone-cp-1-0-15` is present.
  - Phase 3: Deploy test cluster using known-good configuration:
    ```yaml
    name: "mcp-test-cluster-<timestamp>"
    template: "azure-standalone-cp-1-0-15"
    credential: "azure-cluster-credential"
    namespace: "kcm-system"
    config:
      clusterIdentity:
        name: "azure-cluster-identity"
        namespace: "kcm-system"
      controlPlane:
        rootVolumeSize: 32
        vmSize: "Standard_A4_v2"
      controlPlaneNumber: 1
      location: "westus2"
      subscriptionID: "b90d4372-6e37-4eec-9e5a-fe3932d1a67c"
      worker:
        rootVolumeSize: 32
        vmSize: "Standard_A4_v2"
      workersNumber: 1
    ```
  - Phase 4: Poll ClusterDeployment status (with timeout) until `status.conditions` shows `Ready=True` or timeout (10 minutes).
  - Phase 5: Delete cluster via MCP.
  - Phase 6: Verify deletion completed (resource no longer exists).
  - Cleanup: Ensure deletion runs even if earlier phases fail (deferred cleanup).

- Test baseline derived from existing `test1` cluster but uses unique names to avoid conflicts.
- Logs should capture each phase for debugging.
- Skip tests gracefully if environment variables are missing.

## Configuration
- Add optional env vars: `CLUSTER_GLOBAL_NAMESPACE` (default `kcm-system`), `CLUSTER_DEFAULT_NAMESPACE_DEV` (same), `CLUSTER_DEPLOY_FIELD_OWNER` (default `mcp.clusters`). Document them.
- No persistent storage required.

## Observability
- Counters: `clusters_list_credentials_total`, `clusters_list_templates_total`, `clusters_deploy_total`, `clusters_delete_total` (all labelled by outcome).
- Histograms for deployment and deletion duration.
- Logs include namespace/mode, template, credential, cluster name, and result.

## Risks & mitigations
- **RBAC**: if caller lacks permission to list/apply resources, ensure we fail fast with meaningful errors.
- **Config schema drift**: we rely on template-provided schema; missing schema may allow invalid configs—Document this and guide users to rely on doc defaults.
- **Namespace filter edge cases**: ensure regex evaluation is consistent with other tools; reuse existing helpers to avoid divergence.

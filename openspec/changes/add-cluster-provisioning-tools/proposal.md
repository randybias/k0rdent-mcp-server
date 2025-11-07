# Change: Add cluster provisioning tools

## Why
- Operators following the "Create a cluster" guide (docs.k0rdent.io/latest/user/user-create-cluster) must leave Claude Code to discover credentials, browse `ClusterTemplate` options, and craft `ClusterDeployment` manifests manually.
- The MCP server already exposes read-only views of core k0rdent CRDs, but lacks lifecycle tooling to deploy new child clusters, slowing down day-two operations.
- Automating credential/template discovery and cluster deployment through MCP keeps workflows inside Claude Code, reduces copy/paste errors, and honours namespace segregation (global resources in `kcm-system`, tenant-specific resources elsewhere).

## What Changes
- Introduce MCP tools under `k0.clusters.*` for:
  - Listing accessible `Credential` CRs (global in `kcm-system` plus namespaces allowed by the current session/namespace filter).
  - Listing available `ClusterTemplate` CRs split into global (`kcm-system`) and local (namespaces allowed for the caller).
  - Deploying a new `ClusterDeployment` tied to a chosen credential/template, with support for passing configuration from the user guide (name, labels, config block).
  - Deleting an existing `ClusterDeployment` with safety checks and cleanup verification.
- Reuse the runtime session's dynamic client for CRUD, apply namespace-filter enforcement, and respect dev vs production modes (Dev mode defaults to `kcm-system`; production requires caller-owned namespace input).
- Provide structured responses that surface metadata (cloud provider tags, versions) so users can make informed selections inside Claude.
- Add validation around template/credential existence, config schema checks (basic required fields), and surface friendly MCP errors when prerequisites are missing.
- Implement comprehensive live integration tests using the existing Azure infrastructure (`test1` cluster with `azure-cluster-credential`) to validate end-to-end deployment and deletion workflows.

## Impact
- **Affected specs**: new `tools-clusters` capability capturing list/install behaviour and namespace rules; potential updates to runtime-config spec for dev-mode namespace defaults.
- **Affected code**:
  - New cluster manager helper package for credential/template retrieval and deployment.
  - Core tool registration (`internal/tools/core`) for new MCP endpoints.
  - Runtime wiring to detect dev mode (AuthMode / env) and expose namespace resolution helpers.
  - Tests covering credential/template listing and deployment flows (fake dynamic client fixtures).
  - Documentation for cluster provisioning via MCP.
- **Breaking**: No – new features only.

## Out of Scope
- Updating existing `ClusterDeployment` configurations (deployment supports idempotent re-apply but not arbitrary field updates).
- Advanced template parameter editing (beyond passing a config object as documented).
- Cluster health/status monitoring – remains read-only via existing tools.
- Credential bootstrap (still handled by catalog utilities or external scripts).
- Credential or template creation/deletion through MCP.

## Acceptance
- `openspec validate add-cluster-provisioning-tools --strict` passes.
- `k0.clusters.listCredentials()` returns credentials from `kcm-system` plus caller-authorised namespaces, including provider labels and readiness indicators.
- `k0.clusters.listTemplates(scope)` differentiates global vs local `ClusterTemplate` resources, enforcing namespace filters.
- `k0.clusters.deploy()` creates or updates a `ClusterDeployment` using supplied name/template/credential/config; Dev mode defaults to namespace `kcm-system`, production mode requires a namespace allowed by the filter.
- `k0.clusters.delete()` removes a `ClusterDeployment` with proper finalizer handling and returns deletion status.
- Tool responses include enough metadata to align with the documented cluster creation workflow (template description, required parameters, credential summary).
- Unit tests cover happy path, forbidden namespace, missing credential/template, and server-side apply conflicts.
- Live integration tests deploy and delete a test cluster using Azure credentials (`azure-cluster-credential`) and template (`azure-standalone-cp-1-0-15`), verifying the full lifecycle.
- Documentation shows end-to-end usage from discovery through deployment and deletion.

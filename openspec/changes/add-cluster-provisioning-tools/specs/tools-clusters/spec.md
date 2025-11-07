# Cluster Tools (delta)

## ADDED Requirements

### Requirement: List accessible credentials
- The MCP server **SHALL** expose `k0rdent.providers.listCredentials(namespace?) -> CredentialSummary[]` for discovering k0rdent credentials.
- By default, the tool **SHALL** return credentials from `kcm-system` plus any namespaces permitted by the current namespace filter.
- When `namespace` is provided, the tool **SHALL** restrict results to that namespace after verifying it is allowed.
- Each `CredentialSummary` **SHALL** include `name`, `namespace`, `labels`, `provider` (derived from labels or annotations), `createdAt`, and `ready` (boolean based on status conditions).
- If the caller lacks access to the requested namespace, the tool **SHALL** return an MCP error with code `forbidden`.

#### Scenario: List default credentials
- WHEN `k0rdent.providers.listCredentials()` is called in production mode with namespace filter `^team-`
- THEN the response contains credentials from `kcm-system` and all namespaces matching `^team-`
- AND each entry reports provider/ready metadata

#### Scenario: Forbidden namespace
- WHEN `k0rdent.providers.listCredentials(namespace="finance")` is called and the namespace does not match the filter
- THEN the server returns an MCP error with code `forbidden`
- AND no credential data is returned

### Requirement: List cluster templates
- The MCP server **SHALL** expose `k0rdent.clusterTemplates.list(scope?, namespace?) -> ClusterTemplateSummary[]` for discovering `ClusterTemplate` resources.
- `scope` defaults to `"all"`; when `"global"` it **SHALL** include only templates from the global namespace (`kcm-system`), and when `"local"` only templates in namespaces allowed by the filter.
- Each summary **SHALL** include `name`, `namespace`, `description`, `cloud`, `version`, `labels`, and a `configSchema` outline (required keys + top-level fields if available).
- The tool **SHALL** enforce namespace filtering identical to the credential tool.

#### Scenario: Global template discovery
- WHEN `k0rdent.clusterTemplates.list(scope="global")` is called
- THEN the response enumerates templates from `kcm-system` with their metadata and config schema outline

#### Scenario: Local template discovery
- WHEN `k0rdent.clusterTemplates.list(scope="local")` is called with namespace filter `^team-`
- THEN only templates from namespaces whose names match `^team-` are returned

### Requirement: List cluster deployments
- The MCP server **SHALL** expose `k0rdent.clusters.list(namespace?) -> ClusterDeploymentSummary[]` for listing deployed clusters.
- By default, the tool **SHALL** return clusters from all namespaces permitted by the current namespace filter.
- When `namespace` is provided, the tool **SHALL** restrict results to that namespace after verifying it is allowed.
- Each `ClusterDeploymentSummary` **SHALL** include `name`, `namespace`, `template`, `labels`, `ready` (boolean), and `createdAt`.

### Requirement: Deploy cluster
- The MCP server **SHALL** expose `k0rdent.cluster.deploy(input) -> DeployResult` to create or update `ClusterDeployment` resources.
- `input` **SHALL** accept: `name`, `template`, `credential`, optional `namespace`, optional `labels`, and `config` (object mirroring the documented cluster config).
- `input` **MAY** accept optional wait parameters:
  - `wait` (boolean): If true, poll the ClusterDeployment until ready or timeout
  - `pollInterval` (duration string): How often to check status (default: "30s")
  - `provisionTimeout` (duration string): Maximum time to wait for provisioning (default: "30m")
  - `stallThreshold` (duration string): Warn if no progress detected for this duration (default: "10m")
- The tool **SHALL** resolve the target namespace as follows:
  - If `input.namespace` is provided, use it after verifying filter access.
  - Else if runtime is in dev mode (`AuthModeDevAllowAny`), default to `kcm-system`.
  - Else choose the first namespace allowed by the filter; if none exist, return `forbidden`.
- The tool **SHALL** verify that the referenced template and credential exist (global or local as appropriate) before applying the deployment.
- The server **SHALL** apply the `ClusterDeployment` via server-side apply, label it with `k0rdent.mirantis.com/managed=true`, and use field owner `mcp.clusters`.
- When `wait=true`, the tool **SHALL** poll the ClusterDeployment's status conditions, tracking state changes for stall detection and logging warnings when no progress is detected.
- Re-running the tool with identical inputs **SHALL** result in the resource being updated without error (idempotent).

#### Scenario: Dev mode deployment
- GIVEN the server runs with `AuthMode=DEV_ALLOW_ANY`
- WHEN `k0rdent.cluster.deploy` is called without `namespace`
- THEN the `ClusterDeployment` is applied in `kcm-system`
- AND the result reports `namespace="kcm-system"`

#### Scenario: Production deployment with filter
- GIVEN namespace filter `^team-`
- WHEN `k0rdent.cluster.deploy` is called without `namespace`
- THEN the server selects the first namespace matching `^team-`
- AND applies the resource there

#### Scenario: Forbidden namespace
- WHEN `k0rdent.cluster.deploy(namespace="prod-secure", ...)` is called and the namespace fails the filter
- THEN the server returns MCP error `forbidden`
- AND the resource is not applied

#### Scenario: Missing template
- WHEN `k0rdent.cluster.deploy` references a template that does not exist in the resolved namespace or global namespace
- THEN the tool refreshes its view and returns MCP error `invalidParams` listing available templates

#### Scenario: Deploy with wait
- WHEN `k0rdent.cluster.deploy(wait=true, pollInterval="30s", provisionTimeout="30m")` is called
- THEN the tool applies the ClusterDeployment
- AND polls its status every 30 seconds
- AND returns when Ready=True or after 30 minutes timeout
- AND logs warnings if no progress detected for 10 minutes (stall threshold)

### Requirement: Deployment feedback
- `DeployResult` **SHALL** include the `name`, `namespace`, `uid`, `resourceVersion`, and an `status` field set to `"created"` or `"updated"`.
- The tool **SHALL** surface Kubernetes validation errors (e.g., schema mismatch) as MCP `invalidParams` with the underlying message preserved.

#### Scenario: Validation failure surfaced
- WHEN Kubernetes returns a validation error for the requested `spec.config`
- THEN `k0rdent.cluster.deploy` returns MCP error `invalidParams` containing the API server message
- AND logs the failure with structured context (template, credential, namespace)

### Requirement: Delete cluster
- The MCP server **SHALL** expose `k0rdent.cluster.delete(name, namespace?, wait?, pollInterval?, deletionTimeout?) -> DeleteResult` to remove `ClusterDeployment` resources.
- The tool **SHALL** resolve the target namespace using the same rules as deploy (explicit namespace with filter check, dev mode default, or filter-based selection).
- The delete operation **SHALL** remove the resource using foreground propagation policy to ensure proper finalizer execution and child resource cleanup.
- `input` **MAY** accept optional wait parameters:
  - `wait` (boolean): If true, poll until the resource is deleted (default: false)
  - `pollInterval` (duration string): How often to check for deletion (default: "60s")
  - `deletionTimeout` (duration string): Maximum time to wait for deletion (default: "20m")
- By default (`wait=false`), the tool **SHALL** return immediately after initiating deletion, trusting the CAPI provider to complete cleanup.
- When `wait=true`, the tool **SHALL** poll the resource until it no longer exists (NotFound) or the timeout is exceeded.
- If the resource does not exist, the tool **SHALL** return success (idempotent deletion).
- `DeleteResult` **SHALL** include `name`, `namespace`, and `status` set to `"deleted"` or `"not_found"`.

#### Scenario: Delete in dev mode
- GIVEN the server runs with `AuthMode=DEV_ALLOW_ANY`
- WHEN `k0rdent.cluster.delete(name="test-cluster")` is called
- THEN the tool deletes the `ClusterDeployment` from `kcm-system`
- AND returns `status="deleted"`

#### Scenario: Delete non-existent cluster
- WHEN `k0rdent.cluster.delete` targets a deployment that does not exist
- THEN the tool returns `status="not_found"` without error

#### Scenario: Forbidden namespace on delete
- WHEN `k0rdent.cluster.delete(namespace="restricted", ...)` is called and namespace fails filter
- THEN the server returns MCP error `forbidden`

#### Scenario: Delete with wait
- WHEN `k0rdent.cluster.delete(name="test-cluster", wait=true, pollInterval="60s", deletionTimeout="20m")` is called
- THEN the tool initiates deletion using foreground propagation
- AND polls the resource every 60 seconds
- AND returns when the resource no longer exists (NotFound) or after 20 minutes timeout

### Requirement: Metrics & logging
- The server **SHALL** emit structured logs for each list/deploy/delete call (operation, namespace scope, template/credential names, duration, outcome).
- Prometheus metrics **SHALL** track counts and durations for list/deploy/delete operations (labelled by outcome). These metrics integrate with existing telemetry configuration.

#### Scenario: Metrics updated on deploy
- WHEN `k0rdent.cluster.deploy` completes successfully
- THEN the `clusters_deploy_total` counter increments with label `outcome="success"`
- AND the deployment duration histogram records the elapsed time

#### Scenario: Metrics updated on delete
- WHEN `k0rdent.cluster.delete` completes successfully
- THEN the `clusters_delete_total` counter increments with label `outcome="success"`

### Requirement: Live integration testing
- The project **SHALL** provide live integration tests under `test/integration` guarded by the `live` build tag.
- Tests **SHALL** validate the full cluster lifecycle (list credentials, list templates, deploy, verify, delete) using real Azure infrastructure.
- The baseline test **SHALL** use existing Azure resources: credential `azure-cluster-credential`, template `azure-standalone-cp-1-0-15`, and a test deployment configuration matching the `test1` cluster pattern (westus2, Standard_A4_v2 VMs, 1 control plane + 1 worker).
- Tests **SHALL** clean up deployed resources on completion or failure to prevent resource leaks.
- Tests **SHALL** skip gracefully when required environment variables (`K0RDENT_MGMT_KUBECONFIG_PATH`, `AUTH_MODE`) are not set.

#### Scenario: Full cluster lifecycle test
- GIVEN live integration tests run with Azure credentials configured
- WHEN the test executes the full workflow (list → deploy → verify → delete)
- THEN credentials and templates are successfully listed
- AND a test cluster deploys without errors
- AND the deployment reaches ready state
- AND deletion completes with proper cleanup

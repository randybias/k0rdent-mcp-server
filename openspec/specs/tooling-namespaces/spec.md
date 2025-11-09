# tooling-namespaces Specification

## Purpose
TBD - created by archiving change refactor-tool-namespace-hierarchy. Update Purpose after archive.
## Requirements
### Requirement: Tool namespace hierarchy
- MCP tool names **SHALL** follow a scoped hierarchy of the form `k0rdent.<plane>.<category>[.<action>]`, where `<plane>` is one of `catalog`, `mgmt`, `provider`, `child`, `children`, or `regional`.
- Catalog discovery operations **SHALL** remain under `k0rdent.catalog.*`, while management-plane installs (e.g., `k0rdent.mgmt.serviceTemplates.install_from_catalog`) live under `k0rdent.mgmt.*`.
- Management-cluster scoped tools (e.g., listing ServiceTemplates installed in the management plane) **SHALL** live under `k0rdent.mgmt.*`.
- Provider-specific operations (e.g., AWS/Azure/GCP cluster deployment with provider-specific parameters) **SHALL** live under `k0rdent.provider.*`.
- Single child-cluster scoped tools **SHALL** live under `k0rdent.child.*` and require a cluster identifier input; cross-child operations belong to `k0rdent.children.*`; regional control-plane operations belong to `k0rdent.regional.*`.
- New tools **SHALL** declare their namespace scope in registration metadata so automated linting can verify compliance.

#### Scenario: Catalog install tool rename
- WHEN `k0rdent.mgmt.serviceTemplates.install_from_catalog` is registered
- THEN its final tool name remains under the management plane
- AND its registration metadata marks the scope as `mgmt`.

#### Scenario: Provider-specific deployment tool
- WHEN a tool deploys AWS clusters with AWS-specific parameters
- THEN it is registered as `k0rdent.provider.aws.clusterDeployments.deploy`
- AND its registration metadata marks the scope as `provider`
- AND the tool exposes AWS-specific parameters (region, instanceType).

#### Scenario: Child-cluster pod logs tool
- WHEN a tool targets a specific workload cluster (e.g., pod logs)
- THEN it is registered as `k0rdent.child.pods.logs`
- AND its schema requires a `cluster` selector.

#### Scenario: Validation
- WHEN a developer attempts to register `k0rdent.mgmt.podLogs.get`
- THEN validation fails because the `<plane>` segment is missing and tooling points to the hierarchy spec.

### Requirement: Existing tool mapping
- The following tools **SHALL** follow the hierarchy as the authoritative mapping:

| Plane    | Tool Name                                                | Status       | Notes |
|----------|----------------------------------------------------------|--------------|-------|
| catalog  | `k0rdent.catalog.serviceTemplates.list`                  | ✅ Implemented | Lists catalog ServiceTemplates (discover-only). |
| mgmt     | `k0rdent.mgmt.serviceTemplates.install_from_catalog`     | ✅ Implemented | Installs a catalog template into the mgmt cluster. |
| mgmt     | `k0rdent.mgmt.serviceTemplates.delete`                   | ✅ Implemented | Deletes a ServiceTemplate from the mgmt cluster. |
| mgmt     | `k0rdent.mgmt.serviceTemplates.list`                     | ✅ Implemented | Lists ServiceTemplates via API package. |
| mgmt     | `k0rdent.mgmt.providers.list`                            | ✅ Implemented | Lists available infrastructure providers (AWS, Azure, Google, etc.). |
| mgmt     | `k0rdent.mgmt.providers.listCredentials`                 | ✅ Implemented | Lists credentials for a provider (requires provider input). |
| mgmt     | `k0rdent.mgmt.providers.listIdentities`                  | ✅ Implemented | Lists deployment identities mapped to credentials. |
| mgmt     | `k0rdent.mgmt.clusterTemplates.list`                     | ✅ Implemented | Lists installed ClusterTemplates. |
| mgmt     | `k0rdent.mgmt.clusterDeployments.list`                   | ✅ Implemented | Lists ClusterDeployments via cluster manager. |
| mgmt     | `k0rdent.mgmt.clusterDeployments.listAll`                | ✅ Implemented | Direct API listing (selector support). |
| mgmt     | `k0rdent.mgmt.clusterDeployments.delete`                 | ✅ Implemented | Deletes ClusterDeployment CRs. |
| mgmt     | `k0rdent.mgmt.namespaces.list`                           | ✅ Implemented | Lists namespaces in mgmt cluster. |
| mgmt     | `k0rdent.mgmt.podLogs.get`                               | ✅ Implemented | Fetches pod logs from mgmt cluster. |
| mgmt     | `k0rdent.mgmt.podLogs`                                   | ✅ Implemented | Streaming pod log tails (resource template). |
| mgmt     | `k0rdent.mgmt.events.list`                               | ✅ Implemented | Lists K8s events in mgmt cluster. |
| mgmt     | `k0rdent.mgmt.events`                                    | ✅ Implemented | Streaming namespace events (resource template). |
| mgmt     | `k0rdent.mgmt.graph.snapshot`                            | Deferred | Removed by change `remove-graph-feature`; future spec will reintroduce scoped graph relationships. |
| mgmt     | `k0rdent.mgmt.graph`                                     | Deferred | Removed by change `remove-graph-feature`; future spec will define graph streaming semantics. |
| mgmt     | `k0rdent.mgmt.multiClusterServices.list`                 | ✅ Implemented | Lists MultiClusterService CRs. |
| provider | `k0rdent.provider.aws.clusterDeployments.deploy`         | ✅ Implemented | Deploys AWS cluster with AWS-specific parameters. |
| provider | `k0rdent.provider.azure.clusterDeployments.deploy`       | ✅ Implemented | Deploys Azure cluster with Azure-specific parameters. |
| provider | `k0rdent.provider.gcp.clusterDeployments.deploy`         | ✅ Implemented | Deploys GCP cluster with GCP-specific parameters. |

**Note**: `k0rdent.mgmt.clusterDeployments.deploy` was intentionally **not implemented**. Provider-specific tools replace this generic tool for better AI agent discoverability and provider-specific validation.

- Future tools **MUST NOT** introduce new planes without spec approval; instead they extend the `category`/`action` portion within approved planes.

#### Scenario: Approval gate
- WHEN the team reviews tool naming
- THEN the table above is used as the authoritative list to approve or reject migrations.

### Requirement: Registration metadata
- `mcp.AddTool` callers **SHALL** provide scope metadata (e.g., `Meta["plane"]="mgmt"`, `Meta["category"]="podLogs"`) so lints/documentation generators know which namespace segment applies.
- Tool registrations **SHALL** fail CI validation when metadata is missing or mismatched with the tool name.

#### Scenario: Registration with metadata
- WHEN registering `k0rdent.mgmt.namespaces.list`
- THEN the registration metadata includes `plane="mgmt"` and `category="namespaces"`.

### Requirement: Namespace linting
- CI tooling **SHALL** lint tool names to ensure they follow the hierarchy, rejecting ad-hoc prefixes.
- The lint **SHALL** verify the `<plane>` segment matches the declared metadata and that `<category>` uses lowercase dotted words.

#### Scenario: Lint failure
- WHEN a tool is registered as `k0rdent.foo.bar`
- THEN linting fails with guidance referencing this spec until the name uses an approved plane.

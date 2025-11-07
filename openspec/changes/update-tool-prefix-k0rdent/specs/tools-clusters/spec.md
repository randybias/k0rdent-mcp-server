## MODIFIED Requirements

### Requirement: List accessible credentials
- The MCP tool name **SHALL** change from `k0rdent.clusters.listCredentials` to `k0rdent.clusters.listCredentials`.

#### Scenario: List default credentials
- WHEN `k0rdent.clusters.listCredentials()` is called in production mode with namespace filter `^team-`
- THEN the response contains credentials from `kcm-system` and all namespaces matching `^team-`

### Requirement: List cluster templates
- The MCP tool name **SHALL** change from `k0rdent.clusters.listTemplates` to `k0rdent.clusters.listTemplates`.

#### Scenario: Global template discovery
- WHEN `k0rdent.clusters.listTemplates(scope="global")` is called
- THEN the response enumerates templates from `kcm-system` with metadata and config schema outline

### Requirement: Deploy cluster
- The MCP tool name **SHALL** change from `k0rdent.clusters.deploy` to `k0rdent.clusters.deploy`.

#### Scenario: Dev mode deployment
- GIVEN the server runs with `AuthMode=DEV_ALLOW_ANY`
- WHEN `k0rdent.clusters.deploy` is called without `namespace`
- THEN the `ClusterDeployment` is applied in `kcm-system`

### Requirement: Deployment feedback
- Any returned status, logging, or metrics entries **SHALL** reference `k0rdent.clusters.*` identifiers rather than the legacy prefix.

#### Scenario: Validation failure surfaced
- WHEN Kubernetes returns a validation error for the requested `spec.config`
- THEN `k0rdent.clusters.deploy` returns an MCP `invalidParams` error containing the API server message and logs the failure using the updated tool identifier

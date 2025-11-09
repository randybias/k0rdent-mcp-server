# tools-clusters Specification

## Purpose
TBD - created by archiving change update-cluster-list-details. Update Purpose after archive.
## Requirements
### Requirement: Cluster deployment summaries include full context
- `k0rdent.mgmt.clusterDeployments.list` **SHALL** return `ClusterDeploymentSummary` objects that include:
  - metadata: `name`, `namespace`, `labels`, `age`
  - spec context: `templateRef` (name + version), `credentialRef`, `clusterIdentityRef`, `cloudProvider`, `region`
  - status context: `phase`, `ready`, `message`, `conditions[]` (type/status/reason/message/lastTransitionTime)
  - ops context: `kubeconfigSecret`, `managementURL` (when labels/annotations expose it)

#### Scenario: Summary for Azure baseline cluster
- GIVEN a ClusterDeployment created from template `azure-standalone-cp-1-0-15` using credential `azure-cluster-credential`
- WHEN `k0rdent.mgmt.clusterDeployments.list(namespace="kcm-system")` returns the entry
- THEN the summary includes `templateRef.name="azure-standalone-cp-1-0-15"`, `credentialRef.name="azure-cluster-credential"`, `cloudProvider="azure"`, and `region="westus2"`, along with readiness and condition data derived from status.

#### Scenario: Status propagation
- WHEN the underlying ClusterDeployment reports phase `Provisioning` with a failing condition
- THEN the MCP summary exposes `phase="Provisioning"`, `ready=false`, and the failing condition details so users can see the root cause without extra calls.

### Requirement: Documentation and tests reflect enriched fields
- The cluster provisioning docs **SHALL** describe the expanded response schema with examples.
- Live/integration tests **SHALL** assert the presence of the new fields so regressions are caught.

#### Scenario: Live test assertion
- WHEN the live test lists clusters after creating one
- THEN it verifies `templateRef.name` and `credentialRef.name` are populated in the response.


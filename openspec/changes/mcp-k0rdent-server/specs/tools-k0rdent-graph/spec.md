# k0rdent + Graph (delta)

## ADDED Requirements

### Requirement: k0rdent CRD listing
- The server **SHALL** provide the following tools:
  - `k0rdent.k0rdent.serviceTemplates.list()`
  - `k0rdent.k0rdent.clusterDeployments.list(selector?)`
  - `k0rdent.k0rdent.multiClusterServices.list(selector?)`
- Each list tool **SHALL** return at least name, namespace, labels, and a concise spec summary.

#### Scenario: List ServiceTemplates
- WHEN the list tool is invoked  
- THEN ServiceTemplates are returned with name/namespace/labels/spec summary

### Requirement: Graph snapshot & deltas
- The server **SHALL** provide `k0rdent.graph.snapshot(ns?, kinds?) -> {nodes, edges}`.
- The server **SHALL** provide `sub.k0rdent.graph(ns?, kinds?)` to stream `add|update|delete` deltas.
- Graph edges **SHALL** be derived from Kubernetes ownerReferences/selectors and k0rdent CRD relationships (e.g., `ClusterDeployment.spec.serviceSpec.services[].template`; MultiClusterService selectors/precedence).

#### Scenario: ClusterDeployment links to ServiceTemplate
- GIVEN a ClusterDeployment referencing a ServiceTemplate in `.spec.serviceSpec.services[]`  
- WHEN `k0rdent.graph.snapshot()` is called  
- THEN the graph contains an edge from the ClusterDeployment to the referenced ServiceTemplate

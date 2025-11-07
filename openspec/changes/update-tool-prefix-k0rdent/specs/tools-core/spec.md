## MODIFIED Requirements

### Requirement: List namespaces
- The MCP tool name **SHALL** be updated from `k0rdent.namespaces.list()` to `k0rdent.namespaces.list()`.

#### Scenario: Namespaces returned
- WHEN the tool `k0rdent.namespaces.list()` is called
- THEN it returns the current namespaces with name/labels/status

### Requirement: Namespace events
- The MCP tool name **SHALL** be updated from `k0rdent.events.*` to `k0rdent.events.*` (e.g., `k0rdent.events.list` → `k0rdent.events.list`).

#### Scenario: List Warning events for a pod
- WHEN called with `k0rdent.events.list(types="Warning", forKind="Pod", forName="my-pod")`
- THEN only Warning events about `my-pod` in the namespace are returned

### Requirement: Pod logs (snapshot & follow)
- The MCP tool names **SHALL** be updated from `k0rdent.podLogs.*` to `k0rdent.podLogs.*`.

#### Scenario: Tail last 100 lines
- WHEN called with `k0rdent.podLogs.get(tailLines=100)`
- THEN the last 100 lines are returned

#### Scenario: Follow stream
- WHEN called with `k0rdent.podLogs.get(follow=true)`
- THEN the server streams new log lines until cancelled

### Requirement: K0rdent resource listing tools
- The MCP tool names for ServiceTemplates, ClusterDeployments, and MultiClusterServices **SHALL** be updated from `k0rdent.k0rdent.*` to `k0rdent.k0rdent.*` (e.g., `k0rdent.k0rdent.serviceTemplates.list` → `k0rdent.k0rdent.serviceTemplates.list`).

#### Scenario: List ServiceTemplates
- WHEN `k0rdent.k0rdent.serviceTemplates.list()` is called
- THEN ServiceTemplates are returned with name/namespace/labels/spec summary

### Requirement: Graph subscription tools
- The MCP tool names **SHALL** be updated from `k0rdent.graph.*` to `k0rdent.graph.*`.

#### Scenario: Graph snapshot
- WHEN `k0rdent.graph.snapshot()` is called
- THEN the graph snapshot is returned using the updated tool identifier

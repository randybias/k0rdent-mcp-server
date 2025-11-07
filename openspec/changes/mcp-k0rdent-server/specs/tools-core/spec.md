# Core Tools (delta)

## ADDED Requirements

### Requirement: List namespaces
- The server **SHALL** provide the MCP tool `k0rdent.namespaces.list() -> Namespace[]`, returning each namespace’s name, labels, and phase/status.

#### Scenario: Namespaces returned
- WHEN the tool is called  
- THEN it returns the current namespaces with name/labels/status

### Requirement: Namespace events
- The server **SHALL** provide `k0rdent.events.list(namespace, sinceSeconds?, limit?, types?, forKind?, forName?)`.
- The implementation **SHALL** read Events from `events.k8s.io/v1` when available and **SHALL** fall back to core `v1` Events otherwise.
- The tool **SHALL** support field selectors for the involved/regarding object (kind & name).

#### Scenario: List Warning events for a pod
- WHEN called with `types=Warning`, `forKind=Pod`, `forName=my-pod`  
- THEN only Warning events about `my-pod` in the namespace are returned

### Requirement: Pod logs (snapshot & follow)
- The server **SHALL** provide `k0rdent.podLogs.get(ns, pod, container?, tailLines?, sinceSeconds?, previous?, follow?)`.
- When `follow=true`, the server **SHALL** stream new log lines until cancelled.
- If the target Pod has multiple containers, the tool **SHALL** require `container` to be specified.

#### Scenario: Tail last 100 lines
- WHEN called with `tailLines=100`  
- THEN the last 100 lines are returned

#### Scenario: Follow stream
- WHEN called with `follow=true`  
- THEN the server streams new log lines until cancelled

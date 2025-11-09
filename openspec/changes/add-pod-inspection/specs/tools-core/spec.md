# Core Tools (delta)

## ADDED Requirements

### Requirement: Pod list tool
- The server **SHALL** expose the MCP tool `k0rdent.pods.list(namespace: string) -> PodSummary[]` that lists pods in the provided namespace.
- Each `PodSummary` **SHALL** include: `name`, `namespace`, `phase`, `readyContainers`, `totalContainers`, `restartCount`, `nodeName` (if scheduled), and `startTime` (RFC3339 string).
- The tool **SHALL** sort results alphabetically by pod name.
- When a namespace filter is configured, the tool **SHALL** reject requests for namespaces that do not match the filter with a `forbidden` MCP error.

#### Scenario: List pods in an allowed namespace
- WHEN `k0rdent.pods.list(namespace="team-a")` is called and the namespace matches the active filter
- THEN the response contains one entry per pod in `team-a` with the summary fields listed above
- AND `readyContainers` reflects the number of containers in `Ready=True` state while `totalContainers` reflects the total init+app containers

#### Scenario: Namespace outside filter is rejected
- WHEN `k0rdent.pods.list(namespace="kube-system")` is called and the namespace filter excludes `kube-system`
- THEN the server returns an MCP error with code `forbidden`
- AND no pod data is returned

### Requirement: Pod inspect tool
- The server **SHALL** expose the MCP tool `k0rdent.pods.inspect(namespace: string, pod: string) -> PodDetail` for retrieving detailed status of a single pod.
- `PodDetail` **SHALL** include: metadata (`name`, `namespace`, `uid`, `labels`, `annotations`, `nodeName`, `podIP`, `hostIP`, `startTime`), top-level status (`phase`, `reason`, `message`), pod conditions (type, status, reason, message, lastTransitionTime), and container statuses for both init and app containers (ready flag, restart count, current state, last termination state).
- The tool **SHALL** enforce the namespace filter before fetching the pod.
- When the pod does not exist, the tool **SHALL** return an MCP error with code `notFound`.

#### Scenario: Inspect multi-container pod
- WHEN `k0rdent.pods.inspect(namespace="team-a", pod="web-0")` is called for a pod with multiple containers
- THEN the response includes each container with its `ready`, `restartCount`, `state` (`running|waiting|terminated`), and the last termination reason/message if available
- AND pod conditions include the most recent `Ready` condition with its status and transition timestamp

#### Scenario: Pod not found
- WHEN `k0rdent.pods.inspect(namespace="team-a", pod="does-not-exist")` is called
- THEN the server returns an MCP error with code `notFound`
- AND the error message states that the pod was not found in `team-a`

#### Scenario: Namespace filtered out
- WHEN `k0rdent.pods.inspect(namespace="kube-system", pod="coredns-123")` is called and `kube-system` fails the namespace filter
- THEN the server returns an MCP error with code `forbidden`
- AND no pod metadata or status is revealed

# Change: Add pod inspection tools

## Why
- MCP clients currently have no way to enumerate pods or inspect an individual pod's status, making day-to-day troubleshooting workflows impossible without leaving Claude Code.
- Operators need a lightweight view of pod readiness (Ready containers, restart counts, assigned node) and detailed container state (waiting vs running, last termination reason) to triage incidents.
- KubeView exposes pod listings and drill-down detail panes; the k0rdent MCP Server must reach feature parity for the same management cluster personas.

## What Changes
- Add a `k0rdent.pods.list` MCP tool that returns a namespace-scoped list of pods with summary fields (phase, Ready/total containers, restart count, node, age) while respecting the configured namespace filter.
- Add a `k0rdent.pods.inspect` MCP tool that returns full metadata and status for a specific pod: phase/reason/message, node and IPs, conditions, container statuses (ready flag, restart count, state, last termination info).
- Wire pod access through the existing runtime session so the tools use the per-request bearer token and reuse shared logging/metrics patterns.
- Cover the new tools with unit tests using the fake Kubernetes client and extend documentation so clients know how to call them.

## Impact
- **Affected specs**: `tools-core` (new requirements for pod list & inspect tools)
- **Affected code**:
  - `internal/tools/core/` (new pod tool implementations and registration)
  - `internal/runtime/` or helper packages for pod summarisation helpers
  - `test/` (unit tests for pod list/inspect behaviour)
  - Developer documentation describing pod tooling
- **Breaking**: No; the change only adds new MCP tools.

## Out of Scope
- Mutating pod operations (delete/evict/restart) and exec/attach capabilities
- Cross-namespace aggregation beyond what the current namespace filter already permits
- Pagination or advanced filtering (label selectors, node selectors) for pod listings

## Acceptance
- `openspec validate add-pod-inspection --strict` passes
- `k0rdent.pods.list(namespace)` returns only pods within the namespace that survives the active namespace filter, including Ready/total, restarts, phase, node, and age fields
- `k0rdent.pods.inspect(namespace, pod)` returns metadata, conditions, and per-container statuses (current state, last termination, restart counts) for the requested pod
- Unit tests cover success and failure flows (namespace filtered out, pod not found, multi-container pod summaries)
- Documentation examples show how to call both tools from an MCP client

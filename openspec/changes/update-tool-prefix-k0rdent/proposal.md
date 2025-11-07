# Change: Update tool namespace prefix to k0rdent

## Why
- MCP tool names previously used a short `k0` prefix (e.g., `k0rdent.namespaces.list`). This collided with other Mirantis offerings such as k0s and k0smotron, causing confusion when users browse Claude’s tool catalog or inspect resource URIs.
- Renaming the top-level namespace to `k0rdent.` aligns the tooling with the product name, clarifies ownership, and prevents collisions should other MCP servers adopt that legacy prefix.
- Aligning both tools and resources ensures clients, logs, and metrics present a consistent identity for the k0rdent MCP server.

## What Changes
- Update every MCP tool registration so names move from the legacy prefix to `k0rdent.*`, keeping the remainder of each identifier intact (e.g., `k0rdent.namespaces.list` → `k0rdent.namespaces.list`).
- Adjust any resource URIs, notification channels, or metrics labels that embed the old prefix.
- Refresh specs, tests, docs, and client examples to reflect the new namespace.
- Provide compatibility guidance (e.g., changelog entry) and consider a short-lived alias period if clients might still request the old names.

## Impact
- **Affected specs**: `tools-core`, `tools-catalog`, `tools-clusters` (and any other pending tool specs) to reflect the new prefixes.
- **Affected code**:
  - `internal/tools/core/*.go` registrations and log statements.
  - Tests referencing tool names.
  - Documentation and examples.
  - Metrics/telemetry code where the tool name is emitted.
- **Breaking**: Yes – MCP clients must call the new tool names. We may optionally provide server-side aliases or a transition notice.

## Out of Scope
- Renaming package paths or binary names.
- Implementing long-term dual-support for both prefixes (only consider brief aliasing for compatibility during rollout planning).

## Acceptance
- `openspec validate update-tool-prefix-k0rdent --strict` passes.
- All tool registrations use the `k0rdent.` prefix and tests/docs match the new names.
- Specs for affected capabilities list the updated tool identifiers.
- Server logs and metrics reference the new prefix; optional compatibility alias (if implemented) is documented.

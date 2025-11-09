# Change: remove-graph-feature

## Why
- The current `k0rdent.mgmt.graph` snapshot/resource pair does not return meaningful data because the relationship model was never specified beyond a stub, yet it still adds Kubernetes watches, MCP resource templates, and subscription routing complexity.
- Idle graph watchers and tooling increase CPU/memory footprint and error surface (e.g., watch reconnects, notification retries) without delivering user value.
- We need to explicitly defer the graph capability until we can scope the relationship model, data contracts, and tooling ergonomics via a future OpenSpec change rather than keeping vestigial code paths.

## What Changes
- Remove the GraphManager implementation, registration hooks, and MCP tool/resource declarations so the server no longer exposes `k0rdent.mgmt.graph.*` endpoints.
- Update the authoritative tooling namespaces spec to drop the "Implemented" status for `k0rdent.mgmt.graph.snapshot` and `k0rdent.mgmt.graph`, clarifying that graph tooling is deferred and requires a future, fully scoped change.
- Capture this placeholder change so future work can link to it when defining the graph relationship model.

## Impact
- MCP clients stop seeing or invoking the broken graph tools; subscription routing now only permits `events` and `podlogs`.
- Server footprint shrinks (no idle watchers) and the remaining specs more accurately reflect supported capabilities.
- Future graph work must introduce a new change/spec to define behaviors before reintroducing tools.

## Out of Scope
- Designing the new relationship schema, edge semantics, or streaming format.
- Replacing the graph feature with any interim visualization (e.g., tabular joins or alternative APIs).
- Updating pending changes that referenced GraphManager lifecycle fixes; those changes can be rebased or trimmed separately once this removal lands.

## Acceptance
- `k0rdent.mgmt.graph.snapshot` tool and `k0rdent.mgmt.graph` resource template are no longer registered or discoverable via MCP.
- Subscription router refuses `k0rdent://graph` URIs because no handler is registered.
- `openspec/specs/tooling-namespaces/spec.md` lists graph tooling as deferred (not implemented) and references this change as the placeholder.
- `go test ./...` passes (with `GOCACHE` redirected inside the repo if needed) and `openspec validate remove-graph-feature --strict` succeeds.

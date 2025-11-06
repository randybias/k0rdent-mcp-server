# Change: extend-live-cluster-tests

## Why
- Current live test coverage only validates namespace listing; it does not verify pod log access or k0rdent CRD behaviour.
- We want a consistent pattern for future live tests that fail fast rather than silently skipping when configuration is incomplete.

## What Changes
- Add additional live tests covering pod logs and k0rdent CRD listing via the MCP tools.
- Centralize common live test helpers (env validation, MCP session handshake, SSE parsing) for reuse.
- Document expectations for future live tests to rely on these helpers.

## Impact
- Increases confidence that foundational Kubernetes operations and k0rdent-specific tooling work against a real cluster.
- Makes future live tests easier to author with consistent behaviour.

## Acceptance
- New helpers exist under `test/integration` for live tests.
- Live tests cover namespaces list, pod logs retrieval, and the three k0rdent list tools, all using the shared helper.
- Documentation explains how to run the live suite and references the helper utilities.
- `openspec validate extend-live-cluster-tests --strict` passes.

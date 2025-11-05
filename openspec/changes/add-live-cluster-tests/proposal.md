# Change: add-live-cluster-tests

## Why
- We currently rely on ad hoc manual validation against a live Kubernetes cluster, making regressions hard to catch.
- Operators want an optional test suite that exercises the MCP server against an actual management cluster when credentials are available.

## What Changes
- Introduce an opt-in integration test package that connects to a live cluster using the same configuration environment variables as the server (e.g., kubeconfig path).
- Provide a make/CLI entrypoint or documented command to run these tests when a live cluster is available.
- Ensure tests skip automatically when required environment variables are missing.

## Impact
- Improves confidence before deploying changes by running against real infrastructure.
- Keeps default `go test ./...` fast by skipping live tests unless explicitly requested.

## Acceptance
- A documented command (e.g., `go test ./test/integration -tags=live`) executes live-cluster tests.
- Tests skip gracefully when the necessary env vars are absent.
- Live tests cover at least namespace listing via the MCP tools pipeline.
- `openspec validate add-live-cluster-tests --strict` passes.

# Change: fix-watcher-lifecycle

## Why
- GraphManager, EventManager, and PodLogManager spawn goroutines with `context.Background()`, causing watchers to continue running after HTTP server shutdown.
- No graceful cleanup of Kubernetes watch connections during shutdown, leading to resource leaks and incomplete cleanup.
- Hardcoded 1-second sleep for watch reconnection provides no exponential backoff, causing aggressive retry storms during Kubernetes API outages.
- No circuit breaker pattern when the Kubernetes API becomes persistently unavailable, wasting resources on futile connection attempts.
- Server shutdown in `main.go:187-193` does not wait for active watchers to complete, potentially losing in-flight notifications.

## What Changes
- **BREAKING**: Modify watcher initialization to accept a parent context from the HTTP server lifecycle, not `context.Background()`.
- Add a `sync.WaitGroup` to track active watcher goroutines and ensure the server waits for all watchers to complete before exiting.
- Implement exponential backoff with jitter for watch reconnection (start at 1s, cap at 60s).
- Add a circuit breaker that trips after N consecutive failures and requires a cooldown period before retrying.
- Update `cmd/server/main.go` shutdown logic to:
  1. Signal context cancellation to all watchers
  2. Wait for all watchers to drain via WaitGroup
  3. Only then complete HTTP server shutdown
- Ensure all three watcher components (GraphManager, EventManager, PodLogManager) follow the same lifecycle pattern.

## Impact
- Affected specs: watcher-lifecycle (new), graph-manager (new), events (new), podlogs (new)
- Affected code:
  - `internal/tools/core/graph.go:271-298` (GraphManager watcher lifecycle)
  - `internal/tools/core/events.go:88-95,160` (EventManager watcher lifecycle)
  - `internal/tools/core/podlogs.go:160` (PodLogManager watcher lifecycle)
  - `cmd/server/main.go:187-193` (server shutdown)
  - New internal/kube/watchutil package for shared backoff/circuit breaker logic
- Migration: Server restarts will now cleanly terminate all watches before shutting down. No API changes for MCP clients.

## Acceptance
- All watchers receive a cancellable context derived from the HTTP server lifecycle, not `context.Background()`.
- Server shutdown waits for all active watchers to complete before exiting (verified via WaitGroup).
- Watch reconnection uses exponential backoff with jitter (1s → 2s → 4s → ... → 60s cap).
- Circuit breaker trips after 5 consecutive failures and requires 30s cooldown before allowing new connection attempts.
- Integration tests demonstrate graceful shutdown: start watchers, trigger shutdown, verify all watchers stop within 5s.
- `openspec validate fix-watcher-lifecycle --strict` passes.

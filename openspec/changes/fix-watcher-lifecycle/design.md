# Design: Watcher Lifecycle Management

## Context
The k0rdent MCP server runs in the management cluster and maintains long-lived Kubernetes watch connections for:
1. **GraphManager**: Watches ServiceTemplate, ClusterDeployment, and MultiClusterService CRDs
2. **EventManager**: Watches namespace-scoped Kubernetes events
3. **PodLogManager**: Streams pod logs via Kubernetes API

Currently, all three managers spawn goroutines with `context.Background()`, which prevents graceful shutdown. The server's HTTP lifecycle does not control watcher lifecycles, and there's no mechanism to wait for watchers to finish during shutdown.

### Current Problems
1. **Orphaned goroutines**: Watchers continue after HTTP server stops
2. **Resource leaks**: Kubernetes watch connections not closed cleanly
3. **Aggressive retries**: Hardcoded 1-second sleep causes retry storms during API outages
4. **No failure isolation**: No circuit breaker to back off when API is persistently unavailable
5. **Incomplete shutdown**: Server exits before in-flight notifications are delivered

## Goals / Non-Goals

### Goals
- Ensure all watchers terminate gracefully when HTTP server shuts down
- Implement exponential backoff with jitter for watch reconnection
- Add circuit breaker to prevent resource waste during persistent API failures
- Provide unified lifecycle management across all three watcher types
- Maintain existing MCP API surface (no breaking changes for clients)

### Non-Goals
- Leader election for watch distribution (already exists, not in scope)
- Watch resume from bookmark (future enhancement)
- Watch filtering/predicate pushdown (future enhancement)
- Metrics/instrumentation for watch health (future enhancement)

## Decisions

### Decision 1: Shared watchutil Package
Create `internal/kube/watchutil` package providing:
- **Backoff**: Exponential backoff with jitter (1s → 2s → 4s → ... → 60s cap)
- **CircuitBreaker**: Tracks consecutive failures, trips after threshold, requires cooldown

**Rationale**: All three managers need identical backoff/circuit breaker logic. Centralizing prevents drift and simplifies testing.

**Alternatives considered**:
- Inline implementations: Rejected due to duplication and maintenance burden
- Third-party library (e.g., go-kit/circuitbreaker): Rejected to minimize dependencies; our needs are simple

### Decision 2: Context Propagation
Pass HTTP server context to all managers, derive watcher contexts from it.

**Rationale**: HTTP server context is canceled when server shuts down, providing automatic propagation to all watchers.

**Alternatives considered**:
- Separate shutdown channel: Rejected as context cancellation is idiomatic Go
- Direct cancellation calls: Rejected as it couples managers to shutdown logic

### Decision 3: WaitGroup for Drain
Each manager maintains a `sync.WaitGroup` tracking active watcher goroutines. Shutdown calls `Stop()` which cancels context and waits on WaitGroup.

**Rationale**: Standard Go pattern for coordinating goroutine shutdown. Simple, testable, no race conditions.

**Alternatives considered**:
- Done channels per watcher: Rejected as WaitGroup is more idiomatic for N goroutines
- Timeout-only drain: Rejected as it can't guarantee all watchers stopped

### Decision 4: Backoff Parameters
- Initial: 1s
- Max: 60s
- Jitter: 0-20% of current delay
- Reset on successful watch

**Rationale**: Balances quick recovery (1s initial) with avoiding API hammering (60s cap). Jitter prevents thundering herd.

**Alternatives considered**:
- Fibonacci backoff: Rejected as exponential is simpler and well-understood
- Higher cap (e.g., 5m): Rejected as 60s is sufficient for transient API issues

### Decision 5: Circuit Breaker Parameters
- Failure threshold: 5 consecutive failures
- Cooldown: 30s
- Reset on any successful connection

**Rationale**: 5 failures * initial backoff ≈ 15s before tripping. 30s cooldown allows API to recover without excessive load.

**Alternatives considered**:
- Lower threshold (3): Rejected as too aggressive for transient failures
- Adaptive cooldown: Rejected as added complexity without clear benefit

## Risks / Trade-offs

### Risk: Shutdown Timeout
If a watcher is blocked on I/O, `Stop()` may hang waiting for WaitGroup.

**Mitigation**: Add timeout to `Stop()` method (default 10s). After timeout, log error and proceed. Set context cancellation before waiting so goroutines have signal to exit.

### Risk: Backoff State Loss
If server restarts frequently, backoff state is lost and retries start at 1s.

**Mitigation**: Acceptable for MVP. Backoff is per-process, and restarts indicate broader issues. Future: persist backoff state if needed.

### Risk: Circuit Breaker Too Aggressive
If circuit breaker trips during valid maintenance windows, watchers won't recover quickly.

**Mitigation**: 30s cooldown is short enough for most maintenance windows. Operators can force server restart to reset circuit breaker if needed. Future: add metrics to monitor circuit breaker state.

### Trade-off: Increased Complexity
Adding backoff and circuit breaker increases code complexity.

**Justification**: Current hardcoded 1-second sleep is production-hostile. Proper retry logic is essential for reliability. Complexity is isolated in watchutil package with comprehensive tests.

## Migration Plan

### Phase 1: watchutil Package
1. Create `internal/kube/watchutil` package with backoff and circuit breaker
2. Add unit tests
3. No impact to existing code

### Phase 2: Manager Updates (in parallel)
1. Update GraphManager, EventManager, PodLogManager with WaitGroup and context propagation
2. Integrate watchutil backoff and circuit breaker
3. Add `Stop()` methods
4. Update unit tests for each manager

### Phase 3: Server Shutdown
1. Update `cmd/server/main.go` to call `Stop()` on all managers before `httpServer.Shutdown()`
2. Add integration test for graceful shutdown
3. Add logging for shutdown progress

### Rollback
If critical issues arise:
1. Revert `main.go` shutdown changes (preserves existing behavior)
2. Watchers continue with `context.Background()` (no worse than current state)
3. No data loss as watchers are read-only

## Open Questions
- **Q**: Should circuit breaker state be exposed via health endpoint?
  - **A**: Not in MVP. Add in future if operators request it.

- **Q**: Should backoff/circuit breaker parameters be configurable via env vars?
  - **A**: Not in MVP. Hardcode sane defaults. Add configuration if monitoring shows tuning is needed.

- **Q**: Should watchers log backoff/circuit breaker events?
  - **A**: Yes, at INFO level for circuit breaker state changes, DEBUG for backoff delays. Helps operators diagnose API issues.

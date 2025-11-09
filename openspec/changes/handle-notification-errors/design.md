# Design: Handle Notification Errors

## Context
MCP notifications are sent from three locations in the codebase (graph.go, events.go, podlogs.go) whenever resource state changes. Currently, all notification errors are silently discarded using `_ = server.ResourceUpdated()`. This creates blind spots in observability and leaves subscribers potentially working with stale data.

The system needs a unified approach to handle notification failures consistently across all resource types while maintaining the existing non-blocking notification semantics.

## Goals / Non-Goals

### Goals
- Provide visibility into notification delivery failures through structured logging
- Enable operators to monitor notification health via Prometheus metrics
- Improve reliability through automatic retry of transient failures
- Maintain non-blocking behavior (notifications do not block resource processing)
- Consistent error handling across all notification types

### Non-Goals
- Guaranteed delivery semantics (notifications remain best-effort)
- Notification queuing or persistence
- Client-side acknowledgment protocol
- Change to existing MCP protocol or wire format
- Synchronous notification delivery

## Decisions

### Decision: Centralized Notification Helper
Create a shared notification helper function in `internal/tools/core/notifications.go` that encapsulates logging, metrics, and retry logic.

**Rationale:**
- DRY principle: Avoids duplicating error handling across three locations
- Consistency: Ensures uniform behavior for all notification types
- Testability: Single point to test error handling logic
- Maintainability: Changes to retry policy or metrics apply everywhere

**Alternatives considered:**
- Inline error handling at each call site: Rejected due to code duplication and inconsistency risk
- Middleware/interceptor pattern: Rejected as overkill for this scope, would require MCP SDK modifications

### Decision: Exponential Backoff Retry (3 attempts)
Retry transient failures with exponential backoff: 100ms, 200ms, 400ms.

**Rationale:**
- 3 attempts provides reasonable resilience without excessive delay
- Exponential backoff reduces thundering herd on recovery
- Total max delay ~700ms acceptable for best-effort notifications
- Aligns with common retry patterns (e.g., gRPC default retry)

**Alternatives considered:**
- Fixed interval retry: Rejected, can amplify load during outages
- More attempts (5+): Rejected, increases latency without proportional benefit for best-effort delivery
- Circuit breaker: Rejected as premature; metrics will inform if needed

### Decision: Error Classification (Retryable vs Non-Retryable)
Classify errors based on Go error types and patterns:
- **Retryable:** network errors (net.Error with Temporary()==true), context.DeadlineExceeded, "connection refused", "connection reset"
- **Non-retryable:** JSON marshaling errors, invalid parameters, context.Canceled

**Rationale:**
- Prevents retrying errors that cannot succeed (e.g., bad JSON)
- Focuses retry attempts on transient failures
- Standard Go error inspection patterns

**Alternatives considered:**
- Retry all errors: Rejected, wastes attempts on permanent failures
- Allowlist specific errors: Rejected, brittle and hard to maintain

### Decision: Prometheus Counter Metric
Use counter metric `mcp_notification_failures_total` with labels:
- `resource_type`: graph, events, podlogs
- `error_type`: network, timeout, serialization, other

**Rationale:**
- Counters are appropriate for cumulative failure counts
- Labels enable filtering by resource type and error category
- Follows Prometheus naming conventions (suffix: _total, snake_case)
- Enables alerting on rate of failures

**Alternatives considered:**
- Separate metric per resource type: Rejected, harder to aggregate
- Histogram for retry latency: Deferred to future work if needed
- Gauge for current failure state: Rejected, counter more useful for alerting

### Decision: Structured Logging with slog
Log failures using existing slog infrastructure with structured fields:
- `resource_type`: graph|events|podlogs
- `uri`: resource URI
- `error`: error message
- `attempt`: retry attempt number (1-3)

**Rationale:**
- Consistent with existing structured logging patterns (add-structured-logging change)
- Enables log aggregation and filtering in observability tools
- slog is already integrated throughout the codebase

## Risks / Trade-offs

### Risk: Retry delays impact throughput
**Mitigation:** Notifications are already async; retries happen in background. Max delay (700ms) acceptable for best-effort delivery.

### Risk: Metrics cardinality explosion
**Mitigation:** Only 2 labels with fixed, small value sets (3 resource types × 4 error types = 12 series max).

### Risk: Dependency on add-prometheus-metrics
**Mitigation:** Document dependency explicitly. If metrics infra not ready, stub out metric calls (no-op).

### Trade-off: No guaranteed delivery
**Accepted:** Notifications remain best-effort. Clients should implement polling fallback if they require guaranteed consistency.

## Migration Plan

### Phase 1: Logging (No Dependencies)
1. Create notification helper with logging (no metrics/retry yet)
2. Replace `_ = server.ResourceUpdated()` with logging calls
3. Deploy and verify logs appear on failures

### Phase 2: Metrics (Depends on add-prometheus-metrics)
1. Add Prometheus counter metric registration
2. Instrument notification helper with metric increments
3. Deploy and verify metrics in Prometheus

### Phase 3: Retry Logic
1. Implement error classification
2. Add retry loop with exponential backoff
3. Update tests to cover retry scenarios
4. Deploy and monitor retry success rates

### Rollback Plan
- Phase 1: Revert helper calls to `_ = server.ResourceUpdated()`
- Phase 2: Remove metric registration and increments
- Phase 3: Remove retry logic, revert to immediate failure

## Open Questions
- Q: Should retry policy be configurable via environment variables?
  - A: Deferred to future work. Start with hardcoded policy; make configurable if operators request it.

- Q: Should we add a metric for successful notifications?
  - A: No. Focus on failures; success is the default. Can add later if throughput monitoring needed.

- Q: What about notification latency metrics?
  - A: Deferred. Start with failure counts; add latency histogram if performance issues observed.

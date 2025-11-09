# Change: Handle MCP Notification Errors

## Why
MCP notification failures are silently ignored throughout the codebase, leaving subscribers potentially unaware of stale data. When `server.ResourceUpdated()` fails (network issues, client disconnects, serialization errors), the error is discarded with `_ =`, providing no visibility into delivery failures.

## What Changes
- Add ERROR-level structured logging for all notification failures
- Add Prometheus counter metrics tracking notification failures by resource type
- Implement retry logic with exponential backoff for transient failures
- Add test coverage for notification failure scenarios
- Document notification failure behavior in tool specifications

**Files affected:**
- `internal/tools/core/graph.go:424` - Graph delta notifications
- `internal/tools/core/events.go:210` - Event update notifications
- `internal/tools/core/podlogs.go:231` - Pod logs update notifications

## Impact
- Affected specs: mcp-notifications (new)
- Affected code: internal/tools/core/*.go (notification sending functions)
- Dependencies: add-prometheus-metrics (for metrics infrastructure)
- Non-breaking: Changes are additive (logging, metrics, retries)
- Observability: Operators gain visibility into notification delivery health

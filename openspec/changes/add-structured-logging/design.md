# Design: Structured Logging Abstraction

## Goals
- Provide consistent, structured logging across all server components.
- Support default JSON logs to stdout while leaving room for an additional sink (e.g., OpenTelemetry, Splunk) without hard-coding a specific vendor.
- Capture contextual metadata (request IDs, MCP session IDs, tool info) automatically to reduce logging boilerplate.

## Non-Goals
- Implement a concrete external logging backend (will remain a stub/sink interface for now).
- Replace metrics/tracing; focus is on logs only.

## Approach
1. **Logger Package**  
   - Create `internal/logging` with:
     - `Logger` wrapper built on top of `slog` (Go 1.21+) configured for JSON output.
     - `Sink` interface with a default no-op implementation and hooks to register additional sinks.
     - Context helpers for enriching logs with request/session metadata.
   - Provide initialisation function that reads env (`LOG_LEVEL`, `LOG_EXTERNAL_SINK_ENABLED`) and returns:
     - Base `slog.Logger` for stdout.
     - Optional sink (stub) invoked asynchronously to avoid blocking hot paths.

2. **HTTP & MCP Integration**  
   - Middleware in `internal/server` that:
     - Injects request ID (reuse chi's `RequestIDCtxKey`).
     - Adds MCP session ID to context when available.
     - Logs request start/end, status code, duration.
   - Update MCP session factory to pass enriched logger into tool registration runtime.

3. **Tool Instrumentation**  
   - Wrap tool handlers to emit debug/info logs containing tool name, parameters (redacted if needed), namespace selectors, success/failure, latency.
   - For follow/future streaming tools, log start/stop events.

4. **Configuration**  
   - Extend existing `config.Settings` with log-specific options sourced from env.
   - Provide tests ensuring invalid levels default safely, external sink toggle works, etc.

## Alternatives Considered
- Using a heavier framework (zap, zerolog) — rejected to keep dependencies minimal; `slog` in stdlib provides structured logging with minimal overhead.
- Dedicating to OpenTelemetry immediately — deferred until requirements solidify.

## Testing Strategy
- Unit tests for logging package (level parsing, sink invocation, context enrichment).
- Handler tests that assert log entries include request IDs (using test sink capturing logs).
- Tool handler tests verifying logs emit expected metadata on success/failure.

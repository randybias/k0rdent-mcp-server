# Logging (delta)

## ADDED Requirements

### Requirement: Structured stdout logging
- The server **SHALL** emit JSON-formatted logs to stdout using a consistent schema (timestamp, level, message, component).
- Logs **SHALL** include correlation identifiers when present (HTTP request ID, MCP session ID, tool name).

#### Scenario: HTTP request logged
- WHEN an HTTP request hits the MCP endpoint
- THEN a log entry is written with the request ID, method, path, response status, and duration in JSON form

### Requirement: External sink hook
- The server **SHALL** provide a pluggable logging sink interface that forwards every log entry.
- The external sink **SHALL** be optional and disabled by default; when enabled it **MUST NOT** block the main execution path.

#### Scenario: External sink enabled
- GIVEN `LOG_EXTERNAL_SINK_ENABLED=true`
- WHEN the server logs an event
- THEN the sink receives the log payload asynchronously without impacting request latency

### Requirement: Contextual instrumentation
- Configuration load, auth checks, client creation, and tool handlers **SHALL** log success and failure events at appropriate levels.
- Tool execution logs **SHALL** include target namespace(s) and parameters where safe, redacting sensitive values when necessary.

#### Scenario: Tool call failure logged
- WHEN a tool handler returns an error
- THEN an error log entry includes the tool name, reason, and correlation identifiers

### Requirement: Log configuration
- The server **SHALL** accept `LOG_LEVEL` to control minimum log level (default `INFO`).
- Invalid or missing levels **SHALL** default to `INFO`.

#### Scenario: Invalid log level input
- WHEN `LOG_LEVEL=LOUD` (invalid)
- THEN the server starts with level INFO and logs a warning about the invalid value

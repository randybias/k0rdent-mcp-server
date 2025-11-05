# Change: add-structured-logging

## Why
- Current server logs are ad-hoc and limited to startup messages, making troubleshooting difficult.
- Production requires consistent structured logging across HTTP transport, MCP sessions, and tool handlers.
- We need a path to emit logs to stdout (for container platforms) **and** to an eventual external logging system without rewriting core code.

## What Changes
- Introduce a centralized logging framework offering JSON-structured output to stdout and hooks for forwarding to an external sink (TBD provider).
- Add request/session level context (request IDs, session IDs, namespace, tool names) to logs across HTTP handlers, MCP transports, and tool executions.
- Instrument critical code paths (config load, auth gate, client factory, runtime sessions, tool operations) with leveled logs following best practices.
- Provide configuration toggles for log level and enabling/disabling the external sink while keeping sane defaults.

## Impact
- Improves observability and debuggability for operators.
- Enables future integration with logging providers with minimal code changes.
- Slight performance overhead from structured logging, mitigated by leveled filters.
- Requires updating deployment manifests to surface new env vars (in future work).

## Acceptance
- Logs emitted to stdout are structured (JSON) and include timestamps, severity, component, and correlation IDs.
- When the external sink is enabled (mock implementation for now), identical log entries are forwarded without blocking the main code path.
- Unit/Integration tests cover log enrichment for HTTP requests and tool invocations.
- `openspec validate add-structured-logging --strict` passes.

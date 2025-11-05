## MODIFIED Requirements
### Requirement: Structured stdout logging
- The server **SHALL** emit JSON-formatted logs to stdout using a consistent schema (timestamp, level, message, component).
- Logs **SHALL** include correlation identifiers when present (HTTP request ID, MCP session ID, tool name).
- On startup, the server **MUST** print a human-readable configuration summary to STDOUT before structured logging begins.
- The server **SHALL** log a startup configuration summary once per process launch detailing key environment-derived settings.

#### Scenario: Startup configuration logged
- WHEN the server process finishes initialization
- THEN a human-readable summary is printed to STDOUT with listen address, auth mode, kubeconfig source/context, namespace filter (if any), log level, external sink enabled flag, and PID file path
- AND a log entry is written containing the same information for structured logging consumers

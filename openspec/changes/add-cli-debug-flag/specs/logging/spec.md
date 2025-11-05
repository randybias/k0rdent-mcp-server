## MODIFIED Requirements
### Requirement: Structured stdout logging
- The server **SHALL** emit JSON-formatted logs to stdout using a consistent schema (timestamp, level, message, component).
- Logs **SHALL** include correlation identifiers when present (HTTP request ID, MCP session ID, tool name).
- The CLI **MUST** provide a `--debug`/`-d` flag that explicitly enables debug logging, overriding other log-level configuration.

#### Scenario: CLI debug flag forces log level
- GIVEN an operator runs `k0rdent-mcp start --debug`
- THEN the server initializes with DEBUG log level regardless of LOG_LEVEL or `--log-level`
- AND a warning is emitted when conflicting log level options are supplied.

#### Scenario: CLI help documents debug flag
- WHEN `k0rdent-mcp start --help` is executed
- THEN the help output describes the `--debug` flag (or its short alias) and states that it enables debug logging.

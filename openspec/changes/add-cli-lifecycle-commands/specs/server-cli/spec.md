## ADDED Requirements
### Requirement: Server lifecycle CLI
The MCP server MUST ship with a CLI that manages process startup and shutdown with discoverable usage.

#### Scenario: Display help
- **GIVEN** an operator runs `k0rdent-mcp --help` or `k0rdent-mcp start --help`
- **THEN** the CLI prints usage describing the `start` and `stop` commands, available flags, and exits with code 0.

#### Scenario: Start server with overrides
- **GIVEN** an operator runs `k0rdent-mcp start --env LISTEN_ADDR=127.0.0.1:18080 --log-level debug --pid-file /tmp/k0rdent.pid`
- **WHEN** the command executes successfully
- **THEN** the process applies the provided environment overrides before loading configuration, sets slog to debug level, writes its PID to `/tmp/k0rdent.pid`, and begins serving traffic.

#### Scenario: Stop server gracefully
- **GIVEN** a running server started with `k0rdent-mcp start --pid-file /tmp/k0rdent.pid`
- **AND** the PID file contains the running process ID
- **WHEN** an operator runs `k0rdent-mcp stop --pid-file /tmp/k0rdent.pid`
- **THEN** the CLI sends a termination signal to that PID, waits for the process to shut down gracefully, removes the PID file, and exits with code 0.

#### Scenario: Missing PID file fails fast
- **GIVEN** no process is running and `/tmp/k0rdent.pid` does not exist
- **WHEN** an operator runs `k0rdent-mcp stop --pid-file /tmp/k0rdent.pid`
- **THEN** the CLI prints an error explaining the missing PID file and exits with a non-zero status.

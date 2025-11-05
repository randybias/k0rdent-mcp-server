# Change: add-startup-config-log

## Why
- Operators want a single place to confirm which environment variables and runtime options are active after the CLI launches the MCP server.
- Today only minimal bootstrap messages are printed, forcing manual inspection of env vars, flags, or kubeconfig sources, which is error prone.

## What Changes
- Emit a human-readable startup summary to STDOUT before structured logging begins so operators immediately see the effective configuration.
- Emit a matching structured startup summary log entry that includes the resolved configuration: listen address, auth mode, kubeconfig source/context, namespace filter, log level, external sink status, and PID file path when applicable.
- Ensure the summaries are written once per process startup and clearly labeled for operators.

## Impact
- Improves debuggability and support handoff by surfacing the effective configuration without digging through env vars or code.
- Adds a small amount of logging overhead but limited to process startup.

## Out of Scope
- Redacting or masking specific env var values (assume non-sensitive configuration only).
- Persisting configuration snapshots to disk.
- Structured logging schema changes beyond adding new fields to the startup message.

## Acceptance
- Running `k0rdent-mcp start` prints a human-readable startup summary to STDOUT before structured logging activates, enumerating all relevant environment/config values.
- Running `k0rdent-mcp start` also logs a "startup configuration" event that enumerates all relevant environment/config values.
- The log includes listen address, auth mode, kubeconfig source + context, namespace filter pattern (if any), log level, external sink enabled flag, and PID file path.
- `go test ./...` passes and the new change validates with `openspec validate add-startup-config-log --strict`.

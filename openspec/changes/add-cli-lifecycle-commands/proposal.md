# Change: add-cli-lifecycle-commands

## Why
- Operators currently rely on default Go `main` behavior, making it hard to script lifecycle actions or discover runtime options.
- We need consistent CLI ergonomics for starting and stopping the MCP server with explicit debugging and environment overrides.
- Providing usage help and flag-driven configuration reduces mistakes when deploying locally or in CI.

## What Changes
- Introduce a dedicated CLI entrypoint with `start` and `stop` commands for the MCP server binary.
- Allow overriding runtime environment variables from the command line (e.g., kubeconfig, listen address) without exporting them globally.
- Expose log-level controls (info/debug/warn/error) and usage/help output for both commands.
- Persist process metadata (PID file) so the `stop` command can trigger graceful shutdown via POSIX signals.

## Impact
- Enables scripts and operators to manage the MCP server lifecycle consistently.
- Simplifies debugging by making log-level toggles explicit.
- Minimizes accidental misconfiguration by surfacing usage guidance directly in the binary.

## Out of Scope
- Service managers or daemonization beyond writing a PID file.
- Windows-specific process management.
- Packaging (Dockerfile/systemd) changes.

## Acceptance
- `openspec validate add-cli-lifecycle-commands --strict` passes.
- Running `k0rdent-mcp start --help` documents flags for log-level, env overrides, and PID file location.
- `start` writes the running process PID to the configured file and honors any `--env KEY=VALUE` overrides.
- `stop` reads the PID file, sends a graceful termination signal, waits for exit, and removes the PID file.
- Log level flag correctly switches the slog handler between debug and other supported levels.

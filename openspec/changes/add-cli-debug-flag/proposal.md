# Change: add-cli-debug-flag

## Why
- Operators want a quick toggle to run the server in debug mode without manually setting LOG_LEVEL.
- Providing a dedicated CLI flag reduces errors and makes the lifecycle commands self-describing.

## What Changes
- Add a `--debug` (and shorthand `-d`) option to the `start` command that forces the log level to DEBUG.
- Document flag precedence when both `--log-level` and `--debug` are provided.

## Impact
- Simplifies local troubleshooting; minimal code impact limited to CLI flag parsing.

## Acceptance
- `k0rdent-mcp start --help` lists `--debug`/`-d` and indicates it enables debug logging.
- When `--debug` is provided, the server starts with DEBUG log level regardless of LOG_LEVEL.
- Combining `--debug` with `--log-level` results in DEBUG level and outputs a warning about conflicting options.
- Tests verify flag parsing precedence.
- `openspec validate add-cli-debug-flag --strict` passes.

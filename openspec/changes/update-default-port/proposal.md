# Change: update-default-port

## Why
- The server currently defaults to listening on port 8080, but we want to use 6767 as the standard MCP HTTP port.
- Aligning the default port avoids conflicts with other services and makes documentation consistent.

## What Changes
- Update the server’s default listen address from `:8080` to `:6767`.
- Adjust all references to the default endpoint (documentation, tests, integration helpers, sample commands) to reflect the new port.
- Ensure help text and startup summaries show the new default.

## Impact
- Developers and operators will connect to `http://127.0.0.1:6767/mcp` unless they override the listen address.
- Tests and integration instructions must be updated to match the new port.

## Acceptance
- `cmd/server/main.go` uses `:6767` as the default listen address.
- All documentation and tests referencing the old `127.0.0.1:8080` endpoint are updated to `127.0.0.1:6767`.
- Running `k0rdent-mcp start` without `--listen` prints startup summaries showing port 6767.
- `openspec validate update-default-port --strict` passes.

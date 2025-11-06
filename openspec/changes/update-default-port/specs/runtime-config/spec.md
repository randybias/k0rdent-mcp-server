## MODIFIED Requirements
### Requirement: Server default configuration
- The MCP HTTP server **SHALL** listen on port 6767 by default when `LISTEN_ADDR` is not provided.
- Documentation and tooling **MUST** reference the 6767 default endpoint (`http://127.0.0.1:6767/mcp`) unless overridden.

#### Scenario: Default listen address
- GIVEN no `LISTEN_ADDR` or CLI `--listen` override is provided
- WHEN the server starts
- THEN it binds to port 6767 and the startup summary/help text reflects that port

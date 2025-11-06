## MODIFIED Requirements
### Requirement: Optional live cluster integration tests
- The project **MUST** provide an integration test package (e.g., under `test/integration`) guarded by a build tag (e.g., `live`) that exercises the MCP server against a real cluster.
- The tests **MUST** use environment variables consistent with server configuration (kubeconfig path, auth mode) and fail fast when not present or invalid.
- Documentation **MUST** explain how to invoke the live tests and reference the shared helper utilities.
- Live tests **MUST** cover namespace listing, pod log retrieval, and k0rdent CRD listing via the MCP tools, all using the shared helpers.

#### Scenario: Live tests validate core MCP tools
- GIVEN the required environment variables are set and the server is running
- WHEN the live test suite runs
- THEN it exercises namespaces, pod logs, and k0rdent CRD listing via shared helpers and fails if any call or authentication fails

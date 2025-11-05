## ADDED Requirements
### Requirement: Optional live cluster integration tests
- The project **MUST** provide an integration test package (e.g., under `test/integration`) guarded by a build tag (e.g., `live`) that exercises the MCP server against a real cluster.
- The tests **MUST** use environment variables consistent with server configuration (kubeconfig path, auth mode) and skip automatically when not present.
- Documentation **MUST** explain how to invoke the live tests.

#### Scenario: Live tests skipped when not configured
- GIVEN the live integration tests are run without required environment variables
- THEN the tests are reported as skipped with a helpful message

#### Scenario: Live tests run against cluster
- GIVEN the necessary environment variables and kubeconfig are provided
- WHEN the live tests run with the `live` build tag
- THEN they connect to the cluster and validate namespace listing via the MCP toolchain without failure

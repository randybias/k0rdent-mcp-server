# Live Integration Tests

The MCP server provides an optional integration test suite that exercises the
running server against a live Kubernetes management cluster. All tests are
guarded by the `live` build tag so default unit test runs stay fast.

## Prerequisites
- The server must be running locally (default endpoint `http://127.0.0.1:8080/mcp`).
- A kubeconfig with appropriate credentials.

## Environment Variables
- `K0RDENT_MGMT_KUBECONFIG_PATH` – path to the kubeconfig used for integration tests (must be readable).
- `AUTH_MODE` – authentication mode used by the running server (e.g., `DEV_ALLOW_ANY`).
- `K0RDENT_MCP_ENDPOINT` – optional, override the MCP endpoint if the server is not on the default address.

## Running the tests
```
go test -tags=live ./test/integration
```

Each live test relies on the helpers in `test/integration` (`requireLiveEnv`,
`newLiveClient`, `CallTool`, `extractSSEPayload`). Tests fail fast when required
configuration is missing or a tool call fails, making issues obvious during CI
or local validation. Current coverage includes namespaces, pod logs, and
the k0rdent CRDs. Future live tests should reuse the same helpers.

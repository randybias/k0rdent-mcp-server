# Live Integration Tests

The MCP server provides an optional integration test suite that exercises the
running server against a live Kubernetes management cluster. All tests are
guarded by the `live` build tag so default unit test runs stay fast.

## Prerequisites
- The server must be running locally (default endpoint `http://127.0.0.1:6767/mcp`).
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
or local validation. Current coverage includes namespaces, pod logs, k0rdent
CRDs, and catalog operations. Future live tests should reuse the same helpers.

## Test Coverage

### Catalog Tests (`test/integration/catalog_live_test.go`)

The catalog test suite validates the catalog discovery and installation functionality:

**TestCatalogListLive**
- Lists all catalog entries without filters
- Verifies entry structure (slug, title, versions)
- Checks for well-known applications (e.g., minio)
- Validates version metadata completeness

**TestCatalogListWithFilterLive**
- Tests filtering by application slug
- Verifies filtered results contain only requested app
- Validates filter accuracy

**TestCatalogListRefreshLive**
- Tests cache behavior with and without refresh parameter
- Ensures refresh=true forces cache update
- Validates both cached and fresh data returns

**TestCatalogInstallNginxIngressLive**
- Complete end-to-end installation workflow
- Discovers nginx-ingress in catalog
- Installs ServiceTemplate to management cluster
- Verifies ServiceTemplate creation
- Checks for HelmRepository creation (if applicable)
- Optionally validates deployment to child cluster (if available)
- Includes cleanup of test resources

**TestCatalogInstallIdempotencyLive**
- Validates server-side apply idempotency
- Installs same template twice
- Ensures both operations succeed without conflicts
- Uses lightweight app (minio) for faster execution

### Prerequisites for Catalog Tests

In addition to the standard live test prerequisites, catalog tests require:
- Network access to `https://github.com/k0rdent/catalog`
- Writable cache directory (default: `/var/lib/k0rdent-mcp/catalog`)
- RBAC permissions to create ServiceTemplate and HelmRepository resources
- For full nginx test: Access to a child cluster (optional, test degrades gracefully)

### Running Only Catalog Tests

```bash
go test -tags=live -v ./test/integration -run Catalog
```

# Tasks: Use kgst for catalog installs

## Phase 1: Setup and Dependencies

### 1.1 Add Helm Go SDK dependency
- [ ] Add `helm.sh/helm/v3` to `go.mod` with version `v3.14.0` or later
- [ ] Run `go mod tidy` to fetch dependencies
- [ ] Verify successful compilation with new imports
- [ ] Update `go.sum` in version control

**Validation:** `go build ./...` succeeds with Helm SDK imports

### 1.2 Create Helm client package
- [ ] Create `internal/helm/` directory
- [ ] Create `internal/helm/client.go` with `Client` struct
- [ ] Implement `NewClient(restConfig *rest.Config, namespace string, logger *slog.Logger) (*Client, error)` constructor
- [ ] Implement `action.Configuration` initialization in constructor
- [ ] Wire Helm debug output to structured logger
- [ ] Add unit tests for client construction with mock REST config

**Validation:** Unit tests for `helm.NewClient()` pass

### 1.3 Implement OCI chart loader
- [ ] Add `LoadChart(ctx context.Context, chartURL string, version string) (*chart.Chart, error)` method to `Client`
- [ ] Implement OCI registry client configuration
- [ ] Implement chart pulling from `oci://ghcr.io/k0rdent/catalog/charts/kgst`
- [ ] Implement chart identity verification (name, version)
- [ ] Add error handling for network failures
- [ ] Add unit tests with mock registry responses

**Validation:** Unit tests for chart loading pass; mock errors are handled correctly

## Phase 2: Core Installation Logic

### 2.1 Implement kgst values construction
- [ ] Create `BuildKGSTValues(template, version, namespace string) map[string]interface{}` function
- [ ] Set `chart: "<template>:<version>"` format
- [ ] Set `repo.name: "k0rdent-catalog"`
- [ ] Set `repo.spec.url: "oci://ghcr.io/k0rdent/catalog/charts"`
- [ ] Set `repo.spec.type: "oci"`
- [ ] Set `namespace: <namespace>`
- [ ] Set `k0rdentApiVersion: "v1beta1"`
- [ ] Set `skipVerifyJob: false`
- [ ] Add unit tests for values construction with various inputs

**Validation:** Unit tests verify correct values structure for minio, valkey, prometheus examples

### 2.2 Implement Helm install/upgrade
- [ ] Add `InstallOrUpgrade(ctx context.Context, releaseName string, chart *chart.Chart, values map[string]interface{}) (*release.Release, error)` method
- [ ] Configure `action.Upgrade` with `Install: true`
- [ ] Set `Namespace` to target namespace
- [ ] Set `Wait: true` for hook completion
- [ ] Set `Timeout: 5*time.Minute`
- [ ] Set `Atomic: true` for rollback on failure
- [ ] Invoke `RunWithContext()` with provided context
- [ ] Add error handling and logging for each operation
- [ ] Add unit tests with mock Helm actions

**Validation:** Unit tests verify correct action configuration; mock release succeeds

### 2.3 Extract release information
- [ ] Implement `ExtractAppliedResources(release *release.Release) []string` helper function
- [ ] Parse release manifest to identify created resources (ServiceTemplate, HelmRepository)
- [ ] Format resource names as `<namespace>/<kind>/<name>`
- [ ] Handle cases where release is nil or manifest is empty
- [ ] Add unit tests with sample release manifests

**Validation:** Unit tests correctly extract ServiceTemplate and HelmRepository from mock releases

## Phase 3: Integration with Catalog Tool

### 3.1 Refactor catalog install tool
- [ ] Modify `catalogInstallTool.install()` in `internal/tools/core/catalog.go`
- [ ] Replace manifest fetching (`t.manager.GetManifests()`) with Helm client creation
- [ ] Replace direct apply loop with calls to Helm client methods
- [ ] Preserve existing namespace resolution logic (call existing `resolveTargetNamespaces()`)
- [ ] Iterate over resolved namespaces, creating Helm release in each
- [ ] Update return structure to include Helm release info
- [ ] Preserve existing error handling and logging patterns
- [ ] Add detailed logging for Helm operations (chart pull, install, verify job)

**Validation:** Code compiles; existing tool signature unchanged

### 3.2 Update error handling
- [ ] Add error parsing for Helm verification job failures
- [ ] Extract "chart not found" errors from Helm output
- [ ] Extract validation webhook errors from Helm rollback messages
- [ ] Convert Helm errors to appropriate MCP error types (mcp.NewError)
- [ ] Add context to error messages (release name, namespace, chart details)
- [ ] Add unit tests for error parsing with sample Helm error outputs

**Validation:** Unit tests correctly parse and categorize Helm errors

### 3.3 Preserve idempotency
- [ ] Verify `helm upgrade --install` behavior preserves idempotency
- [ ] Test repeated installations with same values (should be no-op)
- [ ] Test repeated installations with different values (should upgrade)
- [ ] Ensure return status reflects operation ("created" vs "updated")
- [ ] Add integration test for repeated installations

**Validation:** Integration test shows repeated install returns success without creating duplicate resources

## Phase 4: Testing

### 4.1 Add unit tests for Helm integration
- [ ] Test `helm.NewClient()` with valid and invalid REST configs
- [ ] Test `LoadChart()` with successful pull and network failure
- [ ] Test `BuildKGSTValues()` with various parameter combinations
- [ ] Test `InstallOrUpgrade()` with mock Helm actions
- [ ] Test error parsing for verification job failures
- [ ] Achieve >80% code coverage for `internal/helm/` package

**Validation:** `go test ./internal/helm/... -cover` shows >80% coverage

### 4.2 Update integration tests
- [ ] Modify existing `test/integration/catalog_test.go`
- [ ] Update test expectations to account for Helm releases
- [ ] Add test case for minio installation (should still work)
- [ ] Add test case for keda installation (should still work)
- [ ] Add test case for valkey installation (should now succeed)
- [ ] Add test case for prometheus installation (should now succeed)
- [ ] Add test case for non-existent chart (should fail with clear error)
- [ ] Add test case for verification job timeout (if feasible to simulate)
- [ ] Verify ServiceTemplate and HelmRepository are created correctly

**Validation:** All integration tests pass; valkey and prometheus now succeed

### 4.3 Add regression tests
- [ ] Test backward compatibility of MCP tool signature
- [ ] Test namespace resolution logic unchanged (DEV_ALLOW_ANY, OIDC_REQUIRED)
- [ ] Test namespace filter enforcement still works
- [ ] Test all_namespaces flag still creates multiple releases
- [ ] Test return structure matches expected format
- [ ] Add test ensuring existing MCP clients can parse responses

**Validation:** Regression test suite passes; no breaking changes detected

## Phase 5: Observability and Ops

### 5.1 Add structured logging
- [ ] Log Helm client creation (debug level)
- [ ] Log chart loading start and completion (debug level)
- [ ] Log kgst values being passed (debug level, sanitize if needed)
- [ ] Log Helm install/upgrade start (info level)
- [ ] Log verification job status (info level for success, error level for failure)
- [ ] Log ServiceTemplate creation (info level)
- [ ] Log release final status (info level)
- [ ] Include context fields: tool, release_name, namespace, chart, version
- [ ] Verify logs are structured (JSON format in production)

**Validation:** Manual testing shows clear, structured logs for all operations

### 5.2 Update RBAC documentation
- [ ] Document new RBAC requirements in `docs/` or README
- [ ] List required permissions: Secrets (create/update/delete), Jobs (create/delete), Pods (create/delete), Pod logs (get)
- [ ] Provide example ClusterRole or Role YAML
- [ ] Explain why each permission is needed (Helm release storage, verify job)
- [ ] Add troubleshooting section for RBAC errors

**Validation:** Documentation review confirms completeness

### 5.3 Add configuration for kgst version
- [ ] Add environment variable or config field for kgst chart version (default: "2.0.0")
- [ ] Update chart loading to use configured version
- [ ] Add logging to show which kgst version is being used
- [ ] Document how to override kgst version if needed

**Validation:** Can override kgst version via environment variable; correct version is used

## Phase 6: Cleanup and Documentation

### 6.1 Remove unused code
- [ ] Remove or deprecate `GetManifests()` method from `internal/catalog/manager.go` if no longer used
- [ ] Remove direct manifest application code from `catalog.go`
- [ ] Remove v1alpha1 to v1beta1 conversion logic (handled by kgst now)
- [ ] Remove manual HelmRepository creation logic
- [ ] Clean up unused imports
- [ ] Run `go mod tidy` to remove unnecessary dependencies

**Validation:** Code compiles; unused code is removed; no dead code remains

### 6.2 Update tool documentation
- [ ] Update MCP tool description for `install_from_catalog` to mention kgst
- [ ] Add note about pre-install verification in tool description
- [ ] Document typical installation time (10-30 seconds including verification)
- [ ] Update examples to reflect new behavior (valkey, prometheus now work)
- [ ] Add troubleshooting section for common Helm errors

**Validation:** Documentation review confirms accuracy and completeness

### 6.3 Update README and deployment guide
- [ ] Update README to mention Helm v3 SDK dependency
- [ ] Add network requirements section: access to `ghcr.io` required
- [ ] Update RBAC requirements section with new permissions
- [ ] Add notes about Helm release storage (Secrets in namespace)
- [ ] Update troubleshooting section with Helm-specific issues
- [ ] Add FAQ entry explaining why kgst is used

**Validation:** README review confirms all changes documented

## Phase 7: Deployment and Validation

### 7.1 Build and test locally
- [ ] Build MCP server binary with Helm integration
- [ ] Run against local k0rdent cluster
- [ ] Manually test minio installation
- [ ] Manually test valkey installation (confirm it now works)
- [ ] Manually test prometheus installation (confirm it now works)
- [ ] Verify Helm releases are created correctly (`helm list -A`)
- [ ] Verify ServiceTemplates are created correctly (`kubectl get servicetemplates -A`)
- [ ] Check for errors in server logs

**Validation:** All manual tests pass; valkey and prometheus install successfully

### 7.2 Run full test suite
- [ ] Run all unit tests: `go test ./...`
- [ ] Run all integration tests: `go test ./test/integration/...`
- [ ] Run linter: `golangci-lint run`
- [ ] Check for race conditions: `go test -race ./...`
- [ ] Verify code coverage meets threshold
- [ ] Fix any failing tests or issues

**Validation:** All tests pass; no regressions detected

### 7.3 Update container image
- [ ] Ensure Dockerfile includes necessary Helm SDK dependencies (no binary needed, SDK is compiled in)
- [ ] Build container image with new code
- [ ] Test image in Kubernetes (deploy to test namespace)
- [ ] Verify image can pull kgst chart from ghcr.io
- [ ] Verify image has correct RBAC permissions
- [ ] Check image size (should increase slightly due to Helm SDK)

**Validation:** Container image runs successfully in Kubernetes; catalog installs work

### 7.4 OpenSpec validation
- [ ] Run `openspec validate use-kgst-for-catalog-installs --strict`
- [ ] Fix any validation errors
- [ ] Ensure all scenarios are testable and tested
- [ ] Verify all requirements have at least one scenario
- [ ] Check for spec consistency (no conflicting requirements)

**Validation:** `openspec validate` passes with no errors

## Ongoing Maintenance

### Post-deployment monitoring
- [ ] Monitor installation latency (target: <30 seconds for typical templates)
- [ ] Monitor success/failure rates for catalog installations
- [ ] Watch for OCI registry availability issues
- [ ] Track verify job failure patterns
- [ ] Collect feedback on error message clarity

### Future enhancements (out of scope for initial implementation)
- [ ] Add `helm uninstall` support to delete tool
- [ ] Expose `skipVerifyJob` parameter for advanced users
- [ ] Cache kgst chart locally to reduce registry pulls
- [ ] Support custom kgst versions per installation
- [ ] Add metrics for Helm operation duration
- [ ] Implement retry logic for transient registry failures

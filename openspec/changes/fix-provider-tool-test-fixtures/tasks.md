# Tasks: Fix Provider Tool Test Fixtures

## Phase 1: Investigate Test Fixture Requirements

### Task 1: Analyze Credential CRD Schema
- [ ] Read Credential CRD definition from k0rdent codebase
- [ ] Document required fields for minimal test fixture
- [ ] Identify relationship with ClusterIdentity resources
- [ ] Document provider-specific credential structure (AWS, Azure, GCP)

**Dependencies**: None

### Task 2: Audit All Provider Test Fixtures
- [ ] Review AWS provider test fixtures in `clusters_aws_test.go`
- [ ] Review Azure provider test fixtures in `clusters_azure_test.go`
- [ ] Review GCP provider test fixtures in `clusters_gcp_test.go`
- [ ] Document missing CRDs in each provider test
- [ ] Identify common patterns for test fixture creation

**Dependencies**: Task 1

## Phase 2: Implement Test Fixture Improvements

### Task 3: Register k0rdent CRD Schemes in Test Setup
- [ ] Import k0rdent CRD scheme packages
- [ ] Add k0rdent CRDs to test scheme registration
- [ ] Create helper function for scheme setup if needed
- [ ] Verify scheme includes Credential, ClusterIdentity, ClusterTemplate CRDs

**Dependencies**: Task 2

### Task 4: Create Credential Test Fixtures
- [ ] Create `makeAzureCredential()` helper function
- [ ] Create `makeAWSCredential()` helper function
- [ ] Create `makeGCPCredential()` helper function
- [ ] Add fixtures to test dynamic clients
- [ ] Ensure credentials match those referenced in deployment inputs

**Dependencies**: Task 3

### Task 5: Create ClusterIdentity Test Fixtures (if needed)
- [ ] Determine if ClusterIdentity resources are required
- [ ] Create ClusterIdentity fixtures if needed
- [ ] Link credentials to cluster identities correctly

**Dependencies**: Task 4

## Phase 3: Fix Azure Provider Tests

### Task 6: Fix TestAzureClusterDeployTool_ValidDeploy
- [ ] Add Credential fixture to test setup
- [ ] Remove `t.Skip()` call
- [ ] Run test and verify it passes
- [ ] Verify ClusterDeployment is created with correct credential reference

**Dependencies**: Task 4

### Task 7: Fix TestAzureClusterDeployTool_DefaultValues
- [ ] Add Credential fixture to test setup
- [ ] Remove `t.Skip()` call
- [ ] Run test and verify default values are applied correctly
- [ ] Verify credential reference is correct

**Dependencies**: Task 4

### Task 8: Fix TestAzureClusterDeployTool_TemplateSelection
- [ ] Add Credential fixture to "select latest from multiple versions" subtest
- [ ] Remove conditional `t.Skip()` call
- [ ] Run test and verify template selection works correctly
- [ ] Verify credential lookup succeeds

**Dependencies**: Task 4

## Phase 4: Verify Other Provider Tests

### Task 9: Review and Fix AWS Provider Tests (if needed)
- [ ] Run AWS provider tests
- [ ] Add Credential fixtures if any tests fail
- [ ] Verify all AWS tests pass

**Dependencies**: Task 4

### Task 10: Review and Fix GCP Provider Tests (if needed)
- [ ] Run GCP provider tests
- [ ] Add Credential fixtures if any tests fail
- [ ] Verify all GCP tests pass

**Dependencies**: Task 4

## Phase 5: Cleanup and Documentation

### Task 11: Refactor Common Test Helpers
- [ ] Extract common fixture creation patterns
- [ ] Create shared test helper file if beneficial
- [ ] Document test fixture conventions
- [ ] Ensure consistency across all provider tests

**Dependencies**: Tasks 6, 7, 8, 9, 10

### Task 12: Update Test Documentation
- [ ] Document test fixture requirements in test file comments
- [ ] Add examples of proper test setup
- [ ] Document relationship between Credential and deployment tests

**Dependencies**: Task 11

### Task 13: Verify Full Test Suite
- [ ] Run `go test ./...` and verify all tests pass
- [ ] Verify no tests are skipped
- [ ] Check test coverage for provider tools
- [ ] Run tests with race detector: `go test -race ./internal/tools/core`

**Dependencies**: Tasks 9, 10, 11, 12

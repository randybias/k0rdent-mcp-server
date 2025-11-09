# Proposal: Fix Provider Tool Test Fixtures

## Problem

Three Azure provider tool tests are currently skipped because the fake Kubernetes client test fixtures don't include Credential resources:

- `TestAzureClusterDeployTool_ValidDeploy`
- `TestAzureClusterDeployTool_DefaultValues`
- `TestAzureClusterDeployTool_TemplateSelection/select_latest_from_multiple_versions`

These tests fail with: `credential azure-cred not found in namespace kcm-system: credentials.k0rdent.mirantis.com "azure-cred" not found`

The tests attempt to deploy clusters using Credential resources but the fake dynamic client doesn't have these resources registered.

## Goals

1. Fix test fixtures to include Credential resources so provider deployment tests pass
2. Re-enable the 3 currently skipped Azure provider tests
3. Verify AWS and GCP provider tests don't have similar issues
4. Ensure test fixtures reflect real Kubernetes cluster state with all required CRDs

## Non-Goals

- Changing the provider deployment tool implementation
- Adding live/integration tests (already exist)
- Modifying Credential CRD structure

## Constraints

- Must maintain compatibility with existing unit test structure
- Should use fake clients consistently across all provider tests
- Avoid requiring real Kubernetes clusters for unit tests

## Success Criteria

- All 3 skipped Azure provider tests pass without `t.Skip()`
- Test fixtures include Credential resources
- Test fixtures properly register k0rdent CRD schemes
- AWS and GCP provider tests continue to pass
- All unit tests in `internal/tools/core` pass

## Dependencies

- Depends on: None
- Blocks: None
- Related: `add-provider-specific-deploy-tools` (parent implementation)

## Open Questions

1. Should test fixtures also include ClusterIdentity resources?
2. Do we need realistic Credential data or minimal fixtures?
3. Should we create helper functions for common test fixtures across providers?

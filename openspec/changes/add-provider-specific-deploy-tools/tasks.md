# Implementation Tasks: Provider-Specific Cluster Deployment Tools

## Phase 1: Core Implementation

### 1. Add Template Selection Logic

- [x] Add `SelectLatestTemplate` method to `internal/clusters/Manager`
  - Accept provider name and namespace parameters
  - List all templates in the namespace
  - Filter by provider prefix pattern (e.g., `aws-standalone-cp-`)
  - Sort filtered templates by semantic version
  - Return latest template name
  - Handle case where no matching templates exist

- [x] Implement version comparison helper
  - Create `compareVersions(v1, v2 string) int` function
  - Parse semantic versions (e.g., "1.0.14")
  - Compare major, minor, patch numbers
  - Return 1 if v1 > v2, -1 if v1 < v2, 0 if equal

- [x] Add unit tests for template selection
  - Test selecting latest from multiple versions
  - Test filtering by provider prefix
  - Test handling no matching templates
  - Test version sorting (1.0.14 vs 1.0.15, 1.1.0 vs 1.0.99, etc.)

### 2. Implement AWS Deployment Tool

- [x] Create `internal/tools/core/clusters_aws.go`
  - Define `awsClusterDeployTool` struct with session
  - Define `awsClusterDeployInput` struct with jsonschema tags
  - Define `awsNodeConfig` struct for control plane and worker config
  - Define `awsClusterDeployResult` type alias
  - Add proper struct tags with descriptions and defaults

- [x] Implement AWS deploy handler
  - Extract and validate input parameters
  - Default namespace to "kcm-system" if not specified
  - Call `SelectLatestTemplate(ctx, "aws", namespace)`
  - Build config map from structured input
  - Apply default values (controlPlaneNumber=3, workersNumber=2, rootVolumeSize=32)
  - Create `clusters.DeployRequest` with auto-selected template
  - Call existing `session.Clusters.Deploy()` logic
  - Return deployment result

- [x] Register AWS tool in `registerClusters`
  - Add AWS tool registration with proper name: `k0rdent.provider.aws.clusterDeployments.deploy`
  - Set description explaining auto-selection and validation
  - Add metadata: plane="aws", category="clusterDeployments", action="deploy", provider="aws"
  - Bind to AWS deploy handler

### 3. Implement Azure Deployment Tool

- [x] Create `internal/tools/core/clusters_azure.go`
  - Define `azureClusterDeployTool` struct
  - Define `azureClusterDeployInput` with Azure-specific fields (location, subscriptionID)
  - Define `azureNodeConfig` with vmSize field (not instanceType)
  - Add jsonschema tags with descriptions

- [x] Implement Azure deploy handler
  - Follow same pattern as AWS handler
  - Use "azure" for template selection
  - Build config map with location and subscriptionID
  - Apply default values (controlPlaneNumber=3, workersNumber=2, rootVolumeSize=30)
  - Call existing deploy logic

- [x] Register Azure tool in `registerClusters`
  - Add registration with name: `k0rdent.provider.azure.clusterDeployments.deploy`
  - Set appropriate metadata with provider="azure"

### 4. Implement GCP Deployment Tool

- [x] Create `internal/tools/core/clusters_gcp.go`
  - Define `gcpClusterDeployTool` struct
  - Define `gcpClusterDeployInput` with GCP-specific fields (project, region, network)
  - Define `gcpNodeConfig` with instanceType field
  - Define `gcpNetworkConfig` with name field (nested structure)
  - Add jsonschema tags with descriptions

- [x] Implement GCP deploy handler
  - Follow same pattern as AWS/Azure handlers
  - Use "gcp" for template selection
  - Build config map including nested network.name structure
  - Apply default values
  - Call existing deploy logic

- [x] Register GCP tool in `registerClusters`
  - Add registration with name: `k0rdent.provider.gcp.clusterDeployments.deploy`
  - Set appropriate metadata with provider="gcp"

### 5. Remove Generic Deploy Tool

- [x] Remove `k0rdent.mgmt.clusterDeployments.deploy` tool registration
  - Kept generic tool per user feedback (early development, can remove later if not needed)
  - Both generic and provider-specific tools now coexist

- [x] Refactor validation logic
  - Created common constants for default values
  - Created `validateAndDefaultNodeCounts()` function
  - Eliminated code duplication across all provider tools
  - Three-state validation: 0=default, negative=error, positive=use

- [x] Live testing validation
  - Deployed AWS cluster to ap-southeast-1 (Singapore)
  - Deployed Azure cluster to westus2
  - Verified refactored validation works correctly
  - Verified defaults applied correctly (3 CP, 2 workers)
  - Verified template auto-selection
  - Successfully deleted test clusters

- [ ] Update documentation
  - Document migration path from generic to provider-specific tools
  - Explain breaking change and rationale (early development, cleaner API)
  - Document common validation function
  - Document default values and constants

### 6. Add MCP Prompt Templates

- [ ] Create prompt infrastructure (if not exists)
  - Add prompts registration to MCP server
  - Create prompts directory structure
  - Implement prompt handler/provider

- [ ] Create AWS deployment example prompt
  - Prompt name: `k0rdent/examples/deploy-aws-cluster`
  - Include parameterized example (clusterName, region)
  - Add field explanations with AWS-specific examples
  - Include common variations (production, dev, different regions)
  - Add link to k0rdent AWS documentation

- [ ] Create Azure deployment example prompt
  - Prompt name: `k0rdent/examples/deploy-azure-cluster`
  - Include parameterized example (clusterName, location, subscriptionID)
  - Explain Azure-specific field names (location vs region, vmSize vs instanceType)
  - Include common variations
  - Add link to k0rdent Azure documentation

- [ ] Create GCP deployment example prompt
  - Prompt name: `k0rdent/examples/deploy-gcp-cluster`
  - Include parameterized example (clusterName, project, region, networkName)
  - Explain nested network configuration
  - Include common variations
  - Add link to k0rdent GCP documentation

- [ ] Register prompts with MCP server
  - Add prompts to server initialization
  - Test prompts/list returns all examples
  - Test prompts/get retrieves templates with parameter substitution

## Phase 2: Testing

### 6. Unit Tests

- [ ] Create `internal/clusters/templates_selection_test.go`
  - Test `SelectLatestTemplate` with multiple versions
  - Test filtering by provider prefix
  - Test no matching templates error
  - Test version comparison logic
  - Test with mixed provider templates

- [ ] Create `internal/tools/core/clusters_aws_test.go`
  - Test deploy with valid input
  - Test default value application
  - Test template auto-selection
  - Test config map building
  - Test error handling (template not found)
  - Test integration with existing validation

- [ ] Create `internal/tools/core/clusters_azure_test.go`
  - Test deploy with valid Azure input
  - Test location and subscriptionID are required
  - Test vmSize field (not instanceType)
  - Test default values
  - Test template selection

- [ ] Create `internal/tools/core/clusters_gcp_test.go`
  - Test deploy with valid GCP input
  - Test nested network.name field
  - Test project and region are required
  - Test default values
  - Test template selection

### 7. Integration Tests

- [ ] Create integration tests for provider tools
  - Test AWS tool against live cluster
  - Test Azure tool against live cluster
  - Test GCP tool with mock (no live environment)
  - Verify template auto-selection works
  - Verify validation still catches errors

- [ ] Test MCP schema exposure
  - Verify tools appear in MCP tools list
  - Verify tool schemas are properly generated from structs
  - Verify required fields are marked correctly
  - Verify descriptions are present
  - Verify default values are included

### 8. AI Agent Testing

- [ ] Manual testing with AI agent (Claude)
  - List MCP tools and verify provider tools are visible
  - Inspect AWS tool schema and verify fields are discoverable
  - Have agent deploy AWS cluster without hard-coded knowledge
  - Have agent deploy Azure cluster and discover different field names
  - Verify agent can extract required fields from tool schema
  - Confirm single-step workflow (no schema fetching needed)

## Phase 3: Documentation

### 9. API Documentation

- [ ] Create `docs/provider-specific-deployment.md`
  - Explain provider-specific tool approach
  - Show example for each provider (AWS, Azure, GCP)
  - Document template auto-selection behavior
  - Explain when to use provider tools vs generic tool
  - Show AI agent discovery workflow

- [ ] Update `docs/cluster-provisioning.md`
  - Add section on provider-specific tools
  - Update deployment examples to use provider tools
  - Keep generic tool examples for advanced scenarios
  - Add decision guide (which tool to use when)

- [ ] Add usage examples
  - AWS cluster deployment example
  - Azure cluster deployment with different field names
  - GCP cluster deployment with nested network config
  - Show MCP tool schema introspection
  - Show AI agent learning workflow

### 10. Tool Documentation

- [ ] Enhance tool descriptions
  - Clarify auto-selection behavior in each provider tool
  - Document default values
  - Add examples in descriptions
  - Link to full documentation

- [ ] Update README or main docs
  - List new provider-specific tools
  - Explain advantages for AI agents
  - Show before/after comparison

## Phase 4: Validation

### 11. Manual Testing

- [ ] Test AWS tool with live cluster
  - Deploy cluster using AWS tool
  - Verify template is auto-selected correctly
  - Verify validation catches missing region
  - Verify defaults are applied
  - Verify cluster provisions successfully

- [ ] Test Azure tool with live cluster
  - Deploy cluster using Azure tool
  - Verify location and subscriptionID validation
  - Verify vmSize field works correctly
  - Verify defaults are applied
  - Verify cluster provisions successfully

- [ ] Test GCP tool (mock or live if available)
  - Test with valid GCP input
  - Verify nested network.name validation
  - Verify project and region validation

- [ ] Test generic tool still works
  - Verify backward compatibility
  - Test with custom template
  - Verify full flexibility maintained

### 12. Performance Validation

- [ ] Measure template selection overhead
  - Target: < 50ms for template query and selection
  - Verify no impact on overall deployment time
  - Confirm no caching needed

- [ ] Verify resource usage
  - Check memory footprint of new tools
  - Confirm no goroutine leaks
  - Test concurrent deployments

### 13. Code Review Preparation

- [ ] Review all changes for consistency
  - Check code style matches project conventions
  - Verify struct tags follow patterns
  - Ensure error messages are clear
  - Validate logging follows project standards

- [ ] Run full test suite
  - All new unit tests pass
  - All integration tests pass
  - No regressions in existing tests
  - Test coverage > 80% for new code

- [ ] Verify OpenSpec compliance
  - Run `openspec validate add-provider-specific-deploy-tools --strict`
  - Ensure all requirements have scenarios
  - Fix any validation errors

## Future Enhancements (Not in This Change)

These items are noted for future work but not required for this change:

- Add vSphere-specific deployment tool
- Add OpenStack-specific deployment tool
- Add EKS/AKS/GKE managed service tools
- Support template version pinning (optional templateVersion field)
- Add more optional fields to provider tools based on user feedback
- Generate tool schemas automatically from Helm chart schemas
- Add provider-specific credential validation
- Support multiple template patterns per provider (standalone vs hosted)

## Dependencies

- Existing cluster deployment logic (`internal/clusters/deploy.go`)
- Existing validation rules (from `fix-azure-config-validation`)
- ClusterTemplate listing functionality
- MCP Go SDK struct-to-schema conversion
- Live k0rdent cluster for testing

## Validation Checklist

Before marking this change complete:

- [ ] All three provider tools implemented (AWS, Azure, GCP)
- [ ] Template auto-selection works for all providers
- [ ] Tool schemas properly expose all required fields
- [ ] Unit tests pass with >80% coverage
- [ ] Integration tests pass with live cluster (AWS, Azure)
- [ ] AI agent can successfully deploy without hard-coded knowledge
- [ ] Generic tool remains functional for backward compatibility
- [ ] Documentation is complete and accurate
- [ ] No regressions in existing functionality
- [ ] OpenSpec validation passes

## Notes

- Provider tools optimize for AI agent discoverability, not code reuse
- Template auto-selection uses latest stable version matching provider pattern
- Generic tool remains for custom templates and advanced scenarios
- MCP Go SDK automatically generates JSON Schema from Go struct tags
- Existing validation logic is reused; no changes to validation rules needed
- No breaking changes - all new tools are additive

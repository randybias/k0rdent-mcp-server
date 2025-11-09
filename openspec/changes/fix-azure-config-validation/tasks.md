# Implementation Tasks: Multi-Provider Cluster Deployment Configuration Validation

## Phase 1: Core Validation Implementation

### 1. Add Validation Types and Functions

- [x] Create `internal/clusters/validation.go` with core validation types
  - Define `ValidationError` type with field, message, and code
  - Define `ValidationResult` type with errors, warnings, and provider
  - Define `ProviderType` enum (AWS, Azure, GCP, Unknown)
  - Add helper functions for building validation errors

- [x] Implement `DetectProvider` function
  - Match template name patterns (`aws-*`, `azure-*`, `gcp-*`)
  - Return appropriate ProviderType
  - Handle unknown providers gracefully

- [x] Create `internal/clusters/validation_aws.go`
  - Implement `ValidateAWSConfig` function
  - Check for required `region` field (string, non-empty)
  - Return structured validation result with provider="aws"

- [x] Create `internal/clusters/validation_azure.go`
  - Implement `ValidateAzureConfig` function
  - Check for required `location` field (string, non-empty)
  - Check for required `subscriptionID` field (string, non-empty)
  - Return structured validation result with provider="azure"

- [x] Create `internal/clusters/validation_gcp.go`
  - Implement `ValidateGCPConfig` function
  - Check for required `project` field (string, non-empty)
  - Check for required `region` field (string, non-empty)
  - Check for required `network.name` field (string, non-empty)
  - Return structured validation result with provider="gcp"

- [x] Add validation dispatch function
  - Create `ValidateConfig` function that dispatches to provider-specific validators
  - Handle unknown providers (no validation)

### 2. Integrate Validation into Deploy Flow

- [x] Update `internal/clusters/deploy.go`
  - Import validation package
  - Add provider detection call after template resolution
  - Call provider-specific validation before building ClusterDeployment
  - Return validation errors as `ErrInvalidRequest` with formatted details
  - Add structured logging for validation failures (include provider)

- [x] Add provider-specific error formatting
  - Format errors with provider context (AWS/Azure/GCP cluster)
  - Include example configuration for the detected provider
  - Add documentation URL for the provider
  - Format multiple errors clearly

### 3. Update Error Handling

- [x] Enhance error messages in `internal/clusters/errors.go` (if exists)
  - Add validation error formatting
  - Include example of correct configuration
  - Reference documentation URL

- [x] Update MCP tool response format
  - Ensure validation errors include helpful context
  - Format error messages for human readability

## Phase 2: Testing

### 4. Unit Tests

- [x] Create `internal/clusters/validation_test.go`
  - Test `DetectProvider` function with all patterns
  - Test helper functions (hasNonEmptyString, getNestedField)

- [x] Create `internal/clusters/validation_aws_test.go`
  - Test valid AWS config passes validation
  - Test missing region returns error
  - Test empty strings are treated as missing

- [x] Create `internal/clusters/validation_azure_test.go`
  - Test valid Azure config passes validation
  - Test missing subscriptionID returns error
  - Test missing location returns error
  - Test empty strings are treated as missing

- [x] Create `internal/clusters/validation_gcp_test.go`
  - Test valid GCP config passes validation
  - Test missing project returns error
  - Test missing region returns error
  - Test missing network.name returns error
  - Test nested field validation

- [x] Create `internal/clusters/deploy_validation_test.go`
  - Test deploy rejects invalid AWS config
  - Test deploy rejects invalid Azure config
  - Test deploy rejects invalid GCP config
  - Test deploy accepts valid configs for all providers
  - Test deploy passes through non-AWS/Azure/GCP configs
  - Test error message format and content for each provider

### 5. Integration Tests

- [x] Update `test/integration/clusters_live_test.go`
  - Add AWS validation test cases (missing region)
  - Add Azure validation test cases (missing subscriptionID, missing location)
  - Verify error messages are helpful for both providers
  - Ensure valid configs still work for both providers
  - *Note: GCP validation covered by unit tests only (no live environment)*

- [x] Add multi-provider negative test suite
  - Test AWS deploy without region
  - Test Azure deploy without subscriptionID
  - Verify immediate errors (not delayed cluster failures)
  - Check error message content for each provider
  - *GCP validation logic tested in unit tests*

## Phase 3: Documentation

### 6. Update Documentation

- [x] Update `docs/cluster-provisioning.md`
  - Add "Configuration Validation" section covering all providers
  - Document required fields for AWS (region)
  - Document required fields for Azure (location, subscriptionID)
  - Document required fields for GCP (project, region, network.name)
  - Show validation error examples for each provider
  - Add troubleshooting entries for validation failures per provider
  - Update baseline configuration sections for all providers

- [x] Update tool description in MCP manifest (if applicable)
  - Mention multi-provider pre-flight validation
  - Link to validation documentation

- [x] Add inline code comments
  - Document provider detection logic
  - Explain validation dispatch mechanism
  - Document provider-specific validation rules
  - Note future extensibility points (vSphere, OpenStack, etc.)

### 7. Update Examples

- [x] Verify all provider examples include required fields
  - Check AWS examples include region
  - Check Azure examples include location and subscriptionID
  - Check GCP examples include project, region, network.name
  - Update test fixtures for all providers

- [x] Add validation error examples to docs for each provider
  - Show AWS request that triggers validation
  - Show Azure request that triggers validation
  - Show GCP request that triggers validation
  - Show error response format for each
  - Show corrected requests

## Phase 4: Validation and Deployment

### 8. OpenSpec Validation

- [x] Run `openspec validate fix-azure-config-validation --strict`
  - Fix any validation errors
  - Ensure all requirements have scenarios

### 9. Manual Testing

- [x] Test against live k0rdent management cluster (AWS)
  - Deploy valid AWS cluster (verify no regression) ✅ Passed
  - Attempt deploy without region (verify rejection) ✅ Rejected with helpful error
  - Verify error messages are clear and helpful ✅ Error includes examples and docs link

- [x] Test against live k0rdent management cluster (Azure)
  - Deploy valid Azure cluster (verify no regression) ✅ Passed
  - Attempt deploy without subscriptionID (verify rejection) ✅ Rejected with helpful error
  - Attempt deploy without location (verify rejection) ✅ Rejected with helpful error
  - Verify error messages are clear and helpful ✅ Errors include examples and docs link

- [x] GCP validation (unit tests only)
  - Unit tests verify GCP validation logic ✅ 100% coverage
  - Live testing deferred (no GCP environment available)
  - Can be tested later when GCP access is available

### 10. Code Review Preparation

- [x] Review all changes for consistency
  - Check code style matches project conventions ✅
  - Verify error messages are user-friendly ✅
  - No backward compatibility concerns (invalid configs never worked)

- [x] Run full test suite
  - Unit tests pass ✅ 237 tests passing
  - Integration tests pass (including live tests) ✅ All live tests passed
  - No regressions in existing functionality ✅

## Future Enhancements (Not in This Change)

These items are noted for future work but not required for this change:

- Schema-based validation using ClusterTemplate `spec.schema`
- Validation for vSphere templates
- Validation for OpenStack templates
- Cloud-specific validations (e.g., Azure VM SKU validation against Azure API)
- Cross-resource validation (e.g., verify clusterIdentity exists)
- Configuration suggestions/autocomplete
- Validation for optional but recommended fields
- Machine type/instance size validation against cloud APIs

## Dependencies

- Access to k0rdent management cluster for live testing
- **AWS account for live integration tests** (with valid AWS credentials)
- **Azure subscription for live integration tests** (with valid Azure credentials)
- GCP validation implemented and unit-tested (live testing deferred - no GCP environment)
- User assistance may be needed for provider-specific test parameters

## Validation Checklist

Before marking this change complete:

- [x] All unit tests pass (AWS, Azure, GCP)
- [x] Live integration tests pass for AWS and Azure
- [x] GCP unit tests pass (live testing deferred)
- [x] Documentation is updated and accurate for all three providers
- [x] Error messages are clear and actionable
- [x] No regressions in existing functionality
- [x] Code follows project conventions
- [x] OpenSpec validation passes with `--strict`

## Notes

- **GCP validation is fully implemented and unit-tested** but not live-tested due to environment availability
- GCP validation can be live-tested in the future when GCP access is available
- All validation logic follows the same pattern, so GCP unit tests provide confidence

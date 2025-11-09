# Capability: Multi-Provider Cluster Provisioning Configuration Validation

## ADDED Requirements

### Requirement: Cloud cluster deployments SHALL validate provider-specific required configuration fields before submission

The MCP server **SHALL** validate cloud cluster deployment configurations to ensure provider-specific required fields are present before creating ClusterDeployment resources. This validation prevents late failures during cloud resource provisioning and provides immediate, actionable feedback to users.

**Scope:**
- Applies to AWS (`aws-*`), Azure (`azure-*`), and GCP (`gcp-*`) ClusterTemplates
- Validates configuration structure before Kubernetes resource creation
- Uses unified tool (`k0rdent.mgmt.clusterDeployments.deploy`) with provider-specific validation dispatch
- Does not replace Kubernetes admission control or cloud provider API validation

**Rationale:**
- Cloud deployments fail 5-15 minutes into provisioning if required fields are missing
- Early validation saves time and reduces unnecessary cloud costs
- Clear error messages improve user experience
- Matches k0rdent's unified ClusterDeployment API architecture

#### Scenario: Deploy Azure cluster with missing subscriptionID

**Given** a user attempts to deploy an Azure cluster
**And** the configuration is missing the `subscriptionID` field
**When** the `k0rdent.mgmt.clusterDeployments.deploy` tool is called
**Then** the deployment **SHALL** be rejected with a validation error
**And** the error message **SHALL** indicate that `config.subscriptionID` is required
**And** the error message **SHALL** include an example of valid Azure configuration
**And** the error message **SHALL** reference the Azure documentation URL

**Example request (invalid):**
```json
{
  "name": "test-azure-cluster",
  "template": "azure-standalone-cp-1-0-17",
  "credential": "azure-cluster-credential",
  "config": {
    "location": "westus2"
  }
}
```

**Example error response:**
```
invalid configuration for Azure cluster:
  - config.subscriptionID: required field missing (Azure subscription ID required)

Example valid configuration:
{
  "location": "westus2",
  "subscriptionID": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  ...
}

See: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/
```

#### Scenario: Deploy AWS cluster with missing region

**Given** a user attempts to deploy an AWS cluster
**And** the configuration is missing the `region` field
**When** the `k0rdent.mgmt.clusterDeployments.deploy` tool is called
**Then** the deployment **SHALL** be rejected with a validation error
**And** the error message **SHALL** indicate that `config.region` is required
**And** the error message **SHALL** include an example of valid AWS configuration
**And** the error message **SHALL** reference the AWS documentation URL

**Example request (invalid):**
```json
{
  "name": "test-aws-cluster",
  "template": "aws-standalone-cp-1-0-16",
  "credential": "aws-cluster-credential",
  "config": {
    "controlPlane": {"instanceType": "t3.small"},
    "worker": {"instanceType": "t3.small"}
  }
}
```

#### Scenario: Deploy GCP cluster with missing project and network

**Given** a user attempts to deploy a GCP cluster
**And** the configuration is missing the `project` or `network.name` fields
**When** the `k0rdent.mgmt.clusterDeployments.deploy` tool is called
**Then** the deployment **SHALL** be rejected with a validation error
**And** the error message **SHALL** indicate which required fields are missing
**And** the error message **SHALL** include an example of valid GCP configuration
**And** the error message **SHALL** reference the GCP documentation URL

**Example request (invalid):**
```json
{
  "name": "test-gcp-cluster",
  "template": "gcp-standalone-cp-1-0-15",
  "credential": "gcp-credential",
  "config": {
    "region": "us-central1"
  }
}
```

#### Scenario: Deploy Azure cluster with all required fields

**Given** a user attempts to deploy an Azure cluster
**And** the configuration includes both `location` and `subscriptionID` fields
**And** both fields contain non-empty string values
**When** the `k0rdent.mgmt.clusterDeployments.deploy` tool is called
**Then** the validation **SHALL** pass
**And** the ClusterDeployment resource **SHALL** be created
**And** no validation errors **SHALL** be returned

#### Scenario: Deploy AWS cluster with all required fields

**Given** a user attempts to deploy an AWS cluster
**And** the configuration includes the `region` field with a non-empty string value
**When** the `k0rdent.mgmt.clusterDeployments.deploy` tool is called
**Then** the validation **SHALL** pass
**And** the ClusterDeployment resource **SHALL** be created

#### Scenario: Deploy GCP cluster with all required fields

**Given** a user attempts to deploy a GCP cluster
**And** the configuration includes `project`, `region`, and `network.name` fields
**And** all fields contain non-empty string values
**When** the `k0rdent.mgmt.clusterDeployments.deploy` tool is called
**Then** the validation **SHALL** pass
**And** the ClusterDeployment resource **SHALL** be created

#### Scenario: Deploy non-validated cluster without validation

**Given** a user attempts to deploy a cluster with a non-AWS/Azure/GCP template
**And** the template name does NOT match patterns `aws-*`, `azure-*`, or `gcp-*`
**When** the `k0rdent.mgmt.clusterDeployments.deploy` tool is called
**Then** provider-specific validation **SHALL NOT** be performed
**And** the deployment **SHALL** proceed with existing validation only
**And** no provider-specific validation errors **SHALL** be returned

**Example:**
- Template `vsphere-hosted-cp-1-0-13` → no provider validation
- Template `openstack-standalone-cp-1-0-16` → no provider validation
- Template `docker-hosted-cp-1-0-2` → no provider validation

### Requirement: Provider detection SHALL be based on ClusterTemplate name patterns

The MCP server **SHALL** detect the cloud provider from the ClusterTemplate name using prefix matching to determine which validation rules to apply.

**Provider Detection Rules:**
- Templates starting with `aws-` → AWS validation
- Templates starting with `azure-` → Azure validation
- Templates starting with `gcp-` → GCP validation
- All other templates → No provider-specific validation

**Rationale:**
- k0rdent uses consistent template naming conventions
- Template names reliably indicate the target cloud provider
- Simple pattern matching is performant and maintainable
- Aligns with k0rdent's unified ClusterDeployment API

#### Scenario: AWS template triggers AWS validation

**Given** a user deploys a cluster with template name starting with `aws-`
**When** the deployment is validated
**Then** AWS-specific validation rules **SHALL** be applied
**And** AWS required fields **SHALL** be checked (`region`)
**And** other provider validations **SHALL NOT** be applied

**Examples:**
- `aws-standalone-cp-1-0-16` → AWS validation
- `aws-hosted-cp-1-0-14` → AWS validation
- `aws-eks-1-0-3` → AWS validation

#### Scenario: Azure template triggers Azure validation

**Given** a user deploys a cluster with template name starting with `azure-`
**When** the deployment is validated
**Then** Azure-specific validation rules **SHALL** be applied
**And** Azure required fields **SHALL** be checked (`location`, `subscriptionID`)
**And** other provider validations **SHALL NOT** be applied

**Examples:**
- `azure-standalone-cp-1-0-17` → Azure validation
- `azure-hosted-cp-1-0-17` → Azure validation
- `azure-aks-1-0-1` → Azure validation

#### Scenario: GCP template triggers GCP validation

**Given** a user deploys a cluster with template name starting with `gcp-`
**When** the deployment is validated
**Then** GCP-specific validation rules **SHALL** be applied
**And** GCP required fields **SHALL** be checked (`project`, `region`, `network.name`)
**And** other provider validations **SHALL NOT** be applied

**Examples:**
- `gcp-standalone-cp-1-0-15` → GCP validation
- `gcp-hosted-cp-1-0-15` → GCP validation
- `gcp-gke-1-0-5` → GCP validation

### Requirement: Validation errors SHALL be clear and actionable with provider-specific context

Validation error messages **SHALL** provide sufficient context for users to understand what went wrong and how to fix their configuration for each cloud provider.

**Error Message Requirements:**
- Include provider name (AWS, Azure, or GCP)
- List all missing required fields with JSON paths
- Provide example of valid configuration for the provider
- Reference provider-specific documentation URL

**Rationale:**
- Generic error messages lead to confusion and support burden
- Provider-specific examples accelerate problem resolution
- Documentation links provide detailed guidance

#### Scenario: AWS validation error includes all required elements

**Given** an AWS cluster deployment fails validation
**When** a validation error is returned
**Then** the error message **SHALL** indicate "AWS cluster" in the message
**And** the error message **SHALL** list missing field as `config.region`
**And** the error message **SHALL** include an example AWS configuration
**And** the error message **SHALL** include URL to AWS quickstart documentation

#### Scenario: Azure validation error includes all required elements

**Given** an Azure cluster deployment fails validation
**When** a validation error is returned
**Then** the error message **SHALL** indicate "Azure cluster" in the message
**And** the error message **SHALL** list missing fields (e.g., `config.subscriptionID`, `config.location`)
**And** the error message **SHALL** include an example Azure configuration
**And** the error message **SHALL** include URL to Azure quickstart documentation

#### Scenario: GCP validation error includes all required elements

**Given** a GCP cluster deployment fails validation
**When** a validation error is returned
**Then** the error message **SHALL** indicate "GCP cluster" in the message
**And** the error message **SHALL** list missing fields (e.g., `config.project`, `config.network.name`)
**And** the error message **SHALL** include an example GCP configuration
**And** the error message **SHALL** include URL to GCP quickstart documentation

## MODIFIED Requirements

### Requirement: ClusterDeployment creation **SHALL** validate configuration before submission

The `DeployCluster` function **SHALL** validate provider-specific configuration requirements based on the ClusterTemplate name pattern before creating ClusterDeployment resources.

**Previous behavior:**
- The function validated only basic request fields (name, template, credential)
- Provider-specific configuration was not validated
- Invalid configurations were accepted and failed during cloud provisioning (5-15 minutes later)

**Changes:**
- **ADD**: Provider detection based on template name pattern
- **ADD**: Provider-specific validation dispatch (AWS, Azure, GCP)
- **ADD**: Configuration field validation with immediate error feedback
- **ADD**: Provider-specific error messages with examples and documentation links
- **MAINTAIN**: Existing validation for name, template, credential remains unchanged
- **REMOVE**: No backward compatibility for invalid configurations (strict validation)

#### Scenario: Validation occurs before ClusterDeployment creation

**Given** a user calls `k0rdent.mgmt.clusterDeployments.deploy`
**And** the template matches an AWS, Azure, or GCP pattern
**And** the configuration is invalid (missing required fields)
**When** the deployment is submitted
**Then** validation **SHALL** fail before any Kubernetes API calls
**And** no ClusterDeployment resource **SHALL** be created
**And** no Kubernetes resources **SHALL** be modified
**And** an error **SHALL** be returned to the caller immediately

#### Scenario: Validation enforces strict requirements without backward compatibility

**Given** a user deploys a cluster with an invalid configuration
**And** the configuration was invalid according to cloud provider documentation
**When** the deployment is submitted
**Then** the deployment **SHALL** be rejected immediately
**And** no compatibility mode or opt-out flag **SHALL** be available
**And** the error message **SHALL** guide the user to fix the configuration

**Rationale:**
- Per user request, no backward compatibility
- Invalid configs never successfully provisioned anyway
- Immediate feedback prevents wasted time (5-15 minutes per failed attempt)
- Reduces unnecessary cloud costs from failed provisioning

## Implementation Notes

### Provider-Specific Required Fields

#### AWS (`aws-*` templates)
- `config.region` (string, non-empty) - AWS region (e.g., "us-east-2")

#### Azure (`azure-*` templates)
- `config.location` (string, non-empty) - Azure region (e.g., "westus2")
- `config.subscriptionID` (string, non-empty) - Azure subscription ID (UUID)

#### GCP (`gcp-*` templates)
- `config.project` (string, non-empty) - GCP project name
- `config.region` (string, non-empty) - GCP region (e.g., "us-central1")
- `config.network.name` (string, non-empty) - VPC network name (e.g., "default")

### Validation Timing

Validation occurs in this order:
1. Basic request validation (name, template, credential) - **existing**
2. Template reference resolution - **existing**
3. **Provider detection from template name - NEW**
4. **Provider-specific configuration validation - NEW**
5. Credential reference resolution - **existing**
6. ClusterDeployment manifest construction - **existing**
7. Server-side apply to Kubernetes - **existing**

### Error Response Format

Validation errors use MCP error code `-32602` (Invalid params) with provider-specific messages.

### Future Extensibility

This design supports future enhancements:
- **vSphere validation**: Add `ValidateVSphereConfig()` and pattern `vsphere-*`
- **OpenStack validation**: Add `ValidateOpenStackConfig()` and pattern `openstack-*`
- **Schema-based validation**: Parse ClusterTemplate `spec.schema` for dynamic rules
- **Cross-resource validation**: Verify clusterIdentity or credential existence

No changes to existing provider validation required for future additions.

### Unified Tool Architecture

**Why a unified tool?**

k0rdent uses a **single ClusterDeployment API** across all providers:
- One `ClusterDeployment` CRD kind for all cloud providers
- Provider abstracted through template selection
- Configuration varies per provider in `spec.config` section

**Benefits:**
- Matches k0rdent's architecture (unified API, provider detection from template)
- Users don't need to choose provider-specific tools
- Single tool to document, maintain, and test
- Consistent user experience across providers
- Provider detection happens automatically

**Alternative rejected:** Provider-specific tools (e.g., `k0rdent.provider.aws.clusterDeployments.deploy`) would:
- Create artificial separation not present in k0rdent
- Require maintaining 3+ separate MCP tools
- Force users to choose the correct tool upfront
- Duplicate code and tests across tools

**Decision:** Use unified tool with provider-specific validation dispatch.

## Testing Requirements

### Unit Tests

- `TestValidateAWSConfig` - Test AWS validation logic
  - Valid config passes
  - Missing region fails
  - Empty region string fails

- `TestValidateAzureConfig` - Test Azure validation logic
  - Valid config passes
  - Missing subscriptionID fails
  - Missing location fails
  - Empty strings treated as missing

- `TestValidateGCPConfig` - Test GCP validation logic
  - Valid config passes
  - Missing project fails
  - Missing region fails
  - Missing network.name fails

- `TestDetectProvider` - Test provider detection
  - `aws-*` templates detected as AWS
  - `azure-*` templates detected as Azure
  - `gcp-*` templates detected as GCP
  - Other templates return unknown

### Integration Tests

- `TestDeployClusterMultiProviderValidation` - Test end-to-end validation
  - AWS deploy rejects invalid config
  - Azure deploy rejects invalid config
  - GCP deploy rejects invalid config
  - Valid configs accepted for all providers
  - Error messages correct for each provider
  - No ClusterDeployment created on validation failure

### Live Tests

- `TestClustersProvisioningAWS` - Test with real AWS credentials
  - Valid AWS config deploys successfully
  - Invalid config rejected immediately
  - *Live testing with available AWS environment*

- `TestClustersProvisioningAzure` - Test with real Azure credentials
  - Valid Azure config deploys successfully
  - Invalid config rejected immediately
  - *Live testing with available Azure environment*

- `TestClustersProvisioningGCP` - Unit tests only
  - GCP validation logic verified through unit tests
  - Live testing deferred (no GCP environment available)
  - *Can be tested later when GCP access is available*

## Documentation Requirements

### Updates Required

1. **docs/cluster-provisioning.md**
   - Add "Configuration Validation" section
   - Document required fields for AWS, Azure, GCP
   - Show validation error examples for each provider
   - Update baseline configurations for all providers

2. **Provider-Specific Documentation**
   - Update AWS quickstart with validation notes
   - Update Azure quickstart with validation notes
   - Update GCP quickstart with validation notes

3. **Error Handling Documentation**
   - Add multi-provider validation errors to common errors
   - Provide troubleshooting per provider
   - Link to provider-specific quickstarts

4. **Code Comments**
   - Document provider detection logic
   - Explain validation dispatch mechanism
   - Note future extensibility points for new providers

## Related Changes

- **Future**: Schema-based validation for all providers
- **Future**: vSphere, OpenStack configuration validation
- **Future**: Cross-resource validation (verify references exist)

## References

- [k0rdent Cluster Deployment](https://docs.k0rdent.io/latest/admin/clusters/deploy-cluster/)
- [AWS Quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/)
- [Azure Quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/)
- [GCP Quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/)
- Current implementation: `internal/clusters/deploy.go`
- Live tests: `test/integration/clusters_live_test.go`
- Cluster provisioning docs: `docs/cluster-provisioning.md`

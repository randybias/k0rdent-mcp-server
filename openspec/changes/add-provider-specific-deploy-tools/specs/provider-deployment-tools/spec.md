# Capability: Provider-Specific Cluster Deployment Tools

## ADDED Requirements

### Requirement: AWS-specific cluster deployment tool

The k0rdent MCP server SHALL provide a tool `k0rdent.provider.aws.clusterDeployments.deploy` that exposes AWS-specific configuration parameters directly in its MCP tool schema, enabling AI agents to discover requirements through standard tool introspection.

**Acceptance Criteria:**
- Tool name is `k0rdent.provider.aws.clusterDeployments.deploy`
- Tool schema includes required fields: `name`, `credential`, `region`, `controlPlane`, `worker`
- Schema defines `controlPlane.instanceType` and `worker.instanceType` as required strings
- Schema includes optional fields with defaults: `controlPlaneNumber` (default: 3), `workersNumber` (default: 2)
- Tool automatically selects the latest stable AWS ClusterTemplate
- Tool builds config map and calls existing deployment logic
- Tool reuses existing validation rules

#### Scenario: Agent discovers AWS cluster requirements from tool schema

**Given** an AI agent has no prior knowledge of AWS cluster configuration
**When** the agent lists MCP tools
**Then** the agent sees `k0rdent.provider.aws.clusterDeployments.deploy` in the list
**When** the agent requests the tool schema via MCP protocol
**Then** the schema includes:
- `region` field marked as required with type "string" and description
- `controlPlane` object with required `instanceType` field
- `worker` object with required `instanceType` field
- `controlPlaneNumber` with default value 3
- `workersNumber` with default value 2

**And** the agent can construct a valid deployment request without additional API calls

#### Scenario: Deploy AWS cluster with auto-selected template

**Given** multiple AWS templates exist: "aws-standalone-cp-1-0-13", "aws-standalone-cp-1-0-14"
**When** an agent calls `k0rdent.provider.aws.clusterDeployments.deploy` with:
```json
{
  "name": "my-aws-cluster",
  "credential": "aws-cluster-credential",
  "region": "us-west-2",
  "controlPlane": {"instanceType": "t3.small"},
  "worker": {"instanceType": "t3.small"}
}
```

**Then** the tool automatically selects "aws-standalone-cp-1-0-14" (latest version)
**And** the tool builds a config map with region, instance types, and defaults
**And** the tool calls existing deploy logic with the selected template
**And** existing validation rules check the configuration
**And** the deployment is created successfully

#### Scenario: AWS tool applies default values

**Given** an agent omits optional fields
**When** the agent calls AWS deploy with minimal input (name, credential, region, instance types only)
**Then** the tool applies defaults:
- `controlPlaneNumber`: 3
- `workersNumber`: 2
- `controlPlane.rootVolumeSize`: 32
- `worker.rootVolumeSize`: 32

**And** the deployment uses these default values

---

### Requirement: Azure-specific cluster deployment tool

The k0rdent MCP server SHALL provide a tool `k0rdent.provider.azure.clusterDeployments.deploy` that exposes Azure-specific configuration parameters with Azure-appropriate field names (`location` not `region`, `vmSize` not `instanceType`).

**Acceptance Criteria:**
- Tool name is `k0rdent.provider.azure.clusterDeployments.deploy`
- Tool schema includes required fields: `name`, `credential`, `location`, `subscriptionID`, `controlPlane`, `worker`
- Schema defines `controlPlane.vmSize` and `worker.vmSize` (not instanceType)
- Tool automatically selects the latest stable Azure ClusterTemplate
- Tool validates location and subscriptionID are provided
- Tool builds config map with Azure-specific field structure

#### Scenario: Agent discovers Azure uses different field names than AWS

**Given** an AI agent has just learned about AWS clusters (which use `region` and `instanceType`)
**When** the agent lists tools and discovers `k0rdent.provider.azure.clusterDeployments.deploy`
**And** the agent inspects the Azure tool schema
**Then** the agent learns that Azure requires:
- `location` field (not `region`)
- `subscriptionID` field (additional requirement)
- `vmSize` for machine sizing (not `instanceType`)

**And** the agent adapts its deployment request to use Azure-specific field names
**And** the agent does not need hard-coded knowledge of these differences

#### Scenario: Deploy Azure cluster with correct field names

**Given** an agent has inspected the Azure tool schema
**When** the agent calls `k0rdent.provider.azure.clusterDeployments.deploy` with:
```json
{
  "name": "my-azure-cluster",
  "credential": "azure-cluster-credential",
  "location": "westus2",
  "subscriptionID": "12345678-1234-1234-1234-123456789abc",
  "controlPlane": {"vmSize": "Standard_A4_v2"},
  "worker": {"vmSize": "Standard_A4_v2"}
}
```

**Then** the tool auto-selects the latest Azure template
**And** the tool builds config with Azure field names (location, subscriptionID, vmSize)
**And** existing Azure validation rules validate the configuration
**And** the deployment is created successfully

#### Scenario: Azure tool requires both location and subscriptionID

**Given** an agent attempts to deploy an Azure cluster
**When** the agent provides location but omits subscriptionID
**Then** validation fails with error indicating subscriptionID is required
**When** the agent provides subscriptionID but omits location
**Then** validation fails with error indicating location is required
**When** the agent provides both fields
**Then** validation passes and deployment proceeds

---

### Requirement: GCP-specific cluster deployment tool

The k0rdent MCP server SHALL provide a tool `k0rdent.provider.gcp.clusterDeployments.deploy` that exposes GCP-specific configuration parameters including nested network configuration.

**Acceptance Criteria:**
- Tool name is `k0rdent.provider.gcp.clusterDeployments.deploy`
- Tool schema includes required fields: `name`, `credential`, `project`, `region`, `network`, `controlPlane`, `worker`
- Schema defines nested `network.name` field as required
- Tool automatically selects the latest stable GCP ClusterTemplate
- Tool builds config map with proper nesting for network configuration

#### Scenario: Agent discovers GCP requires nested network configuration

**Given** an AI agent is deploying a GCP cluster
**When** the agent inspects the GCP tool schema
**Then** the agent learns that GCP requires:
- `project` field (string) - GCP project ID
- `region` field (string) - GCP region
- `network` object with nested `name` field - VPC network name
- `controlPlane.instanceType` and `worker.instanceType` (like AWS, unlike Azure)

**And** the agent understands the nested structure for network configuration

#### Scenario: Deploy GCP cluster with nested network config

**Given** an agent has inspected the GCP tool schema
**When** the agent calls `k0rdent.provider.gcp.clusterDeployments.deploy` with:
```json
{
  "name": "my-gcp-cluster",
  "credential": "gcp-credential",
  "project": "my-gcp-project-123456",
  "region": "us-central1",
  "network": {"name": "default"},
  "controlPlane": {"instanceType": "n1-standard-4"},
  "worker": {"instanceType": "n1-standard-4"}
}
```

**Then** the tool auto-selects the latest GCP template
**And** the tool builds config map with nested network.name structure
**And** existing GCP validation rules validate project, region, and network.name
**And** the deployment is created successfully

---

### Requirement: Automatic template selection per provider

The provider-specific deployment tools SHALL automatically select the latest stable ClusterTemplate for their respective provider, removing template selection complexity from AI agents.

**Acceptance Criteria:**
- Each provider tool queries available templates in the target namespace
- Tool filters templates by provider-specific prefix (e.g., "aws-standalone-cp-")
- Tool sorts filtered templates by semantic version
- Tool selects the latest version
- Tool fails gracefully if no matching templates exist
- Template selection is transparent to the agent (no template parameter needed)

#### Scenario: Auto-select latest AWS template from multiple versions

**Given** the following AWS templates exist in kcm-system namespace:
- "aws-standalone-cp-1-0-12"
- "aws-standalone-cp-1-0-14"
- "aws-standalone-cp-1-0-13"

**And** an Azure template also exists: "azure-standalone-cp-1-0-15"
**When** the AWS deploy tool selects a template
**Then** the tool queries templates in the namespace
**And** the tool filters to only "aws-standalone-cp-*" templates
**And** the tool sorts by version: [1.0.14, 1.0.13, 1.0.12]
**And** the tool selects "aws-standalone-cp-1-0-14" (latest)
**And** the Azure template is not considered

#### Scenario: Handle no matching templates for provider

**Given** no AWS templates exist in the namespace
**When** the AWS deploy tool attempts template selection
**Then** the tool returns an error indicating "No AWS templates available in namespace kcm-system"
**And** the error message is clear and actionable
**And** the deployment does not proceed

#### Scenario: Semantic version sorting handles edge cases

**Given** AWS templates with versions: "1.0.14", "1.1.0", "1.0.99", "2.0.0"
**When** the tool sorts by semantic version
**Then** the sorted order is: ["2.0.0", "1.1.0", "1.0.99", "1.0.14"]
**And** the tool selects "2.0.0" as the latest
**And** version comparison correctly handles major, minor, and patch differences

---

### Requirement: Generic deployment tool is removed

The existing generic deployment tool `k0rdent.mgmt.clusterDeployments.deploy` SHALL be removed in favor of provider-specific tools. This is a breaking change justified by early development stage and the goal of a cleaner, more discoverable API for AI agents.

**Acceptance Criteria:**
- Generic tool `k0rdent.mgmt.clusterDeployments.deploy` is removed from tool registration
- Generic deploy handler code is removed
- Documentation explains the breaking change and migration path
- Error message guides users to provider-specific tools if they attempt to use removed tool
- All references to generic tool are updated in tests and documentation

#### Scenario: Generic tool is no longer available

**Given** the k0rdent MCP server has been updated with provider-specific tools
**When** an agent attempts to list available tools
**Then** `k0rdent.mgmt.clusterDeployments.deploy` does not appear in the tool list
**And** only provider-specific tools are available: `k0rdent.provider.aws.clusterDeployments.deploy`, `k0rdent.provider.azure.clusterDeployments.deploy`, `k0rdent.provider.gcp.clusterDeployments.deploy`

#### Scenario: Documentation guides migration from generic to provider-specific tools

**Given** a user previously used the generic deployment tool
**When** the user reads the migration documentation
**Then** the documentation explains:
- Why the generic tool was removed (discoverability, reduced context overhead for AI agents)
- How to identify which provider-specific tool to use
- How to map generic config to provider-specific fields
- Examples showing before/after for each provider

**And** the migration is straightforward (provider-specific tools are more explicit)

---

### Requirement: Single-step agent workflow

AI agents SHALL be able to deploy clusters in a single step after tool discovery, without needing to fetch schemas separately or make multiple API calls for configuration information.

**Acceptance Criteria:**
- Agent lists MCP tools and sees provider-specific options (one API call)
- Agent inspects tool schema via standard MCP protocol (already cached/included in tool list response)
- Agent has all information needed to construct deployment request
- No separate schema-fetching API calls required
- Context overhead is minimized

#### Scenario: Agent workflow from discovery to deployment

**Given** an AI agent wants to deploy an AWS cluster
**And** the agent has no prior knowledge of k0rdent or AWS configuration
**When** the agent performs the following steps:
1. Call MCP `tools/list` to discover available tools
2. Identify `k0rdent.provider.aws.clusterDeployments.deploy` in the list
3. Inspect the tool's input schema (provided in MCP tools response)
4. Extract required fields from schema: region, controlPlane.instanceType, worker.instanceType
5. Construct deployment request with required parameters
6. Call `k0rdent.provider.aws.clusterDeployments.deploy` with constructed parameters

**Then** the entire workflow requires:
- 1 tool list API call (discovery)
- 1 deployment API call (execution)
- **Total: 2 API calls, zero schema-fetching calls**

**And** the agent successfully deploys without hard-coded AWS knowledge
**And** context overhead is minimal (tool schema is included in standard MCP response)

#### Scenario: Compare context overhead vs schema-fetching approach

**Given** two approaches to cluster deployment:
- **Approach A** (this proposal): Provider-specific tools with embedded schemas
- **Approach B** (alternative): Generic tool + separate schema API

**When** an agent uses Approach A:
- API calls: 2 (list tools, deploy)
- Context: Tool schema (~1-2 KB) included in tool list
- **Total context overhead: ~2 KB**

**When** an agent uses Approach B:
- API calls: 3 (list tools, get schema, deploy)
- Context: Tool list + separate schema (~10-20 KB for full Helm schema)
- **Total context overhead: ~20 KB**

**Then** Approach A reduces:
- API calls by 33% (2 vs 3)
- Context overhead by 90% (2 KB vs 20 KB)
- Agent complexity (no schema parsing logic needed)

**And** Approach A optimizes for agent discoverability and ease of use

---

### Requirement: MCP prompt templates for usage examples

The k0rdent MCP server SHALL provide prompt templates that demonstrate complete working examples of cluster deployment for each provider, enabling AI agents to learn usage patterns through the standard MCP prompts protocol.

**Acceptance Criteria:**
- Server exposes prompts via MCP `prompts/list` and `prompts/get` endpoints
- Prompts are named: `k0rdent/examples/deploy-aws-cluster`, `k0rdent/examples/deploy-azure-cluster`, `k0rdent/examples/deploy-gcp-cluster`
- Each prompt includes complete working example with real parameter values
- Prompts are parameterized (e.g., clusterName, region) for customization
- Examples include inline explanations of each field
- Prompts show common variations (production, development, different regions)
- Links to k0rdent documentation are included

#### Scenario: Agent discovers AWS deployment example via prompts

**Given** an AI agent wants to learn how to deploy an AWS cluster
**When** the agent calls MCP `prompts/list`
**Then** the response includes `k0rdent/examples/deploy-aws-cluster` in the prompts list
**And** the description indicates it's an example of AWS cluster deployment
**When** the agent calls `prompts/get` with name `k0rdent/examples/deploy-aws-cluster`
**Then** the agent receives a complete working example including:
- Tool name: `k0rdent.provider.aws.clusterDeployments.deploy`
- All required fields with example values (region, instanceType, etc.)
- Field explanations describing each parameter
- Common variations (production vs development configurations)
- Documentation link

**And** the example includes prompt arguments for customization (clusterName, region)

#### Scenario: Agent uses prompt template to customize deployment

**Given** an agent has retrieved the AWS deployment example prompt
**When** the agent provides prompt arguments:
```json
{
  "clusterName": "my-production-cluster",
  "region": "eu-west-1"
}
```

**Then** the prompt template substitutes the arguments
**And** the resulting example shows:
- `"name": "my-production-cluster"`
- `"region": "eu-west-1"`

**And** other fields remain as example defaults
**And** the agent can use this as a starting point for deployment

#### Scenario: Prompt examples distinguish provider-specific field names

**Given** an agent has reviewed the AWS example (uses `region` and `instanceType`)
**When** the agent retrieves the Azure example prompt
**Then** the Azure example clearly shows:
- `"location"` field instead of `"region"`
- `"subscriptionID"` as an additional required field
- `"vmSize"` instead of `"instanceType"`

**And** the field explanations highlight these differences
**And** the agent learns provider-specific conventions from examples

---

### Requirement: Validation integration

Provider-specific deployment tools SHALL reuse existing configuration validation rules, ensuring consistent error messages and validation behavior across both provider-specific and generic deployment tools.

**Acceptance Criteria:**
- Provider tools build config maps that existing validation rules can check
- AWS validation rules (from fix-azure-config-validation) apply to AWS tool
- Azure validation rules apply to Azure tool
- GCP validation rules apply to GCP tool
- Validation errors are consistent whether using provider or generic tool
- No duplicate validation logic

#### Scenario: AWS tool validation catches missing region

**Given** existing AWS validation rules require `region` field
**When** an agent calls AWS deploy tool without providing region
**Then** the MCP input validation catches the error (required field)
**Or** if field is provided but empty, existing config validation catches it
**And** the error message indicates "region is required"
**And** the error matches what generic tool would return
**And** no deployment resource is created

#### Scenario: Azure tool validation catches missing subscriptionID

**Given** existing Azure validation rules require `subscriptionID` field
**When** an agent calls Azure deploy tool with location but no subscriptionID
**Then** the MCP input validation catches the error
**And** the error message indicates "subscriptionID is required"
**And** the validation behavior is consistent with generic tool

#### Scenario: Provider tool config passes existing validation

**Given** an agent provides valid configuration via AWS deploy tool
**When** the tool builds the config map
**Then** the config includes all fields that existing validation expects
**And** the config is validated using existing validation logic (fix-azure-config-validation)
**And** validation passes
**And** the deployment proceeds normally

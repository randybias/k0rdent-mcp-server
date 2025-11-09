# Proposal: Add Provider-Specific Cluster Deployment Tools

## Why

AI agents cannot currently discover what configuration parameters are needed for deploying clusters to different cloud providers. The current unified `k0rdent.mgmt.clusterDeployments.deploy` tool accepts a freeform `config` object, forcing agents to have hard-coded knowledge that:
- AWS requires `region` and uses `instanceType` for machine sizing
- Azure requires `location` + `subscriptionID` and uses `vmSize` for machine sizing
- GCP requires `project` + `region` + `network.name` for networking

This creates unnecessary complexity and context overhead for AI agents. They must maintain provider-specific knowledge and perform a two-step process: (1) determine the provider, (2) construct the appropriate config structure.

**The root issue**: We're optimizing for code reuse (one deploy tool) instead of optimizing for AI agent discoverability and ease of use.

## Problem Statement

When an AI agent lists available MCP tools, it sees:
```
k0rdent.mgmt.clusterDeployments.deploy
  - name (string)
  - template (string)
  - credential (string)
  - config (object)  ← Opaque! What goes here?
```

The agent has **no way to discover** what the `config` object should contain. It must either:
1. Be pre-programmed with provider knowledge (defeats discoverability)
2. Fetch schemas in a separate step (adds complexity and context overhead)
3. Trial-and-error until validation errors provide hints (poor UX)

### User Story

**As an AI agent**, I want to deploy a cluster to AWS without prior knowledge of AWS configuration requirements, so that I can discover and use new cloud providers dynamically.

**Current Experience** (poor):
```
Agent: List tools
MCP:   [..., k0rdent.mgmt.clusterDeployments.deploy, ...]
Agent: [Sees deploy tool but config is opaque]
Agent: [Must have hard-coded: AWS needs region field]
Agent: Deploy with {config: {region: "us-west-2", ...}}
```

**Desired Experience** (discoverable):
```
Agent: List tools
MCP:   [..., k0rdent.provider.aws.clusterDeployments.deploy, k0rdent.provider.azure.clusterDeployments.deploy, ...]
Agent: [Sees AWS-specific tool with explicit parameters]
Agent: Inspect k0rdent.provider.aws.clusterDeployments.deploy
MCP:   Parameters: region (string, required), controlPlane.instanceType (string, required), ...
Agent: [Learns requirements from tool schema]
Agent: Deploy using AWS tool with proper parameters
```

## Proposed Solution

Replace the single generic deploy tool with **provider-specific deployment tools**:

**New Tools (replace generic tool):**
- `k0rdent.provider.aws.clusterDeployments.deploy` - AWS-specific parameters
- `k0rdent.provider.azure.clusterDeployments.deploy` - Azure-specific parameters
- `k0rdent.provider.gcp.clusterDeployments.deploy` - GCP-specific parameters
- `k0rdent.provider.vsphere.clusterDeployments.deploy` - vSphere-specific parameters (add when needed)

**Remove:** Generic `k0rdent.mgmt.clusterDeployments.deploy` tool - replaced by provider-specific tools

Each provider-specific tool exposes its configuration parameters **directly in the MCP tool schema**, making them discoverable through standard MCP tool introspection.

### Example: AWS Deploy Tool Schema

```typescript
{
  name: "k0rdent.provider.aws.clusterDeployments.deploy",
  description: "Deploy a new AWS Kubernetes cluster using k0rdent",
  inputSchema: {
    type: "object",
    required: ["name", "credential", "region", "controlPlane", "worker"],
    properties: {
      name: {
        type: "string",
        description: "Name for the cluster deployment"
      },
      credential: {
        type: "string",
        description: "Name of the AWS credential to use"
      },
      region: {
        type: "string",
        description: "AWS region (e.g., 'us-west-2', 'us-east-1', 'eu-west-1')"
      },
      controlPlane: {
        type: "object",
        required: ["instanceType"],
        properties: {
          instanceType: {
            type: "string",
            description: "EC2 instance type for control plane nodes (e.g., 't3.small', 'm5.large')"
          },
          rootVolumeSize: {
            type: "integer",
            description: "Root volume size in GB (default: 32)",
            default: 32
          }
        }
      },
      worker: {
        type: "object",
        required: ["instanceType"],
        properties: {
          instanceType: {
            type: "string",
            description: "EC2 instance type for worker nodes"
          },
          rootVolumeSize: {
            type: "integer",
            description: "Root volume size in GB (default: 32)",
            default: 32
          }
        }
      },
      controlPlaneNumber: {
        type: "integer",
        description: "Number of control plane nodes (default: 3)",
        default: 3
      },
      workersNumber: {
        type: "integer",
        description: "Number of worker nodes (default: 2)",
        default: 2
      },
      namespace: {
        type: "string",
        description: "Namespace for the cluster deployment (default: kcm-system)"
      },
      wait: {
        type: "boolean",
        description: "Wait for cluster to be ready before returning (default: false)"
      }
    }
  }
}
```

### Implementation Approach

**Go Struct-Based Schema Generation:**
```go
// AWS-specific input struct
type awsClusterDeployInput struct {
    Name              string                  `json:"name" jsonschema:"required,description=Name for the cluster deployment"`
    Credential        string                  `json:"credential" jsonschema:"required,description=Name of the AWS credential to use"`
    Region            string                  `json:"region" jsonschema:"required,description=AWS region (e.g. us-west-2, us-east-1)"`
    ControlPlane      awsControlPlaneConfig   `json:"controlPlane" jsonschema:"required"`
    Worker            awsWorkerConfig         `json:"worker" jsonschema:"required"`
    ControlPlaneNumber int                    `json:"controlPlaneNumber,omitempty" jsonschema:"description=Number of control plane nodes,default=3"`
    WorkersNumber     int                     `json:"workersNumber,omitempty" jsonschema:"description=Number of worker nodes,default=2"`
    Namespace         string                  `json:"namespace,omitempty" jsonschema:"description=Namespace for deployment,default=kcm-system"`
    Wait              bool                    `json:"wait,omitempty" jsonschema:"description=Wait for ready before returning"`
}

type awsControlPlaneConfig struct {
    InstanceType    string `json:"instanceType" jsonschema:"required,description=EC2 instance type for control plane"`
    RootVolumeSize  int    `json:"rootVolumeSize,omitempty" jsonschema:"description=Root volume size in GB,default=32"`
}

type awsWorkerConfig struct {
    InstanceType    string `json:"instanceType" jsonschema:"required,description=EC2 instance type for workers"`
    RootVolumeSize  int    `json:"rootVolumeSize,omitempty" jsonschema:"description=Root volume size in GB,default=32"`
}
```

The MCP Go SDK automatically generates JSON Schema from these structs using struct tags, exposing them to AI agents via the standard MCP tools/list protocol.

**Handler implementation:**
```go
func (t *awsClusterDeployTool) deploy(ctx context.Context, req *mcp.CallToolRequest, input awsClusterDeployInput) (*mcp.CallToolResult, deployResult, error) {
    // Convert AWS-specific input to generic DeployRequest
    deployReq := clusters.DeployRequest{
        Name:       input.Name,
        Template:   t.resolveAWSTemplate(input),  // Auto-select appropriate AWS template
        Credential: input.Credential,
        Namespace:  input.Namespace,
        Config:     t.buildAWSConfig(input),       // Build config map from structured input
        Wait:       input.Wait,
    }

    // Use existing deploy logic
    return t.session.Clusters.Deploy(ctx, deployReq)
}
```

### Template Selection Strategy

**Option 1: Auto-select latest template (Recommended)**
```go
func (t *awsClusterDeployTool) resolveAWSTemplate(input awsClusterDeployInput) string {
    // Query for latest aws-standalone-cp template
    // Return: "aws-standalone-cp-1-0-14" (or latest version)
}
```

**Option 2: Allow template override**
```go
type awsClusterDeployInput struct {
    Template string `json:"template,omitempty" jsonschema:"description=Override AWS template (auto-selects latest if omitted)"`
    // ... other fields
}
```

**Decision: Use Option 1** - Simplify for agents, auto-select latest stable template per provider

### Tool Namespace Pattern

**Provider-specific tool namespace: `k0rdent.provider.<provider_name>.*`**

This namespace pattern is intentionally chosen to be extensible for future provider-specific operations beyond cluster deployment:

**Current (this proposal):**
- `k0rdent.provider.aws.clusterDeployments.deploy`
- `k0rdent.provider.azure.clusterDeployments.deploy`
- `k0rdent.provider.gcp.clusterDeployments.deploy`

**Future operations (out of scope for this proposal):**
- `k0rdent.provider.aws.credentials.create` - Create AWS credentials
- `k0rdent.provider.aws.identity.create` - Create AWS cluster identity
- `k0rdent.provider.azure.credentials.create` - Create Azure credentials
- `k0rdent.provider.azure.identity.create` - Create Azure cluster identity
- `k0rdent.provider.gcp.serviceAccounts.create` - Create GCP service accounts
- ... other provider-specific operations

**Rationale:**
- Clear separation: `k0rdent.mgmt.*` for cross-provider management operations, `k0rdent.provider.*` for provider-specific operations
- Discoverability: Agents can filter tools by provider namespace to see all operations available for a provider
- Extensibility: New provider operations follow the same pattern
- Consistency: Same namespace structure across all providers

### MCP Prompts for Usage Examples

**Provide MCP prompt templates** to help AI agents learn usage patterns:

**Example prompts:**
- `k0rdent/examples/deploy-aws-cluster` - Complete AWS deployment example
- `k0rdent/examples/deploy-azure-cluster` - Complete Azure deployment example
- `k0rdent/examples/deploy-gcp-cluster` - Complete GCP deployment example

Each prompt template includes:
- Complete working example with real parameter values
- Inline comments explaining each field
- Common variations (different instance types, regions, etc.)
- Links to documentation

**MCP Prompt Definition Example:**
```json
{
  "name": "k0rdent/examples/deploy-aws-cluster",
  "description": "Example of deploying an AWS Kubernetes cluster using k0rdent",
  "arguments": [
    {
      "name": "clusterName",
      "description": "Name for the cluster",
      "required": true
    },
    {
      "name": "region",
      "description": "AWS region",
      "required": false
    }
  ]
}
```

**Prompt Template Content:**
```markdown
# Deploy AWS Kubernetes Cluster

Here's how to deploy a Kubernetes cluster on AWS using k0rdent:

## Prerequisites
- AWS credential must be configured: `aws-cluster-credential`
- Template will be auto-selected (latest aws-standalone-cp)

## Example Deployment

Use the `k0rdent.provider.aws.clusterDeployments.deploy` tool:

{
  "name": "{{clusterName}}",
  "credential": "aws-cluster-credential",
  "region": "{{region:us-west-2}}",
  "controlPlane": {
    "instanceType": "t3.small",
    "rootVolumeSize": 32
  },
  "worker": {
    "instanceType": "t3.small",
    "rootVolumeSize": 32
  },
  "controlPlaneNumber": 3,
  "workersNumber": 2,
  "namespace": "kcm-system",
  "wait": false
}

## Field Explanations

- **region**: AWS region (e.g., us-west-2, us-east-1, eu-west-1)
- **instanceType**: EC2 instance type (e.g., t3.small, m5.large, t3.medium)
- **rootVolumeSize**: Root disk size in GB (minimum 8, default 32)
- **controlPlaneNumber**: Number of control plane nodes (default 3, minimum 1)
- **workersNumber**: Number of worker nodes (default 2, minimum 1)

## Common Variations

### Production cluster (larger instances)
Change instanceType to "m5.large" or "m5.xlarge"

### Development cluster (minimal)
Use controlPlaneNumber: 1, workersNumber: 1, instanceType: "t3.micro"

### Different region
Change region to your preferred AWS region

For more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/
```

**Benefits:**
- AI agents can call `prompts/list` to discover examples
- Agents can call `prompts/get` to retrieve example templates
- Examples are version-controlled with the server
- Examples show best practices and common patterns
- Reduces trial-and-error for agents learning the API

## Success Criteria

1. ✅ AI agents can list MCP tools and see provider-specific deploy tools
2. ✅ Each provider tool exposes its configuration parameters in the MCP tool schema
3. ✅ Agents can introspect tool schema to learn required fields, types, and descriptions
4. ✅ Agents can deploy AWS clusters without hard-coded provider knowledge
5. ✅ Agents can deploy Azure clusters and discover different field names (vmSize vs instanceType)
6. ✅ Agents can deploy GCP clusters and learn nested config requirements (network.name)
7. ✅ Generic deploy tool remains available for custom/unknown providers
8. ✅ Implementation reuses existing validation and deployment logic
9. ✅ MCP prompt templates provide working examples for each provider

## Impact Analysis

**Affected Components:**
- `internal/tools/core/clusters_aws.go` (new) - AWS-specific tool
- `internal/tools/core/clusters_azure.go` (new) - Azure-specific tool
- `internal/tools/core/clusters_gcp.go` (new) - GCP-specific tool
- `internal/tools/core/clusters.go` (modified) - Register new tools
- `internal/clusters/deploy.go` (unchanged) - Reuse existing logic
- `docs/cluster-provisioning.md` (updated) - Document new tools

**Benefits:**
- ✅ **Single-step discovery**: Agents see all options in tool list
- ✅ **Zero context overhead**: No need to fetch schemas separately
- ✅ **Self-documenting**: Tool schema contains all necessary information
- ✅ **No hard-coded knowledge**: Agents learn dynamically from tool schemas
- ✅ **Provider-appropriate naming**: AWS uses `instanceType`, Azure uses `vmSize`, naturally expressed
- ✅ **Cleaner API**: One clear way to deploy clusters per provider
- ✅ **Reuses existing validation**: Provider-specific tools build config that existing validation checks

**Breaking Changes:**
- ⚠️ **Removes generic tool**: `k0rdent.mgmt.clusterDeployments.deploy` is removed
- **Rationale**: Early development - no production users to break; cleaner API is better
- **Migration**: Any existing integrations update to use provider-specific tools

## Alternatives Considered

### 1. Keep Generic Tool + Add Schema Introspection Tool (Original Proposal)
**Pros:**
- Single deploy tool (code reuse)
- Flexible for any provider

**Cons:**
- ❌ Two-step process (list templates, get schema, deploy)
- ❌ Higher context overhead for agents
- ❌ More complex agent logic
- ❌ Schema retrieval adds latency
- ❌ Agent must parse JSON Schema format

**Decision:** ❌ Optimizes for wrong thing (code reuse over agent UX)

### 2. Embed Schema in Generic Tool Description
**Pros:**
- Single tool
- Schema visible without separate call

**Cons:**
- ❌ Description becomes enormous (all providers)
- ❌ Agent must parse text description
- ❌ Still have opaque `config` object
- ❌ Doesn't leverage MCP's native schema support

**Decision:** ❌ Doesn't use MCP properly

### 3. Dynamic Tool Generation Based on Templates
**Pros:**
- Automatically adapts to new templates
- No manual tool definition

**Cons:**
- ❌ Complex implementation
- ❌ Tool list changes dynamically (confusing for agents)
- ❌ Schema parsing/generation complexity
- ❌ Hard to test and maintain

**Decision:** ❌ Over-engineering; major providers are stable

### 4. One Tool Per Template (ultra-granular)
**Pros:**
- Maximum specificity

**Cons:**
- ❌ Too many tools (16+ templates currently)
- ❌ Version management nightmare
- ❌ Agent must know about template versions

**Decision:** ❌ Too granular; group by provider is right level

### 5. Hybrid: Provider tools + Template parameter
**Pros:**
- Flexibility for version selection
- Still provider-specific

**Cons:**
- ❌ Adds complexity back
- ❌ Agent needs to understand template versioning
- ❌ Auto-selecting latest is simpler

**Decision:** ❌ Keep it simple; auto-select latest template

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Too many tools clutter interface | Low | Low | Only 3-4 provider tools; well-organized by namespace |
| Template selection breaks with new versions | Medium | Low | Document template naming conventions; auto-select latest |
| Schema definitions drift from actual templates | Medium | Medium | Generate schemas from Helm charts (future automation) |
| Generic tool still needed for edge cases | Low | Low | Keep it; clearly document when to use each tool |
| Maintenance burden (multiple tools) | Low | Medium | Share common logic; provider-specific code is minimal |

## Timeline

**Immediate (This Proposal):**
- Day 1: Implement AWS-specific tool with struct-based schema
- Day 1: Implement Azure-specific tool
- Day 1-2: Implement GCP-specific tool
- Day 2: Add template auto-selection logic
- Day 2: Write unit tests for all provider tools
- Day 2-3: Test with live cluster (AWS, Azure)
- Day 3: Update documentation with provider tool examples
- Day 3: Test with AI agent to validate discoverability

**Future (Phase 2):**
- Later: Add vSphere-specific tool
- Later: Add OpenStack-specific tool
- Later: Generate tool schemas automatically from Helm charts
- Later: Add template version selection (if needed)

## Dependencies

- MCP Go SDK struct-to-schema conversion (already available)
- Existing cluster deployment logic (already available)
- Existing validation rules (already available)
- ClusterTemplate listing for auto-selection (already available)

## Open Questions

1. **Should we allow template override in provider tools?**
   - **Proposal**: No for Phase 1; keep simple with auto-selection
   - **Rationale**: Reduces agent complexity; version management is our concern

2. **How do we handle template version updates?**
   - **Proposal**: Auto-select latest stable template matching provider pattern
   - **Logic**: List templates, filter by `{provider}-standalone-cp-*`, sort by version, pick latest
   - **Rationale**: Transparent to agents; they always get latest features

3. **Should we include all Helm chart fields in tool schema?**
   - **Proposal**: No; expose only most common/required fields
   - **Fallback**: Advanced users can use generic tool with full config
   - **Rationale**: Keep provider tools simple and focused on common use cases

4. **What namespace hierarchy should provider tools use?**
   - **Decision**: `k0rdent.provider.{provider_name}.{category}.{action}`
   - **Example**: `k0rdent.provider.aws.clusterDeployments.deploy`
   - **Rationale**:
     - Clear separation: `k0rdent.mgmt.*` for management, `k0rdent.provider.*` for provider-specific
     - Extensible to future provider operations (credentials, identity, etc.)
     - Agents can filter by provider to discover all available operations

5. **Should we remove the generic deploy tool?**
   - **Decision**: Yes, remove it completely
   - **Rationale**: Early development - no backward compatibility needed; cleaner API
   - **Custom templates**: Add provider-specific tools for each supported provider as needed
   - **Future**: If truly custom templates needed, can add `k0rdent.custom.clusterDeployments.deploy` later

## References

- MCP Protocol Specification: Tool schema definitions
- Go SDK struct tags: JSON Schema generation
- Current implementation: `internal/tools/core/clusters.go`
- Helm chart values: Field definitions for each provider
- Related change: `fix-azure-config-validation` (validation rules)
- k0rdent templates: `oci://ghcr.io/k0rdent/kcm/charts`

## Design Principles

This proposal follows these principles:

1. **Optimize for agent discoverability** - Tools are self-describing via MCP schema
2. **Minimize context overhead** - Single-step process, no schema fetching
3. **Use MCP idiomatically** - Leverage protocol's native schema support
4. **Maintain simplicity** - Auto-select templates, expose common fields only
5. **Preserve flexibility** - Keep generic tool for advanced/custom use cases

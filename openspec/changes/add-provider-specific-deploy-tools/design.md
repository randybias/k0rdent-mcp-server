# Design: Provider-Specific Cluster Deployment Tools

## Overview

Add provider-specific MCP tools for cluster deployment that expose configuration parameters directly in tool schemas, enabling AI agents to discover requirements through standard MCP introspection without additional API calls or hard-coded knowledge.

## Architecture

### Component Diagram

```
┌─────────────────┐
│   AI Agent      │
└────────┬────────┘
         │ 1. List tools via MCP
         ▼
┌───────────────────────────────────────────────┐
│  MCP Server Tools List Response              │
│  - k0rdent.provider.aws.clusterDeployments.deploy     │
│  - k0rdent.provider.azure.clusterDeployments.deploy   │
│  - k0rdent.provider.gcp.clusterDeployments.deploy     │
│  - k0rdent.mgmt.clusterDeployments.deploy    │
└────────┬──────────────────────────────────────┘
         │ 2. Agent inspects AWS tool schema
         ▼
┌───────────────────────────────────────────────┐
│  Tool Schema (auto-generated from Go struct)  │
│  {                                            │
│    required: ["region", "controlPlane"...]    │
│    properties: {                              │
│      region: {type: "string", desc: "..."}    │
│      controlPlane: {                          │
│        instanceType: {...}                    │
│      }                                        │
│    }                                          │
│  }                                            │
└────────┬──────────────────────────────────────┘
         │ 3. Agent calls AWS deploy with params
         ▼
┌───────────────────────────────────────────────┐
│  AWS Deploy Tool Handler                     │
│  (internal/tools/core/clusters_aws.go)        │
└────────┬──────────────────────────────────────┘
         │ 4. Build generic config map
         ▼
┌───────────────────────────────────────────────┐
│  Generic Deploy Logic                        │
│  (internal/clusters/deploy.go)               │
│  - Validates config                           │
│  - Creates ClusterDeployment                  │
└───────────────────────────────────────────────┘
```

### Data Flow

1. **Agent discovers tools** - Lists MCP tools, sees provider-specific options
2. **Agent inspects schema** - MCP protocol provides schema automatically
3. **Agent constructs request** - Uses schema to build valid parameters
4. **Tool handler receives** - Structured Go struct with validation
5. **Convert to generic format** - Build config map + auto-select template
6. **Reuse existing logic** - Call existing deploy/validation functions
7. **Return result** - Standard deployment result

## Detailed Design

### 1. Provider-Specific Tool Structs

**AWS Tool (`internal/tools/core/clusters_aws.go`):**
```go
package core

type awsClusterDeployTool struct {
    session *runtime.Session
}

type awsClusterDeployInput struct {
    Name               string                `json:"name" jsonschema:"required,description=Cluster deployment name"`
    Credential         string                `json:"credential" jsonschema:"required,description=AWS credential name"`
    Region             string                `json:"region" jsonschema:"required,description=AWS region (e.g. us-west-2)"`
    ControlPlane       awsNodeConfig         `json:"controlPlane" jsonschema:"required,description=Control plane configuration"`
    Worker             awsNodeConfig         `json:"worker" jsonschema:"required,description=Worker node configuration"`
    ControlPlaneNumber int                   `json:"controlPlaneNumber,omitempty" jsonschema:"description=Number of control plane nodes,default=3,minimum=1"`
    WorkersNumber      int                   `json:"workersNumber,omitempty" jsonschema:"description=Number of worker nodes,default=2,minimum=1"`
    Namespace          string                `json:"namespace,omitempty" jsonschema:"description=Deployment namespace,default=kcm-system"`
    Labels             map[string]string     `json:"labels,omitempty" jsonschema:"description=Labels for the cluster"`
    Wait               bool                  `json:"wait,omitempty" jsonschema:"description=Wait for cluster ready"`
    WaitTimeout        string                `json:"waitTimeout,omitempty" jsonschema:"description=Wait timeout,default=30m"`
}

type awsNodeConfig struct {
    InstanceType   string `json:"instanceType" jsonschema:"required,description=EC2 instance type (e.g. t3.small)"`
    RootVolumeSize int    `json:"rootVolumeSize,omitempty" jsonschema:"description=Root volume size in GB,default=32,minimum=8"`
}

type awsClusterDeployResult clusters.DeployResult
```

**Azure Tool (`internal/tools/core/clusters_azure.go`):**
```go
type azureClusterDeployTool struct {
    session *runtime.Session
}

type azureClusterDeployInput struct {
    Name               string                `json:"name" jsonschema:"required"`
    Credential         string                `json:"credential" jsonschema:"required,description=Azure credential name"`
    Location           string                `json:"location" jsonschema:"required,description=Azure location (e.g. westus2)"`
    SubscriptionID     string                `json:"subscriptionID" jsonschema:"required,description=Azure subscription ID"`
    ControlPlane       azureNodeConfig       `json:"controlPlane" jsonschema:"required"`
    Worker             azureNodeConfig       `json:"worker" jsonschema:"required"`
    ControlPlaneNumber int                   `json:"controlPlaneNumber,omitempty" jsonschema:"default=3,minimum=1"`
    WorkersNumber      int                   `json:"workersNumber,omitempty" jsonschema:"default=2,minimum=1"`
    Namespace          string                `json:"namespace,omitempty" jsonschema:"default=kcm-system"`
    Labels             map[string]string     `json:"labels,omitempty"`
    Wait               bool                  `json:"wait,omitempty"`
    WaitTimeout        string                `json:"waitTimeout,omitempty" jsonschema:"default=30m"`
}

type azureNodeConfig struct {
    VMSize         string `json:"vmSize" jsonschema:"required,description=Azure VM size (e.g. Standard_A4_v2)"`
    RootVolumeSize int    `json:"rootVolumeSize,omitempty" jsonschema:"default=30,minimum=8"`
}
```

**GCP Tool (`internal/tools/core/clusters_gcp.go`):**
```go
type gcpClusterDeployTool struct {
    session *runtime.Session
}

type gcpClusterDeployInput struct {
    Name               string                `json:"name" jsonschema:"required"`
    Credential         string                `json:"credential" jsonschema:"required,description=GCP credential name"`
    Project            string                `json:"project" jsonschema:"required,description=GCP project ID"`
    Region             string                `json:"region" jsonschema:"required,description=GCP region (e.g. us-central1)"`
    Network            gcpNetworkConfig      `json:"network" jsonschema:"required,description=Network configuration"`
    ControlPlane       gcpNodeConfig         `json:"controlPlane" jsonschema:"required"`
    Worker             gcpNodeConfig         `json:"worker" jsonschema:"required"`
    ControlPlaneNumber int                   `json:"controlPlaneNumber,omitempty" jsonschema:"default=3,minimum=1"`
    WorkersNumber      int                   `json:"workersNumber,omitempty" jsonschema:"default=2,minimum=1"`
    Namespace          string                `json:"namespace,omitempty" jsonschema:"default=kcm-system"`
    Labels             map[string]string     `json:"labels,omitempty"`
    Wait               bool                  `json:"wait,omitempty"`
    WaitTimeout        string                `json:"waitTimeout,omitempty" jsonschema:"default=30m"`
}

type gcpNodeConfig struct {
    InstanceType   string `json:"instanceType" jsonschema:"required,description=GCE instance type (e.g. n1-standard-4)"`
    RootVolumeSize int    `json:"rootVolumeSize,omitempty" jsonschema:"default=30,minimum=8"`
}

type gcpNetworkConfig struct {
    Name string `json:"name" jsonschema:"required,description=VPC network name (e.g. default)"`
}
```

### 2. Tool Registration

**In `internal/tools/core/clusters.go`:**
```go
func registerClusters(server *mcp.Server, session *runtime.Session) error {
    // ... existing registrations ...

    // Register AWS deploy tool
    awsDeployTool := &awsClusterDeployTool{session: session}
    mcp.AddTool(server, &mcp.Tool{
        Name:        "k0rdent.provider.aws.clusterDeployments.deploy",
        Description: "Deploy a new AWS Kubernetes cluster. Automatically selects the latest stable AWS template and validates configuration.",
        Meta: mcp.Meta{
            "plane":    "aws",
            "category": "clusterDeployments",
            "action":   "deploy",
            "provider": "aws",
        },
    }, awsDeployTool.deploy)

    // Register Azure deploy tool
    azureDeployTool := &azureClusterDeployTool{session: session}
    mcp.AddTool(server, &mcp.Tool{
        Name:        "k0rdent.provider.azure.clusterDeployments.deploy",
        Description: "Deploy a new Azure Kubernetes cluster. Automatically selects the latest stable Azure template and validates configuration.",
        Meta: mcp.Meta{
            "plane":    "azure",
            "category": "clusterDeployments",
            "action":   "deploy",
            "provider": "azure",
        },
    }, azureDeployTool.deploy)

    // Register GCP deploy tool
    gcpDeployTool := &gcpClusterDeployTool{session: session}
    mcp.AddTool(server, &mcp.Tool{
        Name:        "k0rdent.provider.gcp.clusterDeployments.deploy",
        Description: "Deploy a new GCP Kubernetes cluster. Automatically selects the latest stable GCP template and validates configuration.",
        Meta: mcp.Meta{
            "plane":    "gcp",
            "category": "clusterDeployments",
            "action":   "deploy",
            "provider": "gcp",
        },
    }, gcpDeployTool.deploy)

    // Keep generic tool for backward compatibility
    // (existing k0rdent.mgmt.clusterDeployments.deploy stays)

    return nil
}
```

### 3. Template Auto-Selection

**Shared helper function:**
```go
// internal/clusters/templates.go

func (m *Manager) SelectLatestTemplate(ctx context.Context, provider string, namespace string) (string, error) {
    templates, err := m.ListTemplates(ctx, []string{namespace})
    if err != nil {
        return "", fmt.Errorf("list templates: %w", err)
    }

    // Filter by provider prefix (e.g., "aws-standalone-cp-")
    pattern := fmt.Sprintf("%s-standalone-cp-", provider)
    var matching []ClusterTemplateSummary
    for _, t := range templates {
        if strings.HasPrefix(t.Name, pattern) {
            matching = append(matching, t)
        }
    }

    if len(matching) == 0 {
        return "", fmt.Errorf("no templates found for provider %s", provider)
    }

    // Sort by version (semantic versioning)
    sort.Slice(matching, func(i, j int) bool {
        return compareVersions(matching[i].Version, matching[j].Version) > 0
    })

    // Return latest (first after sort)
    return matching[0].Name, nil
}

func compareVersions(v1, v2 string) int {
    // Simple version comparison: "1.0.14" vs "1.0.15"
    // Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
    parts1 := strings.Split(v1, ".")
    parts2 := strings.Split(v2, ".")

    for i := 0; i < len(parts1) && i < len(parts2); i++ {
        n1, _ := strconv.Atoi(parts1[i])
        n2, _ := strconv.Atoi(parts2[i])
        if n1 != n2 {
            return n1 - n2
        }
    }
    return len(parts1) - len(parts2)
}
```

### 4. Handler Implementation

**AWS Handler Example:**
```go
func (t *awsClusterDeployTool) deploy(ctx context.Context, req *mcp.CallToolRequest, input awsClusterDeployInput) (*mcp.CallToolResult, awsClusterDeployResult, error) {
    logger := logging.WithContext(ctx, t.session.Logger)
    logger.Info("deploying AWS cluster",
        "name", input.Name,
        "region", input.Region,
        "credential", input.Credential,
    )

    // Auto-select latest AWS template
    namespace := input.Namespace
    if namespace == "" {
        namespace = "kcm-system"
    }

    template, err := t.session.Clusters.SelectLatestTemplate(ctx, "aws", namespace)
    if err != nil {
        logger.Error("failed to select AWS template", "error", err)
        return nil, awsClusterDeployResult{}, fmt.Errorf("select template: %w", err)
    }

    logger.Debug("selected AWS template", "template", template)

    // Build config map from structured input
    config := map[string]interface{}{
        "region":             input.Region,
        "controlPlaneNumber": input.ControlPlaneNumber,
        "workersNumber":      input.WorkersNumber,
        "controlPlane": map[string]interface{}{
            "instanceType":   input.ControlPlane.InstanceType,
            "rootVolumeSize": input.ControlPlane.RootVolumeSize,
        },
        "worker": map[string]interface{}{
            "instanceType":   input.Worker.InstanceType,
            "rootVolumeSize": input.Worker.RootVolumeSize,
        },
    }

    // Use defaults if not specified
    if input.ControlPlaneNumber == 0 {
        config["controlPlaneNumber"] = 3
    }
    if input.WorkersNumber == 0 {
        config["workersNumber"] = 2
    }
    if input.ControlPlane.RootVolumeSize == 0 {
        config["controlPlane"].(map[string]interface{})["rootVolumeSize"] = 32
    }
    if input.Worker.RootVolumeSize == 0 {
        config["worker"].(map[string]interface{})["rootVolumeSize"] = 32
    }

    // Create generic deploy request
    deployReq := clusters.DeployRequest{
        Name:        input.Name,
        Template:    template,
        Credential:  input.Credential,
        Namespace:   namespace,
        Labels:      input.Labels,
        Config:      config,
        Wait:        input.Wait,
        WaitTimeout: input.WaitTimeout,
    }

    // Call existing deploy logic (reuses validation!)
    result, err := t.session.Clusters.Deploy(ctx, deployReq)
    if err != nil {
        return nil, awsClusterDeployResult{}, err
    }

    return nil, awsClusterDeployResult(result), nil
}
```

### 5. Error Handling

**Provider tool errors:**
- Template not found → "No AWS templates available in namespace"
- Validation fails → Existing validation errors (from fix-azure-config-validation)
- Deployment fails → Standard deployment errors

**Advantage**: Provider context in error messages is clearer
- "AWS cluster validation failed" vs generic "validation failed"

### 6. Backward Compatibility

**Keep generic deploy tool:**
```go
// Existing tool remains unchanged
k0rdent.mgmt.clusterDeployments.deploy
```

**Use cases for generic tool:**
- Custom templates not matching standard patterns
- Advanced users needing full config flexibility
- Templates for providers without dedicated tools (vSphere, OpenStack initially)
- Backward compatibility for existing integrations

**Documentation will clarify**:
- Use provider tools for common workflows (recommended)
- Use generic tool for custom/advanced scenarios

## Testing Strategy

### Unit Tests

**Per-provider tool tests:**
```go
TestAWSClusterDeploy_ValidInput
TestAWSClusterDeploy_MissingRegion
TestAWSClusterDeploy_TemplateSelection
TestAWSClusterDeploy_DefaultValues
TestAWSClusterDeploy_ValidationIntegration

TestAzureClusterDeploy_ValidInput
TestAzureClusterDeploy_MissingLocation
TestAzureClusterDeploy_MissingSubscriptionID

TestGCPClusterDeploy_ValidInput
TestGCPClusterDeploy_MissingProject
TestGCPClusterDeploy_MissingNetworkName
```

**Template selection tests:**
```go
TestSelectLatestTemplate_AWS
TestSelectLatestTemplate_NoTemplates
TestSelectLatestTemplate_VersionSorting
```

### Integration Tests

```go
TestAWSDeployTool_LiveCluster
TestAzureDeployTool_LiveCluster
TestProviderToolsSchemaExposed  // Verify MCP exposes schemas correctly
```

### AI Agent Testing

Manual validation that AI agents can:
1. List tools and see provider-specific options
2. Inspect tool schema and extract required fields
3. Construct valid deployment requests
4. Successfully deploy clusters without prior knowledge

## Performance Considerations

**No performance impact:**
- Template selection adds ~50ms (one-time query)
- Config building is trivial (map construction)
- Reuses all existing deployment logic
- No additional network calls

**Memory:**
- Three additional tool handlers (negligible)
- No caching needed (stateless conversions)

## Migration Path

**Phase 1: Add Provider Tools**
- ✅ Implement AWS, Azure, GCP tools
- ✅ Keep generic tool unchanged
- ✅ Update documentation

**Phase 2: Adoption**
- Update examples to use provider tools
- Agents naturally discover and adopt new tools
- Generic tool remains for compatibility

**Phase 3: Future Providers**
- Add vSphere tool when needed
- Add OpenStack tool when needed
- Pattern is established and easy to replicate

## Open Implementation Decisions

1. **Should we expose all Helm fields or just common ones?**
   - **Decision**: Start with common required fields only
   - **Rationale**: Simplicity; can add more fields based on feedback
   - **Fallback**: Advanced users can use generic tool

2. **How to handle template version pinning?**
   - **Decision**: Phase 1 auto-selects latest; no pinning
   - **Future**: Add optional `templateVersion` field if needed
   - **Rationale**: YAGNI - most users want latest

3. **Should we validate instance types against cloud APIs?**
   - **Decision**: No - existing validation is sufficient
   - **Rationale**: Cloud APIs change; validation would be brittle
   - **Current**: Validation fails at provision time with cloud error

4. **What about EKS, AKS, GKE (managed services)?**
   - **Decision**: Phase 2 - separate tools or template parameter
   - **Example**: `k0rdent.provider.aws.eksDeployments.deploy`
   - **Rationale**: Keep Phase 1 focused on standalone clusters

## References

- MCP Go SDK: Struct-to-schema generation
- Existing deploy logic: `internal/clusters/deploy.go`
- Validation rules: Added in `fix-azure-config-validation`
- Template patterns: `aws-standalone-cp-*`, `azure-standalone-cp-*`, etc.

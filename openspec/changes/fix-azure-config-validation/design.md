# Design: Multi-Provider Configuration Validation

## Overview

This design adds pre-flight validation for AWS, Azure, and GCP cluster deployment configurations to catch errors before submitting ClusterDeployment resources to Kubernetes. The validation ensures required fields are present for each cloud provider and provides immediate, actionable feedback to users.

**Key architectural decision**: Use a **unified tool** (`k0rdent.mgmt.clusterDeployments.deploy`) with provider-specific validation, matching k0rdent's unified ClusterDeployment API design.

## Architecture

### Component Structure

```
internal/clusters/
├── deploy.go           # Existing: ClusterDeployment creation
├── validation.go       # NEW: Multi-provider validation logic
├── validation_aws.go   # NEW: AWS-specific validation
├── validation_azure.go # NEW: Azure-specific validation
├── validation_gcp.go   # NEW: GCP-specific validation
├── validation_test.go  # NEW: Validation unit tests
└── types.go           # UPDATE: Add validation types
```

### Validation Flow

```
MCP Client
    │
    ├─> k0rdent.mgmt.clusterDeployments.deploy (unified tool)
    │
    └─> DeployCluster(ctx, namespace, req)
         │
         ├─> 1. Validate request fields (existing)
         │
         ├─> 2. Resolve template reference (existing)
         │
         ├─> 3. NEW: Provider-specific validation
         │    │
         │    ├─> Detect provider from template name
         │    │   ├─> aws-*    → AWS validation
         │    │   ├─> azure-*  → Azure validation
         │    │   ├─> gcp-*    → GCP validation
         │    │   └─> other    → No validation
         │    │
         │    ├─> Run provider validator
         │    │   ├─> ValidateAWSConfig(config)
         │    │   ├─> ValidateAzureConfig(config)
         │    │   └─> ValidateGCPConfig(config)
         │    │
         │    └─> Return error if validation fails
         │
         ├─> 4. Resolve credential reference (existing)
         │
         ├─> 5. Build ClusterDeployment (existing)
         │
         └─> 6. Apply to Kubernetes (existing)
```

## Rationale: Unified Tool Architecture

### Why NOT Provider-Specific Tools?

The k0rdent platform uses a **unified ClusterDeployment API** across all providers:

```yaml
apiVersion: k0rdent.mirantis.com/v1beta1
kind: ClusterDeployment  # Same kind for AWS, Azure, GCP, etc.
metadata:
  name: my-cluster
spec:
  template: <provider-template>  # Provider detected from template
  credential: <provider-credential>
  config: <provider-specific-config>
```

**Benefits of unified tool:**
- ✅ Matches k0rdent's architecture (one API, provider abstracted)
- ✅ Users don't need to know provider before using tool
- ✅ Single tool to document, maintain, and test
- ✅ Consistent UX across providers
- ✅ Provider detection happens automatically from template name

**Drawbacks of separate tools:**
- ❌ Would create artificial separation not present in k0rdent
- ❌ Triple the MCP tools (aws, azure, gcp variants)
- ❌ Users must choose correct tool based on provider
- ❌ Duplicated code across tools

**Decision**: Use unified tool with provider-specific validation dispatch.

## Data Structures

### ValidationResult

```go
// ValidationResult holds the outcome of configuration validation
type ValidationResult struct {
    Valid    bool
    Errors   []ValidationError
    Warnings []ValidationError
    Provider string // "aws", "azure", "gcp", or ""
}

// ValidationError describes a configuration validation issue
type ValidationError struct {
    Field   string // JSON path to the field (e.g., "config.subscriptionID")
    Message string // Human-readable error message
    Code    string // Machine-readable error code (e.g., "REQUIRED_FIELD_MISSING")
}
```

### Provider Detection

```go
// ProviderType represents a cloud provider
type ProviderType string

const (
    ProviderAWS     ProviderType = "aws"
    ProviderAzure   ProviderType = "azure"
    ProviderGCP     ProviderType = "gcp"
    ProviderUnknown ProviderType = ""
)

// DetectProvider determines the cloud provider from template name
func DetectProvider(templateName string) ProviderType {
    switch {
    case strings.HasPrefix(templateName, "aws-"):
        return ProviderAWS
    case strings.HasPrefix(templateName, "azure-"):
        return ProviderAzure
    case strings.HasPrefix(templateName, "gcp-"):
        return ProviderGCP
    default:
        return ProviderUnknown
    }
}
```

## Validation Rules by Provider

### AWS Templates (Pattern: `aws-*`)

**Required Fields:**

| Field Path | Type | Validation | Error Code |
|------------|------|------------|------------|
| `config.region` | string | Non-empty string | `REQUIRED_FIELD_MISSING` |

**Field Naming:**
- Control plane/worker use `instanceType`

**Example validation:**
```go
func ValidateAWSConfig(config map[string]interface{}) ValidationResult {
    result := ValidationResult{Valid: true, Provider: "aws"}

    if !hasNonEmptyString(config, "region") {
        result.AddError("config.region", "required field missing (AWS region required)", "REQUIRED_FIELD_MISSING")
    }

    result.Valid = len(result.Errors) == 0
    return result
}
```

### Azure Templates (Pattern: `azure-*`)

**Required Fields:**

| Field Path | Type | Validation | Error Code |
|------------|------|------------|------------|
| `config.location` | string | Non-empty string | `REQUIRED_FIELD_MISSING` |
| `config.subscriptionID` | string | Non-empty string | `REQUIRED_FIELD_MISSING` |

**Field Naming:**
- Control plane/worker use `vmSize`

**Example validation:**
```go
func ValidateAzureConfig(config map[string]interface{}) ValidationResult {
    result := ValidationResult{Valid: true, Provider: "azure"}

    if !hasNonEmptyString(config, "location") {
        result.AddError("config.location", "required field missing (Azure region required)", "REQUIRED_FIELD_MISSING")
    }

    if !hasNonEmptyString(config, "subscriptionID") {
        result.AddError("config.subscriptionID", "required field missing (Azure subscription ID required)", "REQUIRED_FIELD_MISSING")
    }

    result.Valid = len(result.Errors) == 0
    return result
}
```

### GCP Templates (Pattern: `gcp-*`)

**Required Fields:**

| Field Path | Type | Validation | Error Code |
|------------|------|------------|------------|
| `config.project` | string | Non-empty string | `REQUIRED_FIELD_MISSING` |
| `config.region` | string | Non-empty string | `REQUIRED_FIELD_MISSING` |
| `config.network.name` | string | Non-empty string | `REQUIRED_FIELD_MISSING` |

**Field Naming:**
- Control plane/worker use `instanceType`

**Example validation:**
```go
func ValidateGCPConfig(config map[string]interface{}) ValidationResult {
    result := ValidationResult{Valid: true, Provider: "gcp"}

    if !hasNonEmptyString(config, "project") {
        result.AddError("config.project", "required field missing (GCP project name required)", "REQUIRED_FIELD_MISSING")
    }

    if !hasNonEmptyString(config, "region") {
        result.AddError("config.region", "required field missing (GCP region required)", "REQUIRED_FIELD_MISSING")
    }

    if !hasNonEmptyString(config, "network", "name") {
        result.AddError("config.network.name", "required field missing (GCP network name required)", "REQUIRED_FIELD_MISSING")
    }

    result.Valid = len(result.Errors) == 0
    return result
}
```

## Error Formatting

### Multi-Provider Error Messages

Errors are formatted with provider-specific context:

#### AWS Error Example
```
invalid configuration for AWS cluster:
  - config.region: required field missing (AWS region required)

Example valid configuration:
{
  "region": "us-east-2",
  "controlPlane": {"instanceType": "t3.small"},
  "worker": {"instanceType": "t3.small"}
}

See: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/
```

#### Azure Error Example
```
invalid configuration for Azure cluster:
  - config.location: required field missing (Azure region required)
  - config.subscriptionID: required field missing (Azure subscription ID required)

Example valid configuration:
{
  "location": "westus2",
  "subscriptionID": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "controlPlane": {"vmSize": "Standard_A4_v2"},
  "worker": {"vmSize": "Standard_A4_v2"}
}

See: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/
```

#### GCP Error Example
```
invalid configuration for GCP cluster:
  - config.project: required field missing (GCP project name required)
  - config.region: required field missing (GCP region required)
  - config.network.name: required field missing (GCP network name required)

Example valid configuration:
{
  "project": "my-gcp-project",
  "region": "us-central1",
  "network": {"name": "default"},
  "controlPlane": {"instanceType": "n1-standard-4"},
  "worker": {"instanceType": "n1-standard-4"}
}

See: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/
```

## Integration Points

### 1. DeployCluster Function

Add validation call after template resolution:

```go
func (m *Manager) DeployCluster(ctx context.Context, namespace string, req DeployRequest) (DeployResult, error) {
    // ... existing validation ...

    // Resolve template reference
    templateNS, templateName, err := m.ResolveResourceNamespace(ctx, req.Template, namespace)
    if err != nil {
        return DeployResult{}, fmt.Errorf("resolve template namespace: %w", err)
    }

    // NEW: Detect provider and validate configuration
    provider := DetectProvider(templateName)
    if provider != ProviderUnknown {
        result := ValidateConfig(provider, req.Config)
        if !result.Valid {
            logger.Warn("configuration validation failed",
                "provider", provider,
                "errors", len(result.Errors),
            )
            return DeployResult{}, fmt.Errorf("%w: %s",
                ErrInvalidRequest,
                formatValidationErrors(provider, result))
        }
    } else {
        logger.Debug("skipping validation for unknown provider",
            "template", templateName,
        )
    }

    // ... continue with existing flow ...
}
```

### 2. Provider Dispatch

```go
// ValidateConfig dispatches to the appropriate provider validator
func ValidateConfig(provider ProviderType, config map[string]interface{}) ValidationResult {
    switch provider {
    case ProviderAWS:
        return ValidateAWSConfig(config)
    case ProviderAzure:
        return ValidateAzureConfig(config)
    case ProviderGCP:
        return ValidateGCPConfig(config)
    default:
        // No validation for unknown providers
        return ValidationResult{Valid: true}
    }
}
```

## Performance Considerations

### Validation Cost Per Provider

| Provider | Fields Checked | Typical Duration |
|----------|----------------|------------------|
| AWS      | 1              | < 0.5ms          |
| Azure    | 2              | < 0.5ms          |
| GCP      | 3              | < 1ms            |

### Impact on Deploy Operation

- Existing: ~100-200ms (Kubernetes API calls dominate)
- Added: ~0.5-1ms (config validation)
- **Total increase**: < 1%

## Extensibility

### Adding New Providers

To add validation for additional providers (vSphere, OpenStack, etc.):

1. Add provider constant to `ProviderType`
2. Update `DetectProvider()` with new pattern
3. Create `ValidateXXXConfig()` function
4. Add case to `ValidateConfig()` dispatcher
5. Add tests for new provider

Example for vSphere:

```go
const ProviderVSphere ProviderType = "vsphere"

func DetectProvider(templateName string) ProviderType {
    // ... existing cases ...
    case strings.HasPrefix(templateName, "vsphere-"):
        return ProviderVSphere
}

func ValidateVSphereConfig(config map[string]interface{}) ValidationResult {
    result := ValidationResult{Valid: true, Provider: "vsphere"}
    // Add vSphere-specific validation
    return result
}
```

### Future: Schema-Based Validation

```go
// Future enhancement: read validation rules from ClusterTemplate schema
func ValidateAgainstSchema(config map[string]interface{}, schema *ClusterTemplateSchema) ValidationResult {
    // Parse schema and validate dynamically
}
```

## Testing Strategy

### Unit Tests Per Provider

```go
func TestValidateAWSConfig(t *testing.T) {
    tests := []struct {
        name       string
        config     map[string]interface{}
        wantValid  bool
        wantErrors int
    }{
        {
            name: "valid config",
            config: map[string]interface{}{
                "region": "us-east-2",
            },
            wantValid: true,
        },
        {
            name:       "missing region",
            config:     map[string]interface{}{},
            wantValid:  false,
            wantErrors: 1,
        },
    }
    // ... test implementation ...
}

func TestValidateAzureConfig(t *testing.T) { /* ... */ }
func TestValidateGCPConfig(t *testing.T) { /* ... */ }
```

### Integration Tests

Test unified tool with all providers:

```go
func TestDeployClusterMultiProviderValidation(t *testing.T) {
    providers := []struct {
        name       string
        template   string
        config     map[string]interface{}
        expectErr  bool
    }{
        {
            name:     "aws missing region",
            template: "aws-standalone-cp-1-0-16",
            config:   map[string]interface{}{},
            expectErr: true,
        },
        {
            name:     "azure missing subscriptionID",
            template: "azure-standalone-cp-1-0-17",
            config:   map[string]interface{}{"location": "westus2"},
            expectErr: true,
        },
        {
            name:     "gcp missing project",
            template: "gcp-standalone-cp-1-0-15",
            config:   map[string]interface{}{"region": "us-central1"},
            expectErr: true,
        },
    }

    for _, tt := range providers {
        t.Run(tt.name, func(t *testing.T) {
            // Test validation behavior
        })
    }
}
```

### Live Tests

Add provider-specific test cases:

```go
func TestClustersProvisioningAWS(t *testing.T) { /* AWS live test */ }
func TestClustersProvisioningAzure(t *testing.T) { /* Azure live test */ }
func TestClustersProvisioningGCP(t *testing.T) { /* GCP live test */ }
```

## Breaking Changes

### No Backward Compatibility

Per user request, validation is **strictly enforced**:

- Invalid configurations are **rejected immediately**
- No compatibility mode or opt-out flag
- Clear, actionable error messages guide users to fix configs

**Rationale:**
- Invalid configs never successfully provisioned anyway
- Immediate feedback prevents wasted time (5-15 minutes per failed attempt)
- Reduces unnecessary cloud costs from failed provisioning

## Security Considerations

### Input Validation

- Validation prevents some configuration errors
- Does not replace Kubernetes admission control
- No cloud API calls during validation (offline validation)
- Complements existing validation layers

### Information Disclosure

- Validation errors do not expose sensitive information
- Required field names and formats are documented publicly
- No credentials accessed during validation

## Documentation Requirements

### Updates Per Provider

1. **docs/cluster-provisioning.md:**
   - Add "Configuration Validation" section
   - Document required fields for AWS, Azure, GCP
   - Show validation error examples for each provider
   - Update baseline configurations

2. **Provider-specific quickstarts:**
   - Update AWS quickstart with validation notes
   - Update Azure quickstart with validation notes
   - Update GCP quickstart with validation notes

3. **Error handling:**
   - Add multi-provider validation errors to common errors
   - Provide troubleshooting per provider

## References

- [k0rdent Cluster Deployment](https://docs.k0rdent.io/latest/admin/clusters/deploy-cluster/)
- [AWS Quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/)
- [Azure Quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/)
- [GCP Quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/)
- Current implementation: `internal/clusters/deploy.go`
- Live tests: `test/integration/clusters_live_test.go`

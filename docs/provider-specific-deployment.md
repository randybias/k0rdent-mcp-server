# Provider-Specific Deployment Tools

> **Status**: Experimental – Testing varies by provider
> **Last Updated**: 2025-01
> **Prerequisites**: Provider credentials configured in k0rdent cluster

This document explains the provider-specific cluster deployment tools and their design rationale. These tools optimize for AI agent discoverability by exposing provider-specific parameters directly in MCP tool schemas.

**Testing Status:**
- **Azure**: ✅ Tested and working
- **AWS**: ⚠️ Minimally tested
- **GCP**: ❌ Untested, likely broken

## Why Provider-Specific Tools?

### AI Agent Discoverability Challenge

The generic `k0rdent.mgmt.clusterDeployments.deploy` tool accepts a flexible `config` object that can contain any provider-specific fields. While this is powerful and flexible, it presents a discoverability challenge for AI agents:

```json
{
  "name": "k0rdent.mgmt.clusterDeployments.deploy",
  "inputSchema": {
    "config": {
      "type": "object",
      "description": "Cluster configuration (template-specific)"
    }
  }
}
```

An AI agent inspecting this schema learns:
- A `config` object is required
- The structure is "template-specific"
- **But**: What fields are actually needed? What are valid values?

The agent must:
1. Call `k0rdent.mgmt.clusterTemplates.list` to discover templates
2. Parse the `configSchema` field (if present and complete)
3. Guess at reasonable defaults for optional fields
4. Risk deployment failures from missing or invalid fields

### Provider-Specific Solution

Provider-specific tools expose all parameters directly in the MCP tool schema:

```json
{
  "name": "k0rdent.provider.aws.clusterDeployments.deploy",
  "inputSchema": {
    "region": {
      "type": "string",
      "description": "AWS region (e.g. us-west-2, us-east-1, eu-west-1)",
      "required": true
    },
    "controlPlane": {
      "type": "object",
      "properties": {
        "instanceType": {
          "type": "string",
          "description": "EC2 instance type (e.g. t3.small, t3.medium, m5.large)",
          "required": true
        }
      }
    }
  }
}
```

Now the AI agent discovers through standard MCP protocol:
- Exact parameter names (`region`, not `location`)
- Expected value formats with examples
- Which fields are required vs optional
- Default values where applicable
- Provider-specific terminology (AWS: `instanceType`, Azure: `vmSize`)

### Benefits

1. **Zero Documentation Dependency**: Agents learn the API purely from MCP schema introspection
2. **Type Safety**: JSON Schema validation ensures correct parameter types before deployment
3. **Provider Idioms**: Each tool uses native provider terminology (AWS: regions/instanceTypes, Azure: locations/vmSizes)
4. **Better Defaults**: Provider-specific defaults encoded in the tool (e.g., AWS 32GB volumes, Azure 30GB)
5. **Immediate Validation**: Missing required fields fail fast with clear error messages
6. **Enhanced Agent UX**: Agents can generate deployment commands with high confidence

## Available Provider Tools

### AWS: k0rdent.provider.aws.clusterDeployments.deploy

Deploys an AWS-hosted Kubernetes cluster with automatic template selection.

**Complete Tool Schema:**

```json
{
  "name": "k0rdent.provider.aws.clusterDeployments.deploy",
  "description": "Deploy a new AWS Kubernetes cluster. Automatically selects the latest stable AWS template and validates AWS-specific configuration (region, instanceType). Exposes AWS-specific parameters directly in the tool schema for easy agent discovery.",
  "inputSchema": {
    "type": "object",
    "required": ["name", "credential", "region", "controlPlane", "worker"],
    "properties": {
      "name": {
        "type": "string",
        "description": "Cluster deployment name"
      },
      "credential": {
        "type": "string",
        "description": "AWS credential name"
      },
      "region": {
        "type": "string",
        "description": "AWS region (e.g. us-west-2, us-east-1, eu-west-1)"
      },
      "controlPlane": {
        "type": "object",
        "required": ["instanceType"],
        "properties": {
          "instanceType": {
            "type": "string",
            "description": "EC2 instance type (e.g. t3.small, t3.medium, m5.large)"
          },
          "rootVolumeSize": {
            "type": "integer",
            "description": "Root volume size in GB (default: 32)"
          }
        }
      },
      "worker": {
        "type": "object",
        "required": ["instanceType"],
        "properties": {
          "instanceType": {
            "type": "string",
            "description": "EC2 instance type (e.g. t3.small, t3.medium, m5.large)"
          },
          "rootVolumeSize": {
            "type": "integer",
            "description": "Root volume size in GB (default: 32)"
          }
        }
      },
      "controlPlaneNumber": {
        "type": "integer",
        "description": "Number of control plane nodes (default: 3)"
      },
      "workersNumber": {
        "type": "integer",
        "description": "Number of worker nodes (default: 2)"
      },
      "namespace": {
        "type": "string",
        "description": "Deployment namespace (default: kcm-system)"
      },
      "labels": {
        "type": "object",
        "additionalProperties": {
          "type": "string"
        },
        "description": "Labels for the cluster"
      },
      "wait": {
        "type": "boolean",
        "description": "Wait for cluster to be ready before returning"
      },
      "waitTimeout": {
        "type": "string",
        "description": "Maximum time to wait for cluster ready (default: 30m)"
      }
    }
  }
}
```

**Example Usage:**

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.aws.clusterDeployments.deploy",
    "arguments": {
      "name": "dev-cluster-01",
      "credential": "aws-cluster-credential",
      "region": "us-west-2",
      "controlPlane": {
        "instanceType": "t3.medium",
        "rootVolumeSize": 40
      },
      "worker": {
        "instanceType": "t3.large",
        "rootVolumeSize": 50
      },
      "controlPlaneNumber": 3,
      "workersNumber": 5,
      "labels": {
        "environment": "development",
        "team": "platform"
      }
    }
  }
}
```

**Default Values:**
- `controlPlaneNumber`: 3
- `workersNumber`: 2
- `namespace`: "kcm-system" (in DEV_ALLOW_ANY mode)
- `controlPlane.rootVolumeSize`: 32 GB
- `worker.rootVolumeSize`: 32 GB
- `labels`: {} (empty map)
- `wait`: false
- `waitTimeout`: "30m"

### Azure: k0rdent.provider.azure.clusterDeployments.deploy

Deploys an Azure-hosted Kubernetes cluster with automatic template selection.

**Complete Tool Schema:**

```json
{
  "name": "k0rdent.provider.azure.clusterDeployments.deploy",
  "description": "Deploy a new Azure Kubernetes cluster. Automatically selects the latest stable Azure template and validates Azure-specific configuration (location, subscriptionID, vmSize). Exposes Azure-specific parameters directly in the tool schema for easy agent discovery.",
  "inputSchema": {
    "type": "object",
    "required": ["name", "credential", "location", "subscriptionID", "controlPlane", "worker"],
    "properties": {
      "name": {
        "type": "string",
        "description": "Name of the cluster deployment"
      },
      "credential": {
        "type": "string",
        "description": "Azure credential name"
      },
      "location": {
        "type": "string",
        "description": "Azure location (e.g. westus2, eastus, westeurope)"
      },
      "subscriptionID": {
        "type": "string",
        "description": "Azure subscription ID (GUID format)"
      },
      "controlPlane": {
        "type": "object",
        "required": ["vmSize"],
        "properties": {
          "vmSize": {
            "type": "string",
            "description": "Azure VM size (e.g. Standard_A4_v2, Standard_D2s_v3)"
          },
          "rootVolumeSize": {
            "type": "integer",
            "description": "Root volume size in GB (default: 30)"
          }
        }
      },
      "worker": {
        "type": "object",
        "required": ["vmSize"],
        "properties": {
          "vmSize": {
            "type": "string",
            "description": "Azure VM size (e.g. Standard_A4_v2, Standard_D2s_v3)"
          },
          "rootVolumeSize": {
            "type": "integer",
            "description": "Root volume size in GB (default: 30)"
          }
        }
      },
      "controlPlaneNumber": {
        "type": "integer",
        "description": "Number of control plane nodes (default: 3)"
      },
      "workersNumber": {
        "type": "integer",
        "description": "Number of worker nodes (default: 2)"
      },
      "namespace": {
        "type": "string",
        "description": "Target namespace for deployment (default: kcm-system)"
      },
      "labels": {
        "type": "object",
        "additionalProperties": {
          "type": "string"
        },
        "description": "Additional labels to apply to the cluster deployment"
      },
      "wait": {
        "type": "boolean",
        "description": "Wait for cluster to be ready before returning"
      },
      "waitTimeout": {
        "type": "string",
        "description": "Maximum time to wait for provisioning (default: 30m)"
      }
    }
  }
}
```

**Example Usage:**

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.azure.clusterDeployments.deploy",
    "arguments": {
      "name": "prod-cluster-01",
      "credential": "azure-cluster-credential",
      "location": "westus2",
      "subscriptionID": "b90d4372-6e37-4eec-9e5a-fe3932d1a67c",
      "controlPlane": {
        "vmSize": "Standard_D4s_v3",
        "rootVolumeSize": 50
      },
      "worker": {
        "vmSize": "Standard_D8s_v3",
        "rootVolumeSize": 100
      },
      "controlPlaneNumber": 3,
      "workersNumber": 5,
      "labels": {
        "environment": "production",
        "cost-center": "engineering"
      }
    }
  }
}
```

**Default Values:**
- `controlPlaneNumber`: 3
- `workersNumber`: 2
- `namespace`: "kcm-system" (in DEV_ALLOW_ANY mode)
- `controlPlane.rootVolumeSize`: 30 GB
- `worker.rootVolumeSize`: 30 GB
- `labels`: {} (empty map)
- `wait`: false
- `waitTimeout`: "30m"

### GCP: k0rdent.provider.gcp.clusterDeployments.deploy

Deploys a GCP-hosted Kubernetes cluster with automatic template selection.

**Complete Tool Schema:**

```json
{
  "name": "k0rdent.provider.gcp.clusterDeployments.deploy",
  "description": "Deploy a new GCP Kubernetes cluster. Automatically selects the latest stable GCP template and validates GCP-specific configuration (project, region, network.name, instanceType). Exposes GCP-specific parameters directly in the tool schema for easy agent discovery.",
  "inputSchema": {
    "type": "object",
    "required": ["name", "credential", "project", "region", "network", "controlPlane", "worker"],
    "properties": {
      "name": {
        "type": "string",
        "description": "Cluster deployment name"
      },
      "credential": {
        "type": "string",
        "description": "GCP credential name"
      },
      "project": {
        "type": "string",
        "description": "GCP project ID"
      },
      "region": {
        "type": "string",
        "description": "GCP region (e.g. us-central1, us-west1, europe-west1)"
      },
      "network": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {
            "type": "string",
            "description": "VPC network name (e.g. default, custom-vpc)"
          }
        }
      },
      "controlPlane": {
        "type": "object",
        "required": ["instanceType"],
        "properties": {
          "instanceType": {
            "type": "string",
            "description": "GCE instance type (e.g. n1-standard-4, n1-standard-8, n2-standard-4)"
          },
          "rootVolumeSize": {
            "type": "integer",
            "description": "Root volume size in GB (default: 30)"
          }
        }
      },
      "worker": {
        "type": "object",
        "required": ["instanceType"],
        "properties": {
          "instanceType": {
            "type": "string",
            "description": "GCE instance type (e.g. n1-standard-4, n1-standard-8, n2-standard-4)"
          },
          "rootVolumeSize": {
            "type": "integer",
            "description": "Root volume size in GB (default: 30)"
          }
        }
      },
      "controlPlaneNumber": {
        "type": "integer",
        "description": "Number of control plane nodes (default: 3)"
      },
      "workersNumber": {
        "type": "integer",
        "description": "Number of worker nodes (default: 2)"
      },
      "namespace": {
        "type": "string",
        "description": "Deployment namespace (default: kcm-system)"
      },
      "labels": {
        "type": "object",
        "additionalProperties": {
          "type": "string"
        },
        "description": "Labels for the cluster"
      },
      "wait": {
        "type": "boolean",
        "description": "Wait for cluster to be ready before returning"
      },
      "waitTimeout": {
        "type": "string",
        "description": "Maximum time to wait for cluster ready (default: 30m)"
      }
    }
  }
}
```

**Example Usage:**

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.gcp.clusterDeployments.deploy",
    "arguments": {
      "name": "staging-cluster-01",
      "credential": "gcp-credential",
      "project": "my-gcp-project-123456",
      "region": "us-central1",
      "network": {
        "name": "default"
      },
      "controlPlane": {
        "instanceType": "n1-standard-4",
        "rootVolumeSize": 40
      },
      "worker": {
        "instanceType": "n1-standard-8",
        "rootVolumeSize": 80
      },
      "controlPlaneNumber": 3,
      "workersNumber": 4,
      "labels": {
        "environment": "staging",
        "owner": "platform-team"
      }
    }
  }
}
```

**Default Values:**
- `controlPlaneNumber`: 3
- `workersNumber`: 2
- `namespace`: "kcm-system" (in DEV_ALLOW_ANY mode)
- `controlPlane.rootVolumeSize`: 30 GB
- `worker.rootVolumeSize`: 30 GB
- `labels`: {} (empty map)
- `wait`: false
- `waitTimeout`: "30m"

## Template Auto-Selection

All provider-specific tools automatically select the latest stable cluster template for the target provider. This eliminates the need for agents to discover and track template versions.

### Selection Algorithm

The template selection process:

1. **List all templates** in the target namespace (respecting namespace filters)
2. **Filter by provider pattern**:
   - AWS: templates matching `aws-standalone-cp-*`
   - Azure: templates matching `azure-standalone-cp-*`
   - GCP: templates matching `gcp-standalone-cp-*`
3. **Parse semantic versions** from template names (e.g., `azure-standalone-cp-1-0-15` → `1.0.15`)
4. **Sort by version** (descending - highest version first)
5. **Select the latest** (first in sorted list)

### Version Comparison

Template versions are compared using semantic versioning:

```go
// Examples of version comparison:
compareVersions("1.0.15", "1.0.14") // returns 1 (1.0.15 > 1.0.14)
compareVersions("2.0.0", "1.9.9")   // returns 1 (2.0.0 > 1.9.9)
compareVersions("1.0.15", "1.0.15") // returns 0 (equal)
compareVersions("1.0.14", "1.0.15") // returns -1 (1.0.14 < 1.0.15)
```

Version parsing handles:
- Standard semantic versions: `1.0.15`, `2.3.4`
- Missing patch versions: `1.0` → `1.0.0`
- Invalid versions: treated as `0.0.0`

### Template Naming Convention

k0rdent templates follow the pattern:
```
<provider>-standalone-cp-<major>-<minor>-<patch>
```

Examples:
- `aws-standalone-cp-1-0-16` → AWS template v1.0.16
- `azure-standalone-cp-1-0-15` → Azure template v1.0.15
- `gcp-standalone-cp-1-0-15` → GCP template v1.0.15

### Failure Cases

Template selection fails with clear error messages:

**No matching templates:**
```json
{
  "error": {
    "code": -32602,
    "message": "no templates found for provider aws in namespace kcm-system"
  }
}
```

**Solution**: Install the provider's cluster templates from the k0rdent catalog.

## AI Agent Discovery Workflow

This section illustrates how an AI agent uses provider-specific tools through standard MCP protocol interactions.

### Step 1: List Available Tools

Agent requests tool list via MCP:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

Response includes provider-specific tools:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "k0rdent.provider.aws.clusterDeployments.deploy",
        "description": "Deploy a new AWS Kubernetes cluster..."
      },
      {
        "name": "k0rdent.provider.azure.clusterDeployments.deploy",
        "description": "Deploy a new Azure Kubernetes cluster..."
      },
      {
        "name": "k0rdent.provider.gcp.clusterDeployments.deploy",
        "description": "Deploy a new GCP Kubernetes cluster..."
      }
    ]
  }
}
```

Agent learns: Three provider-specific deployment tools are available.

### Step 2: Inspect Tool Schema

Agent requests schema for AWS tool:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/info",
  "params": {
    "name": "k0rdent.provider.aws.clusterDeployments.deploy"
  }
}
```

Response provides complete schema:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "name": "k0rdent.provider.aws.clusterDeployments.deploy",
    "inputSchema": {
      "type": "object",
      "required": ["name", "credential", "region", "controlPlane", "worker"],
      "properties": {
        "region": {
          "type": "string",
          "description": "AWS region (e.g. us-west-2, us-east-1, eu-west-1)"
        },
        "controlPlane": {
          "type": "object",
          "required": ["instanceType"],
          "properties": {
            "instanceType": {
              "type": "string",
              "description": "EC2 instance type (e.g. t3.small, t3.medium, m5.large)"
            }
          }
        }
      }
    }
  }
}
```

Agent learns:
- `region` is required (string) with example values
- `controlPlane.instanceType` is required with AWS-specific terminology
- Optional parameters have defaults documented
- Field descriptions include example values

### Step 3: Discover Available Credentials

Agent lists credentials to find valid `credential` parameter value:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.listCredentials",
    "arguments": {}
  }
}
```

Response shows available credentials:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"credentials\": [{\"name\": \"aws-cluster-credential\", \"namespace\": \"kcm-system\", \"provider\": \"aws\", \"ready\": true}]}"
      }
    ]
  }
}
```

Agent learns: `aws-cluster-credential` is available and ready.

### Step 4: Deploy Cluster

Agent constructs deployment call using discovered information:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.aws.clusterDeployments.deploy",
    "arguments": {
      "name": "agent-cluster-01",
      "credential": "aws-cluster-credential",
      "region": "us-west-2",
      "controlPlane": {
        "instanceType": "t3.medium"
      },
      "worker": {
        "instanceType": "t3.large"
      }
    }
  }
}
```

Response confirms deployment:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"name\": \"agent-cluster-01\", \"namespace\": \"kcm-system\", \"status\": \"created\"}"
      }
    ]
  }
}
```

Agent learns: Cluster deployment initiated successfully.

### Key Observations

1. **Zero External Documentation**: Agent completes entire workflow using only MCP protocol
2. **Schema-Driven Discovery**: Tool schemas provide all necessary guidance
3. **Type-Safe Construction**: Agent constructs valid requests from schema alone
4. **Immediate Feedback**: Missing or invalid parameters fail at validation stage
5. **Provider Idioms**: Agent learns provider-specific terminology naturally from schemas

## Decision Guide

When should you use provider-specific tools vs the generic deployment tool?

### Use Provider-Specific Tools When:

1. **AI Agent Integration**: Building AI agents that discover and use the API automatically
2. **Interactive Applications**: Building UIs/CLIs where parameter discovery matters
3. **Developer Experience**: Optimizing for developers who want clear, discoverable APIs
4. **Type Safety**: Need JSON Schema validation of provider-specific parameters
5. **Consistent Defaults**: Want provider-appropriate defaults (e.g., AWS 32GB volumes, Azure 30GB)
6. **Simpler Workflows**: Deploying clusters without needing to discover/track templates

**Example Scenario**: An AI agent helping users deploy clusters should use provider-specific tools because it can discover all parameters through MCP schema introspection alone.

### Use Generic Tool When:

1. **Template Control**: Need to specify exact template version or custom templates
2. **Multi-Provider Abstraction**: Building higher-level abstractions over multiple providers
3. **Custom Templates**: Using organization-specific or modified templates
4. **Advanced Configurations**: Need to pass provider-specific config not exposed in schema
5. **Programmatic Access**: Have predetermined deployment configurations (IaC, GitOps)

**Example Scenario**: A GitOps system deploying clusters from version-controlled manifests should use the generic tool for explicit template version control.

### Comparison Table

| Aspect | Provider-Specific Tools | Generic Tool |
|--------|------------------------|-------------|
| **Discoverability** | Excellent (schema-driven) | Manual (requires template lookup) |
| **Template Selection** | Automatic (latest stable) | Explicit (full control) |
| **Parameter Validation** | Strong (JSON Schema) | Basic (Kubernetes validation) |
| **Provider Idioms** | Native (instanceType/vmSize) | Generic (config object) |
| **Default Values** | Provider-specific | User-defined |
| **AI Agent Friendly** | Yes (zero documentation) | Moderate (needs template docs) |
| **Version Control** | Automatic (tracks latest) | Explicit (pinned versions) |
| **Custom Templates** | Not supported | Fully supported |
| **Flexibility** | High (common use cases) | Maximum (any configuration) |

### Recommended Approach

For most use cases, prefer provider-specific tools:

```json
{
  "name": "k0rdent.provider.aws.clusterDeployments.deploy",
  "arguments": {
    "name": "my-cluster",
    "credential": "aws-credential",
    "region": "us-west-2",
    "controlPlane": {
      "instanceType": "t3.medium"
    },
    "worker": {
      "instanceType": "t3.large"
    }
  }
}
```

Only use the generic tool when you need explicit template control:

```json
{
  "name": "k0rdent.mgmt.clusterDeployments.deploy",
  "arguments": {
    "name": "my-cluster",
    "template": "aws-standalone-cp-1-0-14",
    "credential": "aws-credential",
    "config": {
      "region": "us-west-2",
      "controlPlane": {
        "instanceType": "t3.medium"
      },
      "worker": {
        "instanceType": "t3.large"
      }
    }
  }
}
```

## Parameters Reference

This section documents common parameters across all provider-specific tools.

### Common Required Parameters

All provider-specific tools require these parameters:

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `name` | string | Unique name for the ClusterDeployment | `"production-01"` |
| `credential` | string | Name of the Credential resource | `"aws-cluster-credential"` |
| `controlPlane` | object | Control plane node configuration | See provider sections |
| `worker` | object | Worker node configuration | See provider sections |

### Common Optional Parameters

All provider-specific tools support these optional parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `controlPlaneNumber` | integer | 3 | Number of control plane nodes |
| `workersNumber` | integer | 2 | Number of worker nodes |
| `namespace` | string | "kcm-system" | Deployment namespace (must match filter) |
| `labels` | object | `{}` | Additional labels for the deployment |
| `wait` | boolean | `false` | Wait for cluster to be ready before returning |
| `waitTimeout` | string | `"30m"` | Maximum wait time (e.g., "30m", "1h") |

### Labels Parameter

The `labels` parameter accepts a map of string key-value pairs:

```json
{
  "labels": {
    "environment": "production",
    "team": "platform",
    "cost-center": "engineering",
    "project": "k8s-migration"
  }
}
```

**Important Notes:**

- **Optional**: The `labels` parameter is completely optional
- **Default**: If omitted, defaults to an empty map `{}`
- **Merging**: User labels are merged with system-added labels
- **System Labels**: The tool automatically adds `k0rdent.mirantis.com/managed=true`
- **Validation**: Label keys and values must follow [Kubernetes label requirements](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set)

**Label Validation Rules:**

- Keys: max 63 characters (prefix optional, up to 253 chars)
- Values: max 63 characters
- Characters: alphanumeric, `-`, `_`, `.`
- Must start and end with alphanumeric character

**Example Without Labels:**

```json
{
  "name": "simple-cluster",
  "credential": "aws-credential",
  "region": "us-west-2",
  "controlPlane": {
    "instanceType": "t3.medium"
  },
  "worker": {
    "instanceType": "t3.large"
  }
}
```

Result: ClusterDeployment has only system-added label `k0rdent.mirantis.com/managed=true`.

### Wait Behavior

When `wait: true`:

1. Tool initiates deployment
2. Polls ClusterDeployment status every 30 seconds
3. Monitors for:
   - `Ready` condition becoming `True`
   - Stalled progress (10-minute threshold)
   - Timeout expiration
4. Returns when cluster is Ready or error occurs

**Wait Timeout Format:**

Valid duration strings:
- `"30m"` - 30 minutes
- `"1h"` - 1 hour
- `"45m"` - 45 minutes
- `"2h30m"` - 2 hours 30 minutes

**Example With Wait:**

```json
{
  "name": "urgent-cluster",
  "credential": "azure-credential",
  "location": "westus2",
  "subscriptionID": "...",
  "controlPlane": {
    "vmSize": "Standard_D4s_v3"
  },
  "worker": {
    "vmSize": "Standard_D8s_v3"
  },
  "wait": true,
  "waitTimeout": "45m"
}
```

The tool will block for up to 45 minutes until the cluster is ready.

### Namespace Resolution

The `namespace` parameter determines where the ClusterDeployment resource is created:

**Dev Mode (AUTH_MODE=DEV_ALLOW_ANY):**
- Omit `namespace`: Defaults to `"kcm-system"`
- Specify `namespace`: Uses specified value (must exist and match filter)

**Production Mode (AUTH_MODE=OIDC_REQUIRED):**
- Omit `namespace`: Uses first namespace matching the namespace filter
- Specify `namespace`: Uses specified value (must match filter)

**Example (Explicit Namespace):**

```json
{
  "name": "team-cluster",
  "credential": "team-a/gcp-credential",
  "namespace": "team-a",
  "project": "team-a-project",
  "region": "us-central1",
  "network": {
    "name": "team-a-vpc"
  },
  "controlPlane": {
    "instanceType": "n1-standard-4"
  },
  "worker": {
    "instanceType": "n1-standard-4"
  }
}
```

Creates ClusterDeployment in `team-a` namespace.

## Related Documentation

- [Cluster Provisioning Tools](./cluster-provisioning.md) - Generic deployment tool and complete cluster lifecycle
- [Live Integration Tests](./live-tests.md) - Testing provider-specific deployments
- [k0rdent Documentation](https://docs.k0rdent.io) - Platform documentation and cluster templates

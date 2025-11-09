# k0rdent Cluster Provisioning Tools

> **Status**: Experimental – Azure tested, AWS minimal, GCP untested
> **Last Updated**: 2025-01
> **Prerequisites**: Admin kubeconfig to k0rdent management cluster

The MCP server provides tools for provisioning and managing k0rdent child clusters programmatically. These tools enable end-to-end cluster lifecycle management from discovering credentials and templates through deployment and deletion.

**Testing Status by Provider:**
- **Azure**: ✅ Tested and working (requires subscription ID)
- **AWS**: ⚠️ Minimally tested, may have issues
- **GCP**: ❌ Untested, likely broken

## Overview

The cluster provisioning tools allow you to:
- **List accessible credentials** - Discover `Credential` resources available for cluster provisioning
- **List cluster templates** - Browse available `ClusterTemplate` resources (global and namespace-local)
- **Deploy clusters** - Create new `ClusterDeployment` resources to provision child clusters
- **Delete clusters** - Remove `ClusterDeployment` resources with proper cleanup
- **Monitor cluster status** - Track deployment progress through existing list tools

All operations respect namespace filters and authentication modes, ensuring secure multi-tenant workflows.

## Architecture

The cluster provisioning system follows the established MCP server patterns:

1. **Cluster Manager**: Core package (`internal/clusters`) that wraps the dynamic Kubernetes client
2. **Namespace Resolution**: Enforces namespace filters and handles dev vs production mode defaults
3. **Server-Side Apply**: Uses managed field owner `mcp.clusters` for tracking resource ownership
4. **Validation**: Checks credential and template existence before deployment
5. **Metrics & Logging**: Structured observability for all operations

## Prerequisites

### Kubernetes Access

- Management cluster access with appropriate RBAC permissions
- Permissions to list and create `Credential`, `ClusterTemplate`, and `ClusterDeployment` resources
- If using namespace filtering, target namespaces must match the configured filter

### Required Resources

Before deploying clusters, ensure:
- At least one `Credential` resource exists (typically in `kcm-system` for shared credentials)
- Appropriate `ClusterTemplate` resources are available (global in `kcm-system` or namespace-local)
- Cloud provider credentials are properly configured and ready

### Cloud Provider Setup

For Azure deployments:
- `ClusterIdentity` resource configured with service principal credentials
- Subscription ID and location configured
- Resource group permissions
- Network configuration (VNet, subnets) if using existing infrastructure

For AWS deployments:
- IAM credentials configured in `Credential` resource
- VPC and subnet configuration
- Appropriate security groups and IAM roles

## Available Tools

This section covers the core cluster provisioning tools. For AWS, Azure, and GCP deployments, consider using the provider-specific deployment tools documented in the [Provider-Specific Deployment Tools](#provider-specific-deployment-tools) section for simplified configuration and automatic template selection.

### k0rdent.mgmt.providers.list

Returns the cloud providers supported by k0rdent credential onboarding (currently AWS, Azure, Google Cloud, and VMware vSphere). Use this tool to discover the `provider` value expected by `k0rdent.mgmt.providers.listCredentials`.

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.list",
    "arguments": {}
  }
}
```

### k0rdent.mgmt.providers.listCredentials

Lists accessible `Credential` resources for cluster provisioning.

**Parameters:**

| Parameter | Type   | Required | Description                                    |
|-----------|--------|----------|------------------------------------------------|
| namespace | string | No       | Filter to specific namespace (must match filter) |
| scope     | string | No       | "global", "local", or "all" (default: "all")   |

**Returns:**

```json
{
  "credentials": [
    {
      "name": "azure-cluster-credential",
      "namespace": "kcm-system",
      "provider": "azure",
      "labels": {
        "cloud.k0rdent.mirantis.com/provider": "azure"
      },
      "createdAt": "2025-11-01T10:30:00Z",
      "ready": true
    },
    {
      "name": "aws-cluster-credential",
      "namespace": "kcm-system",
      "provider": "aws",
      "labels": {
        "cloud.k0rdent.mirantis.com/provider": "aws"
      },
      "createdAt": "2025-11-01T10:35:00Z",
      "ready": true
    }
  ]
}
```

**Example MCP Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.listCredentials",
    "arguments": {}
  }
}
```

**Example with Namespace Filter:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.listCredentials",
    "arguments": {
      "namespace": "team-a"
    }
  }
}
```

**Example with Scope:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.listCredentials",
    "arguments": {
      "scope": "global"
    }
  }
}
```

**Credential Readiness:**

The `ready` field is derived from the credential's `status.conditions`:
- `true` - Credential has been validated and is ready for use
- `false` - Credential validation pending or failed

### k0rdent.mgmt.clusterTemplates.list

Lists available `ClusterTemplate` resources for cluster provisioning.

**Parameters:**

| Parameter | Type   | Required | Description                                    |
|-----------|--------|----------|------------------------------------------------|
| scope     | string | No       | "global", "local", or "all" (default: "all")   |
| namespace | string | No       | Filter to specific namespace (must match filter) |

**Returns:**

```json
{
  "templates": [
    {
      "name": "azure-standalone-cp-1-0-15",
      "namespace": "kcm-system",
      "description": "Azure standalone cluster with control plane",
      "cloud": "azure",
      "version": "1.0.15",
      "labels": {
        "cloud.k0rdent.mirantis.com/provider": "azure",
        "cluster.k0rdent.mirantis.com/type": "standalone"
      },
      "configSchema": {
        "required": ["location", "subscriptionID", "clusterIdentity"],
        "properties": {
          "location": "string",
          "subscriptionID": "string",
          "clusterIdentity": "object",
          "controlPlane": "object",
          "worker": "object",
          "controlPlaneNumber": "integer",
          "workersNumber": "integer"
        }
      }
    }
  ]
}
```

**Example MCP Request (All Templates):**

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterTemplates.list",
    "arguments": {}
  }
}
```

**Example (Global Templates Only):**

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterTemplates.list",
    "arguments": {
      "scope": "global"
    }
  }
}
```

**Example (Local Templates):**

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterTemplates.list",
    "arguments": {
      "scope": "local",
      "namespace": "team-a"
    }
  }
}
```

**Config Schema:**

Templates include a `configSchema` outline derived from the template's `spec.schema` (if present). This helps guide configuration:
- `required` - List of required fields
- `properties` - Top-level configuration fields with types

### k0rdent.mgmt.clusterDeployments.deploy

Creates or updates a `ClusterDeployment` resource to provision a child cluster.

**Parameters:**

| Parameter  | Type   | Required | Description                                    |
|------------|--------|----------|------------------------------------------------|
| name       | string | Yes      | Name for the ClusterDeployment                 |
| template   | string | Yes      | ClusterTemplate name (or namespace/name)       |
| credential | string | Yes      | Credential name (or namespace/name)            |
| namespace  | string | No       | Target namespace (defaults per auth mode)      |
| labels     | object | No       | Additional labels for the deployment           |
| config     | object | Yes      | Cluster configuration (template-specific)      |

**Namespace Resolution:**

- **Dev mode** (`AUTH_MODE=DEV_ALLOW_ANY`): Defaults to `kcm-system` if namespace not specified
- **Production mode** (`AUTH_MODE=OIDC_REQUIRED`): Uses first namespace matching the filter; returns `forbidden` if none match
- **Explicit namespace**: Must pass namespace filter validation

**Returns:**

```json
{
  "name": "my-test-cluster",
  "namespace": "kcm-system",
  "uid": "a1b2c3d4-e5f6-4789-a0b1-c2d3e4f56789",
  "resourceVersion": "12345",
  "status": "created"
}
```

**Status values:**
- `"created"` - New ClusterDeployment was created
- `"updated"` - Existing ClusterDeployment was updated (idempotent)

**Example MCP Request (Azure):**

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.deploy",
    "arguments": {
      "name": "my-azure-cluster",
      "template": "azure-standalone-cp-1-0-15",
      "credential": "azure-cluster-credential",
      "labels": {
        "environment": "test",
        "team": "platform"
      },
      "config": {
        "clusterIdentity": {
          "name": "azure-cluster-identity",
          "namespace": "kcm-system"
        },
        "location": "westus2",
        "subscriptionID": "b90d4372-6e37-4eec-9e5a-fe3932d1a67c",
        "controlPlane": {
          "vmSize": "Standard_A4_v2",
          "rootVolumeSize": 32
        },
        "controlPlaneNumber": 1,
        "worker": {
          "vmSize": "Standard_A4_v2",
          "rootVolumeSize": 32
        },
        "workersNumber": 1
      }
    }
  }
}
```

**Example (AWS):**

```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.deploy",
    "arguments": {
      "name": "my-aws-cluster",
      "template": "aws-standalone-cp-0-0-3",
      "credential": "aws-cluster-identity",
      "namespace": "kcm-system",
      "config": {
        "clusterIdentity": {
          "name": "aws-cluster-identity",
          "namespace": "kcm-system"
        },
        "region": "us-west-2",
        "controlPlane": {
          "instanceType": "t3.medium",
          "rootVolumeSize": 30
        },
        "controlPlaneNumber": 3,
        "worker": {
          "instanceType": "t3.large",
          "rootVolumeSize": 50
        },
        "workersNumber": 2,
        "publicIP": true
      }
    }
  }
}
```

**Example (Explicit Namespace):**

```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.deploy",
    "arguments": {
      "name": "team-cluster",
      "template": "team-a/custom-template",
      "credential": "team-a/aws-credential",
      "namespace": "team-a",
      "config": {
        "region": "eu-west-1",
        "controlPlaneNumber": 1,
        "workersNumber": 3
      }
    }
  }
}
```

**Important Notes:**

- **Idempotent**: Safe to call multiple times; uses server-side apply
- **Managed Label**: Adds `k0rdent.mirantis.com/managed=true` for tracking
- **Field Owner**: Uses `mcp.clusters` as the field manager
- **Validation**: Verifies template and credential exist before applying
- **Config Flexibility**: Accepts any valid config structure for the template

### Provider-Specific Deployment Tools

The MCP server provides streamlined provider-specific deployment tools that automatically select the latest stable template for each cloud provider and expose provider-specific parameters directly in the tool schema. These tools are optimized for AI agent discovery and reduce configuration complexity compared to the generic deployment tool.

#### When to Use Provider-Specific vs Generic Tools

**Use Provider-Specific Tools When:**
- You know the target cloud provider (AWS, Azure, or GCP)
- You want automatic selection of the latest stable template
- You want provider-specific parameter validation and guidance
- You're building AI agents that need discoverable parameters

**Use Generic Tool When:**
- You need to specify an exact template version
- You're using a custom or local template
- You're working with providers other than AWS, Azure, or GCP
- You need maximum flexibility in template selection

#### k0rdent.provider.aws.clusterDeployments.deploy

Deploys an AWS Kubernetes cluster with automatic template selection.

**Key Features:**
- Automatically selects the latest stable AWS template
- AWS-specific parameter validation (region, instanceType)
- Direct exposure of AWS parameters in tool schema

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Cluster deployment name |
| credential | string | Yes | AWS credential name |
| region | string | Yes | AWS region (e.g., us-west-2, us-east-1) |
| namespace | string | No | Target namespace (defaults per auth mode) |
| labels | object | No | Additional labels (defaults to {}) |
| controlPlane | object | Yes | Control plane configuration |
| controlPlane.instanceType | string | Yes | EC2 instance type (e.g., t3.medium, m5.large) |
| controlPlane.rootVolumeSize | integer | No | Root volume size in GB (default: 32) |
| controlPlaneNumber | integer | No | Number of control plane nodes (default: 3) |
| worker | object | Yes | Worker node configuration |
| worker.instanceType | string | Yes | EC2 instance type (e.g., t3.large) |
| worker.rootVolumeSize | integer | No | Root volume size in GB (default: 32) |
| workersNumber | integer | No | Number of worker nodes (default: 2) |
| wait | boolean | No | Wait for cluster ready before returning |
| waitTimeout | string | No | Max wait time (default: 30m) |

**Example:**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.aws.clusterDeployments.deploy",
    "arguments": {
      "name": "my-aws-cluster",
      "credential": "aws-cluster-credential",
      "region": "us-west-2",
      "controlPlane": {
        "instanceType": "t3.medium",
        "rootVolumeSize": 50
      },
      "controlPlaneNumber": 3,
      "worker": {
        "instanceType": "t3.large",
        "rootVolumeSize": 100
      },
      "workersNumber": 5
    }
  }
}
```

**Example with Labels:**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.aws.clusterDeployments.deploy",
    "arguments": {
      "name": "my-aws-cluster",
      "credential": "aws-cluster-credential",
      "region": "us-west-2",
      "labels": {
        "environment": "production",
        "team": "platform"
      },
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

**Template Auto-Selection:**

The tool automatically selects the latest stable AWS template (pattern: `aws-standalone-cp-*`). This ensures you get the most recent version without manually tracking template versions.

#### k0rdent.provider.azure.clusterDeployments.deploy

Deploys an Azure Kubernetes cluster with automatic template selection.

**Key Features:**
- Automatically selects the latest stable Azure template
- Azure-specific parameter validation (location, subscriptionID, vmSize)
- Direct exposure of Azure parameters in tool schema

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Cluster deployment name |
| credential | string | Yes | Azure credential name |
| location | string | Yes | Azure location (e.g., westus2, eastus) |
| subscriptionID | string | Yes | Azure subscription ID (GUID) |
| namespace | string | No | Target namespace (defaults per auth mode) |
| labels | object | No | Additional labels (defaults to {}) |
| controlPlane | object | Yes | Control plane configuration |
| controlPlane.vmSize | string | Yes | Azure VM size (e.g., Standard_A4_v2, Standard_D2s_v3) |
| controlPlane.rootVolumeSize | integer | No | Root volume size in GB (default: 30) |
| controlPlaneNumber | integer | No | Number of control plane nodes (default: 3) |
| worker | object | Yes | Worker node configuration |
| worker.vmSize | string | Yes | Azure VM size (e.g., Standard_A4_v2) |
| worker.rootVolumeSize | integer | No | Root volume size in GB (default: 30) |
| workersNumber | integer | No | Number of worker nodes (default: 2) |
| wait | boolean | No | Wait for cluster ready before returning |
| waitTimeout | string | No | Max wait time (default: 30m) |

**Example:**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.azure.clusterDeployments.deploy",
    "arguments": {
      "name": "my-azure-cluster",
      "credential": "azure-cluster-credential",
      "location": "westus2",
      "subscriptionID": "b90d4372-6e37-4eec-9e5a-fe3932d1a67c",
      "controlPlane": {
        "vmSize": "Standard_D4s_v3",
        "rootVolumeSize": 50
      },
      "controlPlaneNumber": 3,
      "worker": {
        "vmSize": "Standard_D8s_v3",
        "rootVolumeSize": 100
      },
      "workersNumber": 5
    }
  }
}
```

**Example with Labels:**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.azure.clusterDeployments.deploy",
    "arguments": {
      "name": "my-azure-cluster",
      "credential": "azure-cluster-credential",
      "location": "westus2",
      "subscriptionID": "b90d4372-6e37-4eec-9e5a-fe3932d1a67c",
      "labels": {
        "environment": "production",
        "team": "platform"
      },
      "controlPlane": {
        "vmSize": "Standard_A4_v2"
      },
      "worker": {
        "vmSize": "Standard_A4_v2"
      }
    }
  }
}
```

**Template Auto-Selection:**

The tool automatically selects the latest stable Azure template (pattern: `azure-standalone-cp-*`). This ensures you get the most recent version without manually tracking template versions.

#### k0rdent.provider.gcp.clusterDeployments.deploy

Deploys a GCP Kubernetes cluster with automatic template selection.

**Key Features:**
- Automatically selects the latest stable GCP template
- GCP-specific parameter validation (project, region, network.name, instanceType)
- Direct exposure of GCP parameters in tool schema

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| name | string | Yes | Cluster deployment name |
| credential | string | Yes | GCP credential name |
| project | string | Yes | GCP project ID |
| region | string | Yes | GCP region (e.g., us-central1, us-west1) |
| network | object | Yes | VPC network configuration |
| network.name | string | Yes | VPC network name (e.g., default) |
| namespace | string | No | Target namespace (defaults per auth mode) |
| labels | object | No | Additional labels (defaults to {}) |
| controlPlane | object | Yes | Control plane configuration |
| controlPlane.instanceType | string | Yes | GCE instance type (e.g., n1-standard-4, n2-standard-4) |
| controlPlane.rootVolumeSize | integer | No | Root volume size in GB (default: 30) |
| controlPlaneNumber | integer | No | Number of control plane nodes (default: 3) |
| worker | object | Yes | Worker node configuration |
| worker.instanceType | string | Yes | GCE instance type (e.g., n1-standard-4) |
| worker.rootVolumeSize | integer | No | Root volume size in GB (default: 30) |
| workersNumber | integer | No | Number of worker nodes (default: 2) |
| wait | boolean | No | Wait for cluster ready before returning |
| waitTimeout | string | No | Max wait time (default: 30m) |

**Example:**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.gcp.clusterDeployments.deploy",
    "arguments": {
      "name": "my-gcp-cluster",
      "credential": "gcp-credential",
      "project": "my-gcp-project-123456",
      "region": "us-central1",
      "network": {
        "name": "default"
      },
      "controlPlane": {
        "instanceType": "n1-standard-4",
        "rootVolumeSize": 50
      },
      "controlPlaneNumber": 3,
      "worker": {
        "instanceType": "n1-standard-8",
        "rootVolumeSize": 100
      },
      "workersNumber": 5
    }
  }
}
```

**Example with Labels:**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.gcp.clusterDeployments.deploy",
    "arguments": {
      "name": "my-gcp-cluster",
      "credential": "gcp-credential",
      "project": "my-gcp-project-123456",
      "region": "us-central1",
      "network": {
        "name": "default"
      },
      "labels": {
        "environment": "production",
        "team": "platform"
      },
      "controlPlane": {
        "instanceType": "n1-standard-4"
      },
      "worker": {
        "instanceType": "n1-standard-4"
      }
    }
  }
}
```

**Template Auto-Selection:**

The tool automatically selects the latest stable GCP template (pattern: `gcp-standalone-cp-*`). This ensures you get the most recent version without manually tracking template versions.

#### Provider Tool Benefits for AI Agents

These provider-specific tools are designed for optimal AI agent discoverability:

1. **Explicit Parameters**: All provider-specific parameters appear directly in the tool schema, making them visible during tool introspection
2. **Built-in Validation**: Parameter types and requirements are enforced at the tool level
3. **Automatic Template Selection**: No need to track template versions or query template lists
4. **Consistent Patterns**: All three tools follow the same structural pattern, making them easy to learn
5. **Optional Labels**: The `labels` parameter is optional and defaults to an empty object `{}`, simplifying basic deployments

**AI Agent Usage Pattern:**

```python
# Agent discovers tools via MCP introspection
tools = mcp_client.list_tools()
aws_deploy_tool = next(t for t in tools if t.name == "k0rdent.provider.aws.clusterDeployments.deploy")

# Agent sees all parameters in tool schema
print(aws_deploy_tool.parameters)
# Shows: name, credential, region, controlPlane, worker, labels, etc.

# Agent constructs call with required parameters
result = mcp_client.call_tool("k0rdent.provider.aws.clusterDeployments.deploy", {
    "name": "ai-managed-cluster",
    "credential": "aws-cluster-credential",
    "region": "us-west-2",
    "controlPlane": {"instanceType": "t3.medium"},
    "worker": {"instanceType": "t3.large"}
    # labels omitted - defaults to {}
})
```

### k0rdent.mgmt.clusterDeployments.delete

Deletes a `ClusterDeployment` resource to deprovision a child cluster.

**Parameters:**

| Parameter | Type   | Required | Description                                    |
|-----------|--------|----------|------------------------------------------------|
| name      | string | Yes      | Name of the ClusterDeployment to delete        |
| namespace | string | No       | Target namespace (defaults per auth mode)      |

**Namespace Resolution:**

Uses the same rules as `deploy`:
- Dev mode: Defaults to `kcm-system`
- Production mode: Uses first namespace matching filter
- Explicit namespace: Must pass filter validation

**Returns:**

```json
{
  "name": "my-test-cluster",
  "namespace": "kcm-system",
  "status": "deleted"
}
```

**Status values:**
- `"deleted"` - ClusterDeployment was successfully deleted
- `"not_found"` - Resource did not exist (idempotent)

**Example MCP Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.delete",
    "arguments": {
      "name": "my-test-cluster"
    }
  }
}
```

**Example (Explicit Namespace):**

```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.delete",
    "arguments": {
      "name": "team-cluster",
      "namespace": "team-a"
    }
  }
}
```

**Deletion Behavior:**

- **Foreground Propagation**: Uses `DeletePropagationForeground` to ensure finalizers execute
- **Finalizer Handling**: k0rdent's finalizers trigger cleanup of:
  - Cloud provider resources (VMs, disks, networks)
  - CAPI resources (Machine, MachineDeployment, etc.)
  - Child cluster resources
- **Idempotent**: Safe to call multiple times; returns success if already deleted
- **Logging**: Records deletion attempts at INFO level

### k0rdent.mgmt.clusterDeployments.services.apply

Attaches or updates a `spec.serviceSpec.services[]` entry on an existing ClusterDeployment using an installed ServiceTemplate. The tool mirrors the manual workflow documented in [Adding a Service to a ClusterDeployment](https://github.com/k0rdent/docs/blob/main/docs/user/services/add-service-to-clusterdeployment.md) and immediately returns the `.status.services[]` snapshot described in [Checking status](https://github.com/k0rdent/docs/blob/main/docs/user/services/checking-status.md).

**Parameters:**

| Parameter          | Type    | Required | Description |
|--------------------|---------|----------|-------------|
| `clusterNamespace` | string  | Yes      | Namespace containing the ClusterDeployment (must satisfy the session namespace filter) |
| `clusterName`      | string  | Yes      | Name of the ClusterDeployment to mutate |
| `templateNamespace`| string  | Yes      | Namespace of the installed ServiceTemplate (must satisfy the session namespace filter) |
| `templateName`     | string  | Yes      | Name of the ServiceTemplate to reference |
| `serviceName`      | string  | No       | Logical service name (defaults to `templateName` when omitted) |
| `serviceNamespace` | string  | No       | Namespace where the service runs (defaults to `clusterNamespace`) |
| `values`           | object  | No       | Inline Helm values override for the service |
| `valuesFrom`       | array   | No       | List of `{kind: ConfigMap|Secret, name, key, optional}` sources to merge into Helm values |
| `helmOptions`      | object  | No       | Helm execution tweaks (`timeout`, `atomic`, `wait`, `cleanupOnFail`, `disableHooks`, `replace`, `skipCRDs`, `maxHistory`) |
| `dependsOn`        | array   | No       | Service names that **must already exist** in the ClusterDeployment spec before this service reconciles |
| `priority`         | integer | No       | Execution priority (higher values run earlier when conflicts occur) |
| `providerConfig`   | object  | No       | Overrides merged into `.spec.serviceSpec.provider.config` (provider-specific settings) |
| `dryRun`           | bool    | No       | When `true`, performs full validation + merge but does not persist the change |

**Validation Rules:**
- `valuesFrom[].kind` must be `ConfigMap` or `Secret`. Other kinds are rejected.
- `dependsOn[]` must reference existing `serviceName` values already present in the ClusterDeployment. Referencing the new service (self-dependency) is not allowed.
- `templateNamespace`, `clusterNamespace`, and `serviceNamespace` values are all checked against the session namespace filter.

**Returns:**

```json
{
  "service": {
    "name": "minio",
    "namespace": "tenant-a",
    "template": "kcm-system/minio-14-1-2",
    "values": {
      "replicaCount": 2
    }
  },
  "status": {
    "name": "minio",
    "state": "Provisioning",
    "version": "14.1.2",
    "lastTransitionTime": "2025-11-10T08:44:13Z"
  },
  "upgradePaths": [],
  "clusterName": "prod-cluster",
  "clusterNamespace": "tenant-a",
  "dryRun": false
}
```

- `service` echoes the payload that was (or would be) applied.
- `status` contains the matching `.status.services[]` entry so operators can see whether the controller reports `Pending`, `Provisioning`, or `Deployed`.
- `upgradePaths` includes any `.status.servicesUpgradePaths[]` entries related to the service.
- `dryRun` reflects whether the server performed a mutation.

**Example MCP Request (dry-run preview):**

```json
{
  "jsonrpc": "2.0",
  "id": 21,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.services.apply",
    "arguments": {
      "clusterNamespace": "tenant-a",
      "clusterName": "prod-cluster",
      "templateNamespace": "kcm-system",
      "templateName": "minio-14-1-2",
      "serviceName": "minio",
      "dependsOn": ["ingress"],
      "values": {
        "replicaCount": 2,
        "persistence": {
          "size": "200Gi"
        }
      },
      "valuesFrom": [
        {"kind": "Secret", "name": "minio-secrets", "key": "values.yaml"}
      ],
      "helmOptions": {
        "timeout": "10m",
        "atomic": true
      },
      "dryRun": true
    }
  }
}
```

**Example (live apply with provider overrides):**

```json
{
  "jsonrpc": "2.0",
  "id": 22,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.services.apply",
    "arguments": {
      "clusterNamespace": "tenant-a",
      "clusterName": "prod-cluster",
      "templateNamespace": "kcm-system",
      "templateName": "logging-2-4-0",
      "serviceName": "observability",
      "providerConfig": {
        "aws": {
          "iamRole": "arn:aws:iam::123456789012:role/observability"
        }
      }
    }
  }
}
```

**Scenario Tips:**
- Use `dryRun=true` first to review the merged payload before touching production clusters.
- When a service fails to reconcile, re-run the tool without `dryRun` to update values; the latest status block will explain the failure.
- Namespace-filter violations produce `forbidden` errors for both ClusterDeployment and ServiceTemplate namespaces, preventing accidental cross-tenant access.

## Configuration

The cluster manager can be configured via environment variables:

| Variable                          | Default        | Description                           |
|-----------------------------------|----------------|---------------------------------------|
| CLUSTER_GLOBAL_NAMESPACE          | kcm-system     | Namespace for global resources        |
| CLUSTER_DEFAULT_NAMESPACE_DEV     | kcm-system     | Default namespace in dev mode         |
| CLUSTER_DEPLOY_FIELD_OWNER        | mcp.clusters   | Field manager for server-side apply   |

**Example Configuration:**

```bash
export CLUSTER_GLOBAL_NAMESPACE="kcm-system"
export CLUSTER_DEFAULT_NAMESPACE_DEV="kcm-system"
export CLUSTER_DEPLOY_FIELD_OWNER="mcp.clusters"
```

## Configuration Validation

The MCP server performs pre-flight validation on cluster configurations before submitting deployments. This ensures required fields are present and catches configuration errors immediately, rather than failing 5-15 minutes into cloud provisioning.

### Validation by Provider

Validation rules are applied based on the template name pattern. The server detects the provider and validates required fields accordingly.

#### Azure (Templates matching `azure-*`)

**Required Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `config.location` | string | Azure region | `"westus2"`, `"eastus"`, `"centralus"` |
| `config.subscriptionID` | string | Azure subscription ID | `"12345678-1234-1234-1234-123456789abc"` |

**Field Naming:**
- Control plane and worker use `vmSize` (not `instanceType`)

**Example Valid Configuration:**

```json
{
  "location": "westus2",
  "subscriptionID": "12345678-1234-1234-1234-123456789abc",
  "clusterIdentity": {
    "name": "azure-cluster-identity",
    "namespace": "kcm-system"
  },
  "controlPlane": {
    "vmSize": "Standard_A4_v2"
  },
  "worker": {
    "vmSize": "Standard_A4_v2"
  }
}
```

**Validation Error Example:**

Request missing `subscriptionID`:

```json
{
  "name": "test-azure-cluster",
  "template": "azure-standalone-cp-1-0-17",
  "credential": "azure-cluster-credential",
  "config": {
    "location": "westus2",
    "controlPlane": {"vmSize": "Standard_A4_v2"},
    "worker": {"vmSize": "Standard_A4_v2"}
  }
}
```

Error response:

```json
{
  "error": {
    "code": -32602,
    "message": "Azure cluster configuration validation failed:\n  - config.subscriptionID: Azure subscription ID is required (e.g., '12345678-1234-1234-1234-123456789abc')\n\nExample valid Azure configuration:\n{\n  \"location\": \"westus2\",\n  \"subscriptionID\": \"12345678-1234-1234-1234-123456789abc\",\n  \"controlPlane\": {\n    \"vmSize\": \"Standard_A4_v2\"\n  },\n  \"worker\": {\n    \"vmSize\": \"Standard_A4_v2\"\n  }\n}\n\nFor more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/"
  }
}
```

Corrected request:

```json
{
  "name": "test-azure-cluster",
  "template": "azure-standalone-cp-1-0-17",
  "credential": "azure-cluster-credential",
  "config": {
    "location": "westus2",
    "subscriptionID": "12345678-1234-1234-1234-123456789abc",
    "clusterIdentity": {
      "name": "azure-cluster-identity",
      "namespace": "kcm-system"
    },
    "controlPlane": {"vmSize": "Standard_A4_v2"},
    "worker": {"vmSize": "Standard_A4_v2"}
  }
}
```

#### AWS (Templates matching `aws-*`)

**Required Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `config.region` | string | AWS region | `"us-west-2"`, `"us-east-1"`, `"eu-west-1"` |

**Field Naming:**
- Control plane and worker use `instanceType` (not `vmSize`)

**Example Valid Configuration:**

```json
{
  "region": "us-west-2",
  "clusterIdentity": {
    "name": "aws-cluster-identity",
    "namespace": "kcm-system"
  },
  "controlPlane": {
    "instanceType": "t3.medium"
  },
  "worker": {
    "instanceType": "t3.large"
  }
}
```

**Validation Error Example:**

Request missing `region`:

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

Error response:

```json
{
  "error": {
    "code": -32602,
    "message": "AWS cluster configuration validation failed:\n  - config.region: AWS region is required (e.g., 'us-west-2', 'us-east-1', 'eu-west-1')\n\nExample valid AWS configuration:\n{\n  \"region\": \"us-west-2\",\n  \"controlPlane\": {\n    \"instanceType\": \"t3.small\"\n  },\n  \"worker\": {\n    \"instanceType\": \"t3.small\"\n  }\n}\n\nFor more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/"
  }
}
```

Corrected request:

```json
{
  "name": "test-aws-cluster",
  "template": "aws-standalone-cp-1-0-16",
  "credential": "aws-cluster-credential",
  "config": {
    "region": "us-west-2",
    "clusterIdentity": {
      "name": "aws-cluster-identity",
      "namespace": "kcm-system"
    },
    "controlPlane": {"instanceType": "t3.small"},
    "worker": {"instanceType": "t3.small"}
  }
}
```

#### GCP (Templates matching `gcp-*`)

**Required Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `config.project` | string | GCP project ID | `"my-gcp-project-123456"` |
| `config.region` | string | GCP region | `"us-central1"`, `"us-west1"`, `"europe-west1"` |
| `config.network.name` | string | VPC network name | `"default"` or custom VPC name |

**Field Naming:**
- Control plane and worker use `instanceType` (not `vmSize`)

**Example Valid Configuration:**

```json
{
  "project": "my-gcp-project-123456",
  "region": "us-central1",
  "network": {
    "name": "default"
  },
  "clusterIdentity": {
    "name": "gcp-cluster-identity",
    "namespace": "kcm-system"
  },
  "controlPlane": {
    "instanceType": "n1-standard-4"
  },
  "worker": {
    "instanceType": "n1-standard-4"
  }
}
```

**Validation Error Example:**

Request missing `project` and `network.name`:

```json
{
  "name": "test-gcp-cluster",
  "template": "gcp-standalone-cp-1-0-15",
  "credential": "gcp-credential",
  "config": {
    "region": "us-central1",
    "controlPlane": {"instanceType": "n1-standard-4"},
    "worker": {"instanceType": "n1-standard-4"}
  }
}
```

Error response:

```json
{
  "error": {
    "code": -32602,
    "message": "GCP cluster configuration validation failed:\n  - config.project: GCP project ID is required (e.g., 'my-gcp-project-123456')\n  - config.network.name: GCP network name is required (e.g., 'default' or custom VPC name)\n\nExample valid GCP configuration:\n{\n  \"project\": \"my-gcp-project-123456\",\n  \"region\": \"us-central1\",\n  \"network\": {\n    \"name\": \"default\"\n  },\n  \"controlPlane\": {\n    \"instanceType\": \"n1-standard-4\"\n  },\n  \"worker\": {\n    \"instanceType\": \"n1-standard-4\"\n  }\n}\n\nFor more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/"
  }
}
```

Corrected request:

```json
{
  "name": "test-gcp-cluster",
  "template": "gcp-standalone-cp-1-0-15",
  "credential": "gcp-credential",
  "config": {
    "project": "my-gcp-project-123456",
    "region": "us-central1",
    "network": {
      "name": "default"
    },
    "clusterIdentity": {
      "name": "gcp-cluster-identity",
      "namespace": "kcm-system"
    },
    "controlPlane": {"instanceType": "n1-standard-4"},
    "worker": {"instanceType": "n1-standard-4"}
  }
}
```

### Benefits of Pre-Flight Validation

- **Immediate Feedback**: Configuration errors are caught in seconds, not minutes
- **Cost Savings**: Prevents failed provisioning attempts that incur cloud charges
- **Better UX**: Clear, actionable error messages with examples
- **Time Savings**: No waiting 5-15 minutes to discover missing required fields

### Validation Scope

**Current Coverage:**
- AWS templates (pattern: `aws-*`)
- Azure templates (pattern: `azure-*`)
- GCP templates (pattern: `gcp-*`)

**Future Enhancements:**
- vSphere template validation
- OpenStack template validation
- Schema-based validation using ClusterTemplate `spec.schema`
- Cloud-specific validations (e.g., VM SKU availability)

## Azure Baseline Configuration

The following configuration represents a tested baseline for Azure deployments, used in live integration tests:

### Template
- **Name**: `azure-standalone-cp-1-0-15`
- **Namespace**: `kcm-system`
- **Type**: Standalone cluster with embedded control plane

### Credential
- **Name**: `azure-cluster-credential`
- **Namespace**: `kcm-system`
- **Provider**: Azure with service principal authentication

### Cluster Identity
- **Name**: `azure-cluster-identity`
- **Namespace**: `kcm-system`
- **Type**: Service principal credentials

### Configuration

```yaml
config:
  clusterIdentity:
    name: azure-cluster-identity
    namespace: kcm-system
  location: westus2
  subscriptionID: b90d4372-6e37-4eec-9e5a-fe3932d1a67c
  controlPlane:
    vmSize: Standard_A4_v2
    rootVolumeSize: 32
  controlPlaneNumber: 1
  worker:
    vmSize: Standard_A4_v2
    rootVolumeSize: 32
  workersNumber: 1
```

### Resource Requirements

**Control Plane:**
- **VM Size**: Standard_A4_v2 (8 vCPUs, 14 GB RAM)
- **Disk**: 32 GB premium SSD
- **Count**: 1 node

**Worker Nodes:**
- **VM Size**: Standard_A4_v2 (8 vCPUs, 14 GB RAM)
- **Disk**: 32 GB premium SSD
- **Count**: 1 node

### Deployment Timeline

Typical deployment timeline for Azure baseline:
- **Validation**: < 1 second (immediate)
- **Infrastructure Provisioning**: 5-8 minutes
- **Kubernetes Bootstrap**: 3-5 minutes
- **Total**: 10-15 minutes to Ready state

### Cost Estimate

Approximate hourly costs for baseline configuration (westus2):
- Control Plane: $0.42/hour
- Worker: $0.42/hour
- Disks: $0.005/hour (2 × 32 GB premium SSD)
- Network: Variable (typically < $0.01/hour)
- **Total**: ~$0.85/hour

## Usage Workflow

### Complete Cluster Provisioning Flow

1. **List Supported Providers**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.list",
    "arguments": {}
  }
}
```

2. **Discover Available Credentials**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.listCredentials",
    "arguments": {
      "provider": "azure"
    }
  }
}
```

3. **Browse Available Templates**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterTemplates.list",
    "arguments": {
      "scope": "global"
    }
  }
}
```

4. **Deploy New Cluster**

You can use either the generic tool (requires explicit template) or a provider-specific tool (automatic template selection).

**Option A: Provider-Specific Tool (Recommended for AWS/Azure/GCP)**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.provider.azure.clusterDeployments.deploy",
    "arguments": {
      "name": "production-cluster-01",
      "credential": "azure-cluster-credential",
      "location": "westus2",
      "subscriptionID": "b90d4372-6e37-4eec-9e5a-fe3932d1a67c",
      "labels": {
        "environment": "production",
        "project": "platform"
      },
      "controlPlane": {
        "vmSize": "Standard_D4s_v3",
        "rootVolumeSize": 50
      },
      "controlPlaneNumber": 3,
      "worker": {
        "vmSize": "Standard_D8s_v3",
        "rootVolumeSize": 100
      },
      "workersNumber": 5
    }
  }
}
```

**Option B: Generic Tool (for specific template versions or other providers)**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.deploy",
    "arguments": {
      "name": "production-cluster-01",
      "template": "azure-standalone-cp-1-0-15",
      "credential": "azure-cluster-credential",
      "labels": {
        "environment": "production",
        "project": "platform"
      },
      "config": {
        "clusterIdentity": {
          "name": "azure-cluster-identity",
          "namespace": "kcm-system"
        },
        "location": "westus2",
        "subscriptionID": "b90d4372-6e37-4eec-9e5a-fe3932d1a67c",
        "controlPlane": {
          "vmSize": "Standard_D4s_v3",
          "rootVolumeSize": 50
        },
        "controlPlaneNumber": 3,
        "worker": {
          "vmSize": "Standard_D8s_v3",
          "rootVolumeSize": 100
        },
        "workersNumber": 5
      }
    }
  }
}
```

4. **Monitor Deployment Status**

Use `k0rdent.mgmt.clusterDeployments.list` (or the `listAll` variant) to monitor progress without leaving MCP. Each summary now includes template/credential references, provider/region, and the latest conditions so you can diagnose issues immediately:

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.list",
    "arguments": {
      "namespace": "kcm-system"
    }
  }
}
```

Example response (abbreviated):

```json
{
  "clusters": [
    {
      "name": "production-cluster-01",
      "namespace": "kcm-system",
      "templateRef": {
        "name": "azure-standalone-cp-1-0-15",
        "version": "1.0.15"
      },
      "credentialRef": {
        "name": "azure-cluster-credential",
        "namespace": "kcm-system"
      },
      "cloudProvider": "azure",
      "region": "westus2",
      "ready": true,
      "phase": "Ready",
      "conditions": [
        {
          "type": "Ready",
          "status": "True",
          "lastTransitionTime": "2025-05-04T07:22:10Z"
        }
      ],
      "kubeconfigSecret": {
        "name": "production-cluster-01-kubeconfig",
        "namespace": "kcm-system"
      }
    }
  ]
}
```

Key fields:

- `templateRef` – shows the exact ClusterTemplate + version in use.
- `credentialRef` / `clusterIdentityRef` – identify which credentials and identities were applied.
- `cloudProvider` and `region` – inferred from labels/config to simplify filtering.
- `phase`, `ready`, `message`, and detailed `conditions` – mirror the ClusterDeployment status.
- `kubeconfigSecret` / `managementURL` – operational shortcuts for connecting to or viewing the workload cluster.

5. **Delete Cluster (When Done)**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.delete",
    "arguments": {
      "name": "production-cluster-01"
    }
  }
}
```

6. **Verify Deletion**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.clusterDeployments.listAll",
    "arguments": {}
  }
}
```

The cluster should no longer appear in the list.

## Error Handling

### Common Errors

**Missing Credential**

```json
{
  "error": {
    "code": -32602,
    "message": "credential \"unknown-credential\" not found in namespace \"kcm-system\" or \"team-a\""
  }
}
```

**Resolution**: Use `k0rdent.mgmt.providers.listCredentials` to see available credentials.

**Missing Template**

```json
{
  "error": {
    "code": -32602,
    "message": "template \"unknown-template\" not found. Available templates: azure-standalone-cp-1-0-15, aws-standalone-cp-0-0-3"
  }
}
```

**Resolution**: Use `k0rdent.mgmt.clusterTemplates.list` to see available templates.

**Namespace Forbidden**

```json
{
  "error": {
    "code": -32001,
    "message": "namespace \"restricted\" not allowed by namespace filter"
  }
}
```

**Resolution**:
- Deploy to an allowed namespace
- Adjust server's namespace filter configuration
- Contact administrator for access

**Missing Namespace in OIDC Mode**

```json
{
  "error": {
    "code": -32602,
    "message": "namespace must be specified in OIDC_REQUIRED mode (no namespaces match filter)"
  }
}
```

**Resolution**: Specify an explicit `namespace` parameter that matches the configured filter.

**Invalid Configuration (General)**

```json
{
  "error": {
    "code": -32602,
    "message": "validation failed: spec.config.location: Required value"
  }
}
```

**Resolution**: Check the template's `configSchema` and ensure all required fields are provided.

**Azure Configuration Validation Failure**

```json
{
  "error": {
    "code": -32602,
    "message": "Azure cluster configuration validation failed:\n  - config.location: Azure location is required (e.g., 'westus2', 'eastus', 'centralus')\n  - config.subscriptionID: Azure subscription ID is required (e.g., '12345678-1234-1234-1234-123456789abc')\n\nExample valid Azure configuration:\n{\n  \"location\": \"westus2\",\n  \"subscriptionID\": \"12345678-1234-1234-1234-123456789abc\",\n  \"controlPlane\": {\n    \"vmSize\": \"Standard_A4_v2\"\n  },\n  \"worker\": {\n    \"vmSize\": \"Standard_A4_v2\"\n  }\n}\n\nFor more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/"
  }
}
```

**Resolution**:
- Ensure `config.location` is set to a valid Azure region (e.g., `"westus2"`, `"eastus"`)
- Ensure `config.subscriptionID` is set to your Azure subscription ID (format: `"12345678-1234-1234-1234-123456789abc"`)
- Use `vmSize` (not `instanceType`) for control plane and worker configuration
- See the Azure baseline configuration section for a complete working example

**AWS Configuration Validation Failure**

```json
{
  "error": {
    "code": -32602,
    "message": "AWS cluster configuration validation failed:\n  - config.region: AWS region is required (e.g., 'us-west-2', 'us-east-1', 'eu-west-1')\n\nExample valid AWS configuration:\n{\n  \"region\": \"us-west-2\",\n  \"controlPlane\": {\n    \"instanceType\": \"t3.small\"\n  },\n  \"worker\": {\n    \"instanceType\": \"t3.small\"\n  }\n}\n\nFor more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/"
  }
}
```

**Resolution**:
- Ensure `config.region` is set to a valid AWS region (e.g., `"us-west-2"`, `"us-east-1"`, `"eu-west-1"`)
- Use `instanceType` (not `vmSize`) for control plane and worker configuration
- Verify the region matches where your AWS credentials have access
- See [k0rdent AWS quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/) for detailed setup instructions

**GCP Configuration Validation Failure**

```json
{
  "error": {
    "code": -32602,
    "message": "GCP cluster configuration validation failed:\n  - config.project: GCP project ID is required (e.g., 'my-gcp-project-123456')\n  - config.region: GCP region is required (e.g., 'us-central1', 'us-west1', 'europe-west1')\n  - config.network.name: GCP network name is required (e.g., 'default' or custom VPC name)\n\nExample valid GCP configuration:\n{\n  \"project\": \"my-gcp-project-123456\",\n  \"region\": \"us-central1\",\n  \"network\": {\n    \"name\": \"default\"\n  },\n  \"controlPlane\": {\n    \"instanceType\": \"n1-standard-4\"\n  },\n  \"worker\": {\n    \"instanceType\": \"n1-standard-4\"\n  }\n}\n\nFor more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/"
  }
}
```

**Resolution**:
- Ensure `config.project` is set to your GCP project ID (e.g., `"my-gcp-project-123456"`)
- Ensure `config.region` is set to a valid GCP region (e.g., `"us-central1"`, `"us-west1"`)
- Ensure `config.network.name` is set to a valid VPC network name (commonly `"default"`)
- Use `instanceType` (not `vmSize`) for control plane and worker configuration
- Verify your GCP credentials have access to the specified project and network
- See [k0rdent GCP quickstart](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/) for detailed setup instructions

**RBAC Denial**

```json
{
  "error": {
    "code": -32001,
    "message": "clusterdeployments.k0rdent.mirantis.com is forbidden: User \"user@example.com\" cannot create resource \"clusterdeployments\" in namespace \"kcm-system\""
  }
}
```

**Resolution**:
- Ensure user has appropriate RBAC permissions
- Request ClusterRole binding from administrator
- Verify service account configuration

**Cluster Already Exists**

The deploy tool is idempotent, so this is not an error. Re-deploying with the same name will update the existing resource:

```json
{
  "name": "existing-cluster",
  "namespace": "kcm-system",
  "status": "updated"
}
```

**Deletion Timeout**

If deletion takes longer than expected:
- Check cluster finalizers: `kubectl get clusterdeployment -n kcm-system cluster-name -o yaml`
- Monitor cloud provider cleanup
- Review k0rdent controller logs
- Verify cloud credentials are still valid

## Troubleshooting

### Enable Debug Logging

Set the server's log level to debug for detailed cluster operations:

```bash
export LOG_LEVEL=debug
```

Debug logs include:
- Namespace resolution decisions
- Credential and template lookup details
- Server-side apply patches
- Validation outcomes
- Cloud provider API interactions (from k0rdent controllers)

### Inspect ClusterDeployment

View full ClusterDeployment resource:

```bash
kubectl get clusterdeployment -n kcm-system my-cluster -o yaml
```

Key fields to check:
- `status.conditions` - Current state and health
- `status.observedGeneration` - Whether spec changes have been processed
- `status.clusterInfo` - Child cluster connection details
- `metadata.finalizers` - Cleanup handlers

### Monitor Cloud Resources

For Azure:

```bash
az vm list --resource-group k0rdent-my-cluster --output table
az network vnet list --resource-group k0rdent-my-cluster --output table
```

For AWS:

```bash
aws ec2 describe-instances --filters "Name=tag:cluster.x-k8s.io/cluster-name,Values=my-cluster"
aws elb describe-load-balancers --region us-west-2
```

### Check CAPI Resources

View Cluster API resources for detailed provisioning state:

```bash
# List machines
kubectl get machines -n kcm-system

# View cluster status
kubectl get cluster -n kcm-system my-cluster -o yaml

# Check machine health
kubectl get machinedeployment -n kcm-system
```

### Common Issues

**Configuration Validation Failure**

If you encounter validation errors when deploying a cluster, the error message will clearly indicate which fields are missing and provide examples.

**For Azure:**
1. Ensure both `config.location` and `config.subscriptionID` are present
2. Use `vmSize` field for control plane and worker (not `instanceType`)
3. Common locations: `"westus2"`, `"eastus"`, `"centralus"`
4. Subscription ID format: `"12345678-1234-1234-1234-123456789abc"`

**For AWS:**
1. Ensure `config.region` is present
2. Use `instanceType` field for control plane and worker (not `vmSize`)
3. Common regions: `"us-west-2"`, `"us-east-1"`, `"eu-west-1"`

**For GCP:**
1. Ensure `config.project`, `config.region`, and `config.network.name` are all present
2. Use `instanceType` field for control plane and worker (not `vmSize`)
3. Common regions: `"us-central1"`, `"us-west1"`, `"europe-west1"`
4. Network name is commonly `"default"` for the default VPC

**Debug Tips:**
- Review the error message carefully - it includes the exact missing fields
- Compare your config against the validation error example in the response
- Refer to the Configuration Validation section in this document
- Check the provider-specific k0rdent quickstart documentation

**Deployment Stuck in Provisioning**

1. Check k0rdent controller logs:
   ```bash
   kubectl logs -n kcm-system deployment/k0rdent-controller-manager -f
   ```

2. Verify credential validity:
   ```bash
   kubectl get credential -n kcm-system azure-cluster-credential -o yaml
   ```

3. Check cloud provider quotas and limits

**Deletion Stuck with Finalizers**

1. Check finalizers:
   ```bash
   kubectl get clusterdeployment -n kcm-system my-cluster -o jsonpath='{.metadata.finalizers}'
   ```

2. Review controller logs for cleanup errors

3. Manually verify cloud resources are deleted

4. As last resort, remove finalizer manually:
   ```bash
   kubectl patch clusterdeployment -n kcm-system my-cluster --type json -p '[{"op": "remove", "path": "/metadata/finalizers"}]'
   ```

**Network Connectivity Issues**

1. Verify management cluster can reach cloud provider APIs
2. Check network policies and firewall rules
3. Ensure DNS resolution works
4. Test cloud API access from a pod:
   ```bash
   kubectl run -it --rm debug --image=alpine --restart=Never -- sh
   apk add curl
   curl -v https://management.azure.com/
   ```

## Limitations

### Current Limitations

- **Management Cluster Only**: Tools interact with management cluster resources; actual child clusters take time to provision
- **No Update Support**: Cannot update existing deployments (must delete and recreate)
- **No Status Polling**: Tools return immediately; use separate list calls to monitor progress
- **Limited Validation**: Pre-flight config validation covers AWS, Azure, and GCP required fields only; other providers and optional fields not validated
- **No Rollback**: No automated rollback on deployment failure
- **No Cost Estimation**: No built-in cost estimation or quota checking
- **No Multi-Region**: Single region per deployment (no automatic cross-region HA)

### Validation Coverage

**Currently Validated:**
- AWS: Required `region` field
- Azure: Required `location` and `subscriptionID` fields
- GCP: Required `project`, `region`, and `network.name` fields

**Not Yet Validated:**
- vSphere templates
- OpenStack templates
- Optional but recommended fields
- Cloud-specific constraints (e.g., VM SKU availability, region-specific quotas)
- Cross-resource validation (e.g., credential validity, network existence)

### Planned Enhancements

Future versions may include:
- Real-time deployment progress notifications via MCP resources
- Schema-based validation using ClusterTemplate `spec.schema`
- Validation for vSphere and OpenStack providers
- Cloud-specific validations (e.g., Azure VM SKU validation)
- Pre-flight checks (quotas, credentials, network)
- Update/upgrade operations for existing deployments
- Cost estimation and budget enforcement
- Multi-region deployment support
- Automated backup and restore
- Integration with monitoring and alerting

## Security Considerations

### Trust Model

- **Credential Security**: Credentials stored as Kubernetes secrets; never exposed in API responses
- **Field Manager**: Uses dedicated field manager (`mcp.clusters`) to track ownership
- **Namespace Isolation**: Enforces namespace filters to prevent cross-tenant access
- **RBAC Integration**: Respects Kubernetes RBAC for all operations
- **Cloud Provider Auth**: Uses k0rdent's credential handling (no direct cloud API access from MCP server)

### Best Practices

1. **Use Namespace Filters**: Always configure namespace filters in production
2. **Principle of Least Privilege**: Grant minimal RBAC permissions required
3. **Credential Rotation**: Rotate cloud credentials regularly
4. **Audit Logging**: Enable Kubernetes audit logging to track cluster operations
5. **Resource Quotas**: Set resource quotas per namespace to prevent over-provisioning
6. **Network Policies**: Restrict egress from management cluster to cloud APIs
7. **Separate Credentials**: Use different credentials per environment/team
8. **Monitor Costs**: Track cloud spending per cluster and team

## Performance Considerations

### Optimization Tips

- **Credential Caching**: Credentials are listed on-demand; consider implementing caching if list operations are frequent
- **Template Discovery**: Templates rarely change; cache locally after initial discovery
- **Parallel Deployments**: Deploy tool supports concurrent calls for multiple clusters
- **Namespace Scope**: Use `scope="global"` when only global templates are needed
- **Label Selectors**: Use existing list tools with selectors to filter ClusterDeployments

### Resource Usage

- **API Calls**: Each operation makes 2-4 Kubernetes API calls
- **Memory**: Minimal (~10 MB per operation)
- **CPU**: Negligible (< 100ms processing time)
- **Network**: ~1-5 KB per operation

### Deployment Performance

Typical deployment times by cloud provider:
- **Azure**: 10-15 minutes (baseline), 15-25 minutes (HA)
- **AWS**: 8-12 minutes (baseline), 12-20 minutes (HA)
- **vSphere**: 15-20 minutes (baseline), 20-30 minutes (HA)

Factors affecting deployment time:
- Cloud provider API latency
- VM provisioning time
- Kubernetes bootstrap duration
- Network configuration complexity
- Number of nodes

## Related Documentation

- [Live Integration Tests](./live-tests.md) - Testing cluster provisioning
- [k0rdent Cluster Documentation](https://docs.k0rdent.io/latest/user/user-create-cluster) - Detailed cluster creation guide
- [Catalog Tools](./catalog.md) - ServiceTemplate installation
- [k0rdent Documentation](https://docs.k0rdent.io) - Platform documentation

### k0rdent.mgmt.providers.listIdentities

Lists ClusterIdentity resources referenced by credentials, showing the identity kind/provider and which credentials use it. This is useful when following the [credentials process](https://docs.k0rdent.io/latest/admin/access/credentials/credentials-process/) and validating identity wiring before deployments.

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.mgmt.providers.listIdentities",
    "arguments": {}
  }
}
```

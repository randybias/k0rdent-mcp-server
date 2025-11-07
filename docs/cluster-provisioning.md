# k0rdent Cluster Provisioning Tools

The MCP server provides tools for provisioning and managing k0rdent child clusters programmatically. These tools enable end-to-end cluster lifecycle management from discovering credentials and templates through deployment and deletion.

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

### k0.clusters.listCredentials

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
    "name": "k0.clusters.listCredentials",
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
    "name": "k0.clusters.listCredentials",
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
    "name": "k0.clusters.listCredentials",
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

### k0.clusters.listTemplates

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
    "name": "k0.clusters.listTemplates",
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
    "name": "k0.clusters.listTemplates",
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
    "name": "k0.clusters.listTemplates",
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

### k0.clusters.deploy

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
    "name": "k0.clusters.deploy",
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
    "name": "k0.clusters.deploy",
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
    "name": "k0.clusters.deploy",
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

### k0.clusters.delete

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
    "name": "k0.clusters.delete",
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
    "name": "k0.clusters.delete",
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
- **Validation**: < 1 minute
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

1. **Discover Available Credentials**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.clusters.listCredentials",
    "arguments": {}
  }
}
```

2. **Browse Available Templates**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.clusters.listTemplates",
    "arguments": {
      "scope": "global"
    }
  }
}
```

3. **Deploy New Cluster**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.clusters.deploy",
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

Use existing k0rdent tools to monitor progress:

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.k0rdent.clusterDeployments.list",
    "arguments": {}
  }
}
```

Check the `status` field for:
- `Provisioning` - Infrastructure being created
- `Provisioned` - Infrastructure ready, Kubernetes bootstrapping
- `Ready` - Cluster fully operational

5. **Delete Cluster (When Done)**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.clusters.delete",
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
    "name": "k0.k0rdent.clusterDeployments.list",
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

**Resolution**: Use `k0.clusters.listCredentials` to see available credentials.

**Missing Template**

```json
{
  "error": {
    "code": -32602,
    "message": "template \"unknown-template\" not found. Available templates: azure-standalone-cp-1-0-15, aws-standalone-cp-0-0-3"
  }
}
```

**Resolution**: Use `k0.clusters.listTemplates` to see available templates.

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

**Invalid Configuration**

```json
{
  "error": {
    "code": -32602,
    "message": "validation failed: spec.config.location: Required value"
  }
}
```

**Resolution**: Check the template's `configSchema` and ensure all required fields are provided.

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
- **No Validation**: Limited pre-flight config validation (relies on Kubernetes admission)
- **No Rollback**: No automated rollback on deployment failure
- **No Cost Estimation**: No built-in cost estimation or quota checking
- **No Multi-Region**: Single region per deployment (no automatic cross-region HA)

### Planned Enhancements

Future versions may include:
- Real-time deployment progress notifications via MCP resources
- Configuration validation against template schema
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

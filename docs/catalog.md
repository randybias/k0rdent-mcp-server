# k0rdent Catalog Tools

The MCP server provides tools for discovering and installing ServiceTemplates from the official k0rdent catalog. These tools enable programmatic access to the catalog without requiring manual browsing or git operations.

## Overview

The catalog tools allow you to:
- **List available ServiceTemplates** from catalog.k0rdent.io with rich metadata
- **Install ServiceTemplates** directly to the management cluster
- **Filter and search** by application slug
- **Cache catalog data** locally for improved performance

All catalog data is sourced from the official k0rdent catalog repository at https://github.com/k0rdent/catalog.

## Prerequisites

### Network Access

The catalog tools require network connectivity to:
- **GitHub**: `https://github.com/k0rdent/catalog` - for downloading the catalog archive
- **GHCR**: `oci://ghcr.io/k0rdent/catalog` - for Helm chart references (used at deployment time)

If operating in a restricted network environment, you can override the catalog source URL using environment variables (see Configuration section).

### Kubernetes Access

- Management cluster access with appropriate RBAC permissions
- Permissions to create ServiceTemplate and HelmRepository resources
- If using namespace filtering, the target namespace must match the configured filter

### Cache Directory

The catalog manager caches downloaded catalogs to improve performance. Ensure the cache directory:
- Is writable by the server process
- Has sufficient disk space (catalog archives are typically 1-5 MB compressed)
- Persists across server restarts for optimal caching

## Available Tools

### k0.catalog.list

Lists available ServiceTemplates from the k0rdent catalog with optional filtering.

**Parameters:**

| Parameter | Type   | Required | Description                                    |
|-----------|--------|----------|------------------------------------------------|
| app       | string | No       | Filter results by application slug             |
| refresh   | bool   | No       | Force refresh from GitHub (bypass cache)       |

**Returns:**

```json
{
  "entries": [
    {
      "slug": "minio",
      "title": "MinIO Object Storage",
      "summary": "High-performance object storage",
      "tags": ["storage", "s3"],
      "validated_platforms": ["aws", "vsphere"],
      "versions": [
        {
          "name": "minio",
          "version": "14.1.2",
          "repository": "oci://ghcr.io/k0rdent/catalog/minio",
          "service_template_path": "apps/minio/charts/minio-14.1.2/templates/service-template.yaml",
          "helm_repository_path": "apps/k0rdent-utils/charts/k0rdent-catalog-0.1.0/templates/helm-repository.yaml"
        }
      ]
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
    "name": "k0.catalog.list",
    "arguments": {}
  }
}
```

**Example with Filter:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.list",
    "arguments": {
      "app": "minio"
    }
  }
}
```

**Example with Refresh:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.list",
    "arguments": {
      "refresh": true
    }
  }
}
```

### k0.catalog.install

Installs a ServiceTemplate from the catalog to the management cluster.

**Parameters:**

| Parameter      | Type   | Required | Description                                                |
|----------------|--------|----------|------------------------------------------------------------|
| app            | string | Yes      | Application slug (from catalog list)                       |
| template       | string | Yes      | ServiceTemplate name (from version list)                   |
| version        | string | Yes      | Specific version to install                                |
| namespace      | string | No       | Target namespace for installation                          |
| all_namespaces | bool   | No       | Install to all allowed namespaces (cannot combine with namespace) |

**Namespace Behavior:**

The install tool behaves differently based on the server's authentication mode:

- **DEV_ALLOW_ANY mode** (uses kubeconfig): Defaults to `kcm-system` if namespace not specified
- **OIDC_REQUIRED mode** (uses bearer token): Requires explicit `namespace` or `all_namespaces=true`

**Returns:**

```json
{
  "applied": [
    "kcm-system/HelmRepository/k0rdent-catalog",
    "kcm-system/ServiceTemplate/minio-14-1-2"
  ],
  "status": "created"
}
```

**Example MCP Request (Default Namespace):**

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.install",
    "arguments": {
      "app": "minio",
      "template": "minio",
      "version": "14.1.2"
    }
  }
}
```

**Example with Explicit Namespace:**

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.install",
    "arguments": {
      "app": "minio",
      "template": "minio",
      "version": "14.1.2",
      "namespace": "team-a"
    }
  }
}
```

**Example with All Namespaces:**

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.install",
    "arguments": {
      "app": "minio",
      "template": "minio",
      "version": "14.1.2",
      "all_namespaces": true
    }
  }
}
```

## Configuration

The catalog manager can be configured via environment variables:

| Variable                  | Default                                                               | Description                           |
|---------------------------|-----------------------------------------------------------------------|---------------------------------------|
| CATALOG_ARCHIVE_URL       | https://github.com/k0rdent/catalog/archive/refs/heads/main.tar.gz    | Catalog archive source URL            |
| CATALOG_CACHE_DIR         | /var/lib/k0rdent-mcp/catalog                                         | Local cache directory                 |
| CATALOG_DOWNLOAD_TIMEOUT  | 30s                                                                   | HTTP download timeout                 |
| CATALOG_CACHE_TTL         | 6h                                                                    | Cache validity duration               |

**Example Configuration:**

```bash
export CATALOG_ARCHIVE_URL="https://internal-mirror.example.com/k0rdent-catalog.tar.gz"
export CATALOG_CACHE_DIR="/opt/mcp-cache/catalog"
export CATALOG_CACHE_TTL="12h"
```

## Cache Behavior

The catalog manager implements intelligent caching to minimize network requests:

1. **First Request**: Downloads catalog archive from GitHub, extracts contents, builds index, and caches all data
2. **Subsequent Requests**: Uses cached data if within TTL period (default 6 hours)
3. **Cache Expiry**: Automatically refreshes when TTL expires
4. **Manual Refresh**: Use `refresh=true` parameter to force immediate update
5. **Cache Key**: Archives are identified by SHA256 hash to detect upstream changes

**Cache Directory Structure:**

```
/var/lib/k0rdent-mcp/catalog/
├── metadata.json                    # Cache metadata (SHA, timestamp, URL)
└── catalog-<sha256>/               # Extracted catalog contents
    └── apps/
        ├── minio/
        │   ├── data.yaml
        │   └── charts/
        │       └── minio-14.1.2/
        │           └── templates/
        │               └── service-template.yaml
        └── ...
```

**Cache Management:**

- Cache persists across server restarts
- Multiple catalog versions can coexist (identified by SHA)
- Old cache directories are not automatically cleaned up
- Manual cleanup is safe (server will re-download on next request)

## Understanding Installation

When you call `k0.catalog.install`, the server:

1. **Retrieves Manifests**: Fetches ServiceTemplate and optional HelmRepository YAML from the cached catalog
2. **Validates Namespace**: Checks target namespace against configured namespace filter (if enabled)
3. **Applies Resources**: Uses server-side apply with field manager `k0rdent-mcp-server`
4. **Returns Results**: Lists all applied resources with their namespace, kind, and name

**Important Notes:**

- **Management Cluster Only**: Installs resources to the management cluster, not child clusters
- **Does Not Deploy Services**: Creates ServiceTemplate definition only; actual service deployment requires MultiClusterService
- **Idempotent Operations**: Uses server-side apply, so repeated installs are safe
- **No Uninstall Support**: Currently no tool for removing installed templates
- **No Upgrade Support**: No automated upgrade mechanism (manual deletion and reinstall required)

## Usage Workflow

### Typical Installation Flow

1. **Discover Available Applications**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.list",
    "arguments": {}
  }
}
```

2. **Filter to Specific Application**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.list",
    "arguments": {
      "app": "ingress-nginx"
    }
  }
}
```

3. **Review Available Versions** (from response)

Check the `versions` array in the response to see available versions.

4. **Install ServiceTemplate**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.install",
    "arguments": {
      "app": "ingress-nginx",
      "template": "ingress-nginx",
      "version": "4.11.3"
    }
  }
}
```

5. **Create MultiClusterService** (separate operation, not covered by catalog tools)

Use standard Kubernetes tooling or the management API to create a MultiClusterService referencing the installed ServiceTemplate.

## Error Handling

### Common Errors

**Network Failures**

```json
{
  "error": "download catalog archive: Get \"https://github.com/...\": dial tcp: connection refused"
}
```

**Resolution**: Check network connectivity, verify CATALOG_ARCHIVE_URL is reachable, check firewall rules.

**App Not Found**

```json
{
  "error": "app \"unknown-app\" not found in catalog"
}
```

**Resolution**: Use `k0.catalog.list` without filter to see all available apps.

**Version Not Found**

```json
{
  "error": "template \"nginx\" version \"999.0.0\" not found for app \"ingress-nginx\""
}
```

**Resolution**: Use `k0.catalog.list` with app filter to see available versions.

**Namespace Filter Rejection**

```json
{
  "error": "namespace \"forbidden-ns\" not allowed by namespace filter"
}
```

**Resolution**: Install to an allowed namespace or adjust server's namespace filter configuration.

**Missing Namespace in OIDC Mode**

```json
{
  "error": "namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter or 'all_namespaces: true')"
}
```

**Resolution**: In OIDC_REQUIRED mode, you must explicitly specify either a `namespace` or set `all_namespaces: true`.

**Conflicting Parameters**

```json
{
  "error": "cannot specify both 'namespace' and 'all_namespaces'"
}
```

**Resolution**: Choose either a specific namespace OR the all_namespaces flag, not both.

**Kubernetes Apply Failure**

```json
{
  "error": "apply ServiceTemplate minio: servicetemplate.k0rdent.mirantis.com \"minio\" is forbidden: User \"system:serviceaccount:default:mcp-server\" cannot create resource \"servicetemplates\" in API group \"k0rdent.mirantis.com\" at the cluster scope"
}
```

**Resolution**: Ensure service account has appropriate RBAC permissions for ServiceTemplate and HelmRepository resources.

## Limitations

### Current Limitations

- **Management Cluster Only**: Cannot directly install to child clusters
- **No Uninstall Tool**: Manual kubectl delete required to remove templates
- **No Upgrade Tool**: No automated upgrade path (requires manual deletion and reinstallation)
- **No Dependency Management**: Does not handle dependencies between templates
- **No Validation**: Does not validate template compatibility with cluster
- **No Rollback**: No automated rollback mechanism for failed installations
- **No Offline Mode**: Requires network access (no air-gapped support)
- **No Signature Verification**: Does not verify catalog archive signatures

### Planned Enhancements

Future versions may include:
- Air-gapped/offline mode with catalog mirror support
- Signature verification for catalog archives
- Uninstall and upgrade tools
- Dependency resolution and installation
- Pre-installation validation and compatibility checks
- Integration with MultiClusterService creation workflow

## Security Considerations

### Trust Model

- **Catalog Source**: By default, trusts content from https://github.com/k0rdent/catalog
- **No Signature Verification**: Currently does not verify cryptographic signatures on catalog data
- **Server-Side Apply**: Uses field manager to track ownership but does not prevent conflicts

### Best Practices

1. **Use Private Mirror**: In production, consider hosting catalog on private infrastructure
2. **Review Before Install**: Always review ServiceTemplate manifests before installation
3. **Namespace Isolation**: Use namespace filters to restrict installation scope
4. **RBAC Controls**: Apply principle of least privilege to service account permissions
5. **Network Policies**: Restrict egress to only required endpoints (GitHub, GHCR)
6. **Cache Security**: Ensure cache directory has appropriate filesystem permissions (0755 directories, 0644 files)
7. **Audit Logging**: Enable structured logging to track catalog operations

## Performance Considerations

### Optimization Tips

- **Cache Warmup**: Perform an initial `k0.catalog.list` call on server startup to warm cache
- **Adjust TTL**: Increase `CATALOG_CACHE_TTL` if catalog updates are infrequent
- **Local Mirror**: Use `CATALOG_ARCHIVE_URL` to point to a local mirror for faster downloads
- **Persistent Cache**: Mount cache directory to persistent volume in containerized environments

### Resource Usage

- **Disk Space**: Catalog archives are 1-5 MB compressed, 5-20 MB extracted
- **Memory**: Index building requires minimal memory (typically < 10 MB)
- **Network**: Initial download is 1-5 MB; subsequent requests use cache
- **CPU**: Archive extraction and YAML parsing are CPU-bound but complete in < 1 second

## Troubleshooting

### Enable Debug Logging

Set the server's log level to debug to see detailed catalog operations:

```bash
export LOG_LEVEL=debug
```

Debug logs include:
- Cache hit/miss decisions
- Download progress and timing
- Index building details
- Manifest retrieval paths

### Inspect Cache

Check cache metadata to verify state:

```bash
cat /var/lib/k0rdent-mcp/catalog/metadata.json
```

List cached catalog directories:

```bash
ls -la /var/lib/k0rdent-mcp/catalog/
```

### Force Cache Refresh

Use the refresh parameter to bypass cache:

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0.catalog.list",
    "arguments": {
      "refresh": true
    }
  }
}
```

### Manual Cache Cleanup

Safe to remove cache directory (server will re-download):

```bash
rm -rf /var/lib/k0rdent-mcp/catalog/
```

## Related Documentation

- [Live Integration Tests](./live-tests.md) - Testing catalog functionality
- [k0rdent Catalog Repository](https://github.com/k0rdent/catalog) - Upstream catalog source
- [k0rdent Documentation](https://docs.k0rdent.io) - Platform documentation

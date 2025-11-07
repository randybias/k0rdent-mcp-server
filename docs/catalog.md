# k0rdent Catalog Tools

The MCP server provides tools for discovering and installing ServiceTemplates from the official k0rdent catalog. These tools enable programmatic access to the catalog without requiring manual browsing or git operations.

## Overview

The catalog tools allow you to:
- **List available ServiceTemplates** from catalog.k0rdent.io with rich metadata
- **Install ServiceTemplates** directly to the management cluster
- **Delete ServiceTemplates** from the management cluster
- **Filter and search** by application slug
- **Cache catalog data** locally for improved performance

All catalog data is sourced from the official k0rdent catalog repository at https://github.com/k0rdent/catalog.

## Architecture

The catalog system uses a **JSON index-based architecture** for optimal performance:

1. **JSON Index Download**: Fetches a lightweight JSON index (~100 KB) from catalog.k0rdent.io
2. **SQLite Cache**: Stores index data in a persistent SQLite database with timestamp-based invalidation
3. **On-Demand Manifest Fetching**: Downloads individual YAML manifests from GitHub raw URLs only when needed
4. **No Tarball Extraction**: Eliminates the need to download or extract large compressed archives

**Performance Benefits:**
- **18x faster processing** compared to tarball extraction
- **~100 KB download** vs 1-5 MB tarball archives
- **Instant cache validation** using timestamp comparison
- **Reduced disk I/O** with no extraction required

**Cache Invalidation:**
- Index includes a `metadata.generated` timestamp
- Cached data is compared against upstream timestamp
- Automatic refresh when upstream index is newer
- Manual refresh available via `refresh=true` parameter

## Prerequisites

### Network Access

The catalog tools require network connectivity to:
- **Catalog Index**: `https://catalog.k0rdent.io/latest/index.json` - for downloading the JSON index
- **GitHub Raw**: `https://raw.githubusercontent.com/k0rdent/catalog/` - for fetching individual YAML manifests
- **GHCR**: `oci://ghcr.io/k0rdent/catalog` - for Helm chart references (used at deployment time)

If operating in a restricted network environment, you can override the catalog source URL using environment variables (see Configuration section).

### Kubernetes Access

- Management cluster access with appropriate RBAC permissions
- Permissions to create ServiceTemplate and HelmRepository resources
- If using namespace filtering, the target namespace must match the configured filter

### Cache Directory

The catalog manager caches index data in a SQLite database to improve performance. Ensure the cache directory:
- Is writable by the server process
- Has sufficient disk space (SQLite database is typically < 1 MB)
- Persists across server restarts for optimal caching

## Available Tools

### k0rdent.catalog.list

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
    "name": "k0rdent.catalog.list",
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
    "name": "k0rdent.catalog.list",
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
    "name": "k0rdent.catalog.list",
    "arguments": {
      "refresh": true
    }
  }
}
```

### k0rdent.catalog.delete_servicetemplate

Deletes ServiceTemplate resources from the management cluster that were previously installed via the catalog.

**Parameters:**

| Parameter      | Type   | Required | Description                                                |
|----------------|--------|----------|------------------------------------------------------------|
| app            | string | Yes      | Application slug (from catalog list)                       |
| template       | string | Yes      | ServiceTemplate name to delete                             |
| version        | string | Yes      | Specific version to delete                                 |
| namespace      | string | No       | Target namespace for deletion                              |
| all_namespaces | bool   | No       | Delete from all allowed namespaces (cannot combine with namespace) |

**Namespace Behavior:**

The delete tool behaves similarly to install:

- **DEV_ALLOW_ANY mode** (uses kubeconfig): Defaults to `kcm-system` if namespace not specified
- **OIDC_REQUIRED mode** (uses bearer token): Requires explicit `namespace` or `all_namespaces=true`

**Returns:**

```json
{
  "deleted": [
    "kcm-system/ServiceTemplate/minio-14-1-2"
  ],
  "status": "deleted"
}
```

**Example MCP Request (Default Namespace):**

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.catalog.delete_servicetemplate",
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
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.catalog.delete_servicetemplate",
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
  "id": 9,
  "method": "tools/call",
  "params": {
    "name": "k0rdent.catalog.delete_servicetemplate",
    "arguments": {
      "app": "minio",
      "template": "minio",
      "version": "14.1.2",
      "all_namespaces": true
    }
  }
}
```

**Important Notes:**

- **Cascade Deletion**: Only deletes the ServiceTemplate resource; does not delete associated HelmRepository or deployed services
- **Idempotent**: Safe to call multiple times; returns success if resource already deleted
- **No Validation**: Does not check if MultiClusterService resources reference the template before deletion
- **Management Cluster Only**: Deletes from the management cluster, not child clusters

### k0rdent.catalog.install_servicetemplate

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
    "name": "k0rdent.catalog.install_servicetemplate",
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
    "name": "k0rdent.catalog.install_servicetemplate",
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
    "name": "k0rdent.catalog.install_servicetemplate",
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
| CATALOG_INDEX_URL         | https://catalog.k0rdent.io/latest/index.json                         | JSON index source URL                 |
| CATALOG_CACHE_DIR         | /var/lib/k0rdent-mcp/catalog                                         | Local cache directory (SQLite DB)     |
| CATALOG_DOWNLOAD_TIMEOUT  | 30s                                                                   | HTTP download timeout                 |
| CATALOG_CACHE_TTL         | 6h                                                                    | Fallback cache validity duration      |

**Example Configuration:**

```bash
export CATALOG_INDEX_URL="https://internal-mirror.example.com/catalog/index.json"
export CATALOG_CACHE_DIR="/opt/mcp-cache/catalog"
export CATALOG_CACHE_TTL="12h"
```

**Configuration Notes:**

- **CATALOG_INDEX_URL**: Points to the JSON index endpoint; can be overridden for private mirrors
- **CATALOG_CACHE_DIR**: Directory containing `catalog.db` SQLite database file
- **CATALOG_CACHE_TTL**: Used as fallback when timestamp-based validation fails; normally cache is validated by comparing `metadata.generated` timestamps
- **SQLite Database**: Cache is stored in a single SQLite database file for optimal performance

## Cache Behavior

The catalog manager implements intelligent caching using SQLite for optimal performance:

1. **First Request**: Downloads JSON index, parses entries, stores in SQLite database
2. **Subsequent Requests**: Queries local SQLite cache for instant responses
3. **Cache Validation**: Compares cached `metadata.generated` timestamp with upstream index
4. **Automatic Refresh**: Updates cache when upstream timestamp is newer
5. **Manual Refresh**: Use `refresh=true` parameter to force immediate update
6. **Fallback TTL**: Uses CATALOG_CACHE_TTL when timestamp comparison fails

**Cache Directory Structure:**

```
/var/lib/k0rdent-mcp/catalog/
└── catalog.db                      # SQLite database with indexed catalog data
```

**SQLite Schema:**

- **metadata**: Stores index URL, generated timestamp, cache creation time
- **entries**: Stores app slug, title, summary, tags (JSON), validated platforms (JSON)
- **versions**: Stores version name, chart version, repository URL, manifest paths

**Cache Management:**

- Cache persists across server restarts
- Single SQLite file reduces disk I/O
- Timestamp-based validation eliminates need for content hashing
- Manual cleanup is safe (server will re-download on next request)
- Database uses write-ahead logging (WAL) for concurrent reads

## Understanding Installation

When you call `k0rdent.catalog.install_servicetemplate`, the server:

1. **Queries Cache**: Looks up version metadata in SQLite database
2. **Fetches Manifests**: Downloads ServiceTemplate and optional HelmRepository YAML from GitHub raw URLs
3. **Validates Namespace**: Checks target namespace against configured namespace filter (if enabled)
4. **Applies Resources**: Uses server-side apply with field manager `k0rdent-mcp-server`
5. **Returns Results**: Lists all applied resources with their namespace, kind, and name

**Manifest Fetching:**
- Manifests are fetched on-demand from GitHub raw URLs (e.g., `https://raw.githubusercontent.com/k0rdent/catalog/main/apps/minio/...`)
- Not cached locally; always fetched fresh during installation
- Reduces storage requirements and ensures latest manifest content

**Important Notes:**

- **Management Cluster Only**: Installs resources to the management cluster, not child clusters
- **Does Not Deploy Services**: Creates ServiceTemplate definition only; actual service deployment requires MultiClusterService
- **Idempotent Operations**: Uses server-side apply, so repeated installs are safe
- **Uninstall Support**: Use `k0rdent.catalog.delete_servicetemplate` to remove installed templates
- **No Upgrade Support**: No automated upgrade mechanism (manual deletion and reinstall required)

## Usage Workflow

### Typical Installation Flow

1. **Discover Available Applications**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.catalog.list",
    "arguments": {}
  }
}
```

2. **Filter to Specific Application**

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.catalog.list",
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
    "name": "k0rdent.catalog.install_servicetemplate",
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

**Resolution**: Use `k0rdent.catalog.list` without filter to see all available apps.

**Version Not Found**

```json
{
  "error": "template \"nginx\" version \"999.0.0\" not found for app \"ingress-nginx\""
}
```

**Resolution**: Use `k0rdent.catalog.list` with app filter to see available versions.

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

**GitHub Raw URL 404 Errors**

```json
{
  "error": "fetch manifest from https://raw.githubusercontent.com/k0rdent/catalog/main/apps/minio/...: 404 Not Found"
}
```

**Resolution**:
- Verify the catalog version references a valid Git commit or branch
- Check network connectivity to raw.githubusercontent.com
- Ensure manifest paths in index.json are correct
- Try refreshing the cache with `refresh: true` to get updated paths

**JSON Index Parsing Failures**

```json
{
  "error": "parse catalog index: invalid character '<' looking for beginning of value"
}
```

**Resolution**:
- Verify CATALOG_INDEX_URL returns valid JSON (not HTML error page)
- Check if catalog endpoint is behind authentication or firewall
- Inspect the URL directly in a browser to see actual response
- Verify network proxies are not modifying the response

**Manifest Fetch Timeout**

```json
{
  "error": "fetch manifest: context deadline exceeded"
}
```

**Resolution**:
- Increase CATALOG_DOWNLOAD_TIMEOUT if network is slow
- Check if GitHub raw URLs are being rate-limited
- Verify firewall is not blocking raw.githubusercontent.com
- Consider using a local mirror for better reliability

## Limitations

### Current Limitations

- **Management Cluster Only**: Cannot directly install to child clusters
- **No Upgrade Tool**: No automated upgrade path (requires manual deletion and reinstallation)
- **No Dependency Management**: Does not handle dependencies between templates
- **No Validation**: Does not validate template compatibility with cluster
- **No Rollback**: No automated rollback mechanism for failed installations
- **No Offline Mode**: Requires network access (no air-gapped support)
- **No Signature Verification**: Does not verify JSON index signatures
- **GitHub Dependency**: Manifest fetching depends on raw.githubusercontent.com availability

### Planned Enhancements

Future versions may include:
- Air-gapped/offline mode with catalog mirror support
- Signature verification for JSON index and manifests
- Automated upgrade tools with version comparison
- Dependency resolution and installation
- Pre-installation validation and compatibility checks
- Integration with MultiClusterService creation workflow
- Manifest caching for offline operation

## Security Considerations

### Trust Model

- **Catalog Source**: By default, trusts content from https://catalog.k0rdent.io and GitHub raw URLs
- **No Signature Verification**: Currently does not verify cryptographic signatures on index or manifests
- **Server-Side Apply**: Uses field manager to track ownership but does not prevent conflicts
- **GitHub Dependency**: Trusts manifest content from raw.githubusercontent.com

### Best Practices

1. **Use Private Mirror**: In production, consider hosting JSON index and manifests on private infrastructure
2. **Review Before Install**: Always review ServiceTemplate manifests before installation
3. **Namespace Isolation**: Use namespace filters to restrict installation scope
4. **RBAC Controls**: Apply principle of least privilege to service account permissions
5. **Network Policies**: Restrict egress to only required endpoints (catalog.k0rdent.io, raw.githubusercontent.com, GHCR)
6. **Cache Security**: Ensure cache directory has appropriate filesystem permissions (0755 directories, 0644 files)
7. **Audit Logging**: Enable structured logging to track catalog operations
8. **Monitor GitHub Access**: Track GitHub raw URL availability and rate limits

## Performance Considerations

### Optimization Tips

- **Cache Warmup**: Perform an initial `k0rdent.catalog.list` call on server startup to warm SQLite cache
- **Adjust TTL**: Increase `CATALOG_CACHE_TTL` if catalog updates are infrequent (though timestamp validation is primary mechanism)
- **Local Mirror**: Use `CATALOG_INDEX_URL` to point to a local mirror for faster downloads
- **Persistent Cache**: Mount cache directory to persistent volume in containerized environments
- **SQLite Performance**: Cache directory should be on fast storage (SSD recommended) for optimal query performance

### Resource Usage

- **Disk Space**: JSON index is ~100 KB; SQLite database is < 1 MB
- **Memory**: SQLite queries require minimal memory (typically < 5 MB)
- **Network**: Initial download is ~100 KB; manifest fetches are 1-10 KB each on-demand
- **CPU**: JSON parsing and SQLite operations complete in < 100 ms

### Performance Comparison

**JSON Index Architecture vs Legacy Tarball:**
- **Download Size**: ~100 KB vs 1-5 MB (95% reduction)
- **Processing Time**: ~50 ms vs ~900 ms (18x faster)
- **Disk I/O**: Single SQLite file vs extracted directory tree
- **Cache Validation**: Timestamp comparison vs SHA256 hashing

## Troubleshooting

### Enable Debug Logging

Set the server's log level to debug to see detailed catalog operations:

```bash
export LOG_LEVEL=debug
```

Debug logs include:
- Cache hit/miss decisions
- Download progress and timing
- SQLite query details
- Manifest fetch URLs and timing
- Timestamp comparison results

### Inspect Cache

Check SQLite database to verify state:

```bash
sqlite3 /var/lib/k0rdent-mcp/catalog/catalog.db ".tables"
```

Query cache metadata:

```bash
sqlite3 /var/lib/k0rdent-mcp/catalog/catalog.db "SELECT * FROM metadata;"
```

List all cached applications:

```bash
sqlite3 /var/lib/k0rdent-mcp/catalog/catalog.db "SELECT slug, title FROM entries;"
```

### Force Cache Refresh

Use the refresh parameter to bypass cache:

```json
{
  "method": "tools/call",
  "params": {
    "name": "k0rdent.catalog.list",
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

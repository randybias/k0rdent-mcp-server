# Design: Migrate Catalog to JSON Index

## Current Architecture

```
GitHub Tarball (1-5 MB)
    ↓ download & extract
Disk: catalog-<sha>/apps/**/*.yaml
    ↓ parse YAML files
SQLite: apps + service_templates tables
    ↓ query
CatalogEntry[]
```

**Problems:**
- Complex: Tarball handling, extraction, YAML parsing, SQL schema
- Slow: Multiple I/O operations (download, extract, parse, index)
- Large: 1-5 MB download, 5-20 MB extracted
- Brittle: Multiple failure points (tar, YAML, SQL)

## New Architecture

```
JSON Index (~100 KB)
    ↓ download & parse
SQLite Database (persistent cache)
    ↓ query
CatalogEntry[]
```

**Installation Flow:**
```
Install Request
    ↓ lookup in SQLite
Manifest URLs (GitHub raw)
    ↓ fetch on-demand
ServiceTemplate + HelmRepository YAML
    ↓ apply to cluster
```

**Benefits:**
- Simple: Single HTTP fetch + JSON parse (no tarball extraction)
- Fast: No tarball extraction, direct JSON parsing
- Small: ~100 KB vs 1-5 MB download
- Persistent: SQLite cache survives restarts
- Extensible: Can track installs, preferences, analytics in future

## Data Mapping

### JSON Index → CatalogEntry

**JSON Structure:**
```json
{
  "metadata": {
    "generated": "2025-11-06T15:02:01",
    "version": "1.0.0"
  },
  "addons": [
    {
      "name": "minio",
      "description": "High Performance Object Storage",
      "latestVersion": "14.1.2",
      "versions": ["14.1.2"],
      "charts": [
        {
          "name": "minio",
          "versions": ["14.1.2"]
        }
      ],
      "metadata": {
        "tags": ["Storage"],
        "owner": "k0rdent-team"
      }
    }
  ]
}
```

**Mapping:**
- `addons[].name` → `CatalogEntry.Slug`
- `addons[].name` → `CatalogEntry.Title` (capitalize/format)
- `addons[].description` → `CatalogEntry.Summary`
- `addons[].metadata.tags` → `CatalogEntry.Tags`
- `addons[].charts[].name` → `ServiceTemplateVersion.Name`
- `addons[].charts[].versions[]` → `ServiceTemplateVersion.Version`

### Manifest URL Construction

**Pattern:**
```
https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/{slug}/charts/{name}-service-template-{version}/templates/service-template.yaml
```

**HelmRepository:**
```
https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/k0rdent-utils/charts/k0rdent-catalog-1.0.0/templates/helm-repository.yaml
```

## Component Changes

### 1. Manager (internal/catalog/manager.go)

**Remove:**
- `extractTarball()` method
- Tarball extraction logic

**Add:**
- `fetchJSONIndex()` method
- `parseJSONIndex()` method
- `indexTimestamp` field (for tracking `metadata.generated` from JSON)

**Modify:**
- `buildDatabaseIndex()` - parse JSON instead of YAML files, keep SQLite indexing
- `List()` - keep existing SQL queries (no change needed)
- `GetManifests()` - fetch from GitHub raw URLs instead of disk
- `loadOrRefreshIndex()` - download JSON instead of tarball, check timestamp for cache invalidation

### 2. Types (internal/catalog/types.go)

**Keep:**
- `CatalogEntry` (public API)
- `ServiceTemplateVersion` (public API)
- `CacheMetadata` (for cache tracking)
- `Options` (configuration)

**Add:**
- `JSONIndex` struct matching catalog JSON schema
- `JSONAddon` struct for parsing
- `JSONChart` struct for chart data
- `JSONMetadata` struct for tracking timestamp

### 3. Database (internal/catalog/database.go)

**Keep:** Existing SQLite database logic

**Modify (if needed):**
- Simplify schema if beneficial
- Add timestamp tracking field to cache metadata

### 4. Schema (internal/catalog/schema.sql)

**Keep:** Existing schema

**Modify (if needed):**
- Add column for JSON index timestamp
- Simplify if any tarball-specific fields exist

### 5. Index (internal/catalog/index.go)

**Modify:**
- Replace YAML file parsing with JSON parsing
- Add manifest URL construction for GitHub raw URLs
- Update `buildDatabaseIndex()` to populate SQLite from JSON instead of YAML files
- Keep SQLite query logic for filtering

### 6. Config (internal/catalog/config.go)

**Modify:**
- Change `DefaultArchiveURL` to JSON index URL
- Update env var names for clarity
- Keep cache TTL and directory settings

### 7. Tools (internal/tools/core/catalog.go)

**Add:**
- `k0.catalog.delete_servicetemplate` tool handler
- `catalogDeleteInput` type
- `catalogDeleteResult` type

**Rename:**
- `k0.catalog.install` → `k0.catalog.install_servicetemplate`

**Modify:**
- Update `GetManifests` to fetch from GitHub raw URLs instead of disk

### 8. Tests

**Unit Tests:**
- Replace tarball fixtures with JSON fixtures
- Update assertions for JSON parsing
- Keep SQLite-specific tests (no change needed)
- Add timestamp-based cache invalidation tests

**Integration Tests:**
- Update to use JSON index instead of tarball
- Add delete validation tests
- Test install → delete → verify removal flow
- Verify SQLite cache persistence across restarts

## Delete Tool Design

### Tool: k0.catalog.delete_servicetemplate

**Purpose:** Remove ServiceTemplate and associated HelmRepository from namespace(s)

**Parameters:**
```go
type catalogDeleteInput struct {
    App           string   `json:"app"`            // App slug for identification
    Template      string   `json:"template"`       // Template name
    Version       string   `json:"version"`        // Version to delete
    Namespace     string   `json:"namespace,omitempty"`      // Specific namespace
    AllNamespaces bool     `json:"all_namespaces,omitempty"` // Delete from all
}
```

**Returns:**
```go
type catalogDeleteResult struct {
    Deleted []string `json:"deleted"`  // List of deleted resources
    Status  string   `json:"status"`   // "deleted" or "not_found"
}
```

**Logic:**
1. Resolve target namespaces (same as install)
2. For each namespace:
   - Delete ServiceTemplate by name
   - Optionally delete HelmRepository if not used by others
3. Return list of deleted resources

**Namespace Behavior:**
- DEV_ALLOW_ANY mode: Defaults to kcm-system
- OIDC_REQUIRED mode: Requires explicit namespace or all_namespaces

## Cache Strategy

### SQLite Persistent Cache

**Structure:**
```go
type CacheMetadata struct {
    URL              string
    Timestamp        time.Time  // When we fetched it
    IndexTimestamp   string     // metadata.generated from JSON
    LastCheck        time.Time  // Last time we checked for updates
}
```

**Invalidation:**
1. Check cache age against TTL (e.g., 1 hour)
2. If expired, fetch JSON index
3. Compare `metadata.generated` timestamp from JSON with cached `IndexTimestamp`
4. If timestamps match, update `LastCheck` only (no rebuild)
5. If timestamps differ, parse JSON and rebuild SQLite index

### Cache Key

**Current:** `catalog-<tarball-sha>` directory on disk
**New:** Single SQLite database with timestamp tracking

**Benefits:**
- Persistent cache across restarts (no re-download)
- Simple timestamp comparison (no hashing needed)
- Future extensibility for tracking installs, preferences
- Query capabilities for filtering and search

## Error Handling

### JSON Fetch Failures

**Network Error:**
```json
{
  "error": "fetch catalog index: Get \"https://catalog.k0rdent.io/...\": dial tcp: connection refused"
}
```

**Invalid JSON:**
```json
{
  "error": "parse catalog index: invalid character '<' looking for beginning of value"
}
```

**Schema Mismatch:**
```json
{
  "error": "validate catalog index: missing required field 'addons'"
}
```

### Manifest Fetch Failures

**GitHub Rate Limit:**
```json
{
  "error": "fetch manifest: rate limit exceeded, retry after 3600s"
}
```

**Manifest Not Found:**
```json
{
  "error": "fetch service-template.yaml: 404 not found"
}
```

### Delete Failures

**Resource Not Found:**
```json
{
  "deleted": [],
  "status": "not_found",
  "message": "ServiceTemplate \"minio-14-1-2\" not found in namespace \"kcm-system\""
}
```

**Permission Denied:**
```json
{
  "error": "delete ServiceTemplate: servicetemplates.k0rdent.mirantis.com \"minio-14-1-2\" is forbidden"
}
```

## Migration Strategy

### Single-Phase Migration

Since we're keeping SQLite and the same database schema, migration is straightforward:

1. **Update data source**: Change from tarball extraction to JSON index fetch
2. **Update parsing logic**: Replace YAML file parsing with JSON parsing
3. **Update manifest fetching**: Change from disk reads to GitHub raw URL fetches
4. **Add delete tool**: Implement `k0.catalog.delete_servicetemplate`
5. **Rename install tool**: `k0.catalog.install` → `k0.catalog.install_servicetemplate`
6. **Update tests**: Replace tarball fixtures with JSON fixtures
7. **Verify**: Run all unit and integration tests

**Benefits of keeping SQLite:**
- No data migration needed
- Database structure stays the same
- Existing query logic continues to work
- Can be done in single PR

**Timeline:** Single PR with comprehensive testing

## Performance Comparison

### Download Size

| Metric | Tarball | JSON Index | Improvement |
|--------|---------|------------|-------------|
| Compressed | 1-5 MB | ~100 KB | 10-50x smaller |
| Extracted | 5-20 MB | N/A | No extraction needed |

### Processing Time

| Operation | Tarball | JSON Index | Improvement |
|-----------|---------|------------|-------------|
| Download | ~500ms | ~50ms | 10x faster |
| Extract | ~200ms | 0ms | Eliminated |
| Parse | ~300ms | ~10ms | 30x faster |
| Index | ~100ms | 0ms | Eliminated |
| **Total** | ~1100ms | ~60ms | **18x faster** |

### Memory Usage

| Component | Tarball | JSON Index | Change |
|-----------|---------|------------|--------|
| SQLite DB | ~2 MB | ~2 MB | Same (kept for persistence) |
| Extracted Tarball | 5-20 MB | 0 MB | Eliminated |
| Total Disk | 7-22 MB | ~2 MB | 3.5-11x smaller |

## Testing Strategy

### Unit Tests

1. **JSON Parsing**
   - Valid JSON → successful parse
   - Invalid JSON → error
   - Missing fields → error
   - Empty addons → empty results

2. **Manifest URL Construction**
   - Correct pattern for ServiceTemplate
   - Correct pattern for HelmRepository
   - Version formatting (with/without 'v' prefix)

3. **Cache Invalidation**
   - TTL expiry triggers refresh
   - SHA change triggers rebuild
   - ETag 304 updates timestamp only

4. **Delete Tool**
   - Single namespace delete
   - All namespaces delete
   - Namespace filter validation
   - Resource not found handling

### Integration Tests

1. **Live JSON Fetch**
   - Download real index
   - Parse successfully
   - List all addons (expect 79+)
   - Filter by app slug

2. **Live Manifest Fetch**
   - Fetch minio manifests from GitHub
   - Parse YAML successfully
   - Apply to cluster

3. **End-to-End Lifecycle**
   - Install ServiceTemplate → verify exists
   - Delete ServiceTemplate → verify removed
   - Reinstall → verify works again

4. **Cache Behavior**
   - First call downloads
   - Second call uses cache (fast)
   - Expired cache triggers refresh

## Rollback Plan

If issues arise:

1. **Immediate:** Revert commit, redeploy previous version
2. **Data:** No data migration needed (stateless)
3. **Config:** Change `CATALOG_INDEX_URL` back to tarball URL
4. **Tests:** Previous test suite still available in git history

## Open Questions

1. **Manifest caching:** Should we cache fetched manifests in memory?
   - **Decision:** Start without manifest cache, add if needed for performance

2. **HelmRepository sharing:** When multiple templates share a HelmRepository, should delete be reference-counted?
   - **Decision:** For simplicity, delete always removes HelmRepository (user can reinstall if needed)

3. **Version normalization:** JSON has "v1.3.0" and "1.2.46" formats - normalize?
   - **Decision:** Store as-is, normalize during comparison/construction

4. **GitHub URL template:** Hardcode or configure?
   - **Decision:** Hardcode with comment about future configuration if needed

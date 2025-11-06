# Proposal: Migrate Catalog to JSON Index

## Problem

The current catalog implementation downloads and extracts a 1-5 MB tarball from GitHub, then parses YAML files from disk to build an SQLite index. This approach:
- Requires tarball extraction and disk I/O
- Maintains a complex SQLite schema
- Parses multiple YAML files per app
- Stores manifest file paths that still require GitHub access for actual manifests

Meanwhile, an official JSON index exists at `https://catalog.k0rdent.io/latest/index.json` that:
- Contains 79 addons with complete metadata (names, descriptions, versions, charts)
- Includes structured metadata (tags, dependencies, quality metrics, support type)
- Provides direct URLs for documentation and assets
- Is much smaller (~100 KB vs 1-5 MB compressed tarball)
- Eliminates the need for tarball extraction and parsing

## Solution

Replace the tarball-based catalog system with a JSON index-based approach:

1. **Download JSON index** from `https://catalog.k0rdent.io/latest/index.json` instead of tarball
2. **Parse JSON directly** into catalog types (no YAML parsing, no tarball extraction)
3. **Use JSON timestamp** (`metadata.generated`) for cache invalidation instead of SHA256
4. **Keep SQLite database** for persistent cache and future extensibility
5. **Fetch manifests on-demand** from GitHub raw URLs when installing
6. **Add delete tool** (`k0.catalog.delete_servicetemplate`) for removing ServiceTemplates from namespaces
7. **Rename install tool** to `k0.catalog.install_servicetemplate` for clarity

## Benefits

- **Simpler architecture**: No tarball extraction, direct JSON parsing, simpler indexing
- **Faster**: 18x faster download and processing (100 KB JSON vs 1-5 MB tarball)
- **Smaller downloads**: ~100 KB JSON vs 1-5 MB tarball (10-50x reduction)
- **Less disk usage**: No extracted tarball directory tree (~5-20 MB saved)
- **Persistent cache**: SQLite retains cache across restarts (no re-download)
- **Future-proof**: SQLite enables install tracking, preferences, analytics
- **Easier testing**: Mock JSON responses instead of tar archives
- **Complete lifecycle**: Add delete capability for end-to-end testing
- **Clear tool names**: `install_servicetemplate` and `delete_servicetemplate` are unambiguous

## Scope

### In Scope
- Replace tarball download with JSON index fetch
- Parse JSON index into existing `CatalogEntry` types
- Use JSON `metadata.generated` timestamp for cache invalidation
- Keep SQLite database, simplify schema if beneficial
- Fetch manifests from GitHub raw URLs on install
- Rename `k0.catalog.install` to `k0.catalog.install_servicetemplate`
- Add `k0.catalog.delete_servicetemplate` tool for removing ServiceTemplates
- Update all unit tests with JSON fixtures
- Update integration tests to use JSON index
- Update documentation

### Out of Scope
- Changing the MCP tool API response formats (preserve backward compatibility)
- Adding new catalog features beyond delete (e.g., update, rollback)
- Supporting offline/air-gapped modes (separate future work)
- Signature verification (separate future work)
- Removing SQLite database (keeping for persistent cache and extensibility)

## Migration Path

The change is mostly internal with one breaking change for tool naming:
- `k0.catalog.list` - Same parameters and response format (no change)
- `k0.catalog.install` → `k0.catalog.install_servicetemplate` - Renamed for clarity, same parameters and response format
- `k0.catalog.delete_servicetemplate` - New tool with similar parameter pattern to install

## Risks

1. **GitHub raw URL stability**: Manifests fetched from `raw.githubusercontent.com` - if this changes, we need fallback
   - *Mitigation*: Add URL template configuration, document alternative sources

2. **JSON index format changes**: If `catalog.k0rdent.io` changes schema, parsing breaks
   - *Mitigation*: Validate JSON structure, fail gracefully with clear errors

3. **Cache invalidation**: Need to detect when JSON index changes
   - *Mitigation*: Use `metadata.generated` timestamp field from JSON index to track updates

4. **Breaking change**: Tool rename from `k0.catalog.install` to `k0.catalog.install_servicetemplate`
   - *Mitigation*: Document in release notes, consider deprecation period if needed

## Success Criteria

- All existing unit tests pass with JSON-based implementation
- All integration tests pass using JSON index
- Delete tool can remove ServiceTemplates and verify removal
- Documentation updated with new architecture
- No regressions in catalog list/install functionality
- Performance equal or better than tarball approach

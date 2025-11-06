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
3. **Fetch manifests on-demand** from GitHub raw URLs when installing
4. **Remove SQLite dependency** - store parsed JSON in memory with simple cache
5. **Add delete tool** (`k0.catalog.delete`) for removing ServiceTemplates from namespaces

## Benefits

- **Simpler architecture**: No tarball extraction, no SQLite, no complex indexing
- **Faster**: JSON parsing is faster than tarball + YAML + SQLite
- **Smaller downloads**: ~100 KB JSON vs 1-5 MB tarball
- **Less disk usage**: No extracted tarball directory tree
- **Easier testing**: Mock JSON responses instead of tar archives
- **Complete lifecycle**: Add delete capability for end-to-end testing

## Scope

### In Scope
- Replace tarball download with JSON index fetch
- Parse JSON index into existing `CatalogEntry` types
- Fetch manifests from GitHub raw URLs on install
- Remove SQLite database and schema
- Update all unit tests with JSON fixtures
- Update integration tests to use JSON index
- Add `k0.catalog.delete` tool for removing ServiceTemplates
- Update documentation

### Out of Scope
- Changing the MCP tool API signatures (preserve backward compatibility)
- Adding new catalog features beyond delete
- Supporting offline/air-gapped modes (separate future work)
- Signature verification (separate future work)

## Migration Path

The change is mostly internal - the MCP tool interface remains the same:
- `k0.catalog.list` - Same parameters and response format
- `k0.catalog.install` - Same parameters and response format
- `k0.catalog.delete` - New tool with similar parameter pattern

## Risks

1. **GitHub raw URL stability**: Manifests fetched from `raw.githubusercontent.com` - if this changes, we need fallback
   - *Mitigation*: Add URL template configuration, document alternative sources

2. **JSON index format changes**: If `catalog.k0rdent.io` changes schema, parsing breaks
   - *Mitigation*: Validate JSON structure, fail gracefully with clear errors

3. **Cache invalidation**: Need to detect when JSON index changes
   - *Mitigation*: Use ETags or Last-Modified headers, implement SHA256 hashing

## Success Criteria

- All existing unit tests pass with JSON-based implementation
- All integration tests pass using JSON index
- Delete tool can remove ServiceTemplates and verify removal
- Documentation updated with new architecture
- No regressions in catalog list/install functionality
- Performance equal or better than tarball approach

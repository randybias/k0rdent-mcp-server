# Implementation Tasks

## 1. JSON Index Fetching and Parsing
- [x] 1.1 Add `JSONIndex`, `JSONAddon`, `JSONChart`, `JSONMetadata` types to `internal/catalog/types.go`
- [x] 1.2 Update `DefaultArchiveURL` in `config.go` to `https://catalog.k0rdent.io/latest/index.json`
- [x] 1.3 Implement `fetchJSONIndex()` method in `manager.go`
- [x] 1.4 Implement `parseJSONIndex()` method to convert JSON → entries for SQLite
- [x] 1.5 Add unit tests for JSON parsing with valid/invalid/empty fixtures

## 2. Timestamp-Based Cache
- [x] 2.1 Add `IndexTimestamp` field to `CacheMetadata` struct to store `metadata.generated`
- [x] 2.2 Update cache invalidation logic to compare JSON timestamp instead of tarball SHA
- [x] 2.3 Modify `loadOrRefreshIndex()` to check timestamp before rebuilding
- [x] 2.4 Keep existing SQLite database and schema (no removal)
- [x] 2.5 Add unit tests for timestamp-based cache invalidation

## 3. Manifest Fetching
- [x] 3.1 Implement `constructManifestURL()` for ServiceTemplate paths
- [x] 3.2 Implement `constructHelmRepoURL()` for HelmRepository path
- [x] 3.3 Update `GetManifests()` to fetch from GitHub raw URLs instead of disk
- [x] 3.4 Add HTTP client with timeout and retry logic for manifest fetches
- [x] 3.5 Add unit tests with mocked HTTP responses

## 4. Update Indexing Logic
- [x] 4.1 Remove tarball extraction logic from `manager.go`
- [x] 4.2 Update `buildDatabaseIndex()` in `index.go` to parse JSON instead of YAML files
- [x] 4.3 Keep SQLite schema, add `index_timestamp` column if needed
- [x] 4.4 Clean up old tarball-based test fixtures
- [x] 4.5 Update `List()` method to work with JSON-sourced data (should need minimal changes)

## 5. Delete Tool Implementation
- [x] 5.1 Add `catalogDeleteServiceTemplateTool` struct to `internal/tools/core/catalog.go`
- [x] 5.2 Add `catalogDeleteInput` and `catalogDeleteResult` types
- [x] 5.3 Implement `delete()` method with namespace resolution (reuse install logic)
- [x] 5.4 Register `k0rdent.catalog.delete_servicetemplate` tool in `registerCatalog()`
- [x] 5.5 Add unit tests for delete with namespace filtering

## 6. Rename Install Tool
- [x] 6.1 Rename `k0rdent.catalog.install` to `k0rdent.catalog.install_servicetemplate`
- [x] 6.2 Update tool registration in `registerCatalog()`
- [x] 6.3 Update all references in tests
- [x] 6.4 Update documentation with new tool name

## 7. Update Integration Tests
- [x] 7.1 Update `catalog_live_test.go` to expect 79+ addons from JSON index
- [x] 7.2 Update manifest fetch tests to use GitHub raw URLs
- [x] 7.3 Add end-to-end test: install → verify → delete → verify removed
- [x] 7.4 Test SQLite cache persistence across restarts
- [x] 7.5 Ensure all integration tests pass with new implementation

## 8. Documentation Updates
- [x] 8.1 Update `docs/catalog.md` with new architecture (JSON index instead of tarball)
- [x] 8.2 Document `k0rdent.catalog.delete_servicetemplate` tool with examples
- [x] 8.3 Document `k0rdent.catalog.install_servicetemplate` (renamed tool)
- [x] 8.4 Update configuration section (new default URL, keep SQLite references)
- [x] 8.5 Update troubleshooting section (GitHub raw URL issues, JSON parsing errors)
- [x] 8.6 Update performance metrics (faster download, no extraction)

## 9. Validation and Cleanup
- [x] 9.1 Run all unit tests and ensure they pass
- [x] 9.2 Run integration tests with live JSON index
- [x] 9.3 Test delete tool with real cluster
- [x] 9.4 Verify no performance regressions (should be faster)
- [x] 9.5 Run `go mod tidy` (no dependencies removed since keeping SQLite)
- [x] 9.6 Run `openspec validate migrate-catalog-to-json-index --strict`

## Dependencies

- Tasks 1.x must complete before 2.x (need types for timestamp cache)
- Tasks 2.x must complete before 4.x (need cache logic before updating indexing)
- Tasks 1-2 can proceed before 3.x (manifest fetching is independent)
- Tasks 5.x and 6.x can be done in parallel with 1-4 (tool changes are independent)
- Tasks 7-8 depend on 1-6 being complete
- Task 9 is final validation

## Parallelizable Work

- JSON parsing (1.x), Delete tool (5.x), and Rename tool (6.x) can be developed in parallel
- Manifest fetching (3.x) can be done in parallel with cache updates (2.x)
- Documentation (8.x) can start once design is finalized
- Unit tests can be written alongside implementation

## Estimated Effort

- JSON Index + Timestamp Cache: ~4 hours
- Manifest Fetching: ~2 hours
- Update Indexing (no SQLite removal): ~2 hours
- Delete Tool: ~2 hours
- Rename Install Tool: ~1 hour
- Update Tests: ~3 hours
- Documentation: ~2 hours
- Validation: ~1 hour

**Total: ~17 hours** (can be parallelized to ~10-12 hours with multiple developers)

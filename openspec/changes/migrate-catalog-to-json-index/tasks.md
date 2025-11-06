# Implementation Tasks

## 1. JSON Index Fetching and Parsing
- [ ] 1.1 Add `JSONIndex`, `JSONAddon`, `JSONChart`, `JSONMetadata` types to `internal/catalog/types.go`
- [ ] 1.2 Update `DefaultArchiveURL` in `config.go` to `https://catalog.k0rdent.io/latest/index.json`
- [ ] 1.3 Implement `fetchJSONIndex()` method in `manager.go` with ETag support
- [ ] 1.4 Implement `parseJSONIndex()` method to convert JSON → `[]CatalogEntry`
- [ ] 1.5 Add unit tests for JSON parsing with valid/invalid/empty fixtures

## 2. In-Memory Cache
- [ ] 2.1 Replace SQLite database field with `indexCache` struct in Manager
- [ ] 2.2 Implement in-memory filtering in `List()` method (replace SQL queries)
- [ ] 2.3 Update cache invalidation logic to use JSON SHA256 instead of tarball SHA
- [ ] 2.4 Add cache TTL checks and refresh logic
- [ ] 2.5 Add unit tests for cache hit/miss/expiry scenarios

## 3. Manifest Fetching
- [ ] 3.1 Implement `constructManifestURL()` for ServiceTemplate paths
- [ ] 3.2 Implement `constructHelmRepoURL()` for HelmRepository path
- [ ] 3.3 Update `GetManifests()` to fetch from GitHub raw URLs instead of disk
- [ ] 3.4 Add HTTP client with timeout and retry logic for manifest fetches
- [ ] 3.5 Add unit tests with mocked HTTP responses

## 4. Remove Old Implementation
- [ ] 4.1 Delete `internal/catalog/database.go` file
- [ ] 4.2 Delete `internal/catalog/schema.sql` file
- [ ] 4.3 Remove SQLite dependency from `go.mod` (`modernc.org/sqlite`)
- [ ] 4.4 Remove tarball extraction logic from `manager.go`
- [ ] 4.5 Remove `buildDatabaseIndex()` from `index.go`
- [ ] 4.6 Clean up old tarball-based test fixtures

## 5. Delete Tool Implementation
- [ ] 5.1 Add `catalogDeleteTool` struct to `internal/tools/core/catalog.go`
- [ ] 5.2 Add `catalogDeleteInput` and `catalogDeleteResult` types
- [ ] 5.3 Implement `delete()` method with namespace resolution (reuse install logic)
- [ ] 5.4 Register `k0.catalog.delete` tool in `registerCatalog()`
- [ ] 5.5 Add unit tests for delete with namespace filtering

## 6. Update Integration Tests
- [ ] 6.1 Update `catalog_live_test.go` to expect 79+ addons from JSON index
- [ ] 6.2 Update manifest fetch tests to use GitHub raw URLs
- [ ] 6.3 Add end-to-end test: install → verify → delete → verify removed
- [ ] 6.4 Update cache performance tests (should be faster now)
- [ ] 6.5 Ensure all integration tests pass with new implementation

## 7. Documentation Updates
- [ ] 7.1 Update `docs/catalog.md` with new architecture (JSON index instead of tarball)
- [ ] 7.2 Document `k0.catalog.delete` tool with examples
- [ ] 7.3 Update configuration section (new default URL, removed SQLite refs)
- [ ] 7.4 Update troubleshooting section (GitHub raw URL issues, JSON parsing errors)
- [ ] 7.5 Update performance metrics (faster download, no extraction)
- [ ] 7.6 Remove SQLite-specific troubleshooting steps

## 8. Validation and Cleanup
- [ ] 8.1 Run all unit tests and ensure they pass
- [ ] 8.2 Run integration tests with live JSON index
- [ ] 8.3 Test delete tool with real cluster
- [ ] 8.4 Verify no performance regressions (should be faster)
- [ ] 8.5 Run `go mod tidy` to remove unused dependencies
- [ ] 8.6 Run `openspec validate migrate-catalog-to-json-index --strict`

## Dependencies

- Tasks 1.x must complete before 2.x (need types for cache)
- Tasks 2.x must complete before 3.x (need cache for manifest lookups)
- Tasks 1-3 can proceed before 4.x (old system can coexist temporarily)
- Task 5.x can be done in parallel with 1-3
- Tasks 6-7 depend on 1-5 being complete
- Task 8 is final validation

## Parallelizable Work

- JSON parsing (1.x) and Delete tool (5.x) can be developed in parallel
- Documentation (7.x) can start once design is finalized
- Unit tests can be written alongside implementation

## Estimated Effort

- JSON Index + Cache: ~4 hours
- Manifest Fetching: ~2 hours
- Delete Tool: ~2 hours
- Remove Old Code: ~1 hour
- Update Tests: ~3 hours
- Documentation: ~2 hours
- Validation: ~1 hour

**Total: ~15 hours** (can be parallelized to ~8-10 hours with multiple developers)

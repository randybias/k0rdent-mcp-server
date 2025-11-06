# Implementation Tasks

## 1. Catalog manager
- [x] 1.1 Introduce `internal/catalog` package that downloads and extracts the catalog archive into a cache directory.
- [x] 1.2 Parse `data.yaml` and `charts/st-charts.yaml` per app to build an index of available ServiceTemplates (slug, title, summary, versions, manifest paths).
- [x] 1.3 Persist cache metadata (commit sha / ETag, timestamp) and support explicit refresh operations.

## 2. Runtime wiring
- [x] 2.1 Instantiate the catalog manager in `cmd/server` startup and expose it through the runtime/session (respecting graceful shutdown).
- [x] 2.2 Add configuration knobs (`CATALOG_ARCHIVE_URL`, `CATALOG_CACHE_DIR`, download timeout) with sane defaults and logging.

## 3. MCP tools
- [x] 3.1 Register `k0.catalog.list` and `k0.catalog.install` in the core tool registry.
- [x] 3.2 Implement `list` handler to return catalog metadata (with optional slug filter) using cached index and namespace filter awareness.
- [x] 3.3 Implement `install` handler that loads the requested manifest bundle, applies the helper `HelmRepository` (if required), and applies the ServiceTemplate via server-side apply.
- [x] 3.4 Emit structured logs/metrics for list/install success and failure.

## 4. Testing
- [x] 4.1 Add catalog manager unit tests with tarball fixtures covering happy path, corrupt archive, and refresh flows.
- [x] 4.2 Add tool handler tests using fake dynamic client verifying success, missing slug/template/version, namespace filter rejection, and apply conflicts.
- [x] 4.3 Exercise cache invalidation via `--refresh` or similar input flag in tests.

## 5. Documentation & validation
- [x] 5.1 Document the new MCP tools (usage examples, prerequisites) in developer docs.
- [x] 5.2 Update operator guidance to mention required outbound network access and labels applied.
- [x] 5.3 Run `openspec validate add-catalog-install-tool --strict` and ensure CI passes.

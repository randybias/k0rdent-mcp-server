# Catalog Tools (delta)

## ADDED Requirements

### Requirement: Catalog listing tool
- The server **SHALL** expose `k0.catalog.list(app?: string, refresh?: bool) -> CatalogEntry[]` for discovering k0rdent catalog ServiceTemplates.
- Each `CatalogEntry` **SHALL** include the app `slug`, human-readable `title`, `summary`, tags, validated platform flags, and a list of `templates[]` with `name` and `versions[]`.
- The tool **SHALL** use a cached index by default and only re-download the catalog archive when the cache is empty or `refresh=true`.
- When `app` is provided, the result **SHALL** contain only the matching slug (or be empty when not found without raising an error).

#### Scenario: List all catalog entries from cache
- WHEN `k0.catalog.list()` is called after a successful catalog sync
- THEN the server returns metadata for all catalog apps and their ServiceTemplate versions
- AND no network request is made (cache hit)

#### Scenario: Force refresh
- WHEN `k0.catalog.list(refresh=true)` is called
- THEN the server re-downloads the catalog archive before responding
- AND the response reflects the refreshed index

### Requirement: Catalog install tool
- The server **SHALL** expose `k0.catalog.install(app: string, template: string, version: string) -> InstallResult` to install ServiceTemplates from the k0rdent catalog.
- The tool **SHALL** locate the requested manifest in the cached archive and, if missing, trigger a refresh before failing with `invalidParams`.
- Prior to applying the ServiceTemplate, the tool **SHALL** apply the referenced `HelmRepository` manifest when the ServiceTemplate’s `.spec.helm.chartSpec.sourceRef` references a repository bundled in the catalog (e.g., `k0rdent-catalog`).
- The tool **SHALL** apply Kubernetes resources via server-side apply using the runtime session’s dynamic client and label owned objects with `k0rdent.mirantis.com/managed=true`.
- Install operations **SHALL** be idempotent: re-running the tool with the same inputs updates the existing ServiceTemplate without error.
- Namespace filters **SHALL** be enforced: if the target manifest specifies a namespace not permitted by the filter, the tool returns an MCP `forbidden` error.

#### Scenario: Successful install
- WHEN `k0.catalog.install(app="minio", template="minio", version="14.1.2")` is called
- THEN the server applies (or updates) the `k0rdent-catalog` HelmRepository as needed
- AND applies the `ServiceTemplate` CR named `minio-14-1-2`
- AND returns an `InstallResult` indicating `created` or `updated`

#### Scenario: Missing template version
- WHEN `k0.catalog.install(app="minio", template="minio", version="9.9.9")` is called and that version does not exist
- THEN the server refreshes the catalog index
- AND returns an MCP error with code `invalidParams` describing the supported versions

#### Scenario: Namespace filter prevents install
- WHEN the namespace filter excludes `catalog-system` and the manifest declares that namespace
- THEN `k0.catalog.install` returns an MCP error with code `forbidden`
- AND no resources are applied

### Requirement: Catalog cache management
- The catalog manager **SHALL** store the most recent archive metadata (commit SHA and timestamp) and expose it to tool handlers.
- The cache **SHALL** expire after a configurable TTL (default 6 hours); calls after expiry trigger a background refresh before responding.
- Errors during refresh **SHALL** be logged and surfaced to the caller as MCP `unavailable` errors without serving stale, partially parsed data.

#### Scenario: Cache TTL refresh
- GIVEN the cache is older than 6 hours
- WHEN `k0.catalog.list()` is called
- THEN the server downloads a fresh archive before returning results

#### Scenario: Download failure
- WHEN a network error occurs while refreshing the catalog
- THEN the server logs the failure with context (URL, error)
- AND returns an MCP error with code `unavailable`
- AND the previous cache remains untouched

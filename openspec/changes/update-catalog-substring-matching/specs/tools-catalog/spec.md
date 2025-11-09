## MODIFIED Requirements

### Requirement: Catalog listing tool
- The server **SHALL** expose `k0rdent.catalog.serviceTemplates.list(app?: string, refresh?: bool) -> CatalogEntry[]` for discovering k0rdent catalog ServiceTemplates.
- Each `CatalogEntry` **SHALL** include the app `slug`, human-readable `title`, `summary`, tags, validated platform flags, and a list of `templates[]` with `name` and `versions[]`.
- The tool **SHALL** use a cached index by default and only re-download the catalog archive when the cache is empty or `refresh=true`.
- When `app` is provided, the result **SHALL** contain all slugs whose names contain the provided value (case-insensitive substring match); if no slug matches, an empty list is returned without error.

#### Scenario: List all catalog entries from cache
- WHEN `k0rdent.catalog.serviceTemplates.list()` is called after a successful catalog sync
- THEN the server returns metadata for all catalog apps and their ServiceTemplate versions
- AND no network request is made (cache hit)

#### Scenario: Force refresh
- WHEN `k0rdent.catalog.serviceTemplates.list(refresh=true)` is called
- THEN the server re-downloads the catalog archive before responding
- AND the response reflects the refreshed index

#### Scenario: Substring filter
- WHEN `k0rdent.catalog.serviceTemplates.list(app="nginx")` is called and the catalog contains the slug `ingress-nginx`
- THEN the response includes the `ingress-nginx` entry
- AND `app` values that do not match any slug return an empty list without error

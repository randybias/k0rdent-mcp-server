## MODIFIED Requirements

### Requirement: Catalog listing tool
- The MCP tool **SHALL** be renamed from `k0rdent.catalog.list` to `k0rdent.catalog.list` and maintain the same parameters and behaviours.

#### Scenario: List all catalog entries from cache
- WHEN `k0rdent.catalog.list()` is called after a successful catalog sync
- THEN the server returns metadata for all catalog apps and their ServiceTemplate versions without issuing a network request when the cache is warm

### Requirement: Catalog install tool
- The MCP tool **SHALL** be renamed from `k0rdent.catalog.install` to `k0rdent.catalog.install`.

#### Scenario: Successful install
- WHEN `k0rdent.catalog.install(app="minio", template="minio", version="14.1.2")` is called
- THEN the server applies (or updates) the required resources and returns an install result indicating success

### Requirement: Catalog cache management
- Any metrics, logs, or status surfaces that reference the tool name **SHALL** use the `k0rdent.catalog.*` identifiers.

#### Scenario: Download failure
- WHEN a network error occurs while refreshing the catalog via `k0rdent.catalog.list(refresh=true)`
- THEN the server logs the failure with the updated tool name and returns an MCP `unavailable` error while leaving the previous cache intact

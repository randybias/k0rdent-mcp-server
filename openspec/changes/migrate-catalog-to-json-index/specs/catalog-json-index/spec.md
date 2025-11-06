# Spec: Catalog JSON Index

## MODIFIED Requirements

### Requirement: Catalog manager SHALL fetch index from JSON endpoint

The catalog manager **SHALL** download the catalog index from `https://catalog.k0rdent.io/latest/index.json` instead of extracting a tarball archive.

#### Scenario: Successful JSON index fetch

- **GIVEN** the JSON index endpoint is accessible
- **WHEN** the catalog manager fetches the index
- **THEN** it receives a JSON response with `metadata` and `addons` fields
- **AND** the response is parsed into `CatalogEntry` structures

#### Scenario: JSON index fetch failure

- **GIVEN** the JSON index endpoint is unreachable
- **WHEN** the catalog manager attempts to fetch the index
- **THEN** it returns an error indicating the network failure
- **AND** includes the original error message for debugging

#### Scenario: Invalid JSON response

- **GIVEN** the endpoint returns non-JSON content
- **WHEN** the catalog manager attempts to parse the response
- **THEN** it returns an error indicating JSON parse failure
- **AND** includes details about the parse error

### Requirement: Catalog manager SHALL cache parsed index in memory

The catalog manager **SHALL** store the parsed catalog index in memory instead of using an SQLite database.

#### Scenario: First request builds cache

- **GIVEN** the catalog manager has no cached index
- **WHEN** `List()` is called
- **THEN** it fetches the JSON index
- **AND** parses all addons into memory
- **AND** caches the result with timestamp and SHA256

#### Scenario: Subsequent requests use cache

- **GIVEN** the catalog manager has a valid cached index
- **AND** the cache is within TTL period
- **WHEN** `List()` is called
- **THEN** it returns results from memory
- **AND** does not fetch from the network

#### Scenario: Expired cache triggers refresh

- **GIVEN** the catalog manager has a cached index
- **AND** the cache age exceeds TTL
- **WHEN** `List()` is called
- **THEN** it fetches a fresh JSON index
- **AND** rebuilds the in-memory cache

### Requirement: Catalog manager SHALL detect index changes via SHA256

The catalog manager **SHALL** compute SHA256 hash of JSON content to detect changes.

#### Scenario: Unchanged index with ETag

- **GIVEN** the catalog manager has cached index with SHA256
- **WHEN** it fetches with If-None-Match header
- **AND** the server returns 304 Not Modified
- **THEN** it updates the cache timestamp only
- **AND** does not rebuild the index

#### Scenario: Changed index content

- **GIVEN** the catalog manager has cached index with SHA256
- **WHEN** it fetches the index
- **AND** the new content has different SHA256
- **THEN** it rebuilds the cache with new data
- **AND** stores the new SHA256

### Requirement: Catalog manager SHALL fetch manifests on-demand from GitHub

The catalog manager **SHALL** fetch ServiceTemplate and HelmRepository manifests from GitHub raw URLs when installing.

#### Scenario: Fetch ServiceTemplate manifest

- **GIVEN** a catalog entry with name "minio" and version "14.1.2"
- **WHEN** `GetManifests()` is called
- **THEN** it constructs URL: `https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/minio/charts/minio-service-template-14.1.2/templates/service-template.yaml`
- **AND** fetches the manifest via HTTP GET
- **AND** returns the YAML content

#### Scenario: Fetch HelmRepository manifest

- **GIVEN** a catalog entry requires HelmRepository
- **WHEN** `GetManifests()` is called
- **THEN** it constructs URL: `https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/k0rdent-utils/charts/k0rdent-catalog-1.0.0/templates/helm-repository.yaml`
- **AND** fetches the manifest via HTTP GET
- **AND** returns the YAML content

#### Scenario: Manifest fetch failure

- **GIVEN** the GitHub raw URL returns 404
- **WHEN** `GetManifests()` attempts to fetch
- **THEN** it returns an error indicating manifest not found
- **AND** includes the full URL that was attempted

### Requirement: Catalog manager SHALL NOT use SQLite database

The catalog manager **SHALL NOT** create, read, or write to an SQLite database file.

#### Scenario: No database file created

- **GIVEN** a fresh cache directory
- **WHEN** the catalog manager initializes
- **THEN** no `catalog.db` file is created
- **AND** no SQL schema is executed

#### Scenario: No SQL queries executed

- **GIVEN** the catalog manager is operational
- **WHEN** any catalog operation is performed
- **THEN** no SQL queries are constructed or executed
- **AND** all data is accessed from in-memory structures

### Requirement: Catalog manager SHALL support ETag caching

The catalog manager **SHALL** use HTTP ETag headers to minimize redundant downloads.

#### Scenario: Initial fetch stores ETag

- **GIVEN** the JSON index endpoint returns an ETag header
- **WHEN** the catalog manager fetches the index
- **THEN** it stores the ETag value with the cache
- **AND** uses it for subsequent conditional requests

#### Scenario: Conditional request with ETag

- **GIVEN** the catalog manager has a cached index with ETag
- **WHEN** it checks for updates
- **THEN** it sends If-None-Match header with stored ETag
- **AND** handles 304 Not Modified appropriately

## ADDED Requirements

### Requirement: Delete tool SHALL remove ServiceTemplates from namespaces

The `k0.catalog.delete` tool **SHALL** remove ServiceTemplate and associated HelmRepository resources from specified namespace(s).

#### Scenario: Delete from specific namespace

- **GIVEN** a ServiceTemplate exists in namespace "kcm-system"
- **WHEN** `k0.catalog.delete` is called with namespace "kcm-system"
- **THEN** it deletes the ServiceTemplate resource
- **AND** optionally deletes the HelmRepository
- **AND** returns list of deleted resources

#### Scenario: Delete from all allowed namespaces

- **GIVEN** a ServiceTemplate exists in multiple namespaces
- **WHEN** `k0.catalog.delete` is called with `all_namespaces: true`
- **THEN** it deletes the ServiceTemplate from each allowed namespace
- **AND** returns list of all deleted resources

#### Scenario: Namespace filter applies to delete

- **GIVEN** the server has namespace filter configured
- **WHEN** `k0.catalog.delete` is called for a filtered namespace
- **THEN** it validates the namespace against the filter
- **AND** returns error if namespace is not allowed

#### Scenario: Resource not found is not an error

- **GIVEN** a ServiceTemplate does not exist in the target namespace
- **WHEN** `k0.catalog.delete` is called
- **THEN** it returns success with empty deleted list
- **AND** includes status "not_found"

#### Scenario: Delete requires same namespace rules as install

- **GIVEN** the server is in OIDC_REQUIRED mode
- **WHEN** `k0.catalog.delete` is called without namespace parameter
- **THEN** it returns error requiring explicit namespace
- **AND** error message matches install tool pattern

### Requirement: Delete tool SHALL follow same authentication modes as install

The delete tool **SHALL** respect DEV_ALLOW_ANY and OIDC_REQUIRED authentication modes.

#### Scenario: DEV_ALLOW_ANY mode defaults to kcm-system

- **GIVEN** the server is in DEV_ALLOW_ANY mode
- **AND** no namespace filter is configured
- **WHEN** `k0.catalog.delete` is called without namespace parameter
- **THEN** it defaults to namespace "kcm-system"
- **AND** proceeds with deletion

#### Scenario: OIDC_REQUIRED mode requires explicit namespace

- **GIVEN** the server is in OIDC_REQUIRED mode
- **AND** namespace filter is restrictive
- **WHEN** `k0.catalog.delete` is called without namespace parameter
- **THEN** it returns error requiring namespace specification
- **AND** error message includes "OIDC_REQUIRED mode"

### Requirement: Delete tool SHALL provide idempotent operation

The delete tool **SHALL** return success even if resources are already deleted.

#### Scenario: Delete already deleted resource

- **GIVEN** a ServiceTemplate was previously deleted
- **WHEN** `k0.catalog.delete` is called again
- **THEN** it returns success with status "not_found"
- **AND** does not throw an error

### Requirement: Integration tests SHALL verify complete lifecycle

Integration tests **SHALL** test the full install → delete → verify cycle.

#### Scenario: Install and delete ServiceTemplate

- **GIVEN** a clean test namespace
- **WHEN** `k0.catalog.install` creates a ServiceTemplate
- **AND** `k0.catalog.delete` removes it
- **THEN** the ServiceTemplate no longer exists in the namespace
- **AND** reinstallation succeeds

## REMOVED Requirements

### Requirement: ~~Catalog manager SHALL extract tarball archives~~

**REMOVED** - No longer extracting tarballs; using JSON index directly.

### Requirement: ~~Catalog manager SHALL build SQLite index~~

**REMOVED** - No longer using SQLite; storing parsed JSON in memory.

### Requirement: ~~Catalog manager SHALL parse YAML files from disk~~

**REMOVED** - No longer reading YAML from disk; fetching manifests on-demand from GitHub.

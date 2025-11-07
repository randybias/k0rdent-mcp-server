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

### Requirement: Catalog manager SHALL cache parsed index in SQLite database

The catalog manager **SHALL** store the parsed catalog index in SQLite database for persistent caching across restarts.

#### Scenario: First request builds cache

- **GIVEN** the catalog manager has no cached index in SQLite
- **WHEN** `List()` is called
- **THEN** it fetches the JSON index
- **AND** parses all addons into SQLite database
- **AND** stores the `metadata.generated` timestamp in cache metadata

#### Scenario: Subsequent requests use cache

- **GIVEN** the catalog manager has a valid cached index in SQLite
- **AND** the cache is within TTL period
- **WHEN** `List()` is called
- **THEN** it returns results from SQLite queries
- **AND** does not fetch from the network

#### Scenario: Expired cache triggers refresh

- **GIVEN** the catalog manager has a cached index in SQLite
- **AND** the cache age exceeds TTL
- **WHEN** `List()` is called
- **THEN** it fetches a fresh JSON index
- **AND** rebuilds the SQLite database if timestamp changed

### Requirement: Catalog manager SHALL detect index changes via timestamp

The catalog manager **SHALL** use the `metadata.generated` timestamp from JSON to detect changes.

#### Scenario: Unchanged index timestamp

- **GIVEN** the catalog manager has cached index with timestamp
- **WHEN** it fetches the JSON index after TTL expiry
- **AND** the `metadata.generated` field matches cached timestamp
- **THEN** it updates the last check time only
- **AND** does not rebuild the index

#### Scenario: Changed index timestamp

- **GIVEN** the catalog manager has cached index with timestamp
- **WHEN** it fetches the index
- **AND** the new `metadata.generated` field has different timestamp
- **THEN** it rebuilds the SQLite cache with new data
- **AND** stores the new timestamp

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

### Requirement: Catalog manager SHALL use SQLite database for persistent cache

The catalog manager **SHALL** continue using the SQLite database for persistent caching.

#### Scenario: Database persists across restarts

- **GIVEN** the catalog manager has cached data in SQLite
- **WHEN** the server restarts
- **THEN** the catalog manager reads existing cache from SQLite
- **AND** does not need to re-download the JSON index if timestamp is current

#### Scenario: SQL queries work with JSON-sourced data

- **GIVEN** the catalog manager has populated SQLite from JSON index
- **WHEN** `List()` is called with filters
- **THEN** it executes SQL queries as before
- **AND** returns filtered results efficiently

## ADDED Requirements

### Requirement: Delete tool SHALL remove ServiceTemplates from namespaces

The `k0rdent.catalog.delete_servicetemplate` tool **SHALL** remove ServiceTemplate and associated HelmRepository resources from specified namespace(s).

#### Scenario: Delete from specific namespace

- **GIVEN** a ServiceTemplate exists in namespace "kcm-system"
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called with namespace "kcm-system"
- **THEN** it deletes the ServiceTemplate resource
- **AND** optionally deletes the HelmRepository
- **AND** returns list of deleted resources

#### Scenario: Delete from all allowed namespaces

- **GIVEN** a ServiceTemplate exists in multiple namespaces
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called with `all_namespaces: true`
- **THEN** it deletes the ServiceTemplate from each allowed namespace
- **AND** returns list of all deleted resources

#### Scenario: Namespace filter applies to delete

- **GIVEN** the server has namespace filter configured
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called for a filtered namespace
- **THEN** it validates the namespace against the filter
- **AND** returns error if namespace is not allowed

#### Scenario: Resource not found is not an error

- **GIVEN** a ServiceTemplate does not exist in the target namespace
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called
- **THEN** it returns success with empty deleted list
- **AND** includes status "not_found"

#### Scenario: Delete requires same namespace rules as install

- **GIVEN** the server is in OIDC_REQUIRED mode
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called without namespace parameter
- **THEN** it returns error requiring explicit namespace
- **AND** error message matches install tool pattern

### Requirement: Delete tool SHALL follow same authentication modes as install

The delete tool **SHALL** respect DEV_ALLOW_ANY and OIDC_REQUIRED authentication modes.

#### Scenario: DEV_ALLOW_ANY mode defaults to kcm-system

- **GIVEN** the server is in DEV_ALLOW_ANY mode
- **AND** no namespace filter is configured
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called without namespace parameter
- **THEN** it defaults to namespace "kcm-system"
- **AND** proceeds with deletion

#### Scenario: OIDC_REQUIRED mode requires explicit namespace

- **GIVEN** the server is in OIDC_REQUIRED mode
- **AND** namespace filter is restrictive
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called without namespace parameter
- **THEN** it returns error requiring namespace specification
- **AND** error message includes "OIDC_REQUIRED mode"

### Requirement: Delete tool SHALL provide idempotent operation

The delete tool **SHALL** return success even if resources are already deleted.

#### Scenario: Delete already deleted resource

- **GIVEN** a ServiceTemplate was previously deleted
- **WHEN** `k0rdent.catalog.delete_servicetemplate` is called again
- **THEN** it returns success with status "not_found"
- **AND** does not throw an error

### Requirement: Integration tests SHALL verify complete lifecycle

Integration tests **SHALL** test the full install → delete → verify cycle.

#### Scenario: Install and delete ServiceTemplate

- **GIVEN** a clean test namespace
- **WHEN** `k0rdent.catalog.install_servicetemplate` creates a ServiceTemplate
- **AND** `k0rdent.catalog.delete_servicetemplate` removes it
- **THEN** the ServiceTemplate no longer exists in the namespace
- **AND** reinstallation succeeds

## REMOVED Requirements

### Requirement: ~~Catalog manager SHALL extract tarball archives~~

**REMOVED** - No longer extracting tarballs; using JSON index directly.

### Requirement: ~~Catalog manager SHALL parse YAML files from disk~~

**REMOVED** - No longer reading YAML from disk for catalog index; fetching manifests on-demand from GitHub for installation.

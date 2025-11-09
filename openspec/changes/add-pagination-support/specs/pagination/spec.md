# Pagination (delta)

## ADDED Requirements

### Requirement: Pagination parameters for List operations
All List operations (ServiceTemplates, ClusterDeployments, MultiClusterServices, Namespaces, Events) SHALL accept optional pagination parameters:
- `limit` (integer): Maximum number of items to return in a single response. Must be greater than 0 and less than or equal to the configured maximum page size.
- `continue` (string): Opaque continuation token from a previous response. When provided, the server SHALL return the next page of results.

#### Scenario: Default page size applied
- **WHEN** a List tool is called without a `limit` parameter
- **THEN** the server SHALL apply the default page size from `K0RDENT_LIST_PAGE_SIZE` environment variable (default: 100)

#### Scenario: Explicit limit respected
- **WHEN** a List tool is called with `limit=50`
- **THEN** the server SHALL return at most 50 items

#### Scenario: Maximum page size enforced
- **WHEN** a List tool is called with `limit=5000` (exceeds max of 1000)
- **THEN** the server SHALL cap the limit at the configured maximum (1000) and return at most 1000 items

#### Scenario: Invalid limit rejected
- **WHEN** a List tool is called with `limit=0` or `limit=-1`
- **THEN** the server SHALL return an error with a clear message about valid limit values

### Requirement: Continue token in responses
All List operation responses SHALL include a `continue` field:
- When more pages are available, `continue` SHALL contain a non-empty Kubernetes continuation token
- When no more pages are available (last page or single-page result), `continue` SHALL be empty or omitted
- The continue token SHALL be opaque to clients and managed entirely by Kubernetes

#### Scenario: Multi-page result includes continue token
- **WHEN** a List operation returns a full page of results and more items exist
- **THEN** the response SHALL include a non-empty `continue` token

#### Scenario: Last page omits continue token
- **WHEN** a List operation returns the final page of results
- **THEN** the response SHALL have an empty or omitted `continue` field

#### Scenario: Empty result set omits continue token
- **WHEN** a List operation returns zero items
- **THEN** the response SHALL have an empty or omitted `continue` field

#### Scenario: Single page result omits continue token
- **WHEN** a List operation returns fewer items than the limit and no more exist
- **THEN** the response SHALL have an empty or omitted `continue` field

### Requirement: Continue token validation
The server SHALL validate continue tokens received from clients:
- Invalid or expired tokens SHALL result in a clear error message
- The error SHALL indicate that the token is invalid and the client should restart pagination from the beginning
- The server SHALL NOT crash or return partial/incorrect results for invalid tokens

#### Scenario: Invalid continue token rejected
- **WHEN** a List tool is called with an invalid `continue` token
- **THEN** the server SHALL return an error message indicating the token is invalid

#### Scenario: Expired continue token rejected
- **WHEN** a List tool is called with an expired `continue` token (Kubernetes expired it)
- **THEN** the server SHALL return an error message indicating the token has expired and pagination should restart

### Requirement: Configurable pagination defaults
The server SHALL support configuration of pagination behavior via environment variables:
- `K0RDENT_LIST_PAGE_SIZE`: Default page size when client does not specify limit (default: 100)
- `K0RDENT_LIST_MAX_PAGE_SIZE`: Maximum allowed page size to prevent abuse (default: 1000)

#### Scenario: Default page size configured
- **WHEN** `K0RDENT_LIST_PAGE_SIZE=50` is set
- **THEN** List operations without explicit limit SHALL return at most 50 items per page

#### Scenario: Maximum page size configured
- **WHEN** `K0RDENT_LIST_MAX_PAGE_SIZE=500` is set and client requests `limit=1000`
- **THEN** the server SHALL cap the limit at 500

### Requirement: Pagination for all List operations
The following List operations SHALL support pagination:
- `k0rdent.mgmt.serviceTemplates.list(limit?, continue?)`
- `k0rdent.mgmt.clusterDeployments.listAll(selector?, limit?, continue?)`
- `k0rdent.mgmt.multiClusterServices.list(selector?, limit?, continue?)`
- `k0rdent.mgmt.namespaces.list(limit?, continue?)`
- `k0rdent.mgmt.events.list(namespace, sinceSeconds?, limit?, continue?, types?, forKind?, forName?)`

#### Scenario: ServiceTemplates paginated
- **WHEN** `k0rdent.mgmt.serviceTemplates.list(limit=10)` is called
- **THEN** at most 10 ServiceTemplates are returned with a continue token if more exist

#### Scenario: ClusterDeployments paginated with selector
- **WHEN** `k0rdent.mgmt.clusterDeployments.listAll(selector="env=prod", limit=20)` is called
- **THEN** at most 20 matching ClusterDeployments are returned with pagination applied after filtering

#### Scenario: Events paginated with field selectors
- **WHEN** `k0rdent.mgmt.events.list(namespace="default", forKind="Pod", limit=50)` is called
- **THEN** at most 50 events matching the field selector are returned

### Requirement: Namespace filter interaction
Pagination SHALL be applied AFTER namespace filtering (when `K0RDENT_NAMESPACE_FILTER` is configured):
- The Kubernetes API SHALL first filter by namespace regex
- The page limit SHALL apply to the filtered result set
- Continue tokens SHALL be valid for the filtered view

#### Scenario: Pagination after namespace filter
- **WHEN** namespace filter is `^kube-system$` and `limit=5` is requested
- **THEN** the server SHALL return at most 5 items from kube-system namespace only

### Requirement: Memory efficiency
The server SHALL NOT load all resources into memory when pagination is used:
- Only the requested page SHALL be fetched from Kubernetes
- The server SHALL rely on Kubernetes API pagination to limit memory usage
- No client-side pagination or buffering of full result sets SHALL occur

#### Scenario: Large cluster resource usage bounded
- **WHEN** a cluster has 10,000 ServiceTemplates and `limit=100` is used
- **THEN** the server SHALL only fetch and process 100 items per request, not all 10,000

### Requirement: Stateless pagination
The server SHALL NOT maintain any state for pagination:
- Continue tokens are managed entirely by Kubernetes
- No server-side sessions or pagination state SHALL be stored
- Each request with a continue token SHALL be independent

#### Scenario: Stateless token handling
- **WHEN** a client uses a continue token from a previous response
- **THEN** the server SHALL pass it directly to Kubernetes without any server-side lookup or state

### Requirement: Backward compatibility
Pagination parameters SHALL be optional to maintain backward compatibility:
- Existing clients that do not provide pagination parameters SHALL continue to work
- Default page size SHALL apply when no limit is specified
- Clients SHALL NOT be required to handle continue tokens if they are satisfied with the default page

#### Scenario: Client without pagination support
- **WHEN** an existing client calls a List tool without limit or continue parameters
- **THEN** the server SHALL return the first page using the default limit and the client receives results as before

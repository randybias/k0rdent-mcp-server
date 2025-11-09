# Change: Add pagination support to prevent OOM on large clusters

## Why
All List operations currently fetch unlimited resources from Kubernetes, which causes memory exhaustion on large clusters with thousands of ServiceTemplates, ClusterDeployments, or MultiClusterServices. Kubernetes provides built-in pagination via continue tokens, and we need to leverage this to ensure the MCP server remains stable and responsive under scale.

## What Changes
- Add pagination support to every MCP surface we expose that can stream large collections:
  - All `tools/call` handlers that return lists (`k0rdent.catalog.serviceTemplates.list`, `k0rdent.mgmt.serviceTemplates.list`, `k0rdent.mgmt.clusterDeployments.list`, `k0rdent.mgmt.clusterDeployments.listAll`, `k0rdent.mgmt.multiClusterServices.list`, `k0rdent.mgmt.namespaces.list`, `k0rdent.mgmt.events.list`, `k0rdent.mgmt.providers.listCredentials`, `k0rdent.mgmt.providers.listIdentities`, `k0rdent.mgmt.clusterTemplates.list`)
  - The `resources/read` surface for `k0rdent.mgmt.events` (and the initial snapshot used by `resources/subscribe`)
- Introduce reusable pagination helpers (`PaginationOptions`, `PaginationResult`, namespace-aware continue tokens) that work for both single-namespace Kubernetes List calls and cross-namespace aggregations in `internal/clusters`
- Route all List requests through Kubernetes continue tokens (or datastore cursors for catalog data) while keeping existing namespace filters and label selectors intact
- Update MCP tool schemas to accept optional `limit` and `continue` parameters and return structured responses that include the next-token payload
- Allow resource URIs such as `k0rdent://events/{namespace}?limit=100&continue=…` to carry pagination hints so resource reads and initial subscription snapshots never pull unbounded data
- Introduce configurable default page size (environment variable, defaults to 100) and a hard maximum page size (1000) across every surface
- Add comprehensive tests for pagination edge cases (empty results, single page, multi-page, invalid tokens, namespace-spanning tokens, catalog cursors)

## Impact
- **Affected specs**: New capability `pagination` will be created
- **Affected code**:
  - `internal/k0rdent/api/resources.go`: ServiceTemplate / ClusterDeployment / MultiClusterService list helpers
  - `internal/clusters/*.go`: `ListClusters`, `ListCredentials`, `ListIdentities`, `ListTemplates`, namespace enumeration helpers
  - `internal/catalog/manager.go`: Catalog listing queries / cursors
  - `internal/kube/events`: Provider list + subscription snapshot helpers
  - `internal/tools/core/*.go`: MCP tool handlers and schemas for every list-style tool plus `k0rdent.mgmt.events` resource template
  - MCP tool/resource registration metadata and wiring (`internal/mcpserver`, `internal/runtime`)
- **Breaking**: No. Pagination parameters are optional; existing clients continue to work with default limits and no continue handling
- Memory footprint reduced from O(total resources) to O(page size) across tools, resource reads, and subscription snapshots
- Clients can now incrementally fetch large result sets without timing out, regardless of whether they use tools or resource URIs

## Out of Scope
- Caching or pagination state server-side (clients manage continue tokens)
- Automatic pagination in subscriptions (subscriptions remain full-set watchers)
- Pagination for pod logs (already streaming)

## Acceptance
- `openspec validate add-pagination-support --strict` passes
- All list-style MCP tools (`k0rdent.catalog.serviceTemplates.list`, `k0rdent.mgmt.serviceTemplates.list`, `k0rdent.mgmt.clusterDeployments.list`, `k0rdent.mgmt.clusterDeployments.listAll`, `k0rdent.mgmt.multiClusterServices.list`, `k0rdent.mgmt.namespaces.list`, `k0rdent.mgmt.events.list`, `k0rdent.mgmt.providers.listCredentials`, `k0rdent.mgmt.providers.listIdentities`, `k0rdent.mgmt.clusterTemplates.list`) accept optional `limit` and `continue` parameters
- Tool responses include a `continue` token when more pages are available and omit it on the final page
- Default page size is configurable via `K0RDENT_LIST_PAGE_SIZE` (defaults to 100)
- Maximum page size is enforced (1000)
- Tests verify pagination behavior with mock data:
  - Empty result sets return no continue token
  - Single-page results return no continue token
  - Multi-page results return valid continue tokens
  - Invalid continue tokens return clear error messages
- Namespace-spanning operations (credentials, identities, cluster templates, cluster deployments) honor continue tokens that encode both namespace progression and Kubernetes continue data
- Catalog listing cursors round-trip through the manager/database layer so the server never materializes the full index just to serve one page
- `k0rdent://events/{namespace}` `resources/read` requests (and subscription snapshots) honor `limit`/`continue` query parameters and never fetch more than one page at a time
- Server memory usage remains bounded under large-cluster load

## Links / References
- [Kubernetes API Pagination](https://kubernetes.io/docs/reference/using-api/api-concepts/#retrieving-large-results-sets-in-chunks)
- [client-go ListOptions](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ListOptions)

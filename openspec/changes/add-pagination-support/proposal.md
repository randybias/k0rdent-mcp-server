# Change: Add pagination support to prevent OOM on large clusters

## Why
All List operations currently fetch unlimited resources from Kubernetes, which causes memory exhaustion on large clusters with thousands of ServiceTemplates, ClusterDeployments, or MultiClusterServices. Kubernetes provides built-in pagination via continue tokens, and we need to leverage this to ensure the MCP server remains stable and responsive under scale.

## What Changes
- Add pagination support to all List operations using Kubernetes continue tokens
- Introduce configurable default page size (environment variable, defaults to 100)
- Add maximum page size limit (hardcoded to 1000) to prevent abuse
- Update MCP tool schemas to accept optional `limit` and `continue` parameters
- Return continue token in tool responses to enable clients to fetch subsequent pages
- Add comprehensive tests for pagination edge cases (empty results, single page, multi-page, invalid tokens)

## Impact
- **Affected specs**: New capability `pagination` will be created
- **Affected code**:
  - `internal/k0rdent/api/resources.go`: All List functions (ListServiceTemplates, ListClusterDeployments, ListMultiClusterServices)
  - `internal/tools/core/k0rdent.go`: Tool implementations that call List functions
  - `internal/tools/core/namespaces.go`: Namespace listing
  - `internal/tools/core/events.go`: Event listing
  - MCP tool registration and schemas
- **Breaking**: No. Pagination parameters are optional; existing clients continue to work with default limits
- Memory footprint reduced from O(total resources) to O(page size)
- Clients can now incrementally fetch large result sets without timing out

## Out of Scope
- Caching or pagination state server-side (clients manage continue tokens)
- Automatic pagination in subscriptions (subscriptions remain full-set watchers)
- Pagination for pod logs (already streaming)

## Acceptance
- `openspec validate add-pagination-support --strict` passes
- All List tools accept optional `limit` and `continue` parameters
- List responses include `continue` token when more pages are available
- Default page size is configurable via `K0RDENT_LIST_PAGE_SIZE` (defaults to 100)
- Maximum page size is enforced (1000)
- Tests verify pagination behavior with mock data:
  - Empty result sets return no continue token
  - Single-page results return no continue token
  - Multi-page results return valid continue tokens
  - Invalid continue tokens return clear error messages
- Server memory usage remains bounded under large-cluster load

## Links / References
- [Kubernetes API Pagination](https://kubernetes.io/docs/reference/using-api/api-concepts/#retrieving-large-results-sets-in-chunks)
- [client-go ListOptions](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ListOptions)

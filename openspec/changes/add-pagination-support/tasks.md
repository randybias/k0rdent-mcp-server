# Implementation Tasks

## 1. Shared Pagination Primitives
- [ ] 1.1 Add `PaginationOptions` struct with `Limit int64` and `Continue string` fields
- [ ] 1.2 Add `PaginatedResult` generic wrapper with `Continue string` field
- [ ] 1.3 Add helper functions to apply default/max limits and validate user input consistently
- [ ] 1.4 Implement namespace-aware continue token encoder/decoder for aggregated lists (namespace + Kubernetes token payload)
- [ ] 1.5 Provide iterator helpers that resume multi-namespace scans from a decoded token without server-side state

## 2. Configuration
- [ ] 2.1 Add `K0RDENT_LIST_PAGE_SIZE` environment variable (default: 100)
- [ ] 2.2 Add `K0RDENT_LIST_MAX_PAGE_SIZE` environment variable (default: 1000)
- [ ] 2.3 Load pagination config in runtime session initialization
- [ ] 2.4 Document pagination config in runtime-config spec

## 3. Data Providers
- [ ] 3.1 Update `ListServiceTemplates`, `ListClusterDeployments`, and `ListMultiClusterServices` (internal/k0rdent/api) to accept `PaginationOptions` and pass limit/continue to Kubernetes
- [ ] 3.2 Update `clusters.Manager.ListClusters` to use namespace-aware pagination tokens instead of materializing all namespaces
- [ ] 3.3 Update `clusters.Manager.ListTemplates` to share the same namespace-aware pagination helpers
- [ ] 3.4 Update `clusters.Manager.ListCredentials` and `ListIdentities` to page through namespaces + Kubernetes continue tokens
- [ ] 3.5 Add cursor-based pagination to `catalog.Manager.List` so SQLite queries stream only the requested page
- [ ] 3.6 Update `events.Provider.List` to honor `PaginationOptions`, propagate Kubernetes errors, and expose helpers for paged snapshots
- [ ] 3.7 Update `EventManager` initial snapshot logic to fetch events page-by-page and stream each page to subscribers

## 4. MCP Tools & Resources
- [ ] 4.1 Update `k0rdent.catalog.serviceTemplates.list` inputs/outputs and schema to surface `limit`/`continue`
- [ ] 4.2 Update `k0rdent.mgmt.serviceTemplates.list` to pass pagination options to the API helper and include `continue` in responses
- [ ] 4.3 Update both `k0rdent.mgmt.clusterDeployments.list` (cluster manager) and `.listAll` (dynamic client) for pagination
- [ ] 4.4 Update `k0rdent.mgmt.multiClusterServices.list` to page through API helpers
- [ ] 4.5 Update `k0rdent.mgmt.namespaces.list` to accept pagination inputs and stream results in deterministic order
- [ ] 4.6 Update `k0rdent.mgmt.events.list` to forward pagination options to the events provider and surface errors from invalid tokens
- [ ] 4.7 Update `k0rdent.mgmt.providers.listCredentials` to accept pagination inputs and use namespace-aware tokens
- [ ] 4.8 Update `k0rdent.mgmt.providers.listIdentities` to share the same pagination behavior
- [ ] 4.9 Update `k0rdent.mgmt.clusterTemplates.list` to page cluster templates via the manager
- [ ] 4.10 Update MCP schemas/metadata so every list tool documents the new parameters and response shape
- [ ] 4.11 Update the `k0rdent.mgmt.events` resource template to parse `limit`/`continue` query params, include `_meta.pagination`, and ensure `resources/subscribe` snapshots reuse the same helper

## 5. Testing
- [ ] 5.1 Add unit tests for pagination option validation (negative limit, zero limit, > max) and default enforcement
- [ ] 5.2 Add unit tests for namespace-aware cursor encoding/decoding and resuming from mid-namespace
- [ ] 5.3 Add provider-level tests covering multi-page responses for service templates, cluster deployments, multi-cluster services, and events
- [ ] 5.4 Add manager tests covering `ListClusters`, `ListTemplates`, `ListCredentials`, and `ListIdentities` pagination across namespaces
- [ ] 5.5 Add catalog manager tests ensuring cursors round-trip through the database and only fetch requested rows
- [ ] 5.6 Add tool-layer tests for each MCP tool enumerated above (empty page, single page, multi-page, invalid token, max limit)
- [ ] 5.7 Add resource-template tests for `k0rdent://events/{namespace}` `resources/read` + subscription snapshots (query params, invalid token handling)

## 6. Documentation
- [ ] 6.1 Update tool documentation with pagination examples (including how to handle continue tokens)
- [ ] 6.2 Add pagination best practices to project documentation (chunking strategy, namespace-aware tokens, catalog cursors)
- [ ] 6.3 Document continue token lifecycle for both tool responses and resource URIs so clients know how to restart pagination safely

# Pagination Design

## Context
The k0rdent MCP server currently fetches all resources from Kubernetes without limits, which causes memory exhaustion on large clusters. Kubernetes supports native pagination through continue tokens, which we need to leverage to ensure stable operation at scale.

**Constraints:**
- Must use Kubernetes native pagination (ListOptions.Limit and ListOptions.Continue)
- Cannot maintain server-side state (stateless design for HA)
- Must remain backward compatible with existing clients
- Must work with existing namespace filtering
- Must cover every MCP surface we expose: tool calls, resource reads (`resources/read`), and subscription snapshots that issue initial List requests

**Stakeholders:**
- MCP clients (Claude Desktop, API consumers)
- Cluster operators running large k0rdent deployments
- Development team maintaining the server

**MCP Surfaces Impacted:**
- Tool calls that return large lists:
  - `k0rdent.catalog.serviceTemplates.list`
  - `k0rdent.mgmt.serviceTemplates.list`
  - `k0rdent.mgmt.clusterDeployments.list`
  - `k0rdent.mgmt.clusterDeployments.listAll`
  - `k0rdent.mgmt.multiClusterServices.list`
  - `k0rdent.mgmt.namespaces.list`
  - `k0rdent.mgmt.events.list`
  - `k0rdent.mgmt.providers.listCredentials`
  - `k0rdent.mgmt.providers.listIdentities`
  - `k0rdent.mgmt.clusterTemplates.list`
- Resource templates:
  - `k0rdent.mgmt.events` (`resources/read` and the initial snapshot seeded by `resources/subscribe`)
- Subscriptions (`k0rdent://events/...`, `k0rdent://podlogs/...`, `k0rdent://cluster-monitor/...`) which should keep streaming behavior; only the namespace-events snapshot issues paged Lists

## Goals / Non-Goals

**Goals:**
- Bound memory usage on the server for List operations
- Enable clients to incrementally fetch large result sets
- Maintain backward compatibility (pagination optional)
- Support all existing List operations (k0rdent CRDs, namespaces, events, catalog, provider resources)
- Provide clear errors for invalid pagination parameters
- Make default page size configurable for different deployment scenarios
- Ensure MCP resource reads (`k0rdent://events/{namespace}`) and subscription snapshots follow the same pagination semantics as tool calls

**Non-Goals:**
- Server-side caching or state management for pagination
- Automatic pagination in subscriptions (watch-based, not List-based)
- Pagination for pod logs (already streaming)
- Client-side automatic pagination (clients manage continue tokens)
- Sorting or filtering beyond what Kubernetes provides

## Decisions

### Decision 1: Use Kubernetes native continue tokens
**What:** Pass `limit` and `continue` parameters directly to Kubernetes `ListOptions`, return Kubernetes-provided continue token to clients.

**Why:**
- Kubernetes manages token generation, validation, and expiration
- Zero server-side state required (enables stateless HA)
- Tokens are opaque and secure
- Built-in support in client-go

**Alternatives considered:**
- Cursor-based pagination (offset/cursor): Requires server-side state or deterministic ordering, complex with watches
- Offset-based pagination (skip/take): Inefficient for large datasets, requires loading all prior items
- GraphQL-style connections: Over-engineered for our use case, adds unnecessary complexity

**Trade-offs:**
- Continue tokens expire (handled by clear error messages)
- Cannot skip to arbitrary page numbers (acceptable for streaming use case)
- Tokens are opaque (good for security, but cannot extract metadata)

### Decision 2: Environment variable configuration
**What:**
- `K0RDENT_LIST_PAGE_SIZE` (default: 100) - Default page size
- `K0RDENT_LIST_MAX_PAGE_SIZE` (default: 1000) - Maximum allowed page size

**Why:**
- Operators can tune for their deployment size and memory constraints
- 100 items is a reasonable default (balances memory vs round trips)
- 1000 max prevents abuse while allowing bulk operations
- Environment variables match existing configuration pattern

**Alternatives considered:**
- Hardcoded values: Less flexible for different deployment scenarios
- Per-tool configuration: Overly complex, operators want one setting
- Dynamic configuration via API: Adds complexity without clear benefit

**Trade-offs:**
- Cannot change at runtime (requires restart)
- Operators must understand pagination to tune effectively

### Decision 3: Apply pagination after namespace filtering
**What:** Kubernetes API filters by namespace, then pagination is applied to filtered results.

**Why:**
- Namespace filtering uses Kubernetes label selectors (already efficient)
- Ensures page limit applies to visible results, not total cluster resources
- Continue tokens are scoped to filtered view automatically
- Matches operator expectations

**Alternatives considered:**
- Pagination before filtering: Would expose filtered-out resources in page counts
- Separate limits for filtered/unfiltered: Overly complex

**Trade-offs:**
- Page sizes may vary based on namespace filter density

### Decision 4: Optional pagination parameters
**What:** `limit` and `continue` are optional in all List tool schemas.

**Why:**
- Backward compatibility with existing clients
- Clients that don't need pagination aren't forced to handle it
- Default limit provides sensible behavior out of the box
- Clients can opt-in when they need to handle large result sets

**Alternatives considered:**
- Required pagination: Would break existing clients
- Automatic transparent pagination: Complex to implement, hides control from clients

**Trade-offs:**
- Some clients may hit limits unknowingly (mitigated by reasonable default)

### Decision 5: Shared pagination types across all tools
**What:** Create common `PaginationOptions` input and `PaginatedResult` output types used by all List operations.

**Why:**
- Consistent API surface across all tools
- Reduces code duplication
- Easier to document and explain to clients
- Simplifies testing

**Alternatives considered:**
- Per-tool pagination types: More flexible but inconsistent
- Embed pagination directly in each tool: Duplicates logic

**Trade-offs:**
- Slightly less flexibility per tool (acceptable)

### Decision 6: Validation and error handling
**What:**
- Validate limit > 0 and <= max at tool layer
- Return clear error messages for invalid continue tokens
- Log pagination metrics (page size, continue token presence)

**Why:**
- Fail fast with clear errors improves debugging
- Metrics enable monitoring of pagination usage
- Tool layer validation provides immediate feedback

**Alternatives considered:**
- Rely on Kubernetes validation: Errors less clear to end users
- Silent capping: Confusing for clients expecting specific limits

**Trade-offs:**
- Additional validation code (minimal overhead)

### Decision 7: Namespace-aware cursors for aggregated lists
**What:** Encode the namespace scope plus the Kubernetes continue token inside our opaque continue value for list operations that iterate namespace-by-namespace (`ListClusters`, `ListTemplates`, `ListCredentials`, `ListIdentities`). Tokens take the form `base64(json{"namespace":"<ns>","continue":"<kube-token>"})`.

**Why:**
- These managers enumerate per namespace to enforce filters, so a raw Kubernetes continue token is insufficient.
- Encoding namespace context allows us to resume from the precise namespace + item boundary without maintaining server state.
- Keeps tokens opaque to clients while remaining forwards-compatible if we need more fields.

**Alternatives considered:**
- Track pagination state on the server keyed by session: violates stateless requirement and complicates HA.
- Require users to page per namespace manually: leaks implementation details and increases client complexity.

**Trade-offs:**
- Slightly longer continue tokens (base64 JSON) but still opaque/portable.
- Namespace ordering must be deterministic (alphabetical) so tokens remain valid between requests.

### Decision 8: Resource URI pagination for events
**What:** Accept `limit` and `continue` query parameters on `k0rdent://events/{namespace}` URIs. The read handler and subscription snapshot loader will apply these options when calling the events provider, and embed the next token inside `ReadResourceResult._meta["pagination"]`.

**Why:**
- `resources/read` is part of the MCP protocol we rely on; without pagination it can still OOM the server.
- Query parameters keep URIs backward compatible and easy for clients to assemble.
- Surfacing the next token via `_meta.pagination` mirrors how we expose tokens in tool responses.

**Alternatives considered:**
- Stuff pagination hints into the URI path: harder to evolve and conflicts with existing namespace parsing.
- Return pagination data inside the blob payload: clients that rely on `_meta` would miss it and it would mix with event data.

**Trade-offs:**
- Resource clients must be updated to read `_meta.pagination` instead of the blob body for tokens.
- Subscriptions still stream indefinitely—only the initial snapshot is paginated.

## Implementation Approach

### Phase 1: Core pagination primitives
1. Add `PaginationOptions` (`Limit`, `Continue`) and a `PaginatedResult` helper in a shared package.
2. Implement namespace-aware cursor helpers capable of encoding/decoding namespace + Kubernetes tokens.
3. Provide guardrails (`ApplyLimits`, `ValidateContinue`) that every surface can reuse so validation/error strings stay consistent.

### Phase 2: Configuration
1. Add pagination config fields to runtime session.
2. Load from environment variables with defaults (`K0RDENT_LIST_PAGE_SIZE`, `K0RDENT_LIST_MAX_PAGE_SIZE`).
3. Update runtime-config spec documentation.

### Phase 3: Data providers
1. Update `internal/k0rdent/api` list helpers to accept `PaginationOptions` and call Kubernetes with `Limit`/`Continue`.
2. Update `internal/clusters` managers (`ListClusters`, `ListTemplates`, `ListCredentials`, `ListIdentities`) to use namespace-aware cursors instead of materializing all namespaces.
3. Update `internal/catalog/manager.List` to stream results directly from SQLite using limit/offset (or cursor) without loading the full index.
4. Update `internal/kube/events.Provider` to pass `Limit`/`Continue` through to Kubernetes and expose helper methods for paged snapshots.

### Phase 4: MCP layer integration
1. Update MCP tool schemas with pagination parameters and default descriptions.
2. Update each tool implementation to pass pagination options to the underlying provider/helper and to return `continue` in the structured output.
3. Update `k0rdent.mgmt.events` resource template to parse `limit`/`continue` query params, include `_meta.pagination`, and reuse the provider helpers.
4. Teach the `EventManager` subscription snapshot to stream pages sequentially so initial snapshots never load more than one page at a time. Pod logs and cluster monitor subscriptions remain unchanged.

### Phase 5: Testing
1. Unit tests for validation, config clamping, and namespace-aware cursor encoding/decoding.
2. Provider-level tests for Kubernetes pagination (service templates, cluster deployments, multi-cluster services, events).
3. Integration tests for every MCP tool/resource enumerated above (empty page, single page, multi-page, invalid cursor).
4. Manual testing against a real/seeded cluster to confirm `continue` tokens plug into kubectl-style pagination.

## Risks / Trade-offs

**Risk: Continue token expiration**
- **Impact:** Clients receive error mid-pagination
- **Mitigation:** Clear error messages instructing restart from beginning
- **Likelihood:** Low (tokens valid for minutes)

**Risk: Page size too small**
- **Impact:** Too many round trips for large result sets
- **Mitigation:** Configurable default, document tuning guidance
- **Likelihood:** Medium

**Risk: Page size too large**
- **Impact:** Memory spikes on server
- **Mitigation:** Enforced maximum (1000), documented in deployment guide
- **Likelihood:** Low

**Risk: Unexpected namespace filter interactions**
- **Impact:** Page counts vary unpredictably
- **Mitigation:** Document behavior, test with filters
- **Likelihood:** Low

**Trade-off: No arbitrary page access**
- Cannot jump to page N directly
- Acceptable: Use case is streaming/iterating, not random access

**Trade-off: No result count metadata**
- Cannot tell how many total items exist without fetching all
- Acceptable: Count would require full fetch anyway (expensive)

## Migration Plan

### Rollout
1. Deploy with pagination support to staging
2. Test with existing clients (no changes expected)
3. Test with clients using new pagination parameters
4. Deploy to production (backward compatible)
5. Update client documentation with pagination examples

### Rollback
- Remove pagination parameters from tools (returns to unlimited fetch)
- Redeploy previous version (no data migration needed)
- No data loss or corruption risk

### Client Migration
- Existing clients: No changes required, continue working with default page size
- New clients: Adopt pagination for large result sets incrementally
- Provide example code for pagination patterns

## Open Questions

**Q: Should subscriptions support pagination?**
- A: No. Subscriptions use watches (incremental updates), not List operations. Pagination applies to snapshot queries only.

**Q: Should we add result count metadata?**
- A: No. Would require fetching all results, defeating the purpose of pagination. Clients should iterate until no continue token.

**Q: Should default page size vary by resource type?**
- A: No. Single default keeps configuration simple. Operators can tune based on their largest resource type.

**Q: Should we log continue tokens for debugging?**
- A: No. Tokens are opaque and may be sensitive. Log presence/absence only, not token values.

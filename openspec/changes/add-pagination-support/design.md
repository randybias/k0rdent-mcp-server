# Pagination Design

## Context
The k0rdent MCP server currently fetches all resources from Kubernetes without limits, which causes memory exhaustion on large clusters. Kubernetes supports native pagination through continue tokens, which we need to leverage to ensure stable operation at scale.

**Constraints:**
- Must use Kubernetes native pagination (ListOptions.Limit and ListOptions.Continue)
- Cannot maintain server-side state (stateless design for HA)
- Must remain backward compatible with existing clients
- Must work with existing namespace filtering

**Stakeholders:**
- MCP clients (Claude Desktop, API consumers)
- Cluster operators running large k0rdent deployments
- Development team maintaining the server

## Goals / Non-Goals

**Goals:**
- Bound memory usage on the server for List operations
- Enable clients to incrementally fetch large result sets
- Maintain backward compatibility (pagination optional)
- Support all existing List operations (k0rdent CRDs, namespaces, events)
- Provide clear errors for invalid pagination parameters
- Make default page size configurable for different deployment scenarios

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

## Implementation Approach

### Phase 1: API Layer
1. Add `PaginationOptions` struct to `internal/k0rdent/api`
2. Add `PaginatedResult` generic type wrapper
3. Update all List functions to accept `PaginationOptions` and return `PaginatedResult`
4. Add helper to apply default/max limits

### Phase 2: Configuration
1. Add pagination config fields to runtime session
2. Load from environment variables with defaults
3. Update runtime-config spec documentation

### Phase 3: Tool Layer
1. Update MCP tool schemas with pagination parameters
2. Update tool implementations to pass pagination options to API
3. Update tool results to include continue token

### Phase 4: Testing
1. Unit tests for validation and limits
2. Integration tests with mock data (single/multi-page, empty, invalid token)
3. Manual testing against real cluster

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

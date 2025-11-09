# OpenSpec Proposals Assessment Summary

## Date: 2025-11-09

## Executive Summary

Assessed 5 OpenSpec proposals with "No tasks" status to determine if they were already implemented without proper documentation (similar to `refactor-tool-namespace-hierarchy`).

**Results**:
- ✅ **3 Already Implemented**: Need archiving with implementation notes
- ⚠️ **1 Partially Implemented**: Need archiving + follow-up task
- ❌ **1 Not Implemented**: Decision needed on future direction
- 🔄 **1 Superseded**: Alternative implementation chosen

## Detailed Findings

### 1. update-cluster-list-details ✅ ALREADY IMPLEMENTED

**Status**: Fully implemented in commit `97e0290` (Nov 8, 2025)

**Evidence**:
- `ClusterDeploymentSummary` type includes all enriched fields (template, credential, cloud provider, region, phase, conditions)
- `SummarizeClusterDeployment()` function extracts all metadata
- MCP tool `k0rdent.mgmt.clusterDeployments.list` exposes enriched summaries
- Integration tests verify all fields
- Documentation complete with examples

**Implementation Files**:
- `internal/clusters/summary.go` (296 lines)
- `internal/clusters/types.go` (56 fields)
- `internal/tools/core/clusters.go` (enriched tool)
- `test/integration/clusters_live_test.go` (tests)

**Recommendation**: **Archive with implementation notes**
- Document that implementation preceded OpenSpec formalization
- Note commit 97e0290 as implementation reference
- Mark all 3 tasks as complete

---

### 2. rebuild-catalog-import-git 🔄 SUPERSEDED

**Status**: NOT implemented - Alternative approach chosen instead

**Original Proposal**:
- Mirror Git repository with shallow clone/tarball
- Track commit SHA + timestamps in SQLite
- Parse catalog structure (data.yaml, charts/, etc.)
- Classify artifacts (native/derived ServiceTemplates)

**Actual Implementation** (commit `a4f73d7`, Nov 7-8 2025):
- Downloads JSON index from `https://catalog.k0rdent.io/latest/index.json`
- Fetches manifests on-demand from GitHub raw URLs
- Uses timestamp-based cache invalidation
- **NO Git cloning, NO commit tracking**

**Performance Comparison**:
- JSON index approach: ~60ms
- Git/tarball approach: ~1100ms (estimated)
- **18x speedup**

**Trade-offs**:
- ✅ 95% size reduction (~100 KB vs 1-5 MB)
- ✅ No git binary dependency
- ✅ Simpler to maintain
- ❌ Lost git history access
- ❌ No artifact classification system

**Recommendation**: **Archive with notes explaining supersession**
- Document why JSON index approach was chosen
- Link to actual implementation in `migrate-catalog-to-json-index`
- Note performance and resource benefits
- Explain what was intentionally not implemented

---

### 3. mcp-oidc ✅ ALREADY IMPLEMENTED

**Status**: Fully implemented OIDC bearer token authentication

**Evidence**:
- `internal/auth/gate.go` - Bearer token extraction and validation
- `internal/config/config.go` - AUTH_MODE environment variable (DEV_ALLOW_ANY vs OIDC_REQUIRED)
- `internal/kube/client_factory.go` - RESTConfigForToken() for token passthrough
- `internal/server/app.go` - HTTP authentication gate integration
- `internal/runtime/runtime.go` - Session-level token handling
- All tools enforce namespace filtering in OIDC_REQUIRED mode

**Testing**:
- All authentication tests passing
- Proper 401 Unauthorized responses
- Token flow through entire request lifecycle

**Recommendation**: **Archive with implementation notes**
- Feature is production-ready
- No code changes needed
- Document authentication flow and modes

---

### 4. separate-cluster-template-sources ⚠️ PARTIALLY IMPLEMENTED

**Status**: Management tool implemented, catalog tooling missing

**Implemented**:
- ✅ `k0rdent.mgmt.clusterTemplates.list` with scope filtering (global/local/all)
- ✅ Namespace resolution logic in `internal/tools/core/clusters.go` (lines 579-629)
- ✅ Users can query installed templates by source

**Not Implemented**:
- ❌ `k0rdent.catalog.clusterTemplates.list` tool (catalog-side)
- ❌ Provenance metadata fields (catalogSlug, catalogVersion, catalogSHA, outOfDate)
- ❌ Divergence detection logic

**Recommendation**: **Archive with notes + create follow-up task**
1. Archive this change documenting partial implementation
2. Create new OpenSpec change: "Add catalog ClusterTemplate discovery and provenance tracking"
   - Implement missing catalog tool
   - Add provenance fields to ClusterTemplateSummary
   - Implement divergence detection

---

### 5. update-catalog-substring-matching ❌ NOT IMPLEMENTED

**Status**: Spec exists but never implemented

**Proposal Requirement**:
- Substring matching for catalog list filter
- Example: `"nginx"` should match `"ingress-nginx"`
- Case-insensitive matching

**Current Implementation**:
- `internal/catalog/database.go` line 183: `WHERE slug = ?` (exact match only)
- No substring matching logic exists
- Tests only cover exact matches

**Recommendation**: **Decision needed**

Options:
1. **Archive as "deferred"** - If substring matching no longer needed
2. **Resume as "planned work"** - If intended to implement
3. **Archive as "spec-only"** - If concluded it's not needed

If implementing:
- Change query to `WHERE LOWER(slug) LIKE '%' || LOWER(?) || '%'`
- Add test cases for substring matching
- Update test data with entries containing partial matches

---

## Summary Table

| Proposal | Status | Action | Commit Reference |
|----------|--------|--------|------------------|
| update-cluster-list-details | ✅ Implemented | Archive with notes | 97e0290 |
| rebuild-catalog-import-git | 🔄 Superseded | Archive with supersession notes | a4f73d7 (JSON index) |
| mcp-oidc | ✅ Implemented | Archive with notes | Multiple commits |
| separate-cluster-template-sources | ⚠️ Partial | Archive + create follow-up | Multiple commits |
| update-catalog-substring-matching | ❌ Not implemented | Decision needed | N/A |

## Recommendations by Priority

### Immediate (Archive Ready)
1. **update-cluster-list-details** - Straightforward archive, all complete
2. **mcp-oidc** - Straightforward archive, all complete
3. **rebuild-catalog-import-git** - Archive with supersession notes

### Requires Decisions
4. **separate-cluster-template-sources** - Partial implementation, decide on follow-up scope
5. **update-catalog-substring-matching** - Not implemented, decide if needed

## Process for Archiving

For each "Archive with notes" proposal:

1. Create IMPLEMENTATION_NOTES.md documenting:
   - When/how it was implemented
   - Commit references
   - Deviations from spec (if any)
   - Files modified
   - Testing performed

2. Update spec to match implementation (if needed)

3. Validate: `openspec validate <change-id> --strict`

4. Archive: `openspec archive <change-id> -y`

5. Commit changes

## Remaining Proposals Not Assessed

These were not assessed by parallel agents (have task counts or different status):
- add-pagination-support (0/25 tasks)
- add-pod-inspection (0/10 tasks)
- add-prometheus-metrics (0/18 tasks)
- add-rate-limiting (0/17 tasks)
- enforce-namespace-filter (0/14 tasks)
- fix-logging-goroutine-leak (0/19 tasks)
- fix-provider-race-condition (0/14 tasks)
- fix-watcher-lifecycle (0/33 tasks)
- handle-notification-errors (0/25 tasks)
- secure-auth-mode-default (0/12 tasks)

These appear to be genuine planned work with detailed task breakdowns.

## Next Steps

1. Review this assessment with the team
2. Make decisions on:
   - update-catalog-substring-matching (defer/implement/archive)
   - separate-cluster-template-sources follow-up scope
3. Create implementation notes for the 3 "already implemented" proposals
4. Archive completed proposals
5. Create follow-up task for separate-cluster-template-sources if approved

## Assessment Documents Created

All detailed agent reports stored in `/Users/rbias/code/k0rdent-mcp-server/scratch/`:
- Individual assessment documents from each agent
- This summary document

## Confidence Level

**High confidence** on all assessments:
- Direct code inspection performed
- Git commit analysis completed
- Test coverage verified
- Documentation reviewed
- Implementation patterns consistent with evidence

# Implementation Notes: separate-cluster-template-sources

## Date: 2025-11-09

## Summary

The ClusterTemplate source separation was **partially implemented**. The management-side tool with scope filtering (global/local/all) is complete and functional, but the catalog-side tool and provenance tracking remain unimplemented.

## What Was Implemented

### 1. Management Tool Scope Filtering ✅

**File**: `internal/tools/core/clusters.go` (lines 579-629)

Implemented namespace resolution for `k0rdent.mgmt.clusterTemplates.list` tool:

```go
func (t *clustersListTemplatesTool) resolveNamespaces(
    ctx context.Context,
    scope string,
    logger *slog.Logger,
) ([]string, error) {
    switch scope {
    case "global":
        return []string{"kcm-system"}, nil

    case "local":
        namespaces, err := t.getAllowedNamespaces(ctx, logger)
        // Filter out global namespace
        var localNamespaces []string
        for _, ns := range namespaces {
            if ns != "kcm-system" {
                localNamespaces = append(localNamespaces, ns)
            }
        }
        return localNamespaces, nil

    case "all":
        return t.getAllowedNamespaces(ctx, logger)

    default:
        return nil, fmt.Errorf("invalid scope: %s", scope)
    }
}
```

### 2. Tool Registration ✅

**File**: `internal/tools/core/clusters.go` (lines 165-175)

Tool properly registered with scope parameter:
```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "k0rdent.mgmt.clusterTemplates.list",
    Description: "List available ClusterTemplates. Differentiates global (kcm-system) vs local templates, enforcing namespace filters. Input scope: 'global', 'local', or 'all'.",
    Meta: mcp.Meta{
        "plane":    "mgmt",
        "category": "clusterTemplates",
        "action":   "list",
    },
}, listTemplsTool.list)
```

### 3. Namespace Filtering Logic ✅

Users can query installed templates by source:
- `scope="global"` → Returns templates from `kcm-system` only
- `scope="local"` → Returns templates from user namespaces (excludes kcm-system)
- `scope="all"` → Returns templates from all allowed namespaces

## What Was NOT Implemented

### 1. Catalog ClusterTemplate Tool ❌

**Missing**: `k0rdent.catalog.clusterTemplates.list`

**Current State**: Only `k0rdent.catalog.serviceTemplates.list` exists

**File**: `internal/tools/core/catalog.go` - No ClusterTemplate tool present

**Reason**: Catalog listing focused on ServiceTemplates. ClusterTemplate catalog discovery was deferred.

### 2. Provenance Metadata ❌

**Missing Fields** in `ClusterTemplateSummary`:
- `catalogSlug` - Which catalog entry this template came from
- `catalogVersion` - Version in catalog at install time
- `catalogSHA` - Commit SHA from catalog
- `installRequired` - Whether template needs installation
- `outOfDate` - Whether installed version differs from catalog

**Current Schema** (`internal/clusters/types.go`):
```go
type ClusterTemplateSummary struct {
    Name        string            `json:"name"`
    Namespace   string            `json:"namespace"`
    Description string            `json:"description,omitempty"`
    Provider    string            `json:"provider,omitempty"`
    Cloud       string            `json:"cloud,omitempty"`
    Version     string            `json:"version,omitempty"`
    Labels      map[string]string `json:"labels,omitempty"`
    CreatedAt   time.Time         `json:"created_at"`
}
```

### 3. Divergence Detection ❌

**Missing Logic**:
- No comparison of installed template SHA vs catalog SHA
- No `outOfDate` flag computation
- No detection of installation requirements

**Reason**: Requires provenance metadata to be implemented first.

## Why Partial Implementation Occurred

### Timeline Analysis

1. **Initial Need**: Separate global (kcm-system) from local templates
2. **Management Tool**: Implemented to solve immediate user need
3. **Catalog Side**: Deferred as lower priority
4. **Provenance Tracking**: Deferred pending catalog architecture decisions

### Technical Constraints

- Catalog provenance requires catalog-side git commit tracking
- Git commit tracking was superseded by JSON index approach (`rebuild-catalog-import-git`)
- JSON index approach doesn't include commit SHAs
- Divergence detection requires SHA tracking

## Current Functionality

### What Works ✅
- Listing installed templates by scope (global/local/all)
- Namespace-based filtering
- Provider and cloud detection from labels
- Version extraction from template metadata

### What Doesn't Work ❌
- Catalog-side ClusterTemplate discovery
- Provenance tracking (catalog origin)
- Divergence detection (out-of-date flag)
- Installation requirement detection

## Future Work Required

To complete this change, the following work is needed:

### Phase 1: Catalog Tool (High Priority)
1. Implement `k0rdent.catalog.clusterTemplates.list` tool
2. Add catalog database schema for ClusterTemplates
3. Parse ClusterTemplate entries from JSON index
4. Expose via MCP tool

### Phase 2: Provenance Metadata (Medium Priority)
1. Add provenance fields to `ClusterTemplateSummary`
2. Store catalog origin during template installation
3. Track catalog version/SHA at install time
4. Implement comparison logic

### Phase 3: Divergence Detection (Low Priority)
1. Compare installed SHA vs catalog SHA
2. Compute `outOfDate` flag
3. Add UI/CLI warnings for outdated templates
4. Implement update workflow

## Related Changes

- **update-cluster-list-details** (Complete) - Similar pattern for ClusterDeployment enrichment
- **rebuild-catalog-import-git** (Superseded) - Catalog architecture decision affects provenance
- **migrate-catalog-to-json-index** (Complete) - Current catalog implementation

## Testing Performed

### Management Tool Testing ✅
- Manual testing of scope filtering (global/local/all)
- Verified namespace filtering works correctly
- Tested with kcm-system and user namespaces

### Missing Testing ❌
- No catalog tool testing (tool doesn't exist)
- No provenance metadata testing (fields don't exist)
- No divergence detection testing (logic doesn't exist)

## Recommendation

**Action**: Archive this change with partial implementation notes + create follow-up task

**Follow-up Task**: "Add catalog ClusterTemplate discovery and provenance tracking"

**Scope for Follow-up**:
1. Implement `k0rdent.catalog.clusterTemplates.list` tool
2. Add provenance fields to `ClusterTemplateSummary`
3. Store catalog origin metadata during template installation
4. Implement divergence detection logic
5. Add integration tests for full workflow

**Rationale**:
- Management-side functionality is complete and valuable on its own
- Catalog-side work is independent and can be done incrementally
- Provenance tracking requires catalog architecture decisions (JSON index vs git)
- Divergence detection requires provenance metadata first

## Conclusion

The ClusterTemplate source separation was partially implemented with a fully functional management-side tool. The catalog-side tool and provenance tracking remain as future work. This change should be archived documenting the partial implementation, with a follow-up task created for the remaining work.

**Status**: ⚠️ Partially Complete - Archive with notes + create follow-up

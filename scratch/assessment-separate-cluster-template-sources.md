# Assessment: OpenSpec Change "separate-cluster-template-sources"

## Executive Summary
**STATUS: PARTIALLY IMPLEMENTED**

The proposal to differentiate catalog vs. installed ClusterTemplates has been **partially implemented**. The management plane tool (`k0rdent.mgmt.clusterTemplates.list`) now properly differentiates between global and local templates with scope filtering, but the catalog-side tooling for listing ClusterTemplates from the catalog is **NOT YET IMPLEMENTED**.

---

## Evidence of Implementation

### IMPLEMENTED: Management Cluster Template Listing with Scope Separation

**File:** `/Users/rbias/code/k0rdent-mcp-server/internal/tools/core/clusters.go`

**Tool Registration (lines 150-160):**
```go
listTemplsTool := &clustersListTemplatesTool{session: session}
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

**Input Structure (lines 86-89):**
```go
type clustersListTemplatesInput struct {
    Scope     string `json:"scope"`               // "global", "local", or "all"
    Namespace string `json:"namespace,omitempty"` // Optional namespace filter
}
```

**Namespace Resolution Logic (lines 579-629):**
The `resolveTargetNamespaces` method properly implements scope-based namespace selection:
- **"global"** → Returns only `["kcm-system"]`
- **"local"** → Returns all allowed namespaces except `kcm-system`
- **"all"** → Returns all allowed namespaces including `kcm-system`

**Response Type (lines 91-93):**
```go
type clustersListTemplatesResult struct {
    Templates []clusters.ClusterTemplateSummary `json:"templates"`
}
```

---

### NOT IMPLEMENTED: Catalog ClusterTemplate Listing Tool

**Spec Requirement:** Task 1 from `tasks.md`:
> "Update catalog specs to expose a `k0rdent.catalog.clusterTemplates.list` (or equivalent) endpoint that enumerates catalog-only ClusterTemplate artifacts with metadata about required installation."

**Search Results:**
- Searched `/Users/rbias/code/k0rdent-mcp-server/internal/tools/core/catalog.go`
- Only found: `k0rdent.catalog.serviceTemplates.list`
- No tool found for: `k0rdent.catalog.clusterTemplates.list` or similar

**Catalog Package Analysis:**
- File: `/Users/rbias/code/k0rdent-mcp-server/internal/catalog/types.go`
- Contains only `CatalogEntry` and `ServiceTemplateVersion` types
- No `ClusterTemplateVersion` or similar structure defined
- Catalog manager only handles ServiceTemplates, not ClusterTemplates

---

### MISSING: Provenance Metadata in ClusterTemplateSummary

**Spec Requirement:** Task 2 from `tasks.md`:
> "Specify that `k0rdent.mgmt.clusterTemplates.list` ... includes catalog provenance (slug/version) when available."

**Current ClusterTemplateSummary (lines 34-43 in types.go):**
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

**Missing Fields:**
- `catalogSlug` - Link back to catalog origin
- `catalogVersion` - Version in the catalog
- `catalogSHA` / `commitSHA` - Commit SHA for version detection
- `installRequired` - Boolean flag from spec
- `outOfDate` - Boolean flag to indicate divergence from catalog

---

### NOT IMPLEMENTED: Divergence Detection

**Spec Requirement:** Task 3 from `tasks.md`:
> "Define how tooling reports discrepancies (e.g., catalog template exists but is not installed, or installed template is outdated compared to catalog SHA)."

No code implements:
- Comparison of installed template SHA against catalog SHA
- `outOfDate` flag logic
- Installation requirement detection

---

## Related Implemented Changes

### Recent Related Work: "update-cluster-list-details"

**File:** `/Users/rbias/code/k0rdent-mcp-server/openspec/changes/update-cluster-list-details/`

This change (created Nov 8, 2025) enriches `ClusterDeploymentSummary` with:
- `templateRef` (name + version)
- `credentialRef` 
- `cloudProvider` / `region`
- `phase`, `conditions[]`
- `kubeconfigSecret`, `managementURL`

**Status:** Recently implemented in commit `97e0290` (Nov 8, 2025)

This complements the template source separation but does not fully address catalog provenance tracking.

---

## Code References

**Files Implementing Scope Separation:**
- `/Users/rbias/code/k0rdent-mcp-server/internal/tools/core/clusters.go` (lines 348-392, 579-629)
- `/Users/rbias/code/k0rdent-mcp-server/internal/clusters/templates.go` (ListTemplates method)

**Files NOT Updated for Provenance:**
- `/Users/rbias/code/k0rdent-mcp-server/internal/clusters/types.go` (ClusterTemplateSummary)
- `/Users/rbias/code/k0rdent-mcp-server/internal/catalog/types.go` (no ClusterTemplate types)
- `/Users/rbias/code/k0rdent-mcp-server/internal/tools/core/catalog.go` (no clusterTemplates tool)

---

## Assessment

### What WAS Implemented
1. ✅ Scope-based namespace filtering for installed templates (global/local/all)
2. ✅ Tool descriptor acknowledges the scope separation concern
3. ✅ Proper namespace resolution logic in tool handler

### What WAS NOT Implemented
1. ❌ Catalog ClusterTemplate listing tool (`k0rdent.catalog.clusterTemplates.list`)
2. ❌ Provenance metadata fields in `ClusterTemplateSummary`
3. ❌ Divergence detection (installRequired, outOfDate flags)
4. ❌ Cross-reference logic between catalog and installed versions
5. ❌ Schema/database updates to store catalog origin info

---

## Recommendation

### Archive with Clarification

**Rationale:**
1. The namespace scope separation was already implemented in the management tool
2. However, the proposal's core goal—**differentiating catalog vs. installed templates**—is incomplete
3. A new follow-up OpenSpec change should be created to finish the implementation:
   - Add `k0rdent.catalog.clusterTemplates.list` tool
   - Add provenance fields to `ClusterTemplateSummary`
   - Implement divergence detection logic

### Archive Instructions

1. Archive this change with notes:
   - State that scope filtering was implemented in `k0rdent.mgmt.clusterTemplates.list`
   - Note that catalog-side tools and provenance tracking remain TODO
   - Reference the incomplete implementation as foundation for future work

2. Create a new follow-up task/change for:
   - "Add catalog ClusterTemplate discovery and provenance tracking"
   - Link to this change as context
   - Focus on the missing pieces (catalog tool, provenance fields, divergence logic)


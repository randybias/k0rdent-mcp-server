# OpenSpec Assessment: rebuild-catalog-import-git

## Current Status: NOT IMPLEMENTED

The Git-based catalog import pipeline described in the proposal has NOT been implemented. 
The actual implementation uses a JSON index approach instead.

## What the Proposal Asks For

1. **Mirror Git Repository**: Clone or tarball download from https://github.com/k0rdent/catalog 
   - Store commit SHA + generated timestamp in SQLite metadata
   - Shallow clone or tarball mirroring into cache

2. **Git-based Artifact Classification**:
   - Parse apps/<slug>/data.yaml for app metadata
   - Walk charts/ for service-template directories
   - Parse st-charts.yaml for Helm-only entries
   - Detect *-cld.yaml files for ClusterDeployment samples
   - Classify as: native_service_template, derived_service_template, cluster_deployment_sample, or doc_only

3. **Rich Metadata Storage**:
   - Repository URLs
   - Manifest paths
   - Helm repo references
   - HelmRepository references
   - ClusterDeployment sample information

4. **Validation Hooks**:
   - Fail refresh if service-template directory lacks manifest
   - Fail if Helm entry missing repository URL
   - Flag doc_only entries

5. **Tool Output Enhancement**:
   - Expose repository URLs
   - Show sourceType (native vs derived)
   - Identify synthesized ServiceTemplates
   - Display HelmRepository references

## What Was Actually Implemented (a4f73d7)

1. **JSON Index Download** (NOT Git-based):
   - Downloads lightweight JSON index from `https://catalog.k0rdent.io/latest/index.json`
   - Uses timestamp-based cache invalidation (metadata.generated)
   - NO git clone, NO tarball extraction

2. **On-Demand Manifest Fetching**:
   - Individual YAML files fetched from GitHub raw URLs
   - Pattern: `https://raw.githubusercontent.com/k0rdent/catalog/refs/heads/main/apps/{slug}/...`
   - Only downloads when needed

3. **SQLite Storage**:
   - Apps table: slug, title, summary, tags, validated_platforms
   - ServiceTemplates table: app_slug, chart_name, version, service_template_path, helm_repository_path
   - Metadata table: key-value store for cache management

4. **Database Schema**:
   ```sql
   CREATE TABLE apps (
       slug TEXT PRIMARY KEY,
       title TEXT NOT NULL,
       summary TEXT,
       tags TEXT,                    -- JSON array
       validated_platforms TEXT      -- JSON array
   );
   
   CREATE TABLE service_templates (
       id INTEGER PRIMARY KEY,
       app_slug TEXT,
       chart_name TEXT,
       version TEXT,
       service_template_path TEXT,   -- Relative path to YAML
       helm_repository_path TEXT     -- Optional relative path
   );
   ```

## Gap Analysis

### Missing from Current Implementation (vs Proposal)

1. **Git Repository Mirroring**:
   - Not implemented (uses JSON index instead)
   - No shallow clone or tarball mirroring
   - No commit SHA tracking

2. **Artifact Classification**:
   - No concept of "derived_service_template" vs "native_service_template"
   - No detection of *-cld.yaml (ClusterDeployment samples)
   - No doc_only entries tracking
   - No st-charts.yaml parsing for Helm-only entries

3. **Repository URL Field**:
   - ServiceTemplates table lacks repository URL storage
   - Only stores relative paths to manifests
   - Cannot distinguish Helm-only vs native templates

4. **ClusterDeployment Sample Tracking**:
   - Completely missing
   - No tables or metadata for cluster deployment samples
   - Proposal mentions this as key feature

5. **Validation Logic**:
   - No validation hooks that fail refresh on missing files
   - Existing code silently skips incomplete versions

### Current Implementation Trade-offs

**Advantages (vs Git-based proposal)**:
- 18x faster (60ms vs ~1100ms with tarball extraction)
- 95% smaller downloads (~100 KB vs 1-5 MB)
- No need to extract archives
- Simplified dependency (no git binary needed)
- On-demand manifest fetching

**Disadvantages (vs Git-based proposal)**:
- Relies on external JSON index service
- Cannot directly inspect git commit history
- Less control over source truth validation
- Missing ClusterDeployment sample integration
- No sourceType distinction in current schema

## Verdict

**STATUS**: Not Implemented

The proposal describes a Git-based catalog import system that mirrors the entire catalog 
repository with artifact classification and validation. The actual implementation took a 
different direction using a JSON index for performance reasons.

## Recommendation

**Option 1: Archive with Implementation Notes**
Create IMPLEMENTATION_NOTES.md documenting:
- Why JSON index approach was chosen instead
- Performance advantages (18x faster)
- Trade-offs made (what was lost vs proposal)
- Migration path if Git-based approach becomes needed

**Option 2: Continue as Partially Implemented**
The current implementation addresses core goals (rich metadata, validation) but via 
different means. Consider updating proposal to reflect actual architecture.

## Related OpenSpec Changes

- `migrate-catalog-to-json-index`: This is the actual change that was implemented
  (implemented in commit a4f73d7)
- `refactor-tool-namespace-hierarchy`: Similar pattern - documented after organic implementation
  (archived in commit 6135541)

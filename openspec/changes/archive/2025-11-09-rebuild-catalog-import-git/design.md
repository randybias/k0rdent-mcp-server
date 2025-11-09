# Design: Git-based catalog ingestion

## Overview
We replace the brittle JSON index download with a Git mirror of https://github.com/k0rdent/catalog. The catalog cache now stores `catalog.git/` plus extracted metadata in SQLite so we can ingest all artifact types (ServiceTemplate bundles, Helm metadata, ClusterDeployment samples, doc-only entries).

## Flow
1. **Sync**: `git clone --depth=1` (or tarball download) into cache, remembering commit SHA + generated timestamp.
2. **Ingest**:
   - For each `apps/<slug>/data.yaml`, parse title, summary, tags, validated_* flags, doc/support links, deploy snippets.
   - Walk `charts/` for:
     - `*-service-template-*` directories → record manifest path + HelmRepository reference.
     - `st-charts.yaml` → record chart name, repo URL, versions; mark as `derived_service_template` so install logic knows it must synthesize manifests.
   - Detect `*-cld.yaml` files → register them as ClusterDeployment samples tied to their provider.
   - Apps with none of the above are `doc_only` but still tracked for documentation.
3. **Persist**: Write normalized rows into SQLite tables (`apps`, `service_templates`, `helm_sources`, `cluster_deployments`, etc.), capturing repository URLs and manifest paths that were previously blank.
4. **Validate**: Abort refresh if:
   - A ServiceTemplate bundle lacks `templates/service-template.yaml`.
   - An `st-charts.yaml` entry misses a `repository` URL.
   - Declared chart versions don’t have matching assets.
5. **Expose**: Update catalog tools to surface repository URLs, `sourceType` (`native` vs `derived`), presence of ClusterDeployment samples, etc.

## Trade-offs & Considerations
- Git mirroring increases cache size but guarantees consistent metadata; we only sync when the upstream commit SHA changes.
- Parsing YAML in Go requires schema discipline; we will add unit tests covering representative apps (ServiceTemplate, Helm-only, ClusterDeployment-only).
- Synthesizing ServiceTemplates for Helm-only entries needs defaults (HelmRepository name, namespace). We will encode these defaults in the importer and expose warnings in tool responses.

# Change: Rebuild catalog import pipeline using Git source of truth

## Why
- The JSON `index.json` feed omits critical metadata (repository URLs, manifest paths) and produces empty fields in `k0rdent.catalog.serviceTemplates.list`.
- Some catalog apps (e.g., ingress-nginx) only publish Helm metadata while others ship full ServiceTemplate bundles; the current importer cannot distinguish these buckets.
- We need a reliable way to ingest ServiceTemplates, derived Helm charts, and ClusterDeployment samples directly from https://github.com/k0rdent/catalog so installs stop failing with 404s and agents can see what artifacts exist.

## What Changes
- Mirror the Git repository (shallow clone or tarball) into the catalog cache and track commit SHA + generated timestamp in SQLite metadata.
- Parse every `apps/<slug>/data.yaml`, `charts/*-service-template-*`, `charts/st-charts.yaml`, and `*-cld.yaml` to classify artifacts into ServiceTemplate, Helm-derived ServiceTemplate, ClusterDeployment sample, or doc-only entries.
- Persist full metadata (title, summary, tags, validated platforms, repo URLs, manifest paths, helm repo references) so catalog tools can expose accurate information and synthesize ServiceTemplates for Helm-only entries.
- Add validation hooks to ensure each artifact type has the required files, preventing half-ingested catalog states.

## Impact
- Catalog install operations will know when a ServiceTemplate manifest actually exists vs. when they must synthesize from Helm metadata, eliminating the current 404 failure mode for ingress-nginx.
- Tool responses become richer: repository URLs and manifest availability bits can be surfaced to agents.
- Sets the foundation for indexing ClusterDeployment samples and future catalog analytics.

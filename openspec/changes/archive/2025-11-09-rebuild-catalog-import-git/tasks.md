## Tasks
1. [ ] Mirror the catalog Git repo into the cache directory (shallow clone or tarball) and store commit SHA/timestamp in SQLite metadata.
2. [ ] Implement a Git-based importer that walks every `apps/<slug>` directory, parses `data.yaml`, and classifies artifacts (ServiceTemplate bundle, Helm metadata, ClusterDeployment sample, doc-only).
3. [ ] Extend the SQLite schema to capture repository URLs, manifest paths, helm source references, and cluster deployment samples for each app.
4. [ ] Add validation hooks that fail the refresh when required files are missing (e.g., service-template manifest absent, Helm repo URL missing).
5. [ ] Update `k0rdent.catalog.*` tool specs to leverage the richer metadata (e.g., show repository URLs, identify synthesized ServiceTemplates) and document the new fields.

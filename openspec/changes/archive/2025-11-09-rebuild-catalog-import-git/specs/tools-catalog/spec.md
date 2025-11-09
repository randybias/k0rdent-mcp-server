## MODIFIED Requirements

### Requirement: Catalog ingestion source
- The catalog manager **SHALL** synchronize from the Git repository `https://github.com/k0rdent/catalog` (shallow clone or tarball) instead of consuming `index.json`.
- Cache metadata **SHALL** store the upstream commit SHA and generated timestamp so refreshes only trigger when the Git head changes.

#### Scenario: Cache refresh
- WHEN the upstream commit SHA differs from the cached SHA
- THEN the manager clones/fetches the repo, re-ingests metadata, and updates SQLite tables.

### Requirement: Artifact classification
- During ingestion, each app **SHALL** be classified per artifact type: `native_service_template`, `derived_service_template` (from `st-charts.yaml`), `cluster_deployment_sample` (from `*-cld.yaml`), or `doc_only`.
- The importer **SHALL** record at least one row per artifact indicating chart name, version, repository URL (for Helm), manifest path (for native), and HelmRepository reference.

#### Scenario: Native ServiceTemplate
- GIVEN `apps/minio/charts/minio-service-template-14.1.2/templates/service-template.yaml`
- WHEN ingestion runs
- THEN the SQLite record stores the manifest path and HelmRepository reference so installs no longer hit GitHub 404s.

#### Scenario: Derived ServiceTemplate
- GIVEN `apps/ingress-nginx/charts/st-charts.yaml` with repo `https://kubernetes.github.io/ingress-nginx`
- WHEN ingestion runs
- THEN a `derived_service_template` entry is created with the repo URL and version list even though no manifest exists.

### Requirement: Validation
- The importer **SHALL** fail the refresh if a declared artifact is missing its required files (e.g., service-template directory without manifest, Helm entry without repository URL).
- Apps with no artifacts **SHALL** be flagged `doc_only` so tools can explain that they contain documentation only.

#### Scenario: Missing manifest
- WHEN a `*-service-template-*` directory lacks `service-template.yaml`
- THEN the refresh aborts with an MCP `unavailable` error and logs the slug/version pair.

### Requirement: Catalog tool outputs
- `k0rdent.catalog.*` tools **SHALL** expose the richer metadata: repository URL, `sourceType` (`native` vs `derived`), HelmRepository reference, manifest availability, and presence of cluster deployment samples.
- Catalog install flows **SHALL** branch based on `sourceType` (download native manifest vs. synthesize from Helm metadata) and error clearly when a requested artifact is not available.

#### Scenario: Listing entries
- WHEN `k0rdent.catalog.serviceTemplates.list(app="ingress-nginx")` is called
- THEN the response includes `repository="https://kubernetes.github.io/ingress-nginx"`, `sourceType="derived"`, and `hasManifest=false`.

#### Scenario: Install derived template
- WHEN `k0rdent.mgmt.serviceTemplates.install_from_catalog` is called for ingress-nginx
- THEN the tool synthesizes a ServiceTemplate using the stored repo URL + chart version and surfaces that the install is derived (e.g., via status field).

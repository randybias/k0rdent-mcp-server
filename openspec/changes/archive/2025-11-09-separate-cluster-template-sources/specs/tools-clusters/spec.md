## MODIFIED Requirements

### Requirement: Catalog cluster template listing
- The catalog capability **SHALL** expose a tool (e.g., `k0rdent.catalog.clusterTemplates.list`) that lists ClusterTemplate artifacts discovered in the Git catalog, including slug, version, required namespace, and whether installation is required.
- Catalog responses **SHALL** flag templates that are not yet installed in the management cluster.

#### Scenario: Catalog-only template
- GIVEN a ClusterTemplate manifest under `apps/gitlab/charts/...`
- WHEN `k0rdent.catalog.clusterTemplates.list(app="gitlab")` is called
- THEN the response indicates `installRequired=true` until the template exists in the management cluster.

### Requirement: Installed cluster template listing
- `k0rdent.mgmt.clusterTemplates.list` **SHALL** return only the ClusterTemplates currently installed in the management cluster (queried via Kubernetes), regardless of catalog availability.
- Each installed entry **SHALL** include provenance metadata (catalog slug/version/commit SHA when known) so operators can correlate it back to catalog artifacts.

#### Scenario: Installed template with provenance
- WHEN a ClusterTemplate installed from catalog slug `minio` version `14.1.2` is listed
- THEN `k0rdent.mgmt.clusterTemplates.list` includes `catalogSlug="minio"`, `catalogVersion="14.1.2"`, and the commit SHA used during install.

### Requirement: Divergence detection
- When a template exists in the catalog but not in the management cluster, catalog tooling **SHALL** mark it `installRequired`.
- When an installed template’s provenance SHA differs from the latest catalog SHA, the response **SHALL** flag it as `outOfDate`.

#### Scenario: Out-of-date template
- GIVEN a template installed from SHA `abc123`
- AND the catalog now reports SHA `def456`
- WHEN `k0rdent.mgmt.clusterTemplates.list` is called
- THEN the entry includes `outOfDate=true` with the newer SHA so automation can decide to reinstall.

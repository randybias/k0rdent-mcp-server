## Tasks
1. [ ] Update catalog specs to expose a `k0rdent.catalog.clusterTemplates.list` (or equivalent) endpoint that enumerates catalog-only ClusterTemplate artifacts with metadata about required installation.
2. [ ] Specify that `k0rdent.mgmt.clusterTemplates.list` queries the management cluster for installed ClusterTemplates and includes catalog provenance (slug/version) when available.
3. [ ] Define how tooling reports discrepancies (e.g., catalog template exists but is not installed, or installed template is outdated compared to catalog SHA).

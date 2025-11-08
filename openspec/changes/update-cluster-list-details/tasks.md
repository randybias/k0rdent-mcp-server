## Tasks
1. [x] Define the enriched `ClusterDeploymentSummary` schema (template, credential, provider, region, status details) and update specs/docs accordingly.
2. [x] Extend the cluster manager/tool implementation to populate the new fields from ClusterDeployment spec/status, keeping namespace filters intact.
3. [x] Update integration tests (cluster live tests, docs) to assert the new fields and demonstrate usage.

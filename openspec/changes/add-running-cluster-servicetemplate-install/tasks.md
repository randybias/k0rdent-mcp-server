## 1. Implementation
- [ ] 1.1 Extend the k0rdent API package with a helper that fetches a ClusterDeployment, merges/creates the ServiceTemplate entry in `spec.serviceSpec.services[]`, and applies it via server-side apply (dry-run + live).
- [ ] 1.2 Add `k0rdent.mgmt.clusterDeployments.services.apply` to the tool registry; parse/validate inputs (namespaces, template existence, valuesFrom/helmOptions schema) and emit structured logs/metrics.
- [ ] 1.3 Return the updated service spec plus `.status.services[]` snapshot so callers can follow the states documented in "Checking status".
- [ ] 1.4 Unit-test the helper + tool for add vs. update, namespace-filter enforcement, dry-run, and template-not-found errors; add/extend integration tests that apply a real ServiceTemplate to a running cluster and wait for status `Deployed`.
- [ ] 1.5 Update operator documentation (tools catalog + README) to describe the new tool, inputs, dry-run mode, and mention the underlying docs it automates.

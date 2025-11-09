## MODIFIED Requirements
### Requirement: Existing tool mapping
- The `k0rdent.mgmt.graph.snapshot` and `k0rdent.mgmt.graph` rows SHALL be marked as Deferred rather than Implemented until a future change redefines the capability.
- The table SHALL reference change `remove-graph-feature` so downstream tooling understands the graph capability is intentionally absent.

#### Scenario: Graph tooling marked deferred
- WHEN maintainers review the tool namespace table
- THEN the graph entries indicate the capability is "Deferred" and reference `remove-graph-feature`
- AND no tooling named `k0rdent.mgmt.graph.*` is listed as implemented until a replacement spec is approved.

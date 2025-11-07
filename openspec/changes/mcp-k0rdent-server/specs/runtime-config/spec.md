# Runtime Config (delta)

## ADDED Requirements

### Requirement: Kubeconfig-first targeting
- The server **SHALL** accept exactly one of:
  `K0RDENT_MGMT_KUBECONFIG_PATH`, `K0RDENT_MGMT_KUBECONFIG_B64`, or `K0RDENT_MGMT_KUBECONFIG_TEXT`.
- The server **SHALL** honor `K0RDENT_MGMT_CONTEXT` when provided.

#### Scenario: Load from PATH
- WHEN `K0RDENT_MGMT_KUBECONFIG_PATH` points to a valid kubeconfig  
- THEN the server connects using that kubeconfig and (if set) the named context

#### Scenario: Invalid base64
- WHEN `K0RDENT_MGMT_KUBECONFIG_B64` is invalid  
- THEN startup fails fast with a clear error

### Requirement: Namespace filter
- The server **SHALL** support a namespace allow-list filter via `K0RDENT_NAMESPACE_FILTER` (regular expression).
- When the filter is set, the server **SHALL** only return namespaces whose names match the regex in all namespace-listing tools and graph queries.

#### Scenario: Filter applied
- WHEN the filter is set to `^team-`  
- THEN only namespaces matching the regex are returned by `k0rdent.namespaces.list()`

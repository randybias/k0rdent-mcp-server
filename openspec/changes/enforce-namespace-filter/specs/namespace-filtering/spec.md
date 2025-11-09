# Namespace Filtering (delta)

## ADDED Requirements

### Requirement: Global namespace filter enforcement
- When `K0RDENT_NAMESPACE_FILTER` regex is configured, all List and Watch operations **SHALL** be scoped to matching namespaces
- The filter **SHALL** be applied before issuing Kubernetes API calls, not after fetching data
- The filter **SHALL** be evaluated once at startup and cached for performance

#### Scenario: Filter limits List operations
- GIVEN `K0RDENT_NAMESPACE_FILTER=^(dev|staging)$`
- WHEN ListServiceTemplates is called
- THEN only ServiceTemplates in namespaces matching the regex are fetched
- AND namespaces "prod" and "test" are not queried

#### Scenario: Filter limits Watch operations
- GIVEN `K0RDENT_NAMESPACE_FILTER=^team-.*$`
- WHEN GraphManager starts watchers
- THEN only namespaces matching "team-*" are watched
- AND other namespaces are ignored

#### Scenario: Empty filter watches all namespaces
- GIVEN `K0RDENT_NAMESPACE_FILTER` is not set
- WHEN watchers start
- THEN all namespaces are monitored (metav1.NamespaceAll)

### Requirement: Namespace enumeration for filtering
- The server **SHALL** list all namespaces matching the filter regex at operation time
- Namespace list **SHALL** be fetched dynamically to detect new namespaces
- Failed namespace list operations **SHALL** return clear error without partial results

#### Scenario: Dynamic namespace discovery
- GIVEN `K0RDENT_NAMESPACE_FILTER=^app-.*$`
- WHEN a new namespace "app-new" is created
- THEN subsequent List/Watch operations include the new namespace

#### Scenario: Namespace list failure
- GIVEN user lacks RBAC permission to list namespaces
- WHEN ListServiceTemplates is called with filter
- THEN operation fails with error "cannot list namespaces: forbidden"

### Requirement: Consistent filter application
- Namespace filter **SHALL** be applied consistently across all resource types
- The filter **SHALL** apply to: ServiceTemplates, ClusterDeployments, MultiClusterServices, Events, Pods

#### Scenario: Filter applied to all CRDs
- GIVEN `K0RDENT_NAMESPACE_FILTER=^production$`
- WHEN user lists ServiceTemplates, ClusterDeployments, and MultiClusterServices
- THEN all operations only return resources from "production" namespace

#### Scenario: Filter applied to Events
- GIVEN `K0RDENT_NAMESPACE_FILTER=^monitoring$`
- WHEN user subscribes to k0rdent://events/{namespace}
- THEN only events from "monitoring" namespace are streamed

### Requirement: Filter performance optimization
- The server **SHALL** issue separate API calls per matching namespace instead of filtering post-fetch
- Watch operations **SHALL** start one watcher per matching namespace
- Namespace regex **SHALL** be compiled once at startup

#### Scenario: Per-namespace API calls
- GIVEN `K0RDENT_NAMESPACE_FILTER=^(ns1|ns2)$`
- WHEN ListServiceTemplates is called
- THEN two List API calls are made: one for ns1, one for ns2
- AND no call to metav1.NamespaceAll is made

#### Scenario: Regex compiled at startup
- GIVEN `K0RDENT_NAMESPACE_FILTER=^complex-regex.*$`
- WHEN server starts
- THEN regex is compiled once and cached
- AND subsequent operations use cached compiled regex

### Requirement: Filter documentation
- The README **SHALL** document namespace filter behavior and regex syntax
- Documentation **SHALL** include performance implications (per-namespace watchers)
- Documentation **SHALL** warn about regex complexity impact

#### Scenario: Filter behavior documented
- GIVEN a user reads README.md
- THEN namespace filter section explains: regex syntax, scope of enforcement, performance notes
- AND examples show common filter patterns

#### Scenario: Limitations documented
- GIVEN a user reads README.md
- THEN documentation warns: complex regex = slower startup, many matching namespaces = many watchers

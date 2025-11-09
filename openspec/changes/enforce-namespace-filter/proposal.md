# Change: enforce-namespace-filter

## Why
- Namespace filter configuration exists but is inconsistently enforced
- List operations fetch from all namespaces (metav1.NamespaceAll)
- Watch operations monitor all namespaces
- Filter only works in GraphManager tool, not API layer
- Users expect filter to limit scope globally, but it doesn't

## What Changes
- Apply namespace filter at API layer (internal/k0rdent/api/resources.go)
- Apply namespace filter to all watch operations
- Apply filter consistently across all tools
- Add tests verifying filter enforcement at all layers
- Document filter behavior clearly in README.md

## Impact
- Breaking behavior change: filter will now actually work
- Reduced memory usage when filter is configured
- Reduced Kubernetes API load
- Better multi-tenancy support
- May expose bugs in deployments that thought filter was working

## Acceptance
- When K0RDENT_NAMESPACE_FILTER is set, List operations only query matching namespaces
- Watch operations only monitor matching namespaces
- Filter applies to all resource types consistently
- Tests verify filter enforcement at API and tool layers
- Documentation explains filter semantics and limitations
- `openspec validate enforce-namespace-filter --strict` passes

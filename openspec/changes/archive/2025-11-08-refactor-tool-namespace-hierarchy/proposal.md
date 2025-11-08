# Change: Refactor MCP tool namespace hierarchy

## Why
- Tool names have grown organically (`k0rdent.catalog.*`, `k0rdent.cluster.*`, etc.) and no longer convey which plane (management cluster vs child cluster vs regional) a call targets.
- Upcoming management/child operations (e.g., catalog install vs. child-cluster diagnostics) require a predictable namespace layout so agents can infer scope from a tool name alone.
- Without a hierarchy, we risk name collisions and unclear ownership, and it becomes hard to introduce new prefixes (e.g., `k0rdent.mgmt.*`) without a spec describing the rules.

## What Changes
- Define a canonical namespace taxonomy covering catalog, management, child, children-wide, and regional operations.
- Introduce guidelines for routing existing tools into the new prefixes (e.g., `k0rdent.mgmt.clusters.list`, `k0rdent.mgmt.serviceTemplates.install_from_catalog`).
- Require new tools to declare their target scope + namespace segment as part of registration, enabling linting/validation.

## Impact
- Clarifies how agents and users discover the right tool surface for each layer.
- Establishes forward-compatible prefixes before we add more operations, avoiding future breaking renames.
- Enables automated docs/UX grouping by namespace.

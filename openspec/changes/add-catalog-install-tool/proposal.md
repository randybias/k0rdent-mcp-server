# Change: Add catalog install tool for ServiceTemplates

## Why
- Cluster operators currently need to leave Claude Code to browse catalog.k0rdent.io and manually apply ServiceTemplate YAML to install curated apps; the MCP server offers no workflow to help.
- Automating catalog installs through MCP aligns with k0rdent’s goal of making curated services one-command operations; it reduces copy/paste errors and ensures HelmRepository prerequisites ship together with the ServiceTemplate.
- The catalog is sourced from https://github.com/k0rdent/catalog; wiring the MCP server directly to that repository keeps the management plane in sync with the published catalog without duplicating data.

## What Changes
- Add a catalog client that fetches and caches the k0rdent catalog archive (GitHub tarball) and builds an index of applications → ServiceTemplate versions → manifest bundle paths.
- Introduce MCP tools under the `k0.catalog` namespace:
  - `k0.catalog.list(app?)` returns available catalog applications and their ServiceTemplate versions (slug, title, summary, validated platforms).
  - `k0.catalog.install(app, template, version)` downloads the corresponding manifest bundle, installs/updates the required `HelmRepository`, and applies the ServiceTemplate CR.
- Reuse the existing runtime session’s dynamic client to apply resources, performing server-side apply with field ownership and namespace filter enforcement (only install when namespace filter allows the template’s namespace, if any).
- Add structured logging, latency metrics, and error handling around catalog downloads and Kubernetes apply calls; surface validation errors back through MCP error codes.
- Document the new catalog workflow (tool signatures, expected prerequisites like network access to GitHub/ghcr.io, cache invalidation).

## Impact
- **Affected specs**: new `tools-catalog` capability capturing list/install behaviour.
- **Affected code**:
  - New catalog manager package (e.g., `internal/catalog`) for download, caching, index parsing.
  - `internal/tools/core/` (register catalog tools, implement handlers).
  - `internal/runtime/` to expose catalog manager via session or runtime context.
  - `test/` suites for catalog listing/install (HTTP test server + fake dynamic client).
  - Documentation describing catalog usage and operator guidance.
- **Breaking**: No; functionality is additive.

## Out of Scope
- Uninstalling, upgrading, or reconciling existing ServiceTemplates.
- Editing ServiceTemplate values or composing MultiClusterService manifests (only ServiceTemplate install).
- Offline/air-gapped catalog mirroring.
- Signature verification or integrity attestation (future enhancement).

## Acceptance
- `openspec validate add-catalog-install-tool --strict` passes.
- `k0.catalog.list()` returns catalog entries with slug/name/summary and available ServiceTemplate versions without downloading manifests for each call (uses cached index).
- `k0.catalog.install(app="minio", template="minio", version="14.1.2")` creates/updates the `HelmRepository` noted in the bundle and applies the ServiceTemplate CR; repeated calls are idempotent.
- Namespace filter rules remain enforced: installing into disallowed namespace (if template is namespaced) returns `forbidden`.
- Unit tests cover catalog index parsing, cache refresh, successful install, missing template/version, and Kubernetes apply failures.
- Documentation explains prerequisites (network reachability to GitHub & ghcr.io) and provides example MCP requests.

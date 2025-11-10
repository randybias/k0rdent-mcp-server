# Change: Use kgst for catalog installs

## Why
- The MCP server currently installs ServiceTemplates by directly applying manifests from the k0rdent catalog git repository (`/private/tmp/k0rdent-catalog/apps/<name>/charts/<name>-service-template-<version>/templates/service-template.yaml`), bypassing the official installation method documented at catalog.k0rdent.io.
- This direct approach causes failures for ServiceTemplates with operator dependencies (e.g., valkey) or complex validation requirements (e.g., prometheus), because it skips the pre-install verification job and proper Helm lifecycle hooks that kgst provides.
- All catalog documentation instructs users to install via `helm upgrade --install <name> oci://ghcr.io/k0rdent/catalog/charts/kgst --set "chart=<name>:<version>" -n kcm-system`, using the kgst (k0rdent Generic Service Template) Helm chart which provides format validation, pre-install verification jobs, and proper HelmRepository creation with hooks.
- By not following the documented installation process, the MCP server creates an inconsistent experience where some templates work (minio, keda) and others fail validation (valkey, prometheus), making the catalog installation feature unreliable and divergent from official k0rdent workflows.

## What Changes
- Refactor the catalog installation implementation to use the kgst Helm chart instead of directly applying ServiceTemplate manifests.
- Replace the current approach in `internal/tools/core/catalog.go` that fetches and applies raw manifests with a new approach that invokes Helm to install the kgst chart with appropriate parameters.
- Implement Helm client integration using the official Helm Go SDK (helm.sh/helm/v3) to execute `helm upgrade --install` operations programmatically.
- Transform the MCP tool parameters (app, template, version, namespace) into kgst values:
  - `chart: "<template>:<version>"` (e.g., "minio:14.1.2")
  - `repo.name: "k0rdent-catalog"`
  - `repo.spec.url: "oci://ghcr.io/k0rdent/catalog/charts"`
  - `repo.spec.type: "oci"`
  - `k0rdentApiVersion: "v1beta1"` (converted from v1alpha1 if needed)
  - `namespace: <targetNamespace>`
- Preserve the existing namespace resolution logic (DEV_ALLOW_ANY vs OIDC_REQUIRED modes, all_namespaces flag) but pass resolved namespace to Helm as both the release namespace and the `namespace` value.
- Add error handling for Helm installation failures, including parsing Helm output to extract verification job failures and validation errors.
- Maintain idempotency by using `helm upgrade --install` which creates or updates the release.
- Update structured logging to capture Helm execution details, release status, and verification results.
- Remove the direct manifest application code and the catalog manager's manifest retrieval methods that are no longer needed.

## Impact
- **Affected specs**:
  - Modify existing `tools-catalog` capability to reflect Helm-based installation.
  - Add new `helm-integration` capability specifying Helm SDK usage, configuration, and error handling.
- **Related proposals**:
  - `docker-build-system` - Documents Helm SDK dependency for container image builds.
- **Affected code**:
  - `internal/tools/core/catalog.go` - Refactor install method to use Helm client instead of dynamic client apply.
  - `internal/catalog/manager.go` - Remove or deprecate manifest retrieval methods if no longer needed for listing.
  - New `internal/helm/` package for Helm client configuration and execution (or integrate into existing catalog package).
  - Update dependencies in `go.mod` to include `helm.sh/helm/v3/pkg/action` and related packages.
  - `test/integration/catalog_test.go` - Update tests to verify Helm invocation and handle kgst verification behavior.
  - Documentation updates to reflect that catalog installations now follow the official kgst workflow.
- **Breaking**: No user-facing breaking changes; the MCP tool signature remains the same. Internal implementation change only.
- **Dependencies**: Requires Helm v3 SDK; ensure network access to `oci://ghcr.io/k0rdent/catalog/charts` for pulling kgst chart.

## Out of Scope
- Supporting custom kgst parameters beyond the standard catalog installation (e.g., custom `repo.spec.url`, `prefix`, `skipVerifyJob`).
- Implementing helm uninstall for ServiceTemplate removal (the existing delete tool can continue using dynamic client).
- Caching or mirroring the kgst chart locally (always pull from registry on install).
- Supporting air-gapped or offline installation scenarios.
- Modifying the catalog listing functionality (continues to use the git repository index).

## Acceptance
- `openspec validate use-kgst-for-catalog-installs --strict` passes.
- `k0rdent.mgmt.serviceTemplates.install_from_catalog(app="minio", template="minio", version="14.1.2")` uses Helm to install kgst chart with correct values and successfully creates the ServiceTemplate.
- `k0rdent.mgmt.serviceTemplates.install_from_catalog(app="valkey", template="valkey", version="0.1.0")` now succeeds, installing both the valkey-operator dependency and the Valkey CR template without validation errors.
- `k0rdent.mgmt.serviceTemplates.install_from_catalog(app="prometheus", template="prometheus", version="27.5.1")` succeeds, passing kgst's pre-install verification job.
- Installation failures (e.g., chart not found, verification job failure) return clear error messages extracted from Helm output.
- Repeated installations are idempotent (Helm upgrade behavior) and don't cause conflicts.
- Namespace filter enforcement continues to work (installs rejected if target namespace not allowed).
- Integration tests verify ServiceTemplate creation, HelmRepository creation with proper hooks, and verification job execution (when not skipped).
- Documentation reflects the new Helm-based approach and mentions kgst as the underlying installation mechanism.

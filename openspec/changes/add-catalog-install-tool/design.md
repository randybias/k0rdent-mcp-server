# Design: Catalog install tooling

## Overview
We will surface curated ServiceTemplates from the public k0rdent catalog through new MCP tools. The catalog is a static GitHub repository (`https://github.com/k0rdent/catalog`) built into `catalog.k0rdent.io`. Each application slug under `apps/<slug>/` carries:

- `data.yaml` — metadata (title, summary, tags, validation flags).
- `charts/st-charts.yaml` — list of ServiceTemplate chart identifiers (`name`, `version`, Helm repository).
- `charts/<service-template>-<version>/templates/service-template.yaml` — the rendered ServiceTemplate CR.
- Optional HelmRepository manifests (either in the same chart or via a dependency on the `k0rdent-catalog` helper chart).

The MCP server will download the catalog TAR archive, index apps → service template versions, and expose the data through `k0rdent.catalog.*` tools. Install requests will apply the ServiceTemplate (and required HelmRepository) into the management cluster using the existing dynamic client.

## Data flow
1. **Download** – Fetch `https://github.com/k0rdent/catalog/archive/refs/heads/main.tar.gz` using an HTTP client with timeout/retry. Store the archive under `${stateDir}/catalog/<sha>` (stateDir lives beside the server binary or `/var/lib/k0rdent-mcp`).
2. **Cache** – Maintain a cache manifest with ETag / commit sha. Only re-download when missing or `k0rdent.catalog.list --refresh` is called.
3. **Index** – Unpack only the `apps/` tree. For each app slug:
   - Parse `data.yaml` into metadata struct.
   - Parse `charts/st-charts.yaml` for available ServiceTemplate entries.
   - For each entry, capture manifest path (`charts/<name>-service-template-<version>/templates/service-template.yaml`) and search for `helm-repository.yaml` either next to the ServiceTemplate or within the `k0rdent-catalog` helper chart.
   - Record validated platforms (`validated_*` keys) from `data.yaml`.
4. **Serve list** – `k0rdent.catalog.list` reads the index from cache; optional `app` filter returns matching slug. The handler never touches the network unless the cache is empty or refresh requested.
5. **Install** – `k0rdent.catalog.install(app, template, version)` performs:
   - Namespace filter check (if the ServiceTemplate manifest declares a namespace, ensure it matches the configured regex).
   - Ensure the helper `HelmRepository` exists: apply `apps/k0rdent-utils/charts/k0rdent-catalog-*/templates/helm-repository.yaml` prior to ServiceTemplate when the manifest references `name: k0rdent-catalog` (idempotent server-side apply).
   - Decode YAML into unstructured objects and apply via the dynamic client (server-side apply + force option when resource already owned by us).
   - Return MCP result describing created/updated resources.
   - Optionally pre-flight check OCI accessibility by probing `oci://ghcr.io/k0rdent/catalog/charts`? (future enhancement).

## Error handling
- Network failures → surface as MCP errors with code `unavailable` (retryable) and log details.
- Missing app/template/version → MCP `invalidParams` with suggestions from index.
- Kubernetes apply failures → propagate status (e.g., forbidden, validation) to caller with `internal` or `forbidden` codes.
- Cache corruption → drop cache entry and re-download.

## Testing
- Catalog manager unit tests with on-disk fixtures (extract subset of repo into testdata).
- Tool handler tests use `httptest.Server` to serve tarball fixture and `fake.Clientset` / dynamic fake client to assert apply semantics.
- Validate namespace filter blocking by configuring test session with restrictive regex.

## Observability & configuration
- Metrics: histogram for download duration, counter for install successes/failures, gauge for cache age.
- Logging: structured logs including app/template/version, manifest paths, elapsed time.
- Configuration: env vars for override `CATALOG_ARCHIVE_URL`, cache directory, and download timeout. Defaults target GitHub.

## Security considerations
- The server must run with outbound HTTPS to GitHub; no credentials stored.
- No manifest mutation occurs; we trust catalog content. Future work can implement signature verification.
- HelRepository apply ensures resources carry `k0rdent.mirantis.com/managed` label for traceability.

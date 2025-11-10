# Capability: Catalog Tools (ServiceTemplate Installation)

## MODIFIED Requirements

### Requirement: Install ServiceTemplate from catalog
The MCP tool `k0rdent.mgmt.serviceTemplates.install_from_catalog` MUST install ServiceTemplates using the kgst Helm chart following the official catalog installation workflow.

#### Scenario: Install minio ServiceTemplate
**Given** MCP tool call `install_from_catalog(app="minio", template="minio", version="14.1.2", namespace="kcm-system")`
**When** the tool executes
**Then** the server uses Helm to install the kgst chart with values:
```
chart: "minio:14.1.2"
repo:
  name: "k0rdent-catalog"
  spec:
    url: "oci://ghcr.io/k0rdent/catalog/charts"
    type: "oci"
namespace: "kcm-system"
k0rdentApiVersion: "v1beta1"
skipVerifyJob: false
```
**And** creates a Helm release named "minio" in namespace "kcm-system"
**And** the verify-job validates that minio:14.1.2 exists in the catalog
**And** the HelmRepository "k0rdent-catalog" is created/updated via pre-install hook
**And** the ServiceTemplate "minio-14-1-2" is created in namespace "kcm-system"
**And** returns `{applied: ["kcm-system/ServiceTemplate/minio-14-1-2", "kcm-system/HelmRepository/k0rdent-catalog"], status: "created"}`

#### Scenario: Install valkey ServiceTemplate with operator dependency
**Given** MCP tool call `install_from_catalog(app="valkey", template="valkey", version="0.1.0", namespace="kcm-system")`
**When** the tool executes
**Then** the server uses Helm to install the kgst chart
**And** kgst references the valkey Helm chart which includes valkey-operator as a dependency
**And** Helm resolves and installs the valkey-operator dependency
**And** the verify-job confirms valkey:0.1.0 is available
**And** the ServiceTemplate "valkey-0-1-0" is created
**And** the installation succeeds without validation webhook errors
**And** returns status "created"

#### Scenario: Install prometheus ServiceTemplate with verification
**Given** MCP tool call `install_from_catalog(app="prometheus", template="prometheus", version="27.5.1", namespace="kcm-system")`
**When** the tool executes
**Then** the verify-job pulls prometheus:27.5.1 from the catalog to validate it exists
**And** the job succeeds because the chart is available
**And** the ServiceTemplate "prometheus-27-5-1" is created
**And** the installation succeeds without validation webhook errors
**And** returns status "created"

#### Scenario: Installation failure for non-existent chart
**Given** MCP tool call `install_from_catalog(app="fake", template="fake", version="1.0.0", namespace="kcm-system")`
**When** the tool executes
**Then** the verify-job attempts to pull fake:1.0.0 from the catalog
**And** the job fails because the chart doesn't exist
**And** Helm treats the pre-install hook failure as a release failure
**And** no ServiceTemplate is created
**And** returns an MCP error: "chart fake:1.0.0 not found in k0rdent catalog"

#### Scenario: Tool signature unchanged
**Given** existing code or documentation using `install_from_catalog`
**When** the Helm-based implementation is deployed
**Then** the MCP tool name remains "k0rdent.mgmt.serviceTemplates.install_from_catalog"
**And** the input parameters remain: app (string), template (string), version (string), namespace (string, optional), all_namespaces (bool, optional)
**And** the return structure remains: {applied: string[], status: string}
**And** existing MCP clients continue to work without modification

### Requirement: List catalog ServiceTemplates
The MCP tool `k0rdent.catalog.serviceTemplates.list` behavior MUST remain unchanged, continuing to use the local git repository index.

#### Scenario: Listing unaffected by Helm integration
**Given** MCP tool call `list(app="minio")`
**When** the tool executes
**Then** the server reads from the local catalog git repository at `/private/tmp/k0rdent-catalog/apps/minio/data.yaml`
**And** returns catalog entries with slug, title, summary, tags, and available versions
**And** does NOT invoke Helm or pull kgst chart
**And** behavior is identical to pre-Helm implementation

### Requirement: Delete ServiceTemplate
The MCP tool `k0rdent.mgmt.serviceTemplates.delete` behavior SHALL continue using direct Kubernetes deletion rather than `helm uninstall`.

#### Scenario: Delete ServiceTemplate without uninstalling Helm release
**Given** a ServiceTemplate "minio-14-1-2" installed via Helm release "minio"
**And** MCP tool call `delete(app="minio", template="minio", version="14.1.2", namespace="kcm-system")`
**When** the tool executes
**Then** the server uses the dynamic Kubernetes client to delete the ServiceTemplate resource
**And** does NOT invoke `helm uninstall`
**And** the Helm release secret remains (release is orphaned)
**And** the HelmRepository resource is also deleted if specified
**And** returns `{deleted: ["kcm-system/ServiceTemplate/minio-14-1-2"], status: "deleted"}`

**Note:** Future enhancement could integrate `helm uninstall`, but this is out of scope for initial implementation.

### Requirement: Namespace filter enforcement
The server MUST continue to enforce namespace filtering for catalog installations when using Helm.

#### Scenario: Install rejected for disallowed namespace
**Given** session namespace filter allows only ["kcm-system"]
**And** MCP tool call `install_from_catalog(app="minio", template="minio", version="14.1.2", namespace="other-ns")`
**When** validating the target namespace
**Then** the server rejects the installation before invoking Helm
**And** returns an MCP error: "namespace other-ns not allowed"
**And** no Helm release is created

#### Scenario: Install allowed for allowed namespace
**Given** session namespace filter allows ["kcm-system", "custom-ns"]
**And** MCP tool call `install_from_catalog(app="minio", template="minio", version="14.1.2", namespace="custom-ns")`
**When** validating the target namespace
**Then** the server permits the installation
**And** proceeds with Helm invocation in namespace "custom-ns"
**And** creates ServiceTemplate in "custom-ns"

## REMOVED Requirements

### Requirement: Direct manifest application
~~The server MUST fetch ServiceTemplate and HelmRepository manifests from the local catalog git repository and apply them directly using the Kubernetes dynamic client.~~

**Rationale:** Replaced by Helm-based installation using kgst chart.

#### ~~Scenario: Fetch manifests from catalog~~
**Removed:** No longer fetching individual manifest files; Helm renders kgst templates instead.

#### ~~Scenario: Apply ServiceTemplate with dynamic client~~
**Removed:** ServiceTemplate is now created by Helm via kgst chart, not direct apply.

#### ~~Scenario: Convert v1alpha1 to v1beta1~~
**Removed:** kgst handles API version via `k0rdentApiVersion` value, no manual conversion needed.

### Requirement: Manual HelmRepository creation
~~The server MUST create or update the HelmRepository resource before creating the ServiceTemplate.~~

**Rationale:** kgst Helm chart creates HelmRepository via pre-install hook, ensuring correct order automatically.

#### ~~Scenario: Create HelmRepository before ServiceTemplate~~
**Removed:** Helm hooks enforce ordering; pre-install hook creates HelmRepository before main template renders ServiceTemplate.

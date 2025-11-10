# Capability: Helm Integration

## ADDED Requirements

### Requirement: Helm SDK for chart operations
The MCP server MUST use the official Helm v3 Go SDK (`helm.sh/helm/v3`) to perform chart installations rather than shelling out to the helm CLI or manually applying manifests.

#### Scenario: Configure Helm action client
**Given** the MCP server has initialized a runtime session with Kubernetes REST client configuration
**When** preparing to install a ServiceTemplate via kgst
**Then** the server creates an `action.Configuration` using the session's REST client config
**And** sets the target namespace for Helm operations
**And** configures Helm to store release state in Secrets (default storage driver)
**And** wires Helm's debug output to the server's structured logger

#### Scenario: Execute helm upgrade with install flag
**Given** a configured Helm action client
**And** kgst chart values specifying template name, version, and repository details
**When** executing the installation
**Then** the server uses `action.Upgrade` with `Install: true` to create or update the release
**And** sets `Namespace` to the resolved target namespace
**And** sets `Wait: true` to wait for pre-install hooks (verify job) to complete
**And** sets `Timeout: 5*time.Minute` to allow verification job to run
**And** sets `Atomic: true` to rollback on any failure
**And** invokes `RunWithContext()` to support context cancellation

### Requirement: Load kgst chart from OCI registry
The server MUST pull the kgst Helm chart from the official k0rdent OCI registry rather than bundling it locally or generating templates inline.

#### Scenario: Pull kgst chart
**Given** a Helm registry client configured for OCI operations
**When** preparing to install a ServiceTemplate
**Then** the server pulls the chart from `oci://ghcr.io/k0rdent/catalog/charts/kgst`
**And** uses version "2.0.0" (or configured kgst version)
**And** caches the chart locally per Helm's default caching behavior
**And** returns an error if the chart cannot be pulled (network failure, registry unavailable)

#### Scenario: Verify chart identity
**Given** a pulled kgst chart
**When** validating the chart before installation
**Then** the server confirms the chart name is "kgst"
**And** confirms the chart version matches the expected version
**And** returns an error if the chart identity is unexpected

### Requirement: Construct kgst values from MCP parameters
The server MUST transform MCP tool parameters (app, template, version, namespace) into kgst chart values matching the official catalog installation pattern.

#### Scenario: Map parameters to kgst values
**Given** MCP tool call with `app="minio"`, `template="minio"`, `version="14.1.2"`, `namespace="kcm-system"`
**When** preparing kgst values
**Then** the server sets `chart: "minio:14.1.2"` (format: "name:version")
**And** sets `repo.name: "k0rdent-catalog"`
**And** sets `repo.spec.url: "oci://ghcr.io/k0rdent/catalog/charts"`
**And** sets `repo.spec.type: "oci"`
**And** sets `namespace: "kcm-system"`
**And** sets `k0rdentApiVersion: "v1beta1"`
**And** sets `skipVerifyJob: false`
**And** omits the `prefix` field (uses default empty string)

#### Scenario: Enforce chart format validation via kgst
**Given** MCP tool call with malformed version like `version="14-1-2"` (dashes instead of dots)
**When** kgst renders templates
**Then** kgst's built-in validation fails because chart format must be "name:version" with dots
**And** Helm returns an error
**And** the server surfaces this as an installation failure with message indicating invalid format

### Requirement: Handle pre-install verification
The server MUST allow kgst's pre-install verification job to run and handle its success or failure appropriately.

#### Scenario: Verification job succeeds
**Given** a ServiceTemplate installation via kgst
**And** `skipVerifyJob: false`
**When** the kgst verify-job runs
**Then** the job pulls the target chart from the catalog to confirm it exists
**And** the job completes successfully
**And** Helm proceeds to create HelmRepository and ServiceTemplate resources
**And** the installation succeeds

#### Scenario: Verification job fails due to non-existent chart
**Given** a ServiceTemplate installation with `chart="nonexistent:1.0.0"`
**When** the kgst verify-job attempts to pull the chart
**Then** the job fails because the chart doesn't exist
**And** Helm treats the pre-install hook failure as a release failure
**And** the server parses the error to extract "Chart does not exist" message
**And** returns an MCP error: "chart nonexistent:1.0.0 not found in k0rdent catalog"

#### Scenario: Verification job times out
**Given** a ServiceTemplate installation with `Timeout: 5*time.Minute`
**And** the verify-job takes longer than 5 minutes (e.g., network issue)
**When** the timeout expires
**Then** Helm cancels the installation
**And** rolls back any created resources (due to `Atomic: true`)
**And** the server returns an error indicating timeout during verification

### Requirement: Preserve idempotent behavior
The server MUST maintain idempotent installation behavior when using Helm, matching the existing direct-apply behavior.

#### Scenario: First installation
**Given** no existing Helm release for "minio" in namespace "kcm-system"
**When** installing via `helm upgrade --install`
**Then** Helm creates a new release named "minio"
**And** creates the HelmRepository resource
**And** runs the verification job
**And** creates the ServiceTemplate resource
**And** marks the release as "deployed"
**And** the server returns status "created"

#### Scenario: Repeated installation with same values
**Given** an existing Helm release for "minio" version "14.1.2" in namespace "kcm-system"
**When** installing the same template and version again
**Then** Helm detects no changes in values or chart version
**And** does not create a new release revision
**And** the ServiceTemplate and HelmRepository remain unchanged
**And** the server returns status "updated" (or "unchanged")

#### Scenario: Repeated installation with different values
**Given** an existing Helm release for "minio" version "14.1.2"
**When** installing a different version "14.2.0" with the same release name
**Then** Helm creates a new release revision
**And** runs the verification job for the new version
**And** updates the ServiceTemplate to reference the new chart version
**And** the server returns status "updated"

### Requirement: Error handling and observability
The server MUST provide clear error messages and structured logs for Helm operations.

#### Scenario: Network failure pulling kgst chart
**Given** the OCI registry `ghcr.io` is unreachable
**When** attempting to pull the kgst chart
**Then** the Helm registry client returns a network error
**And** the server logs the error with context (registry URL, chart name)
**And** returns an MCP error: "failed to pull kgst chart from ghcr.io: <network error>"

#### Scenario: Validation webhook rejects ServiceTemplate
**Given** Helm successfully creates the ServiceTemplate resource
**But** the k0rdent admission webhook rejects it due to invalid spec
**When** the webhook returns a rejection
**Then** Helm treats this as a release failure
**And** rolls back the release (due to `Atomic: true`)
**And** the server extracts the webhook error message
**And** returns an MCP error including the webhook rejection reason

#### Scenario: Log Helm operations
**Given** a ServiceTemplate installation in progress
**When** Helm executes various operations
**Then** the server logs structured entries for:
- Pulling kgst chart (debug level)
- Starting verification job (debug level)
- Verification job completion (info level if success, error level if failure)
- Creating HelmRepository (debug level)
- Creating ServiceTemplate (info level)
- Release status (info level)
**And** each log entry includes fields: tool="k0rdent.mgmt.serviceTemplates.install_from_catalog", release_name, namespace, chart, version

### Requirement: RBAC compatibility
The server's service account MUST have sufficient permissions to perform Helm operations including verification jobs.

#### Scenario: Required Kubernetes permissions
**Given** the MCP server's service account
**When** performing a Helm installation with kgst
**Then** the service account has permissions to:
- Create/update/delete Secrets (for Helm release storage) in target namespace
- Create/update ServiceTemplate resources (k0rdent.mirantis.com/v1beta1)
- Create/update HelmRepository resources (source.toolkit.fluxcd.io/v1)
- Create/delete Jobs (batch/v1) in target namespace
- Create/delete Pods (v1) in target namespace (for verify-job)
- Get Pod logs (v1/pods/log) in target namespace (for error reporting)

#### Scenario: Insufficient permissions
**Given** the service account lacks permission to create Jobs
**When** Helm attempts to create the verify-job
**Then** the Kubernetes API returns a forbidden error
**And** Helm treats this as a release failure
**And** the server logs the permission error
**And** returns an MCP error indicating RBAC issue: "failed to create verify-job: forbidden"

### Requirement: Namespace handling
The server MUST respect existing namespace resolution logic when configuring Helm operations.

#### Scenario: DEV_ALLOW_ANY mode with default namespace
**Given** MCP server running in DEV_ALLOW_ANY auth mode
**And** MCP tool call without explicit namespace parameter
**When** resolving the target namespace
**Then** the server defaults to "kcm-system"
**And** passes "kcm-system" to Helm as the release namespace
**And** passes "kcm-system" in kgst values as `namespace: "kcm-system"`

#### Scenario: OIDC_REQUIRED mode requires explicit namespace
**Given** MCP server running in OIDC_REQUIRED auth mode
**And** MCP tool call without namespace or all_namespaces parameter
**When** attempting to install
**Then** the existing validation fails before Helm invocation
**And** returns an error: "namespace required in OIDC mode"

#### Scenario: Install to multiple namespaces
**Given** MCP tool call with `all_namespaces=true`
**And** session namespace filter allows ["ns-a", "ns-b"]
**When** executing installation
**Then** the server creates separate Helm releases in "ns-a" and "ns-b"
**And** each release is named after the template (e.g., "minio")
**And** each release creates its own ServiceTemplate and HelmRepository in the respective namespace

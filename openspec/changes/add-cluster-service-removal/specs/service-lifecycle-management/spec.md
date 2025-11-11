# Specification: Service Lifecycle Management

## Overview
This specification defines how operators manage the lifecycle of services attached to ClusterDeployments through MCP tools, covering service addition, updates, and removal operations.

---

## ADDED Requirements

### Requirement: Service Removal via MCP Tool
**ID**: service-removal-001
**Category**: Service Management
**Priority**: High

Operators SHALL be able to remove a service from a running ClusterDeployment using an MCP tool that modifies `spec.serviceSpec.services[]` through server-side apply.

#### Scenario: Remove Service from Multi-Service Cluster
**Given** a ClusterDeployment named "prod-cluster" in namespace "team-alpha" with services ["ingress", "minio", "logging"] in `spec.serviceSpec.services[]`
**When** operator invokes `k0rdent.mgmt.clusterDeployments.services.remove` with clusterNamespace="team-alpha", clusterName="prod-cluster", serviceName="minio"
**Then** the tool returns success with removedService containing the minio entry details
**And** the ClusterDeployment `spec.serviceSpec.services[]` contains only ["ingress", "logging"]
**And** the k0rdent controller reconciles by uninstalling the minio Helm release

#### Scenario: Remove Last Service from Cluster
**Given** a ClusterDeployment with a single service "monitoring" in `spec.serviceSpec.services[]`
**When** operator removes serviceName="monitoring"
**Then** the tool returns success
**And** the ClusterDeployment `spec.serviceSpec.services[]` becomes an empty array

#### Scenario: Idempotent Removal of Already-Removed Service
**Given** a ClusterDeployment with services ["ingress", "logging"]
**When** operator attempts to remove serviceName="minio" (which doesn't exist)
**Then** the tool returns success with removedService=null
**And** message indicates "service not found (already removed)"
**And** the ClusterDeployment `spec.serviceSpec.services[]` remains unchanged

---

### Requirement: Dry-Run Preview for Service Removal
**ID**: service-removal-002
**Category**: Safety & Validation
**Priority**: High

Operators SHALL be able to preview service removal operations without mutating the ClusterDeployment by using a dry-run flag.

#### Scenario: Preview Service Removal
**Given** a ClusterDeployment with services ["ingress", "minio", "logging"]
**When** operator invokes remove with serviceName="minio" and dryRun=true
**Then** the tool returns what would be removed (minio entry details)
**And** shows the resulting services array (["ingress", "logging"])
**And** the actual ClusterDeployment remains unchanged (still has all three services)

---

### Requirement: Namespace Authorization for Service Removal
**ID**: service-removal-003
**Category**: Security
**Priority**: High

Service removal operations SHALL enforce namespace authorization—operators can only remove services from ClusterDeployments in namespaces allowed by their session's namespace filter.

#### Scenario: Remove Service in Allowed Namespace
**Given** an operator session with namespace filter allowing "team-.*"
**And** a ClusterDeployment in namespace "team-alpha"
**When** operator removes a service from that ClusterDeployment
**Then** the operation succeeds

#### Scenario: Reject Removal in Forbidden Namespace
**Given** an operator session with namespace filter allowing "team-.*"
**And** a ClusterDeployment in namespace "production"
**When** operator attempts to remove a service from that ClusterDeployment
**Then** the operation fails with error "namespace 'production' not allowed by namespace filter"

#### Scenario: Global Namespace Always Allowed
**Given** an operator session with a restrictive namespace filter
**When** operator removes a service from a ClusterDeployment in "kcm-system" namespace
**Then** the operation succeeds (global namespace is always accessible)

---

### Requirement: Detailed Removal Result
**ID**: service-removal-004
**Category**: Observability
**Priority**: Medium

Service removal operations SHALL return comprehensive results including the removed service entry, updated services list, and current cluster status.

#### Scenario: Detailed Removal Response
**Given** a successful service removal operation
**Then** the response includes:
- `removedService`: The full service entry that was removed (with name, template, namespace, values, etc.)
- `updatedServices`: The current list of services remaining in `spec.serviceSpec.services[]`
- `message`: A descriptive status message (e.g., "service 'minio' removed successfully")
- `clusterStatus`: Current ClusterDeployment status including `.status.services[]` showing remaining services

---

### Requirement: Error Handling for Nonexistent Resources
**ID**: service-removal-005
**Category**: Error Handling
**Priority**: Medium

Service removal SHALL provide clear error messages when target resources don't exist or are inaccessible.

#### Scenario: ClusterDeployment Not Found
**Given** an operator attempts to remove a service
**When** the specified ClusterDeployment does not exist
**Then** the tool returns error "cluster deployment not found: <namespace>/<name>"
**And** no changes are applied

#### Scenario: Service Name Required
**Given** an operator invokes the removal tool
**When** serviceName parameter is empty or missing
**Then** the tool returns validation error "service name is required"
**And** no ClusterDeployment lookup is performed

---

## MODIFIED Requirements

None—this change adds new capabilities without modifying existing service apply behavior.

---

## REMOVED Requirements

None—all existing service management requirements remain valid.

---

## Cross-References
- **Related Change**: `add-running-cluster-servicetemplate-install` - Provides the apply operation that this removal complements
- **Controller Behavior**: k0rdent ClusterDeployment controller (external to MCP server) reconciles service removals by uninstalling Helm releases
- **API Pattern**: Follows server-side apply pattern established by `ApplyClusterService` in `internal/k0rdent/api/services.go`

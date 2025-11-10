## ADDED Requirements

### Requirement: Azure cluster detail tool provides deep infrastructure inspection

- The MCP server **SHALL** expose `k0rdent.provider.azure.clusterDeployments.detail(name, namespace) -> AzureClusterDeploymentDetail`.
- This tool provides a **complementary view to `k0rdent.mgmt.clusterDeployments.getState`**:
  - `getState` = operational monitoring (basic metadata, deployment progress, service states)
  - `detail` = deep infrastructure inspection (Azure resource IDs, networking topology, provider-specific configurations)
- Input validation **SHALL** ensure both `name` and `namespace` are provided; the tool **SHALL** return `notFound` when either the k0rdent `ClusterDeployment` or provider infrastructure CRs are missing.
- The response **SHALL** merge data from:
  - `ClusterDeployment.k0rdent.mirantis.com` (template, credential, config.location, subscriptionID)
  - `Cluster.cluster.x-k8s.io` (control plane endpoint, infrastructure references)
  - `AzureCluster.infrastructure.cluster.x-k8s.io` (resourceGroup, identityRef, networkSpec, subscriptionID, Azure-specific conditions)
  - Related Helm/Flux annotations when present (management URLs, owner info).

#### Scenario: Successful lookup
- GIVEN `ClusterDeployment azure-prod-cluster` in namespace `kcm-system`
- AND the corresponding `AzureCluster` exists with `spec.resourceGroup=azure-prod-cluster`
- WHEN `k0rdent.provider.azure.clusterDeployments.detail(name="azure-prod-cluster", namespace="kcm-system")` is called
- THEN the response contains `resourceGroup="azure-prod-cluster"`, `subscriptionID`, `location`, `controlPlaneEndpoint.host`, `identityRef`, VNet/subnet/NAT/LB identifiers, and provider-specific condition summaries (`ResourceGroupReady`, `NetworkInfrastructureReady`, etc.).

### Requirement: Response schema structure and fields
- `AzureClusterDeploymentDetail` **SHALL** include the following top-level sections:
  - `metadata`: Shared cluster metadata (same as `getState` for consistency)
    - name, namespace, templateRef (name/namespace/version), credentialRef (name/namespace)
    - cloudProvider (`azure`), region/location
    - createdAt timestamp
  - `azure`: Provider-specific Azure infrastructure details (top-level key for clear AI agent parsing)
    - `resourceGroup`: Azure resource group name
    - `subscriptionID`: Azure subscription GUID
    - `location`: Azure region (e.g., "westus2", "eastus")
    - `identity`: Credential and identity references
      - `credentialRef`: k0rdent Credential reference
      - `clusterIdentityRef`: k0rdent ClusterIdentity reference
      - `azureClusterIdentityRef`: AzureCluster identityRef (kind/name/namespace)
      - Optional: service principal/managed identity details when available
    - `controlPlane`: Control plane endpoint and access
      - `endpoint`: Control plane API server URL (host and port)
      - `kubeconfigSecret`: Reference to kubeconfig secret
      - Optional: k0s/kubernetes version when available
    - `infrastructure`: Azure networking and infrastructure resource IDs
      - `vnet`: VNet details (ARM resource ID, name, resourceGroup)
      - `subnets[]`: Array of subnet objects (ARM resource ID, name, role)
      - `natGateways[]`: NAT gateway configurations and IDs
      - `routeTables[]`: Route table IDs
      - `securityGroups[]`: Network security group IDs
      - `apiServerLoadBalancer`: Load balancer configuration and ID
      - `publicIPs[]`: Public IP addresses and IDs
    - `conditions`: Azure-specific status conditions (ResourceGroupReady, NetworkInfrastructureReady, etc.)
  - `deployment`: Minimal deployment status summary (NOT full monitoring view)
    - `ready`: boolean
    - `phase`: current phase string
    - `conditions`: aggregated high-level conditions
  - `links`: Optional management URLs from annotations
- Each ARM resource identifier (e.g., subnet IDs) **SHALL** be returned exactly as stored in the AzureCluster CR to simplify downstream automation.
- The response structure **SHALL** use top-level `azure` key to clearly delineate provider-specific data for AI agent consumption.

#### Scenario: Partial data handling
- WHEN the AzureCluster exists but optional sections (e.g., NAT gateway) are absent
- THEN the tool returns empty arrays/objects for those sections without failing the call.

### Requirement: Error handling and diagnostics
- If the `ClusterDeployment` exists but the provider CR does not, the tool **SHALL** return an MCP error with code `notFound` and include troubleshooting guidance (e.g., "AzureCluster azure-prod-cluster not found").
- If multiple related CRs are missing required fields, the tool **SHALL** indicate which resource is incomplete (e.g., `azure-prod-cluster: spec.resourceGroup missing`).
- Calls **SHALL** be logged with cluster name, namespace, and provider to aid auditing.

#### Scenario: Missing provider CR
- WHEN `k0rdent.provider.azure.clusterDeployments.detail` is called but the `AzureCluster` resource has been deleted
- THEN the tool responds with `notFound` and message `AzureCluster azure-prod-cluster (kcm-system) not found`, without leaking partial data.

### Requirement: Scope boundaries - what detail tools exclude

- The provider detail tools **SHALL NOT** include:
  - Detailed service deployment states (names, templates, states, conditions of services on the cluster)
  - Real-time provisioning progress updates or phase transitions
  - Recent Kubernetes events or logs
  - Historical deployment timeline or audit trail
- These excluded items **SHALL** be obtained via:
  - Service states: `k0rdent.mgmt.clusterDeployments.getState` tool
  - Progress monitoring: `k0rdent://cluster-monitor` subscription or `getState` tool
  - Events: Existing event listing/subscription tools
  - Logs: Existing pod log tools

#### Scenario: Detail tool excludes service state

```
Given a ClusterDeployment "azure-prod" with 5 services deployed (3 ready, 2 pending)
When k0rdent.provider.azure.clusterDeployments.detail is called
Then the response includes Azure infrastructure details (resourceGroup, VNet IDs, etc.)
But does NOT include:
  - Individual service names
  - Service states (Ready, Pending, Failed)
  - Service conditions or error messages
And the client must call getState to retrieve service deployment status
```

#### Scenario: Detail tool excludes progress monitoring

```
Given a ClusterDeployment "azure-test" is currently in "Provisioning" phase
When k0rdent.provider.azure.clusterDeployments.detail is called
Then the response includes a minimal deployment status:
  - ready: false
  - phase: "Provisioning"
  - High-level conditions
But does NOT include:
  - Estimated progress percentage
  - Phase transition history
  - Recent significant events
  - Real-time progress updates
And the client must use getState or cluster-monitor subscription for progress monitoring
```

### Requirement: Extensibility for other providers
- The spec **SHALL** document that AWS/GCP equivalents must follow the same pattern (`k0rdent.provider.aws.clusterDeployments.detail`, etc.), reusing common metadata fields plus provider-specific sections (e.g., VPC/subnets/ELBs for AWS).
- Implementation planning for AWS/GCP **SHALL** reference this Azure spec to ensure consistent naming and response structure (shared metadata block + provider-specific block at top level).
- All providers **SHALL** use top-level provider keys (`aws`, `gcp`) to clearly delineate provider-specific infrastructure data.

#### Scenario: AWS parity planning
- WHEN defining `k0rdent.provider.aws.clusterDeployments.detail`
- THEN the tool reuses the shared metadata block (`templateRef`, `credentialRef`) and adds top-level `aws` section with AWS-specific details (VPC ID, subnet IDs, security group IDs, ELB configurations, IAM roles) mirroring the Azure structure for consistent UX.

#### Scenario: GCP parity planning
- WHEN defining `k0rdent.provider.gcp.clusterDeployments.detail`
- THEN the tool reuses the shared metadata block and adds top-level `gcp` section with GCP-specific details (project ID, network name, subnet IDs, firewall rule IDs, service account details) mirroring the Azure structure for consistent UX.

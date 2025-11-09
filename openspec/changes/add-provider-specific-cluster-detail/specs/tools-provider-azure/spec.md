## ADDED Requirements

### Requirement: Azure cluster detail tool
- The MCP server **SHALL** expose `k0rdent.provider.azure.clusterDeployments.detail(name, namespace) -> AzureClusterDeploymentDetail`.
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

### Requirement: Response schema fields
- `AzureClusterDeploymentDetail` **SHALL** include the following sections:
  - `metadata`: name, namespace, templateRef (name/version), credentialRef, cloudProvider (`azure`), location, subscriptionID.
  - `controlPlane`: host, port, k0s/kubernetes version (if available), kubeconfigSecret reference.
  - `identity`: `credentialRef`, `clusterIdentityRef`, `azureCluster.identityRef` (kind/name/namespace), and linked service principal/managed identity info when available.
  - `infrastructure`: `resourceGroup`, `vnet` (id/name/resourceGroup), array of `subnets[]` (id/name/role), `natGateways[]`, `routeTables[]`, `securityGroups[]`, `apiServerLoadBalancer`, and any public IPs referenced.
  - `status`: aggregated conditions from ClusterDeployment, Cluster, and AzureCluster, plus derived phase/ready booleans.
  - `links`: optional management URLs from annotations (e.g., `k0rdent.mirantis.com/management-url`, `cloud.k0rdent.mirantis.com/management-url`).
- Each ARM resource identifier (e.g., subnet IDs) **SHALL** be returned exactly as stored in the AzureCluster CR to simplify downstream automation.

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

### Requirement: Extensibility for other providers
- The spec **SHALL** document that AWS/GCP equivalents must follow the same pattern (`k0rdent.provider.aws.clusterDeployments.detail`, etc.), reusing common metadata fields plus provider-specific sections (e.g., VPC/subnets/ELBs for AWS).
- Implementation planning for AWS/GCP **SHALL** reference this Azure spec to ensure consistent naming and response structure (shared metadata block + provider-specific block).

#### Scenario: AWS parity planning
- WHEN defining `k0rdent.provider.aws.clusterDeployments.detail`
- THEN the tool reuses the shared metadata block (`templateRef`, `credentialRef`, status) and adds AWS-specific sections (VPC ID, subnets, security groups, load balancers) mirroring the Azure structure for consistent UX.

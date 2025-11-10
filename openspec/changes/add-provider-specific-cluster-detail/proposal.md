# Change: Add provider-specific cluster detail tools

## Why
- Operators need a single MCP call to retrieve provider-specific metadata (resource groups, subscription IDs, network object IDs, etc.) for a given cluster. Today they must manually inspect multiple CRDs (`ClusterDeployment`, CAPI `Cluster`, and provider CRs such as `AzureCluster`).
- Azure documentation already references the `AzureCluster.spec.resourceGroup` field, but there is no MCP surface (e.g., `k0rdent.provider.azure.clusterDeployments.detail`) to expose it. The same gap exists for AWS (`AWSCluster`, `AWSMachine`), GCP, etc.
- Having standard provider detail tools accelerates troubleshooting and enables automation (drift detection, FinOps) without direct kubectl access.

## What Changes
- Introduce provider-specific detail tools (`k0rdent.provider.<provider>.clusterDeployments.detail`) that return **deep infrastructure inspection views** with provider-specific resource IDs and topology details.
- These tools provide a **complementary view to `getState`**:
  - `getState` = operational monitoring (basic metadata, deployment progress, service states)
  - `detail` = infrastructure inspection (resource IDs, networking topology, provider-specific configurations)
- Start with Azure: `k0rdent.provider.azure.clusterDeployments.detail(name, namespace)` returns resource group, subscription ID, location, control plane endpoint, identity references, VNet/subnet/NAT/LB IDs, Azure-specific conditions, etc.
- Implement AWS: `k0rdent.provider.aws.clusterDeployments.detail(name, namespace)` returns VPC ID, subnet IDs, security group IDs, ELB configurations, IAM roles, etc.
- Implement GCP: `k0rdent.provider.gcp.clusterDeployments.detail(name, namespace)` returns project ID, network name, subnet IDs, firewall rules, service account details, etc.
- All tools follow the same pattern: shared metadata block + provider-specific infrastructure section at top level (e.g., `azure: {...}`, `aws: {...}`, `gcp: {...}`).

## Scope Boundaries

### What These Tools Include:
- Basic cluster metadata (name, namespace, templateRef, credentialRef, provider, region) - same as `getState` for consistency
- Provider-specific infrastructure resource IDs (VNet IDs, subnet IDs, security groups, load balancers, NAT gateways, etc.)
- Provider-specific networking topology (CIDRs, routes, peering, etc.)
- Provider-specific identity and access configurations (service principals, IAM roles, service accounts)
- Control plane endpoint details (host, port, certificate authority)
- Kubeconfig secret references
- Provider-specific status conditions (ResourceGroupReady, NetworkInfrastructureReady, etc.)

### What These Tools Exclude:
- Detailed service deployment states (use `getState` for service monitoring)
- Real-time progress monitoring (use `getState` or cluster-monitor subscription)
- Historical events or logs (use existing event/log tools)
- Cross-cluster comparisons or aggregations (single cluster focus)

## Impact
- Platform teams can obtain complete provider infrastructure topology (Azure resource groups/VNets, AWS VPCs/subnets, GCP networks) from one MCP call, aligning with docs.k0rdent.io guidance.
- Establishes consistent pattern across Azure, AWS, and GCP for infrastructure inspection.
- Complements `getState` operational monitoring with deep infrastructure visibility.
- No behavioral change for existing tools; this is an additive discovery surface.
- AI agents can easily parse provider-specific details due to clear top-level provider keys (`azure`, `aws`, `gcp`).

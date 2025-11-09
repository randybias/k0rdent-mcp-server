# Change: Add provider-specific cluster detail tools

## Why
- Operators need a single MCP call to retrieve provider-specific metadata (resource groups, subscription IDs, network object IDs, etc.) for a given cluster. Today they must manually inspect multiple CRDs (`ClusterDeployment`, CAPI `Cluster`, and provider CRs such as `AzureCluster`).
- Azure documentation already references the `AzureCluster.spec.resourceGroup` field, but there is no MCP surface (e.g., `k0rdent.provider.azure.clusterDeployments.detail`) to expose it. The same gap exists for AWS (`AWSCluster`, `AWSMachine`), GCP, etc.
- Having standard provider detail tools accelerates troubleshooting and enables automation (drift detection, FinOps) without direct kubectl access.

## What Changes
- Introduce a provider namespace (`k0rdent.provider.<provider>.clusterDeployments.detail`) that returns a merged view of ClusterDeployment, CAPI Cluster, and provider-specific CRs.
- Start with Azure: `k0rdent.provider.azure.clusterDeployments.detail(name, namespace)` returns resource group, subscription ID, location, control plane endpoint, identity references, VNet/subnet/NAT/LB IDs, status conditions, etc.
- Define parity requirements so AWS/GCP equivalents can follow the same pattern (tool naming, input shape, output schema with provider-specific sections).

## Impact
- Platform teams can obtain the entire Azure topology (resource group, subnets, NSGs, NAT gateways) from one MCP call, aligning with docs.k0rdent.io guidance.
- Establishes the pattern for future provider detail tools, ensuring consistent ergonomics across Azure, AWS, GCP.
- No behavioural change for existing tools; this is an additive discovery surface.

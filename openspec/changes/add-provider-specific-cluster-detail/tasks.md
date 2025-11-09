## Tasks
1. [ ] Define the `k0rdent.provider.azure.clusterDeployments.detail` tool contract (inputs, outputs, required fields) and document the Azure data sources (ClusterDeployment, Cluster, AzureCluster, related child objects).
2. [ ] Create provider-specific spec delta(s) that describe how detail tools must surface metadata for Azure (resourceGroup, subscriptionID, location, VNet/subnets/NAT/LB IDs, identity references, status conditions, etc.) and set expectations for parallel AWS/GCP implementations.
3. [ ] Outline validation requirements/tests (e.g., mock CRDs or integration coverage) to ensure the tool returns complete data and handles missing provider CRs gracefully.

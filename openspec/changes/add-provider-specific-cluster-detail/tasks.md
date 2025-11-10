## Tasks

### Phase 1: Define Types and Contracts

1. [x] Define provider-specific detail types in `internal/clusters/types.go`
   - Create `ClusterDeploymentDetail` base structure with shared metadata and deployment status
   - Create `AzureClusterDeploymentDetail` with top-level `Azure` field
   - Create `AWSClusterDeploymentDetail` with top-level `AWS` field
   - Create `GCPClusterDeploymentDetail` with top-level `GCP` field
   - Define nested types for each provider (identity, infrastructure, controlPlane)
   - Document clear boundaries: includes infrastructure IDs, excludes service states/progress monitoring

### Phase 2: Implement Cluster Manager Methods

2. [x] Implement Azure detail method in `internal/clusters/detail_azure.go`
   - `GetAzureClusterDetail(ctx, namespace, name) (*AzureClusterDeploymentDetail, error)`
   - Fetch ClusterDeployment, CAPI Cluster, and AzureCluster resources
   - Extract shared metadata (reusing SummarizeClusterDeployment patterns)
   - Extract Azure-specific fields: resourceGroup, subscriptionID, location, identity refs
   - Extract Azure infrastructure: VNet ID, subnet IDs, NSG IDs, NAT gateway IDs, LB IDs
   - Extract control plane endpoint from CAPI Cluster
   - Extract Azure-specific conditions from AzureCluster
   - Return notFound error if provider CR missing

3. [x] Implement AWS detail method in `internal/clusters/detail_aws.go`
   - `GetAWSClusterDetail(ctx, namespace, name) (*AWSClusterDeploymentDetail, error)`
   - Fetch ClusterDeployment, CAPI Cluster, and AWSCluster resources
   - Extract shared metadata
   - Extract AWS-specific fields: region, account ID
   - Extract AWS infrastructure: VPC ID, subnet IDs, security group IDs, ELB configurations, IAM roles
   - Extract control plane endpoint
   - Extract AWS-specific conditions
   - Return notFound error if provider CR missing

4. [x] Implement GCP detail method in `internal/clusters/detail_gcp.go`
   - `GetGCPClusterDetail(ctx, namespace, name) (*GCPClusterDeploymentDetail, error)`
   - Fetch ClusterDeployment, CAPI Cluster, and GCPCluster resources
   - Extract shared metadata
   - Extract GCP-specific fields: project ID, region
   - Extract GCP infrastructure: network name, subnet IDs, firewall rule IDs, service account details
   - Extract control plane endpoint
   - Extract GCP-specific conditions
   - Return notFound error if provider CR missing

### Phase 3: Register MCP Tools

5. [x] Register Azure detail tool in `internal/tools/core/clusters.go`
   - Tool name: `k0rdent.provider.azure.clusterDeployments.detail`
   - Description emphasizing deep infrastructure inspection (complements getState)
   - Input: name (required), namespace (optional, follows standard patterns)
   - Call `session.Clusters.GetAzureClusterDetail()`
   - Return structured response with top-level `azure` key

6. [x] Register AWS detail tool in `internal/tools/core/clusters.go`
   - Tool name: `k0rdent.provider.aws.clusterDeployments.detail`
   - Description consistent with Azure pattern
   - Input: name (required), namespace (optional)
   - Call `session.Clusters.GetAWSClusterDetail()`
   - Return structured response with top-level `aws` key

7. [x] Register GCP detail tool in `internal/tools/core/clusters.go`
   - Tool name: `k0rdent.provider.gcp.clusterDeployments.detail`
   - Description consistent with Azure/AWS patterns
   - Input: name (required), namespace (optional)
   - Call `session.Clusters.GetGCPClusterDetail()`
   - Return structured response with top-level `gcp` key

### Phase 4: Testing

8. [x] Create Azure detail tool tests in `internal/clusters/detail_azure_test.go`
   - Test successful detail fetch with full infrastructure
   - Test partial infrastructure (missing optional fields like NAT gateway)
   - Test missing AzureCluster (notFound error)
   - Test namespace authorization
   - Mock all CRDs (ClusterDeployment, Cluster, AzureCluster)

9. [x] Create AWS detail tool tests in `internal/clusters/detail_aws_test.go`
   - Same test patterns as Azure
   - Mock AWSCluster and related resources

10. [x] Create GCP detail tool tests in `internal/clusters/detail_gcp_test.go`
    - Same test patterns as Azure/AWS
    - Mock GCPCluster and related resources

11. [ ] Add integration tests (optional, may require live clusters)
    - Test detail tools return expected structure for real deployments
    - Verify top-level provider keys (azure, aws, gcp) are present
    - Verify service states are NOT included (boundary check)

### Phase 5: Documentation

12. [ ] Document provider detail tools in `docs/provider-specific-deployment.md` or similar
    - Clarify the two-tier model: getState (operational) vs detail (infrastructure)
    - Provide examples for each provider showing expected output structure
    - Document when to use which tool

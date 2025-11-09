# Proposal: Add Cloud Provider Configuration Validation for Cluster Deployments

## Why

Cloud cluster deployments (AWS, Azure, GCP) currently fail 5-15 minutes into provisioning when required configuration fields are missing. This wastes user time and incurs unnecessary cloud costs. By adding pre-flight configuration validation for all major cloud providers, we can catch these errors immediately at submission time and provide clear, actionable feedback to users.

## Problem Statement

The MCP server's `k0rdent.mgmt.clusterDeployments.deploy` tool currently accepts invalid cloud provider configurations that fail to provision due to missing required fields. Specifically:

1. **Missing provider-specific validation**: The tool does not validate required fields for any cloud provider:
   - **Azure**: Missing `subscriptionID` and `location` validation ([docs](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/))
   - **AWS**: Missing `region` validation ([docs](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/))
   - **GCP**: Missing `project`, `region`, and `network.name` validation ([docs](https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/))

2. **Inconsistent field naming across providers**: Each provider uses different terminology for similar concepts (Azure: `vmSize`, AWS: `instanceType`, GCP: `instanceType`), but the tool accepts any config structure without validation or guidance.

3. **Late failure detection**: Configuration errors are only discovered during cloud resource provisioning (5-15 minutes into deployment) rather than at submission time, wasting time and incurring unnecessary cloud costs.

4. **Poor user experience**: Error messages appear in Kubernetes events (e.g., `AzureCluster`, `AWSCluster` resource warnings) rather than being surfaced immediately to the MCP tool caller.

## Current Behavior

**Example failures for each provider:**

### Azure (Missing subscriptionID)
```json
{
  "name": "test-azure-cluster",
  "template": "azure-standalone-cp-1-0-17",
  "credential": "azure-cluster-credential",
  "config": {
    "location": "westus2",
    "controlPlane": {"vmSize": "Standard_A4_v2"},
    "worker": {"vmSize": "Standard_A4_v2"}
    // Missing: subscriptionID
  }
}
```
**Result:** Fails after 5-10 minutes with "subscriptionID is not set"

### AWS (Missing region)
```json
{
  "name": "test-aws-cluster",
  "template": "aws-standalone-cp-1-0-16",
  "credential": "aws-cluster-credential",
  "config": {
    "controlPlane": {"instanceType": "t3.small"},
    "worker": {"instanceType": "t3.small"}
    // Missing: region
  }
}
```
**Result:** Fails after 8-12 minutes with region configuration errors

### GCP (Missing project)
```json
{
  "name": "test-gcp-cluster",
  "template": "gcp-standalone-cp-1-0-15",
  "credential": "gcp-credential",
  "config": {
    "region": "us-central1",
    "controlPlane": {"instanceType": "n1-standard-4"},
    "worker": {"instanceType": "n1-standard-4"}
    // Missing: project, network.name
  }
}
```
**Result:** Fails after 10-15 minutes with GCP project errors

**Common pattern:**
- Tool returns success immediately
- ClusterDeployment is created
- 5-15 minutes later: Cloud provisioning fails with cryptic errors

## Proposed Solution

Implement pre-flight configuration validation for all major cloud providers (AWS, Azure, GCP) that:

1. **Validates required fields** before submitting the ClusterDeployment
2. **Provides immediate feedback** if configuration is invalid
3. **Enforces strict validation** (no backward compatibility for invalid configs)
4. **Offers clear error messages** that guide users to fix their config
5. **Uses a unified tool** (`k0rdent.mgmt.clusterDeployments.deploy`) with provider-specific validation

### Validation Rules by Provider

k0rdent uses a **unified ClusterDeployment API** across all providers. Provider detection is based on template name patterns.

#### Azure Templates (Pattern: `azure-*`)

**Required fields:**
- `config.location` (string) - Azure region
- `config.subscriptionID` (string) - Azure subscription ID

**Field naming:**
- Control plane/worker use `vmSize` (not `instanceType`)

#### AWS Templates (Pattern: `aws-*`)

**Required fields:**
- `config.region` (string) - AWS region

**Field naming:**
- Control plane/worker use `instanceType` (not `vmSize`)

#### GCP Templates (Pattern: `gcp-*`)

**Required fields:**
- `config.project` (string) - GCP project name
- `config.region` (string) - GCP region
- `config.network.name` (string) - VPC network name

**Field naming:**
- Control plane/worker use `instanceType` (not `vmSize`)

### Implementation Approach

**Single Phase: Multi-Provider Required Field Validation**
- Add provider detection based on template name pattern
- Implement validation functions for AWS, Azure, and GCP
- Return validation error with helpful message if fields are missing
- Enforce strict validation (no backward compatibility mode)
- Document validation behavior in tool description

**Future Enhancement (Not in this change):**
- Extract validation rules from ClusterTemplate's `spec.schema` (if present)
- Add validation for vSphere, OpenStack, and other providers
- Schema-based dynamic validation

## Success Criteria

1. ✅ Deploying Azure cluster without `subscriptionID` or `location` returns immediate error
2. ✅ Deploying AWS cluster without `region` returns immediate error
3. ✅ Deploying GCP cluster without `project`, `region`, or `network.name` returns immediate error
4. ✅ Error messages clearly indicate missing fields and provide examples
5. ✅ Valid configurations work correctly
6. ✅ Documentation reflects validation behavior for all providers
7. ✅ **Live integration tests validate AWS and Azure** (GCP validation implemented but not live-tested due to environment availability)

## Impact Analysis

**Affected Components:**
- `internal/clusters/deploy.go` - Add validation logic
- `internal/clusters/types.go` - Add validation types/functions
- `docs/cluster-provisioning.md` - Document validation behavior
- `test/integration/clusters_live_test.go` - Add validation test cases

**Breaking Changes:**
- ⚠️ **Breaking change**: Invalid configurations will be rejected immediately at submission time
- **Rationale**: Per user request, no backward compatibility. Invalid configs never successfully provisioned anyway; this provides faster feedback and prevents wasted time/costs.

**Benefits:**
- ✅ Faster failure feedback (seconds vs minutes)
- ✅ Reduced cloud costs from failed provisioning attempts
- ✅ Better user experience with actionable error messages
- ✅ Foundation for comprehensive schema-based validation

## Alternatives Considered

### 1. No Validation (Status Quo)
**Pros:**
- No code changes needed
- Maximum flexibility

**Cons:**
- Poor user experience (late failures)
- Wasted time and resources
- Difficult to debug

**Decision:** ❌ Not acceptable for production use

### 2. Full Schema-Based Validation
**Pros:**
- Comprehensive validation for all providers
- Extensible to new templates
- Leverages existing ClusterTemplate schemas

**Cons:**
- More complex implementation
- Requires schema parsing logic
- May not cover all edge cases

**Decision:** ⏸️ Defer to Phase 2; start with explicit Azure validation

### 3. Provider-Specific Tools (e.g., `k0rdent.provider.aws.clusterDeployments.deploy`)
**Pros:**
- Clear separation of concerns per provider
- Could have provider-specific parameters in MCP schema
- Potentially simpler validation logic per tool

**Cons:**
- **k0rdent uses unified ClusterDeployment API**: Per [official documentation](https://docs.k0rdent.io/latest/admin/clusters/deploy-cluster/), there is one `ClusterDeployment` kind for all providers
- Triple the MCP tools to maintain (AWS, Azure, GCP variants)
- Inconsistent with k0rdent's design philosophy
- Users must know provider before choosing tool
- More complex tool discovery and documentation

**Decision:** ❌ Not aligned with k0rdent architecture; use unified tool with provider-specific validation

### 4. Client-Side Validation Only
**Pros:**
- Could be implemented in Claude Code MCP integration
- No server changes needed

**Cons:**
- Only benefits Claude Code users
- Duplicate validation logic across clients
- Other MCP clients wouldn't benefit

**Decision:** ❌ Server-side validation is required for all clients

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing (invalid) configs | Medium | Low | Document in release notes; invalid configs never worked |
| Performance impact from validation | Low | Low | Validation is cheap (field checks only) |
| False positives blocking valid configs | Low | High | Comprehensive testing with known-good configs |
| Incomplete validation rules | Medium | Medium | Start conservative; expand based on user feedback |

## Timeline

**Immediate (This Proposal):**
- Day 1: Implement multi-provider validation (AWS, Azure, GCP)
- Day 1: Update documentation for all providers
- Day 1: Add unit and integration test cases for all providers
- Day 1: Live test with AWS and Azure (GCP unit-tested only)

**Future (Phase 2):**
- Later: Implement schema-based validation
- Later: Add vSphere, OpenStack validation
- Later: Add configuration suggestions/autocomplete
- Later: Live test GCP validation when environment available

## Dependencies

- Access to k0rdent documentation for validation rules (AWS, Azure, GCP)
- **Live AWS environment** for integration testing
- **Live Azure environment** for integration testing
- GCP validation implemented based on documentation (live testing deferred)
- ClusterTemplate resources with accurate schemas (for Phase 2)

## Open Questions

1. **Should we validate VM sizes against Azure's valid SKU list?**
   - **Proposal**: No for Phase 1 (too many SKUs, region-specific)
   - **Future**: Could add optional SKU validation if Azure API is available

2. **How should we handle custom or local templates?**
   - **Proposal**: Only validate templates matching `azure-*` pattern
   - **Future**: Read validation rules from template's `spec.schema`

3. **Should we validate `clusterIdentity` exists?**
   - **Proposal**: No - existing code already validates credential existence
   - **Future**: Could add cross-resource validation if needed

4. **What about validation for other cloud providers?**
   - **Proposal**: Start with Azure (immediate pain point)
   - **Future**: Add AWS, GCP, vSphere validation in separate changes

## References

- [k0rdent Azure Setup Documentation](https://docs.k0rdent.io/latest/admin/installation/prepare-mgmt-cluster/azure/)
- Current implementation: `internal/clusters/deploy.go`
- Live tests: `test/integration/clusters_live_test.go`
- Documentation: `docs/cluster-provisioning.md`
- Related issue: User deployed test-cluster2 without subscriptionID, failed after 5 minutes

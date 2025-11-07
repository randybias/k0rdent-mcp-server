# Implementation Tasks

## 1. Cluster manager foundation
- [x] 1.1 Create `internal/clusters` package with types for credential/template summaries, deploy requests, and delete requests.
- [x] 1.2 Implement credential & template listing functions that respect namespace filters and handle global namespace inclusion.
- [x] 1.3 Implement deploy helper that builds `ClusterDeployment` manifests and performs server-side apply with managed labels/field owner.
- [x] 1.4 Implement delete helper that removes `ClusterDeployment` resources using foreground propagation policy.

## 2. Runtime integration
- [x] 2.1 Instantiate the cluster manager during server startup; wire dev-mode detection (AuthMode) and global namespace configuration.
- [x] 2.2 Expose namespace resolution helper(s) on `runtime.Session` for tools to reuse.
- [x] 2.3 Add metrics counters/histograms for cluster tool operations.

## 3. MCP tools
- [x] 3.1 Register `k0.clusters.listCredentials`, `k0.clusters.listTemplates`, `k0.clusters.deploy`, and `k0.clusters.delete` in core tool registration.
- [x] 3.2 Implement tool handlers with input validation, error mapping, and structured logging.
- [x] 3.3 Ensure handlers enforce namespace filters and dev-mode namespace defaults.
- [x] 3.4 Add metrics tracking for all cluster operations (list, deploy, delete).

## 4. Unit testing
- [x] 4.1 Add unit tests for the cluster manager (list, deploy, delete) using fake dynamic client fixtures.
- [x] 4.2 Add tool handler tests verifying happy path, missing resources, forbidden namespace, and idempotent operations.
- [x] 4.3 Add namespace resolution tests covering dev vs production modes with regex filters.
- [x] 4.4 Add delete tests verifying idempotent deletion and proper error handling.

## 5. Live integration testing
- [x] 5.1 Create `test/integration/clusters_live_test.go` with `//go:build live` tag.
- [x] 5.2 Implement test setup: load kubeconfig, create MCP client, verify environment variables.
- [x] 5.3 Implement Phase 1: List credentials via MCP and verify `azure-cluster-credential` exists.
- [x] 5.4 Implement Phase 2: List templates via MCP and verify `azure-standalone-cp-1-0-15` exists.
- [x] 5.5 Implement Phase 3: Deploy test cluster using Azure baseline configuration (westus2, Standard_A4_v2, 1+1 nodes).
- [x] 5.6 Implement Phase 4: Poll ClusterDeployment status until Ready condition is true (10-minute timeout).
- [x] 5.7 Implement Phase 5: Delete test cluster via MCP.
- [x] 5.8 Implement Phase 6: Verify deletion completed (resource no longer exists).
- [x] 5.9 Add deferred cleanup to ensure test resources are deleted on failure.
- [x] 5.10 Add skip logic for missing environment variables with helpful messages.

## 6. Documentation & validation
- [x] 6.1 Document cluster provisioning and deletion workflow in developer/operator docs with examples.
- [x] 6.2 Document live integration test setup and required environment variables.
- [x] 6.3 Update runtime-config spec with new env knobs (CLUSTER_GLOBAL_NAMESPACE, CLUSTER_DEFAULT_NAMESPACE_DEV, CLUSTER_DEPLOY_FIELD_OWNER).
- [x] 6.4 Document Azure baseline configuration used in tests for reproducibility.
- [x] 6.5 Run `openspec validate add-cluster-provisioning-tools --strict` and ensure readiness for review.

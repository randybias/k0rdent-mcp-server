# Design: Use kgst for catalog installs

## Problem Analysis

### Current Implementation
The MCP server's catalog installation flow:
1. User calls `k0rdent.mgmt.serviceTemplates.install_from_catalog(app, template, version)`
2. `internal/catalog/manager.go` fetches manifests from local git repo at `/private/tmp/k0rdent-catalog/apps/<app>/charts/<template>-service-template-<version>/templates/`
3. `internal/tools/core/catalog.go` directly applies ServiceTemplate and HelmRepository YAMLs using dynamic client
4. Manifests are applied with `FieldManager: "k0rdent-mcp-server"` and `Force: true`

### Why This Fails
**Observed failures:**
- **valkey** (0.1.0): Validation webhook rejects deployment because the template has an operator dependency (`valkey-operator` v0.0.59-chart from OCI registry) that must be installed first
- **prometheus** (27.5.1): Validation webhook rejects with truncated error message, likely due to missing verification
- **minio** (14.1.2) and **keda** (2.17.0): Work because they're simple chart wrappers with no operator dependencies

**Root causes:**
1. **No verification**: kgst runs a pre-install Job that validates the chart exists in the repository before creating the ServiceTemplate
2. **No Helm hooks**: kgst's HelmRepository has `helm.sh/hook: pre-install` annotation ensuring it's created before the ServiceTemplate
3. **No dependency handling**: kgst chart dependencies (like valkey-operator) are resolved by Helm, but direct manifest apply skips this
4. **Format validation skipped**: kgst enforces `name:version` format and fails fast on malformed inputs

### Official Installation Method
**All catalog documentation specifies:**
```bash
helm upgrade --install <release> oci://ghcr.io/k0rdent/catalog/charts/kgst \
  --set "chart=<name>:<version>" \
  -n kcm-system
```

**What kgst provides:**
- **Chart:** `/apps/k0rdent-utils/charts/kgst-2.0.0/`
- **Templates:**
  - `helm-repository.yaml` - Creates HelmRepository with pre-install hook
  - `service-template.yaml` - Creates ServiceTemplate with proper name formatting
  - `verify-job.yaml` - Pre-install/pre-upgrade Job that pulls and validates the chart
  - `NOTES.txt` - Post-install messages
- **Values:**
  - `repo.name` - Default: "k0rdent-catalog"
  - `repo.spec.url` - Default: "oci://ghcr.io/k0rdent/catalog/charts"
  - `repo.spec.type` - Default: "oci"
  - `chart` - Required format: "name:version"
  - `namespace` - Where to create resources (defaults to release namespace)
  - `k0rdentApiVersion` - Default: "v1beta1"
  - `skipVerifyJob` - Default: false
  - `prefix` - Optional prefix for ServiceTemplate name

## Solution Design

### Approach: Helm SDK Integration

Replace direct manifest application with Helm chart installation using the official Helm Go SDK.

### Architecture

```
MCP Tool Call
     ↓
catalog.install(app, template, version, namespace)
     ↓
Resolve target namespace(s) [existing logic]
     ↓
For each namespace:
  ↓
  Configure Helm Action (install/upgrade)
  ↓
  Set kgst chart values:
    - chart: "<template>:<version>"
    - repo.name: "k0rdent-catalog"
    - repo.spec.url: "oci://ghcr.io/k0rdent/catalog/charts"
    - repo.spec.type: "oci"
    - namespace: <targetNS>
    - k0rdentApiVersion: "v1beta1"
  ↓
  Execute helm upgrade --install
    ↓
    Helm pulls kgst chart from OCI registry
    ↓
    Helm processes templates with values
    ↓
    Helm runs pre-install hooks:
      - verify-job.yaml validates chart exists
      - helm-repository.yaml creates HelmRepository
    ↓
    Helm creates ServiceTemplate
    ↓
    Helm marks release as deployed
  ↓
  Capture release info and return
```

### Helm SDK Usage

**Package:** `helm.sh/helm/v3/pkg/action`

**Key components:**
1. **Configuration** (`action.Configuration`):
   - Use in-cluster REST client config from existing `session.Clients.RESTConfig`
   - Set namespace for Helm operations
   - Configure debug logging via `slog`

2. **Install/Upgrade** (`action.Upgrade` with `action.Install` fallback):
   - Use `RunWithContext()` for context cancellation support
   - Set `CreateNamespace: false` (namespace should already exist and be validated)
   - Set `Wait: true` to wait for verification job to complete
   - Set `Timeout: 5*time.Minute` for verification job completion
   - Set `Atomic: true` to rollback on failure

3. **Chart Loading** (`chart.Loader`):
   - Load kgst chart from OCI registry: `oci://ghcr.io/k0rdent/catalog/charts/kgst`
   - Version: "2.0.0" (or detect latest via registry API)
   - Use `registry.Client` for OCI chart pulling

**Example code pattern:**
```go
import (
    "helm.sh/helm/v3/pkg/action"
    "helm.sh/helm/v3/pkg/chart/loader"
    "helm.sh/helm/v3/pkg/cli"
)

func (t *catalogInstallTool) installViaHelm(ctx context.Context, targetNS, template, version string) error {
    // Setup Helm action configuration
    actionConfig := new(action.Configuration)
    if err := actionConfig.Init(
        &restClientGetter{config: t.session.Clients.RESTConfig, namespace: targetNS},
        targetNS,
        "secret",
        logger.Printf,
    ); err != nil {
        return err
    }

    // Prepare install/upgrade action
    client := action.NewUpgrade(actionConfig)
    client.Install = true // Create if not exists
    client.Namespace = targetNS
    client.Wait = true
    client.Timeout = 5 * time.Minute
    client.Atomic = true

    // Load kgst chart from OCI registry
    chartPath := "oci://ghcr.io/k0rdent/catalog/charts/kgst"
    chart, err := loader.LoadArchive(/* pull and extract chart */)
    if err != nil {
        return err
    }

    // Set values
    values := map[string]interface{}{
        "chart": fmt.Sprintf("%s:%s", template, version),
        "repo": map[string]interface{}{
            "name": "k0rdent-catalog",
            "spec": map[string]interface{}{
                "url":  "oci://ghcr.io/k0rdent/catalog/charts",
                "type": "oci",
            },
        },
        "namespace":          targetNS,
        "k0rdentApiVersion":  "v1beta1",
        "skipVerifyJob":      false,
    }

    // Execute upgrade/install
    release, err := client.RunWithContext(ctx, template, chart, values)
    if err != nil {
        return fmt.Errorf("helm upgrade failed: %w", err)
    }

    return nil
}
```

### Error Handling

**Verification job failures:**
- kgst verify-job exits with error if chart doesn't exist
- Helm treats pre-install hook failure as release failure
- Parse Helm error to extract verification job logs
- Return user-friendly message: "chart <name>:<version> not found in k0rdent catalog"

**Validation webhook rejections:**
- Should no longer occur because kgst ensures correct order (HelmRepository → verification → ServiceTemplate)
- If they do occur, surface the webhook message directly

**Network failures:**
- OCI registry unreachable: "failed to pull kgst chart from ghcr.io"
- Helm repository unreachable (during verify): "verification failed: cannot reach chart repository"

### Namespace Handling

**Existing logic preserved:**
- DEV_ALLOW_ANY mode: Default to "kcm-system" if no namespace specified
- OIDC_REQUIRED mode: Require explicit namespace or all_namespaces flag
- Namespace filter enforcement: Check session.NSFilter before installation
- Multiple namespace support: If all_namespaces=true, install to all allowed namespaces

**Helm integration:**
- Pass resolved namespace to Helm as both:
  - Release namespace (where Helm stores release secret)
  - `namespace` value (where kgst creates ServiceTemplate and HelmRepository)

### Idempotency

**Helm upgrade behavior:**
- If release doesn't exist: Install
- If release exists with same values: No-op (Helm detects no changes)
- If release exists with different values: Upgrade

**Release naming:**
- Use template name as release name: `template` (e.g., "minio", "valkey")
- Consistent with catalog documentation examples
- One release per template per namespace

### Dependencies

**Required Go modules:**
```
helm.sh/helm/v3 v3.14.0
```

**Required network access:**
- `ghcr.io` - To pull kgst OCI chart
- Already required for catalog chart pulling during verification job

## Alternative Approaches Considered

### Alternative 1: Shell out to helm CLI
**Pros:**
- Simpler implementation (exec.Command)
- No additional Go dependencies
- Direct use of user's helm binary

**Cons:**
- Requires helm binary in container image
- Harder to test (mocking processes)
- Less control over error handling
- No structured access to release info

**Decision:** Rejected in favor of Helm SDK for better integration and testability.

### Alternative 2: Implement kgst templates inline
**Pros:**
- No Helm dependency
- Full control over template rendering

**Cons:**
- Duplicates kgst logic (format validation, verify job, hooks)
- Must maintain parity with upstream kgst changes
- Defeats the purpose of using the official installation method

**Decision:** Rejected because it doesn't solve the core problem (divergence from official workflow).

### Alternative 3: Fix validation issues in direct application
**Pros:**
- Minimal code change
- No new dependencies

**Cons:**
- Requires reimplementing verify job logic
- Requires handling Helm hooks manually
- Requires resolving operator dependencies manually
- Still diverges from official installation method
- High maintenance burden

**Decision:** Rejected because it continues the anti-pattern of bypassing kgst.

## Migration Strategy

### Backward Compatibility
- MCP tool signature unchanged: `install_from_catalog(app, template, version, namespace, all_namespaces)`
- Return structure unchanged: `{applied: [], status: ""}`
- Namespace resolution logic unchanged
- No user action required

### Testing Strategy
1. **Unit tests** - Mock Helm action.Configuration and chart.Loader
2. **Integration tests** - Use real Helm SDK against test Kubernetes cluster
3. **Regression tests** - Verify minio, keda continue to work
4. **Fix validation tests** - Verify valkey, prometheus now work
5. **Error path tests** - Non-existent chart, network failure, verification failure

### Rollout Plan
1. Implement Helm integration in parallel with existing code
2. Add feature flag (env var) to switch between implementations
3. Test new implementation in dev environment
4. Enable for all catalog installs
5. Remove old direct-apply code after validation
6. Update documentation

## Security Considerations

### OCI Registry Trust
- kgst chart pulled from `ghcr.io/k0rdent/catalog/charts` (official k0rdent registry)
- No signature verification in this proposal (future enhancement)
- Verify chart name and version match expected values

### RBAC Requirements
**Existing requirements:**
- Create/update ServiceTemplate resources
- Create/update HelmRepository resources

**New requirements:**
- Create/delete Jobs (for verify-job)
- Create/delete Pods (for verify-job pods)
- Helm requires creating Secrets for release storage (in release namespace)

**Recommendation:** Document required RBAC permissions in deployment guide.

### Secrets Management
- Helm stores release state in Secrets (default storage driver)
- Secrets named `sh.helm.release.v1.<release-name>.v<revision>`
- Created in release namespace (same as ServiceTemplate)
- Contain rendered templates and values (no sensitive data in our case)

## Performance Considerations

### Latency Impact
**Current approach:** ~2-3 seconds (fetch manifest, apply)
**New approach:** ~10-30 seconds (pull chart, run verify job, wait for completion)

**Breakdown:**
- Pull kgst chart from OCI: ~2-5 seconds (cached after first pull)
- Run verify job: ~5-15 seconds (pull helm image, pull target chart)
- Apply resources: ~1-2 seconds

**Mitigation:**
- Chart caching: Helm caches pulled charts in local filesystem
- Parallel namespace installs: Continue existing behavior
- User expectations: Installation should be slow and careful (verification is a feature)

### Resource Usage
**Verify job:**
- Creates one Job per installation
- Job runs alpine/helm:3.14.0 container (~50MB image)
- Job deleted by Helm after completion (hook-delete-policy: before-hook-creation)

**Memory:**
- Helm SDK: ~10-20MB additional memory per operation

## Open Questions

1. **kgst version pinning:** Should we pin to 2.0.0 or detect latest from registry?
   - **Recommendation:** Pin to 2.0.0 and update explicitly when catalog repo updates
2. **Verify job skip:** Should we expose `skipVerifyJob` parameter to MCP tool?
   - **Recommendation:** No, always run verification for reliability
3. **Custom repositories:** Should we support non-catalog repositories via kgst?
   - **Recommendation:** Out of scope, only support k0rdent catalog
4. **Helm release naming:** Use template name or generate unique name per namespace?
   - **Recommendation:** Use template name (simple, matches catalog examples)
5. **Chart caching location:** Where should Helm cache OCI charts?
   - **Recommendation:** Default Helm cache location (`$HOME/.cache/helm/registry`) or container temp dir

## Success Metrics

1. **Reliability:** All catalog templates install successfully (including valkey, prometheus)
2. **Consistency:** Installation behavior matches `helm upgrade --install` CLI exactly
3. **Maintainability:** No custom kgst logic to maintain
4. **Observability:** Clear error messages for verification failures
5. **Performance:** Installation completes within 30 seconds for typical templates

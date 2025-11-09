# Change: Add Running Cluster ServiceTemplate Install

## Why
- Today, once a ServiceTemplate is installed in the k0rdent management cluster there is no MCP workflow to attach that service to an already running ClusterDeployment; operators must hand-edit YAML per ["Adding a Service to a ClusterDeployment"](https://github.com/k0rdent/docs/blob/main/docs/user/services/add-service-to-clusterdeployment.md) which is error-prone and bypasses guardrails.
- The management cluster is the canonical system of record for ServiceTemplates, so any cluster update flow has to validate against the live CRs instead of a cached database (per the user's direction and the ["Understanding ServiceTemplates"](https://github.com/k0rdent/docs/blob/main/docs/user/services/understanding-servicetemplates.md) guide).
- Operators also need feedback that mirrors ["Checking status"](https://github.com/k0rdent/docs/blob/main/docs/user/services/checking-status.md) so they understand whether the new service reconciled, without crafting repeat kubectl/jsonpath commands.

## What Changes
- Add a management-plane tool (`k0rdent.mgmt.clusterDeployments.services.apply`) that takes a ServiceTemplate already present in the management cluster and updates a target ClusterDeployment's `spec.serviceSpec.services` list (create or mutate) using server-side apply.
- Inputs follow the supported `serviceSpec` parameters in ["ServiceTemplate Parameters"](https://github.com/k0rdent/docs/blob/main/docs/user/services/servicetemplate-parameters.md): service name/namespace, Helm values & references, provider overrides, dependsOn, etc.
- The tool validates:
  - The referenced ServiceTemplate exists in the management cluster namespace.
  - The ClusterDeployment is in a namespace allowed by the session filter.
  - Optional dry-run mode so operators can inspect the patch without mutating the cluster.
- Response returns the updated `ClusterDeployment.spec.serviceSpec.services[]` entry plus the current status snapshot from `.status.services[]` so the caller can see the reconcile state defined in the docs.

## Impact
- Requires new API helper for patching ClusterDeployments plus unit + live tests to cover add/update flows and namespace-filter enforcement.
- Exposes a safer, auditable path for attaching existing ServiceTemplates to running clusters while keeping SQLite strictly catalog-only.
- Sets groundwork for future features (bulk updates, detach/delete) by centralizing service mutations behind one tool.

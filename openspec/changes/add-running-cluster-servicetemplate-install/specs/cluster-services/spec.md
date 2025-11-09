## ADDED Requirements
### Requirement: Management tool applies installed ServiceTemplates to running clusters
- The MCP server SHALL expose `k0rdent.mgmt.clusterDeployments.services.apply` to attach or update a ServiceTemplate entry inside a ClusterDeployment's `spec.serviceSpec.services[]` using server-side apply.
- Inputs SHALL include the target cluster (`clusterNamespace`, `clusterName`), the ServiceTemplate reference (`templateNamespace`, `templateName`), and optional overrides for `serviceName`, `serviceNamespace`, `values`, `valuesFrom[]`, `helmOptions`, `dependsOn[]`, `priority`, `provider.config`, and `dryRun`.
- The tool MUST fetch the ServiceTemplate from the management cluster and fail with `NotFound` if it does not exist, ensuring the management cluster remains the canonical system of record.
- The tool MUST create the service entry when absent and merge updates (name match) when it already exists, matching the kubectl patch flow described in [docs/user/services/add-service-to-clusterdeployment.md](https://github.com/k0rdent/docs/blob/main/docs/user/services/add-service-to-clusterdeployment.md).

#### Scenario: Attach ServiceTemplate to an existing cluster
- GIVEN ServiceTemplate `minio-14-1-2` exists in namespace `kcm-system`
- AND ClusterDeployment `tenant42/my-cluster` is running
- WHEN the tool is called with that template and service name `minio`
- THEN the ClusterDeployment's `spec.serviceSpec.services[]` gains/updates an entry referencing `minio-14-1-2` with the provided name & namespace
- AND the call succeeds without requiring users to craft manual kubectl patches.

### Requirement: Input validation follows ServiceTemplate parameter contract
- The tool MUST validate optional fields against [docs/user/services/servicetemplate-parameters.md](https://github.com/k0rdent/docs/blob/main/docs/user/services/servicetemplate-parameters.md):
  - `valuesFrom[]` entries allow only ConfigMap or Secret refs
  - `dependsOn[]` references existing service names in the ClusterDeployment spec
  - Provider overrides are nested under `.spec.serviceSpec.provider.*`
- Namespace access MUST honor the session's namespace filter for both the ClusterDeployment and the ServiceTemplate namespaces.
- A `dryRun` flag MUST execute the full validation + merge logic but skip the server-side apply, returning the would-be payload so operators can review changes safely.

#### Scenario: Invalid dependsOn rejected
- WHEN the tool is invoked with `dependsOn: ["non-existent"]`
- THEN it returns a validation error explaining the dependency is unknown and does not mutate the ClusterDeployment.

#### Scenario: Dry-run preview
- WHEN `dryRun=true`
- THEN the response echoes the merged service spec (values, dependsOn, provider data) but the ClusterDeployment object in Kubernetes is unchanged.

### Requirement: Response surfaces service spec + reconcile status
- The tool response MUST include:
  - The service spec payload that was applied (or would be applied during dry-run)
  - The matching entry from `.status.services[]` (state, version, failureMessage, lastTransitionTime) and any `.status.servicesUpgradePaths[]` info when present, per [docs/user/services/checking-status.md](https://github.com/k0rdent/docs/blob/main/docs/user/services/checking-status.md)
- After a successful mutation, the tool MUST fetch the latest ClusterDeployment to populate the status block so operators can see whether the service is `Pending`, `Provisioning`, or `Deployed`.

#### Scenario: Status snapshot after apply
- GIVEN ingress-nginx service was just attached to `tenant42/my-cluster`
- WHEN the controller reports `.status.services[].state="Provisioning"`
- THEN the tool returns the applied spec plus that status snapshot so callers can continue polling or decide next actions.

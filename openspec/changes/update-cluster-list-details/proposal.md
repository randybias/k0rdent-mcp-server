# Change: Update cluster listing outputs with full deployment context

## Why
- `k0rdent.mgmt.clusterDeployments.list` currently returns only name/namespace/labels/ready/createdAt; operators need template, credential, provider, region, current phase, and management URLs to triage issues.
- Without these fields, users must make multiple follow-up calls (dynamic client, labels) to learn what template version a deployment uses or which credential it references.
- Live troubleshooting (and the failing tests mentioned) expect richer data so agents can compare catalog vs installed versions and detect drift.

## What Changes
- Expand the cluster list tool (and supporting API summaries) to surface template name/version, credential reference, provider/cloud metadata, and current status (phase/reason/message) derived from ClusterDeployment conditions.
- Include management-plane helper fields such as `age`, `owner`, and optional `kubeconfigSecret`/`clusterIdentity` references when available.
- Ensure the data is exposed consistently across MCP tool responses, docs, and integration tests so downstream agents can rely on the richer schema.

## Impact
- Operators get a single call to understand the full deployment context, aligning MCP output with expectations from the k0rdent UI/docs.
- Future automation (e.g., drift detection, upgrade planning) can run purely via MCP without custom dynamic queries.

# Release Notes

## Tool namespace rename

- All MCP tool and resource identifiers now use the `k0rdent.` prefix (for example, `k0rdent.mgmt.namespaces.list`, `k0rdent.catalog.serviceTemplates.list`, and the `k0rdent://podlogs/...` resource URI).
- The legacy prefix has been removed entirely; update any MCP clients or scripts to use the new identifiers.

## Cluster deployment summaries

- `k0rdent.mgmt.clusterDeployments.list` (and `listAll`) now return full context: template/credential references, provider + region, cluster identity, kubeconfig secret, phase/message, and the full condition set.
- This removes the need for ad-hoc kubectl queries when troubleshooting cluster provisioning—one MCP call shows everything you need to know.

# Release Notes

## Tool namespace rename

- All MCP tool and resource identifiers now use the `k0rdent.` prefix (for example, `k0rdent.namespaces.list`, `k0rdent.catalog.list`, and the `k0rdent://podlogs/...` resource URI).
- The legacy prefix has been removed entirely; update any MCP clients or scripts to use the new identifiers.

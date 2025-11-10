# Proposal: Support Optional Cluster Connection at Startup

## Problem

Currently, the MCP server requires a valid, reachable k0rdent management cluster at startup:

1. **Hard requirement**: `K0RDENT_MGMT_KUBECONFIG_PATH` must point to a valid kubeconfig
2. **Startup blocks**: Server fails to start if cluster connection fails (even with recent timeout improvements)
3. **No cluster switching**: Once connected to a cluster, there's no way to switch to a different management cluster
4. **Agent limitations**: AI agents cannot:
   - Start server without a cluster connection
   - Connect to a cluster after server is running
   - Switch between multiple k0rdent management clusters
   - Explore available clusters before connecting

This prevents useful workflows like:
- Starting the server to explore what clusters are available
- Dynamically connecting to different management clusters based on task
- Running server in environments where cluster availability varies
- Agent-driven cluster selection and management

## Proposed Solution

Make cluster connection optional at startup and add dynamic cluster connection capabilities:

### Phase 1: Optional Startup Connection
- Allow server to start without `K0RDENT_MGMT_KUBECONFIG_PATH`
- Print banner showing "No cluster configured" state
- Tools return appropriate errors/empty results when no cluster is connected
- Health endpoint indicates cluster connection status

### Phase 2: Dynamic Cluster Connection
- Add MCP tools for cluster management:
  - `k0rdent_cluster_connect`: Connect to a cluster with kubeconfig
  - `k0rdent_cluster_disconnect`: Disconnect from current cluster
  - `k0rdent_cluster_status`: Show current connection status
  - `k0rdent_cluster_list_contexts`: List available contexts from kubeconfig
- Connection changes apply to current MCP session
- Existing subscriptions (events, logs, monitors) are cancelled on disconnect

### Phase 3: Multi-Cluster Sessions (Future)
- Per-session cluster context (different MCP clients can connect to different clusters)
- Session-scoped kubeconfig and connection state
- Clean separation between sessions

## Benefits

1. **Better developer experience**: Server starts even when cluster is down
2. **Agent flexibility**: AI agents can manage cluster connections dynamically
3. **Multi-cluster workflows**: Switch between dev/staging/prod clusters
4. **Gradual connection**: Connect when needed, not at startup
5. **Better error handling**: Tools fail gracefully when not connected

## Implementation Notes

### Startup Changes
- Make `K0RDENT_MGMT_KUBECONFIG_PATH` optional in config loader
- Skip cluster ping when no kubeconfig provided
- Return minimal settings with no RestConfig
- Update banner to show "not connected" state

### Runtime State
- Add connection manager to track current cluster connection
- Store RestConfig, context name, and connection metadata
- Provide connection state to tools via runtime session

### Tool Behavior
- Tools check for connection before executing
- Return structured error if not connected: `{"error": "not_connected", "message": "No cluster connection. Use k0rdent_cluster_connect first."}`
- Empty results for list operations when not connected

### Session Management
- Connection state stored in MCP session context
- Tools can check connection status via session
- Subscriptions require active connection

## Migration Path

1. **Backward compatible**: Existing deployments continue to work
2. **Environment variable still works**: `K0RDENT_MGMT_KUBECONFIG_PATH` at startup connects immediately
3. **New capability**: Dynamic connection via tools is additive
4. **No breaking changes**: All existing tools work the same when connected

## Open Questions

1. **Security**: Should dynamic kubeconfig loading be restricted in certain auth modes?
2. **Credentials**: How to handle kubeconfig with credentials in OIDC mode?
3. **Persistence**: Should connection state persist across server restarts?
4. **Concurrent requests**: How to handle requests during connection state transitions?

## Related Work

- Recent startup improvements (banner, timeout) already separate config loading from connection
- `secure-auth-mode-default` proposal may need updates for OIDC + dynamic connection
- Existing session/runtime infrastructure can be extended for connection management

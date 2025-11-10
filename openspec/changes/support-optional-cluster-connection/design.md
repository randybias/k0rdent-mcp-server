# Design: Optional Cluster Connection

## Architecture Overview

This design extends the MCP server to support starting without a cluster connection and dynamically managing cluster connections at runtime.

### Current Architecture

```
Startup:
  config.Load() → [parse kubeconfig] → [ping cluster] → RestConfig
  initializeServerWithSettings() → ClientFactory → Runtime
  Tools → Runtime → ClientFactory → K8s API
```

**Problem**: RestConfig is required at startup; failure blocks server initialization.

### Proposed Architecture

```
Startup (no cluster required):
  config.Load() → [parse optional kubeconfig] → Settings (RestConfig may be nil)
  initializeServerWithSettings() → ConnectionManager (initially disconnected)
  HTTP Server starts → MCP tools available

Dynamic Connection:
  k0rdent_cluster_connect tool → ConnectionManager.Connect()
    → [parse kubeconfig] → [ping cluster] → [update session state]

Tool Execution:
  Tool → Session → ConnectionManager.GetConnection()
    → if connected: ClientFactory → K8s API
    → if not connected: return error
```

## Key Components

### 1. ConnectionManager

New component to manage cluster connection state:

```go
type ConnectionManager struct {
    mu          sync.RWMutex
    connection  *ClusterConnection  // nil when not connected
    logger      *slog.Logger
}

type ClusterConnection struct {
    RestConfig      *rest.Config
    ClientFactory   *kube.ClientFactory
    ContextName     string
    ConnectedAt     time.Time
    Source          string  // "startup" | "dynamic"
}

Methods:
- Connect(ctx, kubeconfig, context) error
- Disconnect() error
- GetConnection() (*ClusterConnection, error)
- IsConnected() bool
- GetStatus() ConnectionStatus
```

**Responsibilities**:
- Maintain current cluster connection
- Thread-safe connection state management
- Validate connection before allowing Connect()
- Clean up watchers/subscriptions on Disconnect()

### 2. Config Loading Changes

Update `internal/config/config.go`:

```go
// Current: K0RDENT_MGMT_KUBECONFIG_PATH is required
// Proposed: K0RDENT_MGMT_KUBECONFIG_PATH is optional

func (l *Loader) Load(ctx context.Context) (*Settings, error) {
    // ... existing auth mode, logging, namespace filter ...

    kubeconfigPath, hasPath := l.envLookup(envKubeconfigPath)
    if !hasPath || kubeconfigPath == "" {
        // Return settings with nil RestConfig
        return &Settings{
            RestConfig: nil,  // No cluster connection
            // ... other settings ...
        }, nil
    }

    // Existing kubeconfig loading and ping logic
    // ...
}
```

**Changes**:
- Don't require `K0RDENT_MGMT_KUBECONFIG_PATH`
- Return valid Settings even without kubeconfig
- RestConfig = nil indicates no connection
- Banner shows "Not connected" state

### 3. Session Context Extension

Extend MCP session to include connection manager:

```go
// In internal/runtime/session.go or new internal/connection/manager.go

type Session struct {
    // ... existing fields ...
    connectionMgr *ConnectionManager
}

func (s *Session) GetConnection() (*ClusterConnection, error) {
    return s.connectionMgr.GetConnection()
}
```

### 4. New MCP Tools

Add cluster management tools in `internal/tools/core/cluster_connection.go`:

#### `k0rdent_cluster_connect`
```go
Input: {
  "kubeconfig": "base64-encoded kubeconfig content",
  "context": "optional context name"
}
Output: {
  "connected": true,
  "context": "k0rdent-prod",
  "server": "https://k8s.example.com:6443"
}
```

#### `k0rdent_cluster_disconnect`
```go
Input: {}
Output: {
  "disconnected": true,
  "message": "Disconnected from k0rdent-prod"
}
```

#### `k0rdent_cluster_status`
```go
Output: {
  "connected": true | false,
  "context": "k0rdent-prod" | null,
  "server": "https://..." | null,
  "connected_at": "2025-11-10T12:00:00Z" | null,
  "source": "startup" | "dynamic" | null
}
```

#### `k0rdent_cluster_list_contexts`
```go
Input: {
  "kubeconfig": "base64-encoded kubeconfig content"
}
Output: {
  "contexts": [
    {"name": "k0rdent-dev", "cluster": "dev-cluster"},
    {"name": "k0rdent-prod", "cluster": "prod-cluster"}
  ],
  "current": "k0rdent-dev"
}
```

### 5. Tool Behavior Updates

All existing tools must check connection status:

```go
func (t *SomeTool) Execute(ctx context.Context, params map[string]any) (any, error) {
    conn, err := t.session.GetConnection()
    if err != nil {
        return nil, &NotConnectedError{
            Message: "No cluster connection. Use k0rdent_cluster_connect first.",
        }
    }

    // Use conn.ClientFactory for K8s operations
    // ...
}
```

**Error response format**:
```json
{
  "error": "not_connected",
  "message": "No cluster connection. Use k0rdent_cluster_connect first.",
  "suggestion": "Call k0rdent_cluster_connect with a valid kubeconfig"
}
```

### 6. Subscription Lifecycle

Subscriptions (events, logs, monitors) depend on cluster connection:

**On Disconnect**:
1. Cancel all active subscriptions
2. Send final message to subscribers: `{"status": "disconnected", "reason": "cluster connection closed"}`
3. Clean up watchers and channels

**On Connect**:
1. Subscriptions do NOT automatically restart
2. Agents must re-subscribe after connecting

## Data Flow

### Startup Without Cluster

```
1. Server starts
2. config.Load() returns Settings with RestConfig=nil
3. ConnectionManager initialized (disconnected state)
4. Banner shows "Not connected"
5. HTTP/MCP server starts
6. Tools return not_connected errors
```

### Startup With Cluster (Existing Behavior)

```
1. Server starts
2. config.Load() reads K0RDENT_MGMT_KUBECONFIG_PATH
3. Creates RestConfig, pings cluster
4. ConnectionManager initialized with connection
5. Banner shows connection details
6. HTTP/MCP server starts
7. Tools work normally
```

### Dynamic Connection

```
1. Agent calls k0rdent_cluster_connect
2. ConnectionManager.Connect():
   a. Parse kubeconfig
   b. Create RestConfig
   c. Ping cluster (10s timeout)
   d. Create ClientFactory
   e. Update connection state
3. Return success
4. Subsequent tool calls use new connection
```

### Dynamic Disconnection

```
1. Agent calls k0rdent_cluster_disconnect
2. ConnectionManager.Disconnect():
   a. Cancel all subscriptions
   b. Close watchers
   c. Set connection = nil
3. Return success
4. Subsequent tool calls return not_connected
```

## Thread Safety

**ConnectionManager** uses `sync.RWMutex`:
- `Connect()` and `Disconnect()` take write lock
- `GetConnection()` and `IsConnected()` take read lock
- Tool execution gets read lock (many concurrent reads allowed)
- Connection changes block tool execution temporarily

## Security Considerations

### DEV_ALLOW_ANY Mode
- Dynamic kubeconfig loading is allowed
- No additional restrictions
- Suitable for development only

### OIDC_REQUIRED Mode
- **Option A (Restrictive)**: Disable dynamic kubeconfig loading
  - Only allow startup connection via environment
  - k0rdent_cluster_connect tool returns permission error
  - Prevents credential confusion

- **Option B (Flexible)**: Allow but validate
  - Dynamic kubeconfig can provide cluster endpoint/CA only
  - Credentials must come from OIDC bearer token
  - Requires kubeconfig validation logic

**Recommendation**: Start with Option A for OIDC mode, add Option B later if needed.

## Testing Strategy

### Unit Tests
- ConnectionManager: connect, disconnect, concurrent access
- Config loading with/without kubeconfig
- Tool error handling when not connected

### Integration Tests
- Start server without cluster
- Connect dynamically
- Execute tools after connection
- Disconnect and verify errors
- Reconnect to different cluster

### Manual Testing
- Agent workflow: start → connect → work → disconnect → reconnect
- Multiple contexts in kubeconfig
- Connection failure handling
- Subscription cleanup on disconnect

## Migration & Backward Compatibility

### Existing Deployments
- No changes required
- `K0RDENT_MGMT_KUBECONFIG_PATH` continues to work
- Startup connects immediately as before
- All existing tools work identically

### New Deployments
- Can omit `K0RDENT_MGMT_KUBECONFIG_PATH`
- Use dynamic connection tools
- More flexible cluster management

### Deprecation
- No deprecations in this change
- Existing behavior is preserved
- New capability is additive

## Performance Impact

### Startup
- Slightly faster when no cluster configured (no ping)
- Same performance when cluster configured

### Runtime
- Minimal overhead: one read lock per tool execution
- Connection state cached in manager
- No impact on K8s API calls

### Connection Changes
- Connect/Disconnect are rare operations
- Brief blocking of tool execution during state change (write lock)
- Acceptable for agent-driven workflows

## Future Enhancements

### Per-Session Connections
- Different MCP clients connect to different clusters
- Session-scoped ConnectionManager
- Requires session ID tracking and isolation

### Connection Pooling
- Maintain connections to multiple clusters
- Switch between them without reconnecting
- More complex state management

### Auto-Reconnect
- Detect connection loss
- Attempt reconnection with backoff
- Notify subscribers of status changes

### Kubeconfig Discovery
- List available kubeconfig files
- Merge multiple kubeconfigs
- Integration with credential managers

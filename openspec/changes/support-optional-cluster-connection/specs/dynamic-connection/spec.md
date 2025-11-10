# Spec: Dynamic Cluster Connection Management

## Capability

Enable dynamic connection to k0rdent management clusters after server startup.

## ADDED Requirements

### Requirement: ConnectionManager Thread Safety

ConnectionManager **SHALL** provide thread-safe access to cluster connection state.

#### Scenario: Concurrent tool execution during connection

Given the ConnectionManager is in the process of connecting to a cluster
When multiple tools call GetConnection() concurrently
Then all GetConnection() calls MUST block until connection completes
And after connection completes, all calls MUST receive the new connection
And no race conditions MUST occur

#### Scenario: Multiple readers with no writers

Given the ConnectionManager has an active connection
When 100 tools call GetConnection() concurrently
Then all calls MUST complete without blocking each other
And all calls MUST receive the same connection instance
And performance MUST not degrade

### Requirement: Connection Validation

Connect operation **SHALL** validate cluster before accepting connection.

#### Scenario: Connect with valid cluster

Given the server is not connected
When ConnectionManager.Connect() is called with valid kubeconfig
Then the connection MUST perform a discovery ping within 10 seconds
And if the ping succeeds, the connection MUST be established
And subsequent tool calls MUST use the new connection

#### Scenario: Connect with unreachable cluster

Given the server is not connected
When ConnectionManager.Connect() is called with kubeconfig to unreachable cluster
Then the connection attempt MUST timeout after 10 seconds
And the connection MUST NOT be established
And the server MUST remain in disconnected state
And an error MUST be returned with timeout details

### Requirement: Subscription Cleanup

Disconnect **SHALL** cancel all active subscriptions before completing.

#### Scenario: Disconnect with active subscriptions

Given the server is connected to a cluster
And there are 3 active event subscriptions
And there is 1 active pod log subscription
When ConnectionManager.Disconnect() is called
Then all subscription watchers MUST be cancelled
And each subscription MUST receive a final message:
  ```json
  {"status": "disconnected", "reason": "cluster connection closed"}
  ```
And all channels MUST be closed
And the operation MUST complete within 5 seconds

### Requirement: Idempotent Disconnect

Disconnect **SHALL** be callable multiple times safely.

#### Scenario: Disconnect when already disconnected

Given the server is not connected to any cluster
When ConnectionManager.Disconnect() is called
Then the operation MUST return success (nil error)
And no cleanup operations MUST be performed
And logs MUST indicate already disconnected state

### Requirement: Single Connection Enforcement

Only one cluster connection **SHALL** be active at a time.

#### Scenario: Connect when already connected

Given the server is connected to cluster A
When ConnectionManager.Connect() is called for cluster B
Then the operation MUST return an "already_connected" error
And the existing connection to cluster A MUST remain active
And the error MUST include current connection details

### Requirement: Connection State Access

Tools **SHALL** access connection state via session.

#### Scenario: Tool gets connection when connected

Given the server is connected to a cluster
When a tool calls session.GetConnection()
Then the tool MUST receive the active ClusterConnection
And the connection MUST include RestConfig, ClientFactory, and metadata

#### Scenario: Tool gets connection when not connected

Given the server is not connected
When a tool calls session.GetConnection()
Then the tool MUST receive a NotConnectedError
And the error MUST suggest using k0rdent_cluster_connect

## Data Structures

### ConnectionManager

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
```

### Methods

```go
// Connect establishes connection to cluster
// Returns error if already connected or cluster unreachable
func (cm *ConnectionManager) Connect(ctx context.Context, kubeconfig []byte, contextName string) error

// Disconnect closes current connection and cancels subscriptions
// Idempotent - returns nil if already disconnected
func (cm *ConnectionManager) Disconnect() error

// GetConnection returns current connection or error if not connected
// Thread-safe for concurrent tool execution
func (cm *ConnectionManager) GetConnection() (*ClusterConnection, error)

// IsConnected returns true if currently connected
func (cm *ConnectionManager) IsConnected() bool

// GetStatus returns current connection status
func (cm *ConnectionManager) GetStatus() ConnectionStatus
```

## Thread Safety

### Read Lock (Multiple Concurrent Readers)

- `GetConnection()` - tool execution
- `IsConnected()` - status checks
- `GetStatus()` - status queries

### Write Lock (Exclusive)

- `Connect()` - blocks all tools during connection
- `Disconnect()` - blocks all tools during disconnection

### Lock Ordering

1. Connection state changes are rare (agent-driven)
2. Tool execution is frequent
3. RWMutex allows many concurrent tool executions
4. Brief blocking during connection changes is acceptable

## Subscription Lifecycle

### On Disconnect

1. Cancel context for all subscription watchers
2. Send final message to subscribers:
   ```json
   {
     "status": "disconnected",
     "reason": "cluster connection closed"
   }
   ```
3. Close channels
4. Clean up Kubernetes watch resources
5. Log subscription cleanup

### On Connect

- Subscriptions do NOT automatically restart
- Agents must call subscribe tools again after connecting
- New subscriptions use new connection

## Error Handling

### Connect Errors

```json
{
  "error": "connect_failed",
  "message": "Failed to connect to cluster: <reason>",
  "details": {
    "context": "k0rdent-prod",
    "reason": "connection_timeout | invalid_kubeconfig | auth_failed"
  }
}
```

### Already Connected Error

```json
{
  "error": "already_connected",
  "message": "Already connected to k0rdent-prod. Disconnect first.",
  "current_connection": {
    "context": "k0rdent-prod",
    "connected_at": "2025-11-10T12:00:00Z",
    "source": "dynamic"
  }
}
```

### Not Connected Error

```json
{
  "error": "not_connected",
  "message": "No cluster connection. Use k0rdent_cluster_connect first."
}
```

## Testing

### Unit Tests

- ConnectionManager: concurrent Connect/Disconnect/GetConnection
- Thread safety: multiple readers during write lock
- Subscription cleanup on disconnect
- Idempotent disconnect

### Integration Tests

- Connect → execute tools → disconnect → verify errors
- Connect → disconnect → reconnect to different cluster
- Connect → start subscription → disconnect → verify cleanup
- Multiple concurrent tool executions during connection

### Stress Tests

- 100 concurrent GetConnection calls during Connect
- Rapid connect/disconnect cycles
- Many subscriptions active during disconnect

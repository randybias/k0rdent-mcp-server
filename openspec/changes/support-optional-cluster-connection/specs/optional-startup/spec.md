# Spec: Optional Cluster Connection at Startup

## Capability

Allow the MCP server to start without a connected k0rdent management cluster.

## ADDED Requirements

### Requirement: Server Startup Without Cluster

Server **SHALL** start successfully without a cluster connection.

#### Scenario: Start without kubeconfig

Given the `K0RDENT_MGMT_KUBECONFIG_PATH` environment variable is not set
When the server starts
Then the server MUST start successfully within 1 second
And the startup banner MUST display "Not connected"
And the RestConfig in Settings MUST be nil

#### Scenario: Start with unreachable cluster

Given `K0RDENT_MGMT_KUBECONFIG_PATH` points to a valid kubeconfig
And the cluster endpoint is unreachable
When the server starts
Then the server MUST start successfully within 11 seconds
And the startup banner MUST display connection failure status
And the Settings object MUST be returned with RestConfig populated

### Requirement: Tool Error Handling

All cluster-dependent tools **SHALL** return structured errors when not connected.

#### Scenario: Execute tool without connection

Given the server is running without a cluster connection
When an agent calls any cluster-dependent tool (e.g., k0rdent_mgmt_clusterDeployments_list)
Then the tool MUST return a structured error
And the error MUST include:
  - `error`: "not_connected"
  - `message`: "No cluster connection. Use k0rdent_cluster_connect first."
  - `suggestion`: "Call k0rdent_cluster_connect with a valid kubeconfig"

### Requirement: Startup Banner Display

Startup banner **SHALL** clearly show connection status.

#### Scenario: Banner when not connected

Given no kubeconfig is configured
When the server prints the startup banner
Then the banner MUST include "Cluster Status: Not connected"
And the banner MUST NOT include kubeconfig context or source fields

#### Scenario: Banner when connection failed

Given kubeconfig is configured but cluster is unreachable
When the server prints the startup banner
Then the banner MUST include "Connection Status: Failed - cluster unreachable"
And the banner MUST include the kubeconfig context name
And the banner MUST include the kubeconfig source

### Requirement: Health Endpoint Extension

Health endpoint **SHALL** report cluster connection status.

#### Scenario: Health check without connection

Given the server is running without a cluster connection
When the health endpoint is queried
Then the response MUST include `"connected": false`
And the response MUST include `"context": null`

#### Scenario: Health check with connection

Given the server is connected to a cluster
When the health endpoint is queried
Then the response MUST include `"connected": true`
And the response MUST include the context name

### Requirement: Backward Compatibility

Existing startup behavior **SHALL** be preserved when kubeconfig is provided.

#### Scenario: Traditional startup with kubeconfig

Given `K0RDENT_MGMT_KUBECONFIG_PATH` points to a reachable cluster
When the server starts
Then the server MUST connect to the cluster immediately
And all tools MUST work as before
And the startup banner MUST show connection details
And existing deployments MUST work without changes

## Error Handling

### Not Connected Error Format

```json
{
  "error": "not_connected",
  "message": "No cluster connection. Use k0rdent_cluster_connect first.",
  "suggestion": "Call k0rdent_cluster_connect with a valid kubeconfig"
}
```

### Startup Banner - Not Connected

```
========================================
K0rdent MCP Server Startup Summary
  Listen Address:       127.0.0.1:6767
  Auth Mode:            DEV_ALLOW_ANY
  Cluster Status:       Not connected
  Namespace Filter:     <none>
  Log Level:            INFO
  External Sink:        false
  PID File:             k0rdent-mcp.pid
========================================
```

### Startup Banner - Connection Failed

```
========================================
K0rdent MCP Server Startup Summary
  Listen Address:       127.0.0.1:6767
  Auth Mode:            DEV_ALLOW_ANY
  Kubeconfig Source:    path
  Kubeconfig Context:   k0rdent-prod
  Connection Status:    Failed - cluster unreachable
  Namespace Filter:     <none>
  Log Level:            INFO
  External Sink:        false
  PID File:             k0rdent-mcp.pid
========================================
```

## Implementation Notes

### Config Loading Changes

- `internal/config/config.go::Load()` must not return error when kubeconfig missing
- Return `Settings` with `RestConfig = nil` when no kubeconfig
- Banner display logic must handle nil RestConfig

### Tool Behavior

All existing tools must check connection before executing:

```go
func (t *Tool) Execute(ctx context.Context, params map[string]any) (any, error) {
    conn, err := t.session.GetConnection()
    if err != nil {
        return nil, &NotConnectedError{
            Message:    "No cluster connection. Use k0rdent_cluster_connect first.",
            Suggestion: "Call k0rdent_cluster_connect with a valid kubeconfig",
        }
    }
    // ... use conn.ClientFactory for K8s operations
}
```

## Testing

### Unit Tests

- Config loading without kubeconfig
- Config loading with unreachable cluster
- Tool execution when not connected
- Banner display with various connection states

### Integration Tests

- Start server without kubeconfig
- Verify tools return not_connected errors
- Verify health endpoint shows disconnected state
- Start server with kubeconfig and verify connection

### Backward Compatibility Tests

- Existing deployments with K0RDENT_MGMT_KUBECONFIG_PATH continue to work
- Startup connects immediately when kubeconfig is valid
- All tools work normally when connected

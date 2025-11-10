# Spec: Cluster Management Tools

## Capability

Provide MCP tools for agents to manage cluster connections dynamically.

## ADDED Requirements

### Requirement: k0rdent_cluster_connect Tool

The k0rdent_cluster_connect tool **SHALL** connect to a k0rdent management cluster using kubeconfig.

#### Input Schema

```json
{
  "type": "object",
  "properties": {
    "kubeconfig": {
      "type": "string",
      "description": "Base64-encoded kubeconfig content"
    },
    "context": {
      "type": "string",
      "description": "Optional context name from kubeconfig. Uses current-context if not specified."
    }
  },
  "required": ["kubeconfig"]
}
```

#### Success Response

```json
{
  "connected": true,
  "context": "k0rdent-prod",
  "server": "https://k8s.example.com:6443",
  "connected_at": "2025-11-10T12:00:00Z"
}
```

#### Error Responses

**Already Connected**:
```json
{
  "error": "already_connected",
  "message": "Already connected to k0rdent-dev. Disconnect first.",
  "current_connection": {
    "context": "k0rdent-dev",
    "server": "https://dev.k8s.example.com:6443",
    "connected_at": "2025-11-10T11:00:00Z"
  }
}
```

**Invalid Kubeconfig**:
```json
{
  "error": "invalid_kubeconfig",
  "message": "Failed to parse kubeconfig: <details>"
}
```

**Connection Failed**:
```json
{
  "error": "connection_failed",
  "message": "Failed to connect to cluster: connection timeout",
  "details": {
    "context": "k0rdent-prod",
    "server": "https://k8s.example.com:6443",
    "reason": "context deadline exceeded"
  }
}
```

#### Scenario: Connect with valid kubeconfig

Given the server is not connected to any cluster
And the agent has a valid base64-encoded kubeconfig
When the agent calls k0rdent_cluster_connect with the kubeconfig
Then the tool MUST decode the base64 kubeconfig
And the tool MUST parse the kubeconfig
And the tool MUST resolve the context (explicit or current-context)
And the tool MUST create a RestConfig
And the tool MUST perform a discovery ping within 10 seconds
And if the ping succeeds, the connection MUST be established
And the response MUST include `"connected": true`
And the response MUST include context name and server URL

#### Scenario: Connect when already connected

Given the server is connected to cluster A
When the agent calls k0rdent_cluster_connect for cluster B
Then the tool MUST return an error with type "already_connected"
And the error MUST include current connection details
And the existing connection MUST remain active

#### Scenario: Connect with invalid kubeconfig

Given the server is not connected
When the agent calls k0rdent_cluster_connect with invalid base64 data
Then the tool MUST return an error with type "invalid_kubeconfig"
And the connection MUST NOT be established

#### Scenario: Connect to unreachable cluster

Given the server is not connected
And the kubeconfig points to an unreachable cluster
When the agent calls k0rdent_cluster_connect
Then the tool MUST timeout after 10 seconds
And the tool MUST return an error with type "connection_failed"
And the error MUST include timeout details

### Requirement: k0rdent_cluster_disconnect Tool

The k0rdent_cluster_disconnect tool **SHALL** disconnect from current cluster and cancel all subscriptions.

#### Input Schema

```json
{
  "type": "object",
  "properties": {},
  "required": []
}
```

#### Success Response

```json
{
  "disconnected": true,
  "message": "Disconnected from k0rdent-prod",
  "previous_connection": {
    "context": "k0rdent-prod",
    "server": "https://k8s.example.com:6443",
    "connected_at": "2025-11-10T12:00:00Z",
    "duration": "15m30s"
  }
}
```

#### Idempotent Response (Already Disconnected)

```json
{
  "disconnected": true,
  "message": "Already disconnected"
}
```

#### Scenario: Disconnect when connected

Given the server is connected to a cluster
And there are active subscriptions
When the agent calls k0rdent_cluster_disconnect
Then all active subscriptions MUST be cancelled
And each subscription MUST receive a final disconnect message
And the connection MUST be closed
And the response MUST include `"disconnected": true`
And the response MUST include previous connection details
And the operation MUST complete within 5 seconds

#### Scenario: Disconnect when not connected

Given the server is not connected to any cluster
When the agent calls k0rdent_cluster_disconnect
Then the tool MUST return success (idempotent behavior)
And the response MUST include `"message": "Already disconnected"`

### Requirement: k0rdent_cluster_status Tool

The k0rdent_cluster_status tool **SHALL** show current cluster connection status.

#### Input Schema

```json
{
  "type": "object",
  "properties": {},
  "required": []
}
```

#### Response (Connected)

```json
{
  "connected": true,
  "context": "k0rdent-prod",
  "server": "https://k8s.example.com:6443",
  "connected_at": "2025-11-10T12:00:00Z",
  "source": "dynamic",
  "duration": "5m30s",
  "active_subscriptions": {
    "events": 2,
    "podlogs": 1,
    "cluster-monitor": 1
  }
}
```

#### Response (Not Connected)

```json
{
  "connected": false,
  "context": null,
  "server": null,
  "connected_at": null,
  "source": null
}
```

#### Scenario: Status when connected

Given the server is connected to a cluster
When the agent calls k0rdent_cluster_status
Then the response MUST include `"connected": true`
And the response MUST include context name
And the response MUST include server URL
And the response MUST include connected_at timestamp
And the response MUST include connection source ("startup" or "dynamic")
And the response MUST include connection duration
And the response MUST include active subscription counts
And the tool MUST NOT make any Kubernetes API calls
And the tool MUST complete in under 100ms

#### Scenario: Status when not connected

Given the server is not connected to any cluster
When the agent calls k0rdent_cluster_status
Then the response MUST include `"connected": false`
And the response MUST include null values for connection fields
And the tool MUST complete in under 100ms

### Requirement: k0rdent_cluster_list_contexts Tool

The k0rdent_cluster_list_contexts tool **SHALL** list available contexts from a kubeconfig without connecting.

#### Input Schema

```json
{
  "type": "object",
  "properties": {
    "kubeconfig": {
      "type": "string",
      "description": "Base64-encoded kubeconfig content"
    }
  },
  "required": ["kubeconfig"]
}
```

#### Success Response

```json
{
  "contexts": [
    {
      "name": "k0rdent-dev",
      "cluster": "dev-cluster",
      "namespace": "default",
      "user": "dev-admin"
    },
    {
      "name": "k0rdent-prod",
      "cluster": "prod-cluster",
      "namespace": "kcm-system",
      "user": "prod-admin"
    }
  ],
  "current": "k0rdent-dev"
}
```

#### Error Response

```json
{
  "error": "invalid_kubeconfig",
  "message": "Failed to parse kubeconfig: <details>"
}
```

#### Scenario: List contexts from valid kubeconfig

Given the agent has a valid base64-encoded kubeconfig with multiple contexts
When the agent calls k0rdent_cluster_list_contexts
Then the tool MUST decode the base64 kubeconfig
And the tool MUST parse the kubeconfig
And the tool MUST extract all contexts with metadata (name, cluster, namespace, user)
And the tool MUST identify the current-context
And the response MUST include a list of contexts
And the response MUST indicate which context is current
And the tool MUST NOT connect to any cluster
And the tool MUST complete in under 100ms

#### Scenario: List contexts from invalid kubeconfig

Given the agent provides invalid base64-encoded data
When the agent calls k0rdent_cluster_list_contexts
Then the tool MUST return an error with type "invalid_kubeconfig"
And the tool MUST NOT attempt to connect to any cluster

## Security Considerations

### DEV_ALLOW_ANY Mode

- All tools are available
- No restrictions on kubeconfig content
- Dynamic connection is allowed

### OIDC_REQUIRED Mode

**Option A (Restrictive - Initial Implementation)**:
- `k0rdent_cluster_connect`: Return permission error
- `k0rdent_cluster_disconnect`: Allow (disconnect startup connection)
- `k0rdent_cluster_status`: Allow (read-only)
- `k0rdent_cluster_list_contexts`: Return permission error

**Option B (Flexible - Future)**:
- Allow dynamic connection but validate kubeconfig
- Kubeconfig can provide cluster endpoint/CA only
- Credentials must come from OIDC bearer token
- Requires kubeconfig validation logic

**Initial implementation MUST use Option A**.

## Testing

### Unit Tests

- Tool input validation
- Base64 decoding
- Error response formats
- Idempotent disconnect

### Integration Tests

- Full connect → status → disconnect flow
- Connect to multiple clusters sequentially
- List contexts without connecting
- Error handling for invalid kubeconfig
- Subscription cleanup after disconnect

### Security Tests

- Verify permission errors in OIDC mode
- Verify DEV_ALLOW_ANY allows all tools
- Test with malformed kubeconfig
- Test with unreachable cluster

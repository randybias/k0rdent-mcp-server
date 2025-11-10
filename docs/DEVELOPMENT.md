# Development Guide

This guide covers setting up a local development environment for the k0rdent MCP server.

## Prerequisites

- Go 1.24 or later
- kubectl installed and configured
- Docker (for running local k0rdent cluster)
- kind or similar tool (for Kubernetes clusters)

## Setting Up a Test k0rdent Cluster

The MCP server requires an existing k0rdent management cluster. Here's how to set one up locally:

### Option 1: Using kind + k0rdent Installation

1. **Create a kind cluster**:
   ```bash
   kind create cluster --name k0rdent-dev
   ```

2. **Install k0rdent**:
   Follow the installation instructions at https://docs.k0rdent.io/quick-start/

   Quick version:
   ```bash
   # Install k0rdent operator
   kubectl apply -f https://github.com/k0rdent/kcm/releases/latest/download/install.yaml

   # Wait for deployment
   kubectl wait --for=condition=Available=True deployment/kcm-controller-manager -n kcm-system --timeout=5m
   ```

3. **Verify installation**:
   ```bash
   kubectl get deployments -n kcm-system
   kubectl get clustertemplate -A
   ```

### Option 2: Using an Existing k0rdent Cluster

If you already have access to a k0rdent management cluster:

1. **Get the kubeconfig**:
   ```bash
   # Copy kubeconfig to a local file
   cp ~/.kube/config ./test-kubeconfig
   ```

2. **Verify access**:
   ```bash
   kubectl --kubeconfig=./test-kubeconfig get ns
   kubectl --kubeconfig=./test-kubeconfig get clustertemplate -A
   ```

## Getting Admin Kubeconfig

The MCP server currently requires **admin-level access** to the k0rdent cluster. RBAC limitations are not yet supported.

### For kind Clusters

kind clusters provide admin access by default. Just use the generated kubeconfig:

```bash
kind get kubeconfig --name k0rdent-dev > ./dev-kubeconfig
```

### For Production Clusters

**WARNING**: Do NOT use production clusters for development.

If you must use a shared cluster, ensure your kubeconfig has sufficient permissions:

```bash
# Check your permissions
kubectl auth can-i --list --kubeconfig=./test-kubeconfig
```

Required permissions (minimum):
- Full access to `kcm-system` namespace
- Read access to all namespaces
- Create/delete ClusterDeployments
- Read/list ServiceTemplates, ClusterTemplates
- Read events and pod logs

## Building the Server

### Standard Build

```bash
# Clone repository
git clone https://github.com/k0rdent/k0rdent-mcp-server.git
cd k0rdent-mcp-server

# Build
go build -o server cmd/server/main.go
```

### Development Build with Debug Info

```bash
go build -gcflags="all=-N -l" -o server cmd/server/main.go
```

### Running Tests

```bash
# Unit tests only
go test ./...

# With coverage
go test -cover ./...

# Integration tests (requires cluster)
go test -tags=integration ./... -v
```

**Note**: Integration tests may create real cloud resources. Use carefully.

## Configuring for Development

### Set Environment Variables

```bash
# Required: Point to kubeconfig
export K0RDENT_MGMT_KUBECONFIG_PATH=./dev-kubeconfig

# Optional: Specify context
export K0RDENT_MGMT_CONTEXT=kind-k0rdent-dev

# Optional: Custom port (default is 6767)
export LISTEN_ADDR=:3000

# Optional: Namespace filter regex
export K0RDENT_NAMESPACE_FILTER='^(kcm-system|default)$'

# Optional: Debug logging
export LOG_LEVEL=debug
```

### Verify Configuration

```bash
# Test kubeconfig access
kubectl --kubeconfig=./dev-kubeconfig get ns

# Start server
./server start

# In another terminal, check server is running
curl http://localhost:6767/health  # or your custom port
```

## Common Development Tasks

### Running the Server

```bash
# Standard mode (uses environment variables)
./server start

# With debug logging (override via flag)
./server start --debug

# Override listen address via flag
./server start --listen :8080

# Set log level via flag
./server start --log-level debug
```

### Watching Logs

```bash
# Server logs
tail -f k0rdent-mcp-server.logs

# Kubernetes events
kubectl --kubeconfig=./dev-kubeconfig get events -n kcm-system --watch

# Pod logs
kubectl --kubeconfig=./dev-kubeconfig logs -n kcm-system <pod-name> --follow
```

### Testing MCP Tools

Use an MCP client like Claude Desktop or the MCP inspector:

1. **Configure client** to connect to `http://localhost:6767/mcp` (or your custom port)

2. **Test basic operations**:
   - List namespaces
   - List cluster templates
   - List credentials

3. **Test deployments** (creates real cloud resources):
   - Deploy Azure cluster (safest, most tested)
   - Monitor deployment progress
   - Delete cluster when done

### Iterating on Code

Typical development cycle:

```bash
# 1. Make code changes
vim pkg/tools/deploy.go

# 2. Run tests
go test ./pkg/tools -v

# 3. Rebuild
go build -o server cmd/server/main.go

# 4. Restart server (stop previous one first)
./server start
```

### Debugging

#### Enable Verbose Logging

Set environment variable:
```bash
export LOG_LEVEL=debug
./server start
```

Or use command-line flag:
```bash
./server start --debug
```

#### Using Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Build with debug symbols
go build -gcflags="all=-N -l" -o server cmd/server/main.go

# Run with debugger
dlv exec ./server -- start
```

## Troubleshooting Development Issues

### Server Won't Start

**Check kubeconfig path**:
```bash
kubectl --kubeconfig=./dev-kubeconfig cluster-info
```

**Check port availability**:
```bash
lsof -i :6767  # or your custom port
# If port is in use, either kill the process or change LISTEN_ADDR environment variable
```

**Check logs for errors**:
```bash
cat k0rdent-mcp-server.logs
```

### Cannot Connect to Cluster

**Verify kubeconfig**:
```bash
kubectl --kubeconfig=./dev-kubeconfig get ns
```

**Check context**:
```bash
kubectl config get-contexts --kubeconfig=./dev-kubeconfig
kubectl config use-context <context-name> --kubeconfig=./dev-kubeconfig
```

**Test k0rdent resources**:
```bash
kubectl --kubeconfig=./dev-kubeconfig get clustertemplate -A
```

### RBAC Permission Errors

The server currently requires admin access. If you see permission errors:

1. **Verify permissions**:
   ```bash
   kubectl auth can-i --list --kubeconfig=./dev-kubeconfig
   ```

2. **Use admin kubeconfig** (for kind clusters):
   ```bash
   kind get kubeconfig --name k0rdent-dev > ./dev-kubeconfig
   ```

3. **For other clusters**, request admin access or use a cluster where you have full permissions.

### MCP Client Connection Issues

**Check server is running**:
```bash
curl http://localhost:6767/health  # default port
```

**Verify MCP endpoint**:
```bash
curl http://localhost:6767/mcp
# Should return MCP protocol response
```

**Check client configuration**:
- URL should be `http://localhost:6767/mcp` (or your custom port)
- No authentication required for localhost

### Provider Deployment Failures

**Azure**:
- Verify credentials exist: `kubectl get azureclusteridentity -A`
- Check subscription ID is correct
- Review events: `kubectl get events -n kcm-system`

**AWS**:
- Verify credentials: `kubectl get awsclusterstaticidentity -A`
- Check region is valid
- Review CloudFormation console for stack errors

**GCP** (untested):
- May not work at all (known limitation)
- If testing, verify credentials exist
- Check project ID and region

## Testing Against Different Configurations

### Testing with Namespace Filters

Set environment variable with regex pattern:
```bash
export K0RDENT_NAMESPACE_FILTER='^(kcm-system|test-namespace)$'
./server start
```

Verify only filtered namespaces are accessible.

### Testing OIDC (Not Yet Supported)

OIDC support is not implemented. Use kubeconfig mode only.

### Testing with Limited RBAC (Not Recommended)

Limited RBAC is not well-tested. For development:
1. Create a service account with limited permissions
2. Generate kubeconfig for that service account
3. Set `K0RDENT_MGMT_KUBECONFIG_PATH` to point to it
4. Expect issues - RBAC enforcement is incomplete

## Code Organization

```
k0rdent-mcp-server/
├── cmd/
│   └── server/          # Server entry point
├── pkg/
│   ├── api/             # MCP API handlers
│   ├── kube/            # Kubernetes client wrapper
│   ├── tools/           # MCP tool implementations
│   ├── subscriptions/   # MCP subscriptions (monitoring, events)
│   └── catalog/         # Catalog operations
├── docs/                # Documentation
└── openspec/            # Change proposals

# No config.yaml - configuration via environment variables
```

## Next Steps

- Read [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines
- Check `openspec list` for proposed changes and ongoing work
- Join k0rdent community channels for questions
- Start with small improvements (tests, docs, bug fixes)

## Resources

- k0rdent Documentation: https://docs.k0rdent.io
- MCP Protocol: https://modelcontextprotocol.io
- kind Documentation: https://kind.sigs.k8s.io
- Go Testing: https://go.dev/doc/tutorial/add-a-test

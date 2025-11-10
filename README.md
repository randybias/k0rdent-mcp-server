# k0rdent MCP Server

⚠️ **Experimental Development Tool** – Early stage, expect issues

🚧 **Localhost Only** – No TLS, admin kubeconfig required

🤖 **Developed with AI-Assistance** – Code quality and security not production-ready

## **USE AT YOUR OWN RISK**

You use this experimental MCP server at your own risk.  Neither Randy Bias nor Mirantis, Inc. take any responsibility for your usage.  This is a proof of concept for using AI agents to control k0rdent management and child clusters through the MCP protocol.  It is absolutely NOT production-ready.

## What This Is

An experimental MCP server that exposes k0rdent cluster management capabilities to AI assistants through the Model Context Protocol. This is a **development tool** for k0rdent developers and early adopters who want to explore MCP integration, not a production-ready solution.

**Key Points:**
- Runs on localhost only (no TLS support)
- Requires admin kubeconfig to an existing k0rdent management cluster
- Does NOT provision a management cluster for you
- Built with AI assistance - code quality needs improvement
- Provider support varies: Azure tested, AWS minimal, GCP untested

## What This Isn't

- ❌ Not production-ready
- ❌ Not a standalone tool (needs existing k0rdent cluster)
- ❌ Not secure for network exposure (localhost only)
- ❌ Not fully tested across all providers
- ❌ Not suitable for RBAC-restricted environments (requires admin access)

## Prerequisites (All Required)

Before starting, you must have:

1. **Existing k0rdent management cluster** – This tool does NOT create one for you. You need a running k0rdent installation.
2. **Admin kubeconfig** – Full cluster access required. RBAC limitations not tested.
3. **Go 1.24+** – To build from source.
4. **MCP-compatible client** – Claude Desktop recommended.
5. **k0rdent knowledge** – Understanding of ClusterDeployments, ServiceTemplates, credentials, etc.
6. **Localhost deployment** – No remote access, no TLS.

## Known Limitations & Issues

Read this section carefully before using:

### Provider Support
- **GCP**: Not tested, may not work at all
- **Azure**: Works but requires manual subscription ID parameter (not auto-detected)
- **AWS**: Minimally tested, expect issues

### Authentication & Security
- **Only admin kubeconfig** – No OIDC support, no RBAC enforcement
- **AI-assisted code** – Not security-reviewed, use at your own risk
- **Localhost only** – No TLS, runs on 127.0.0.1 only
- **No auth modes** – Only kubeconfig-based access

### Functionality Gaps
- **Catalog operations** – Synchronization may have bugs
- **Concurrent operations** – Race conditions possible
- **Error recovery** – May leave orphaned cloud resources
- **Resource cleanup** – Not guaranteed on failures

### Deployment Warnings
- **Creates real cloud resources** – Costs apply to your cloud account
- **May leave orphans** – Failed deployments may not clean up completely
- **Experimental** – Expect crashes and unexpected behavior

## Quick Start (Experimental)

1. **Clone the repository**
   ```bash
   git clone https://github.com/randybias/k0rdent-mcp-server.git
   cd k0rdent-mcp-server
   ```

2. **Build the server**
   ```bash
   go build -o server cmd/server/main.go
   ```

3. **Set required environment variables**
   ```bash
   # Required: Point to your k0rdent cluster kubeconfig
   export K0RDENT_MGMT_KUBECONFIG_PATH=/path/to/admin-kubeconfig

   # Optional: Override default (defaults to 127.0.0.1:6767 for security)
   export LISTEN_ADDR=127.0.0.1:3000

   # Optional: Set log level
   export LOG_LEVEL=debug
   ```

4. **Start the server**
   ```bash
   ./server start
   ```

5. **Configure Claude Code** (see detailed instructions below)

6. **Try safe operations first**
   - List namespaces (safest)
   - List cluster templates
   - List credentials

   **WARNING**: Cluster deployment operations will create real cloud resources and incur costs.

## Claude Code Setup

### Installation

1. **Install Claude Code** (if not already installed)
   ```bash
   # macOS/Linux
   curl -fsSL https://claude.com/install.sh | sh

   # Or download from: https://claude.com/claude-code
   ```

2. **Configure MCP Server Connection**

   Add the k0rdent MCP server to your Claude Code configuration:

   ```bash
   # Edit Claude Code MCP settings
   # Location: ~/.config/claude-code/mcp_settings.json (Linux/macOS)
   #           %APPDATA%\claude-code\mcp_settings.json (Windows)
   ```

   Add this server configuration:
   ```json
   {
     "mcpServers": {
       "k0rdent": {
         "url": "http://localhost:6767/mcp",
         "transport": "http"
       }
     }
   }
   ```

3. **Start the k0rdent MCP server** (as shown in Quick Start above)

4. **Launch Claude Code** and verify connection
   ```bash
   claude
   ```

### Example Usage

Once connected, you can use natural language to interact with your k0rdent clusters:

#### Safe Exploration Commands
```
"List all namespaces in the management cluster"
"Show me all available cluster templates"
"What credentials are configured for Azure?"
"List all cluster deployments"
```

#### Cluster Deployment Examples
```
"Deploy a new Azure cluster named 'dev-cluster' in eastus region using Standard_D2s_v3 VMs"

"Create an AWS cluster with 3 control plane nodes and 5 workers in us-west-2"

"Show me the provisioning status of cluster 'prod-cluster' in namespace 'kcm-system'"
```

#### Service Management Examples
```
"List all ServiceTemplates available in the catalog"

"Install the ingress-nginx ServiceTemplate from the catalog"

"Apply the monitoring ServiceTemplate to cluster 'dev-cluster'"

"What services are currently running on cluster 'prod-cluster'?"
```

#### Monitoring and Troubleshooting
```
"Subscribe to provisioning updates for cluster 'dev-cluster' in namespace 'kcm-system'"

"Show me recent events in the kcm-system namespace"

"Get logs from pod 'controller-manager-xyz' in namespace 'kcm-system'"

"What's the current state of my cluster deployment 'staging-cluster'?"
```

#### Cleanup
```
"Delete cluster 'dev-cluster' from namespace 'kcm-system'"

"Remove the test-cluster and wait for deletion to complete"
```

### Tips for Using with Claude Code

- **Start with read-only operations** to familiarize yourself with your cluster state
- **Use natural language** – Claude Code understands intent, not just commands
- **Ask for confirmations** on destructive operations (Claude Code will prompt you)
- **Monitor costs** – cluster deployments create real cloud resources
- **Use subscriptions** for long-running operations like cluster provisioning
- **Check cluster state** before and after operations with `getState` tool

## What Works (Tested Minimally)

These features have been tested and should work:

- **Azure Cluster Deployment** – Works if you provide subscription ID
- **Cluster Monitoring** – Subscribe to provisioning progress via `k0rdent://cluster-monitor/{namespace}/{name}`
- **Namespace Operations** – List namespaces and basic K8s operations
- **Event Streaming** – Watch namespace events via `k0rdent://events/{namespace}`
- **Pod Logs** – Tail container logs via `k0rdent://podlogs/{namespace}/{pod}/{container}`
- **Service Attachments** – Attach ServiceTemplates to running clusters (needs more testing)
- **Credential Management** – List provider credentials

## What's Untested or Broken

These features may not work:

- **GCP Deployments** – Completely untested, likely broken
- **AWS Deployments** – Minimally tested, may have issues
- **Catalog Operations** – Known bugs in synchronization
- **Non-admin Access** – RBAC filtering not implemented
- **Concurrent Operations** – Race conditions likely
- **Error Recovery** – May fail ungracefully
- **Resource Cleanup** – Orphaned resources possible on failures

## Configuration

The server is configured entirely through environment variables (no config file):

### Required Variables

```bash
# Kubeconfig path (required)
export K0RDENT_MGMT_KUBECONFIG_PATH=/path/to/kubeconfig
```

### Optional Variables

```bash
# Server configuration
export LISTEN_ADDR=127.0.0.1:6767           # Listen address (default: 127.0.0.1:6767)
                                            # Use 0.0.0.0:6767 to bind to all interfaces (NOT RECOMMENDED - no TLS)
export AUTH_MODE=DEV_ALLOW_ANY              # Auth mode (default: DEV_ALLOW_ANY)
                                            # Options: DEV_ALLOW_ANY, OIDC_REQUIRED

# Kubernetes configuration
export K0RDENT_MGMT_CONTEXT=my-context      # Override kubeconfig context
export K0RDENT_NAMESPACE_FILTER='^kcm-.*'   # Namespace filter regex

# Logging configuration
export LOG_LEVEL=info                       # Log level (debug, info, warn, error)
export LOG_EXTERNAL_SINK_ENABLED=false      # Enable external JSON logging

# Cluster provisioning defaults
export CLUSTER_GLOBAL_NAMESPACE=kcm-system           # Global namespace (default: kcm-system)
export CLUSTER_DEFAULT_NAMESPACE_DEV=kcm-system      # Dev mode namespace
export CLUSTER_DEPLOY_FIELD_OWNER=mcp.clusters       # Server-side apply owner
```

**Note**: No config.yaml file is used. All configuration is via environment variables or command-line flags (`--listen`, `--debug`, `--log-level`).

## Tools Overview

The server exposes the following MCP tools:

| Tool Name | Purpose | Status |
|-----------|---------|--------|
| **Cluster Management** | | |
| `k0rdent.mgmt.clusterDeployments.list` | List all ClusterDeployments | Works |
| `k0rdent.mgmt.clusterDeployments.listAll` | List ClusterDeployments with selector | Works |
| `k0rdent.mgmt.clusterDeployments.getState` | Get cluster state including services (WIP) | Works |
| `k0rdent.mgmt.clusterDeployments.delete` | Delete a ClusterDeployment | Works |
| `k0rdent.provider.aws.clusterDeployments.deploy` | Deploy child cluster to AWS provider | Minimal testing |
| `k0rdent.provider.azure.clusterDeployments.deploy` | Deploy child cluster to Azure provider | Tested, requires subscriptionID |
| `k0rdent.provider.gcp.clusterDeployments.deploy` | Deploy child cluster to GCP provider | Untested |
| **Service Templates and Service Management** | | |
| `k0rdent.mgmt.clusterDeployments.services.apply` | Apply ServiceTemplate to cluster | Mostly work; may be edge cases; doesn't handle params |
| `k0rdent.mgmt.serviceTemplates.list` | List installed ServiceTemplates mgmt server | Works |
| `k0rdent.mgmt.serviceTemplates.install_from_catalog` | Install ServiceTemplate to mgmt server from catalog | May have bugs; mostly tested |
| `k0rdent.mgmt.serviceTemplates.delete` | Delete ServiceTemplate from mgmt server | Works |
| `k0rdent.mgmt.multiClusterServices.list` | List MultiClusterServices | Untested |
| **Catalog Operations** | | |
| `k0rdent.catalog.serviceTemplates.list` | List catalog ServiceTemplates | Works |
| **Provider & Credentials** | | |
| `k0rdent.mgmt.providers.list` | List infrastructure providers | Works |
| `k0rdent.mgmt.providers.listCredentials` | List provider credentials | Works |
| `k0rdent.mgmt.providers.listIdentities` | List ClusterIdentity resources | Works |
| **Cluster Templates** | | |
| `k0rdent.mgmt.clusterTemplates.list` | List ClusterTemplates | Works |
| **Kubernetes Operations** | | |
| `k0rdent.mgmt.namespaces.list` | List namespaces | Works |
| `k0rdent.mgmt.events.list` | List namespace events | Works |
| `k0rdent.mgmt.podLogs.get` | Get pod logs | Works |

### MCP Resources (Subscriptions)

The server also provides streaming resources (largely untested):

| Resource URI | Purpose | Status |
|--------------|---------|--------|
| `k0rdent://cluster-monitor/{namespace}/{name}` | Stream cluster provisioning updates | Tested on Azure |
| `k0rdent://events/{namespace}` | Stream namespace events | Works |
| `k0rdent://podlogs/{namespace}/{pod}/{container}` | Stream pod logs | Works |

For detailed tool documentation, see `docs/` directory.

## Documentation

- [Cluster Provisioning](docs/cluster-provisioning.md) – Deployment workflows (Azure focus)
- [Provider-Specific Tools](docs/provider-specific-deployment.md) – Per-provider deployment details
- [Cluster Monitoring](docs/features/cluster-monitoring.md) – Real-time provisioning updates
- [Catalog Operations](docs/catalog.md) – Installing service templates
- [Live Tests](docs/live-tests.md) – Test playbooks for validation
- [Troubleshooting Guide](docs/TROUBLESHOOTING.md) – Common issues and solutions
- [Contributing Guide](CONTRIBUTING.md) – Development workflow and OpenSpec process
- [Development Setup](docs/DEVELOPMENT.md) – Local development environment

For proposed changes and specifications, see the `openspec/` directory or run `openspec list`.

## Contributing

This experimental project was built with AI assistance. Code quality and security need improvement. Contributions are welcome, especially:

- Testing GCP and AWS deployment paths
- Fixing catalog synchronization bugs
- Improving error handling and recovery
- Adding proper RBAC support
- Security review and hardening
- Fixing AI-generated code issues
- Writing tests for untested code paths

See [CONTRIBUTING.md](CONTRIBUTING.md) for the OpenSpec workflow and development guidelines.

## Security & Disclaimer

**READ THIS BEFORE USING:**

- ⚠️ **Not production-ready** – Experimental software, use at own risk
- ⚠️ **AI-assisted code** – May contain security vulnerabilities
- ⚠️ **Admin access required** – No RBAC enforcement, assumes full cluster access
- ⚠️ **Localhost only** – No TLS, not safe for network exposure
- ⚠️ **Creates real cloud resources** – Costs apply to your accounts
- ⚠️ **May leave orphaned resources** – Failed operations may not clean up
- ⚠️ **No warranty** – Use at your own risk

**Recommendations:**
- Use non-production clusters only
- Set up cloud cost alerts before deploying
- Review cloud resources after operations
- Keep admin kubeconfig secure
- Do not expose server to network

## Roadmap (Maybe)

Potential future improvements (no promises):

- Fix and test GCP deployment path
- Stabilize AWS deployments
- Fix catalog synchronization bugs
- Add RBAC support (non-admin access)
- Add TLS support for remote access
- Security review and hardening
- Production deployment options
- Improved error handling and recovery

See `openspec list` for detailed proposed changes.

## Getting Help

- **Issues**: https://github.com/randybias/k0rdent-mcp-server/issues
- **Discussions**: https://github.com/randybias/k0rdent-mcp-server/discussions
- **k0rdent Docs**: https://docs.k0rdent.io
- **MCP Protocol**: https://modelcontextprotocol.io

For development questions, see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Add license information here]

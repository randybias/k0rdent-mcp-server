# k0rdent MCP Server

⚠️ **Experimental Development Tool** – Early stage, expect issues
🚧 **Localhost Only** – No TLS, admin kubeconfig required
🤖 **AI-Assisted** – Code quality and security not production-ready

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

   # Optional: Override default port (6767)
   export LISTEN_ADDR=:3000

   # Optional: Set log level
   export LOG_LEVEL=debug
   ```

4. **Start the server**
   ```bash
   ./server start
   ```

5. **Connect Claude Desktop**
   Configure your MCP client to connect to `http://localhost:6767/mcp` (or your custom port)

6. **Try safe operations first**
   - List namespaces (safest)
   - List cluster templates
   - List credentials

   **WARNING**: Cluster deployment operations will create real cloud resources and incur costs.

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
# Kubeconfig (choose ONE method):
export K0RDENT_MGMT_KUBECONFIG_PATH=/path/to/kubeconfig  # Path to file
# OR
export K0RDENT_MGMT_KUBECONFIG_B64=<base64-encoded>      # Base64-encoded
# OR
export K0RDENT_MGMT_KUBECONFIG_TEXT=<kubeconfig-yaml>    # Direct YAML
```

### Optional Variables

```bash
# Server configuration
export LISTEN_ADDR=:6767                    # Listen address (default: :6767)
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

The server exposes MCP tools for:

| Category | Tools | Status |
|----------|-------|--------|
| Cluster Management | list, deploy, delete | Azure works, AWS minimal, GCP untested |
| Monitoring | cluster-monitor subscription | Tested on Azure |
| Troubleshooting | events, pod logs | Basic functionality works |
| Catalog | list, install ServiceTemplates | May have bugs |
| Credentials | list providers, credentials | Works |
| Templates | list ClusterTemplates | Works |

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

- **Issues**: https://github.com/k0rdent/k0rdent-mcp-server/issues
- **Discussions**: https://github.com/k0rdent/k0rdent-mcp-server/discussions
- **k0rdent Docs**: https://docs.k0rdent.io
- **MCP Protocol**: https://modelcontextprotocol.io

For development questions, see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Add license information here]

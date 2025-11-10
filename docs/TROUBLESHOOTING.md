# Troubleshooting Guide

Common issues and solutions when running the k0rdent MCP server.

## Server Issues

### Server Won't Start

#### Port Already in Use

**Error**: `bind: address already in use`

**Solution**:
```bash
# Check what's using port 6767 (default)
lsof -i :6767

# Kill the process or change port via environment variable
export LISTEN_ADDR=127.0.0.1:3001  # Use different port
./server start
```

#### Kubeconfig Not Found

**Error**: `failed to load kubeconfig: stat /path/to/kubeconfig: no such file or directory`

**Solution**:
- Verify the path in `K0RDENT_MGMT_KUBECONFIG_PATH` is correct
- Use absolute path, not relative
- Check file permissions: `ls -la /path/to/kubeconfig`
- Verify environment variable is set: `echo $K0RDENT_MGMT_KUBECONFIG_PATH`

#### Invalid Kubeconfig

**Error**: `unable to connect to cluster`

**Solution**:
```bash
# Test kubeconfig directly
kubectl --kubeconfig=/path/to/kubeconfig cluster-info

# Check current context
kubectl config current-context --kubeconfig=/path/to/kubeconfig

# View available contexts
kubectl config get-contexts --kubeconfig=/path/to/kubeconfig
```

### Server Crashes on Startup

**Check logs**:
```bash
cat k0rdent-mcp-server.logs
```

Common causes:
- Missing required environment variables (K0RDENT_MGMT_KUBECONFIG_PATH)
- Invalid kubeconfig file
- Missing k0rdent installation on cluster
- Network connectivity issues
- RBAC permission problems

## MCP Client Connection Issues

### 405 Method Not Allowed

**Error**: MCP client reports "405 Method Not Allowed"

**Cause**: Your MCP client doesn't support the Streamable HTTP transport that this server requires.

**Solution**:
- Use Claude Desktop (recommended)
- Update your MCP client to support Streamable HTTP
- Check client documentation for transport support

### Connection Refused

**Error**: `Connection refused to http://localhost:6767/mcp`

**Solution**:
1. **Verify server is running**:
   ```bash
   curl http://localhost:6767/health  # default port
   ```

2. **Check server logs**:
   ```bash
   tail -f k0rdent-mcp-server.logs
   ```

3. **Verify port number** matches `LISTEN_ADDR` environment variable and client configuration

### Client Can't List Tools

**Error**: No tools appear in MCP client

**Solution**:
1. **Check MCP endpoint**: Should be `http://localhost:6767/mcp` (not just `http://localhost:6767`) - use your custom port if set
2. **Verify authentication**: Server on localhost requires no auth
3. **Check server logs** for errors during tool registration

## Kubernetes Access Issues

### RBAC Permission Denied

**Error**: `forbidden: User "..." cannot list resource "clusterdeployments"`

**Cause**: Kubeconfig doesn't have required permissions

**Solution**:
1. **This server requires admin access** (RBAC filtering not implemented)
2. **For kind clusters**:
   ```bash
   kind get kubeconfig --name <cluster-name> > ./admin-kubeconfig
   ```
3. **Verify permissions**:
   ```bash
   kubectl auth can-i --list --kubeconfig=/path/to/kubeconfig
   ```

### Resource Not Found

**Error**: `clusterdeployments.k0rdent.mirantis.com not found`

**Cause**: k0rdent is not installed on the cluster

**Solution**:
1. **Verify k0rdent installation**:
   ```bash
   kubectl get deployment -n kcm-system
   kubectl get crd | grep k0rdent
   ```

2. **Install k0rdent** if missing:
   See https://docs.k0rdent.io/quick-start/

### Context Not Found

**Error**: `context "..." not found`

**Cause**: Specified context doesn't exist in kubeconfig

**Solution**:
```bash
# List available contexts
kubectl config get-contexts --kubeconfig=/path/to/kubeconfig

# Set context via environment variable
export K0RDENT_MGMT_CONTEXT=correct-context-name
```

## Cluster Deployment Issues

### Azure Deployment Failures

#### Missing Subscription ID

**Error**: Deployment created but fails with Azure API errors

**Cause**: Azure requires subscription ID parameter

**Solution**:
```json
{
  "name": "azure-cluster-1",
  "credential": "azure-cred",
  "location": "westus2",
  "subscriptionID": "your-subscription-id-here",
  ...
}
```

#### Invalid Credentials

**Error**: `Authentication failed` or `InvalidClientSecret`

**Cause**: Azure service principal credentials are invalid or expired

**Solution**:
1. **Check AzureClusterIdentity**:
   ```bash
   kubectl get azureclusteridentity -A -o yaml
   ```

2. **Verify secret**:
   ```bash
   kubectl get secret -n kcm-system <secret-name> -o yaml
   ```

3. **Test credentials** with Azure CLI:
   ```bash
   az login --service-principal \
     --username <clientId> \
     --password <clientSecret> \
     --tenant <tenantId>
   ```

### AWS Deployment Failures (Minimally Tested)

#### Missing Credentials

**Error**: `No valid credential found`

**Cause**: AWS credentials not configured

**Solution**:
1. **List AWS credentials**:
   ```bash
   kubectl get awsclusterstaticidentity -A
   ```

2. **Create if missing** - see k0rdent AWS documentation

#### Region Not Available

**Error**: `The requested availability zone is not available`

**Cause**: Invalid region or insufficient capacity

**Solution**:
- Use well-tested regions: `us-west-2`, `us-east-1`, `eu-west-1`
- Check AWS service health dashboard
- Try different region

### GCP Deployment Failures (Untested)

**Status**: GCP deployments are untested and likely broken

**If attempting**:
1. **Expect failures** - this is a known limitation
2. **Check events**:
   ```bash
   kubectl get events -n kcm-system --field-selector involvedObject.name=<cluster-name>
   ```
3. **Please report** issues on GitHub to help improve GCP support

## Catalog Issues

### Catalog Sync Failures

**Error**: `Failed to sync catalog` in logs

**Known Issue**: Catalog synchronization has bugs

**Workaround**:
1. **Restart server** to retry sync
2. **Check network connectivity** to catalog repository
3. **Verify k0rdent catalog** configuration:
   ```bash
   kubectl get helmrepository -n kcm-system
   ```

### ServiceTemplate Not Found

**Error**: `ServiceTemplate "..." not found`

**Solution**:
1. **List available templates**:
   ```bash
   kubectl get servicetemplate -A
   ```

2. **Check namespace** - templates may be namespace-scoped
3. **Sync catalog** if templates are missing

## Performance Issues

### Slow Operations

**Symptoms**: Tool calls take a long time

**Common causes**:
- Large cluster with many resources
- Network latency to cluster
- Inefficient queries (known issue)

**Mitigation**:
- Use namespace filters to limit scope
- Avoid concurrent operations (race conditions possible)
- Monitor server logs for slow queries

### Memory Usage

**Symptoms**: Server memory grows over time

**Known Issue**: Potential resource leaks in watchers

**Workaround**:
- Restart server periodically
- Monitor with: `ps aux | grep server`
- Report issues on GitHub

## Debugging Tips

### Enable Debug Logging

**Via environment variable**:
```bash
export LOG_LEVEL=debug
./server start
```

**Or via command-line flag**:
```bash
./server start --debug
```

### Watch Kubernetes Events

```bash
# Watch all events in kcm-system
kubectl get events -n kcm-system --watch

# Filter for specific cluster
kubectl get events -n kcm-system \
  --field-selector involvedObject.name=<cluster-name> \
  --watch
```

### Check ClusterDeployment Status

```bash
# Get full status
kubectl get clusterdeployment -n kcm-system <name> -o yaml

# Watch status changes
kubectl get clusterdeployment -n kcm-system <name> -w

# Check conditions
kubectl get clusterdeployment -n kcm-system <name> \
  -o jsonpath='{.status.conditions}'
```

### Review Cloud Provider Resources

#### Azure

```bash
# List resource groups
az group list --output table

# Check resources in group
az resource list --resource-group <name> --output table

# View activity log
az monitor activity-log list --resource-group <name>
```

#### AWS

```bash
# List CloudFormation stacks
aws cloudformation list-stacks

# Describe stack
aws cloudformation describe-stacks --stack-name <name>

# Check stack events
aws cloudformation describe-stack-events --stack-name <name>
```

### Capture Server Logs

```bash
# Full logs
cat k0rdent-mcp-server.logs

# Last 100 lines
tail -100 k0rdent-mcp-server.logs

# Follow in real-time
tail -f k0rdent-mcp-server.logs

# Search for errors
grep -i error k0rdent-mcp-server.logs
```

## Getting Help

If you can't resolve the issue:

1. **Check existing issues**: https://github.com/k0rdent/k0rdent-mcp-server/issues
2. **Gather information**:
   - Server logs
   - Config.yaml (redact sensitive info)
   - kubectl version and cluster info
   - Error messages
   - Steps to reproduce
3. **Open an issue** with details above
4. **Join discussions**: https://github.com/k0rdent/k0rdent-mcp-server/discussions

## Known Limitations

These are expected behaviors, not bugs:

- **GCP deployments untested** - May not work
- **AWS minimally tested** - Expect issues
- **Catalog sync bugs** - Known issue, restart server
- **Admin access required** - RBAC filtering not implemented
- **Localhost only** - No TLS, not network-accessible
- **Race conditions** - Avoid concurrent operations
- **Memory leaks possible** - Restart server if memory grows

See README.md for full list of limitations.

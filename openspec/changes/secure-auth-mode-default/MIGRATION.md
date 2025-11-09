# Migration Guide: Secure AUTH_MODE Default

## Overview

**Breaking Change**: Starting with version X.Y.Z, the `AUTH_MODE` environment variable is **required** and no longer defaults to `DEV_ALLOW_ANY`.

**Impact**: All deployments must explicitly set `AUTH_MODE` to either:
- `DEV_ALLOW_ANY` - Development/testing only (insecure)
- `OIDC_REQUIRED` - Production (secure)

**Reason**: The previous default (`DEV_ALLOW_ANY`) allowed any bearer token without validation, creating a critical security vulnerability in production deployments.

## Quick Fix

### For Development Environments

Add this to your environment configuration:

```bash
export AUTH_MODE=DEV_ALLOW_ANY
```

### For Production Environments

Ensure you have OIDC configured, then set:

```bash
export AUTH_MODE=OIDC_REQUIRED
```

## Detailed Migration Steps

### 1. Identify Your Current Configuration

**Check if AUTH_MODE is already set:**
```bash
echo $AUTH_MODE
```

- If it returns a value: You're already explicit (no action needed)
- If empty: You're using the insecure default (migration required)

### 2. Choose Your Authentication Mode

#### Development/Testing: Use DEV_ALLOW_ANY

**When to use:**
- Local development environments
- Automated testing
- Demo environments
- Non-production clusters

**Security warning:** This mode accepts ANY bearer token without validation. Never use in production.

**Configuration:**
```bash
# Shell
export AUTH_MODE=DEV_ALLOW_ANY

# Docker
docker run -e AUTH_MODE=DEV_ALLOW_ANY ...

# Kubernetes
env:
  - name: AUTH_MODE
    value: "DEV_ALLOW_ANY"

# systemd service
Environment="AUTH_MODE=DEV_ALLOW_ANY"
```

#### Production: Use OIDC_REQUIRED

**When to use:**
- Production environments
- Staging environments
- Any cluster with sensitive data

**Prerequisites:**
- Kubernetes API server configured with OIDC authentication
- Valid OIDC provider (e.g., Keycloak, Dex, Google, Okta)
- Client applications capable of obtaining bearer tokens

**Configuration:**
```bash
# Shell
export AUTH_MODE=OIDC_REQUIRED

# Docker
docker run -e AUTH_MODE=OIDC_REQUIRED ...

# Kubernetes
env:
  - name: AUTH_MODE
    value: "OIDC_REQUIRED"

# systemd service
Environment="AUTH_MODE=OIDC_REQUIRED"
```

### 3. Update Your Deployment Configuration

#### Docker Compose Example

**Before:**
```yaml
services:
  k0rdent-mcp-server:
    image: k0rdent/mcp-server:latest
    environment:
      K0RDENT_MGMT_KUBECONFIG_PATH: /etc/kubeconfig
      # AUTH_MODE not set - INSECURE DEFAULT
```

**After (Development):**
```yaml
services:
  k0rdent-mcp-server:
    image: k0rdent/mcp-server:latest
    environment:
      K0RDENT_MGMT_KUBECONFIG_PATH: /etc/kubeconfig
      AUTH_MODE: DEV_ALLOW_ANY  # EXPLICIT
```

**After (Production):**
```yaml
services:
  k0rdent-mcp-server:
    image: k0rdent/mcp-server:latest
    environment:
      K0RDENT_MGMT_KUBECONFIG_PATH: /etc/kubeconfig
      AUTH_MODE: OIDC_REQUIRED  # SECURE
```

#### Kubernetes Deployment Example

**Before:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k0rdent-mcp-server
spec:
  template:
    spec:
      containers:
      - name: server
        image: k0rdent/mcp-server:latest
        env:
        - name: K0RDENT_MGMT_KUBECONFIG_PATH
          value: /etc/kubeconfig
        # AUTH_MODE not set - INSECURE DEFAULT
```

**After (Production):**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k0rdent-mcp-server
spec:
  template:
    spec:
      containers:
      - name: server
        image: k0rdent/mcp-server:latest
        env:
        - name: K0RDENT_MGMT_KUBECONFIG_PATH
          value: /etc/kubeconfig
        - name: AUTH_MODE
          value: OIDC_REQUIRED  # EXPLICIT AND SECURE
```

#### Systemd Service Example

**Before:**
```ini
[Unit]
Description=k0rdent MCP Server

[Service]
ExecStart=/usr/local/bin/k0rdent-mcp-server
Environment="K0RDENT_MGMT_KUBECONFIG_PATH=/etc/k0rdent/kubeconfig"
# AUTH_MODE not set - INSECURE DEFAULT

[Install]
WantedBy=multi-user.target
```

**After (Production):**
```ini
[Unit]
Description=k0rdent MCP Server

[Service]
ExecStart=/usr/local/bin/k0rdent-mcp-server
Environment="K0RDENT_MGMT_KUBECONFIG_PATH=/etc/k0rdent/kubeconfig"
Environment="AUTH_MODE=OIDC_REQUIRED"  # EXPLICIT AND SECURE

[Install]
WantedBy=multi-user.target
```

### 4. Verify Configuration

After updating, verify the server starts correctly:

#### Expected Success Logs

**With DEV_ALLOW_ANY (Development):**
```
INFO  configuration loaded auth_mode=DEV_ALLOW_ANY
WARN  SECURITY WARNING: AUTH_MODE=DEV_ALLOW_ANY allows any bearer token.
      This mode is INSECURE and must not be used in production.
INFO  server started successfully
```

**With OIDC_REQUIRED (Production):**
```
INFO  configuration loaded auth_mode=OIDC_REQUIRED
INFO  server started successfully
```

#### Expected Failure (AUTH_MODE not set)

```
ERROR failed to load configuration error="AUTH_MODE environment variable is required;
      set to DEV_ALLOW_ANY (development only) or OIDC_REQUIRED (production)"
FATAL startup failed
```

## Troubleshooting

### Error: "AUTH_MODE environment variable is required"

**Cause:** `AUTH_MODE` is not set or is empty.

**Fix:** Add `export AUTH_MODE=DEV_ALLOW_ANY` (dev) or `AUTH_MODE=OIDC_REQUIRED` (prod) to your configuration.

### Warning: "SECURITY WARNING: AUTH_MODE=DEV_ALLOW_ANY"

**Cause:** Server is running in insecure development mode.

**Expected:** This warning is normal for development environments.

**Action Required:** If this appears in production logs, immediately:
1. Stop the server
2. Configure proper OIDC authentication
3. Set `AUTH_MODE=OIDC_REQUIRED`
4. Restart the server

### 401 Unauthorized After Setting OIDC_REQUIRED

**Cause:** Clients are not providing valid bearer tokens, or OIDC is not properly configured.

**Debugging Steps:**

1. Verify Kubernetes API server has OIDC configured:
   ```bash
   kubectl cluster-info dump | grep oidc
   ```

2. Test with a valid bearer token:
   ```bash
   TOKEN=$(kubectl get secret ... -o jsonpath='{.data.token}' | base64 -d)
   curl -H "Authorization: Bearer $TOKEN" http://server:port/mcp
   ```

3. Check server logs for authentication details:
   ```bash
   kubectl logs -l app=k0rdent-mcp-server | grep -i auth
   ```

## Rollback (Not Recommended)

If you must rollback to the previous version:

```bash
# Downgrade to version before breaking change
docker pull k0rdent/mcp-server:v0.X.Y

# Or use git to checkout previous commit
git checkout <previous-release-tag>
```

**Warning:** Rollback reintroduces the security vulnerability. Use only as temporary measure while planning proper OIDC configuration.

## Timeline

- **Version X.Y.Z-rc.1**: Deprecation warning added (if pre-release available)
- **Version X.Y.Z**: Breaking change applied, `AUTH_MODE` required
- **Upgrade deadline**: Immediate action required on upgrade

## Support

If you encounter issues during migration:

1. Check server logs for detailed error messages
2. Review this migration guide
3. Consult the main documentation
4. File an issue with:
   - Deployment method (Docker, Kubernetes, systemd, etc.)
   - Error messages from logs
   - Current AUTH_MODE configuration (if any)
   - Server version

## Security Best Practices

1. **Never use `DEV_ALLOW_ANY` in production** - It accepts any bearer token without validation
2. **Always use `OIDC_REQUIRED` for production** - Enforces proper authentication
3. **Monitor logs for security warnings** - Alert on "SECURITY WARNING" log messages
4. **Audit AUTH_MODE configuration** - Include in security review checklists
5. **Document authentication setup** - Maintain deployment runbooks with AUTH_MODE requirements

## Summary Checklist

- [ ] Identified current AUTH_MODE setting (or lack thereof)
- [ ] Chose appropriate mode (DEV_ALLOW_ANY for dev, OIDC_REQUIRED for prod)
- [ ] Updated deployment configuration (docker-compose.yml, k8s manifests, systemd, etc.)
- [ ] Tested server startup with new configuration
- [ ] Verified expected log messages appear
- [ ] Updated documentation and runbooks
- [ ] Communicated changes to team

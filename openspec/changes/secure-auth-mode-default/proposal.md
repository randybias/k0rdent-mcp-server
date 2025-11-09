# Change: secure-auth-mode-default

## Why
- Current default auth mode is `DEV_ALLOW_ANY` when `AUTH_MODE` env var is unset (config.go:253)
- This creates security risk: production deployments can accidentally run without authentication
- Violates secure-by-default principle
- No prominent warning when running in insecure mode

## What Changes
- Change default auth mode from `DEV_ALLOW_ANY` to `OIDC_REQUIRED`
- Require explicit `AUTH_MODE=DEV_ALLOW_ANY` for development/testing
- Add prominent WARNING log when server starts in DEV_ALLOW_ANY mode
- Update all documentation, examples, and deployment guides
- Provide migration guide for existing deployments
- Update tests to reflect secure default

## Impact
- **BREAKING CHANGE**: Existing deployments without `AUTH_MODE` set will fail to start
- Improves security posture significantly
- Forces explicit opt-in for insecure development mode
- May require deployment updates for users relying on implicit default
- Clear migration path: set `AUTH_MODE=DEV_ALLOW_ANY` explicitly if needed

## Acceptance
- Server refuses to start when `AUTH_MODE` unset and `OIDC_REQUIRED` mode cannot be satisfied
- Server logs prominent WARNING when running in `DEV_ALLOW_ANY` mode
- All documentation examples show explicit `AUTH_MODE` configuration
- Migration guide exists for v1.x users
- Tests verify secure default behavior
- `openspec validate secure-auth-mode-default --strict` passes

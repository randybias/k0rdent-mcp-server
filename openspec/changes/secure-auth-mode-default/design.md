# Design: secure-auth-mode-default

## Context
The current implementation defaults to `DEV_ALLOW_ANY` auth mode when the `AUTH_MODE` environment variable is unset. This violates the principle of secure-by-default and creates risk of production deployments running without authentication.

## Goals
- Make production deployments secure by default
- Force explicit configuration for insecure development mode
- Provide clear migration path for existing users
- Maintain backward compatibility through explicit configuration

## Non-Goals
- Implement new authentication mechanisms
- Change OIDC token validation logic
- Modify bearer token handling

## Key Decisions

### Decision 1: Default to OIDC_REQUIRED
**Choice:** Default auth mode is `OIDC_REQUIRED` when `AUTH_MODE` unset

**Rationale:**
- Secure by default is industry best practice
- Forces operators to think about authentication
- Prevents accidental production exposure
- OIDC is the production-ready mode

**Alternatives Considered:**
- Fail startup without AUTH_MODE: Too strict, breaks all deployments
- Keep DEV_ALLOW_ANY default: Maintains security risk
- Add third mode "REQUIRE_CONFIG": Unnecessary complexity

**Trade-offs:**
- Security: Significantly improved
- Usability: Requires explicit config in dev
- Backward Compatibility: Breaking change

### Decision 2: Prominent Warning for DEV_ALLOW_ANY
**Choice:** Log WARNING on every startup when `DEV_ALLOW_ANY` is active

**Rationale:**
- Makes insecure mode obvious in logs
- Helps catch accidental dev mode in production
- No performance impact (one log line)
- Easy to spot in log aggregation

### Decision 3: Migration Strategy
**Choice:** Provide MIGRATION.md with clear steps and version compatibility

**Migration Path:**
1. Check current deployment for `AUTH_MODE` env var
2. If unset and dev/test environment: Add `AUTH_MODE=DEV_ALLOW_ANY`
3. If unset and production: Add `AUTH_MODE=OIDC_REQUIRED` (should already be working)
4. Update deployment automation/scripts
5. Test in dev/staging before production rollout

**Rollback:** Set `AUTH_MODE=DEV_ALLOW_ANY` to restore old behavior

## Risks & Mitigation

### Risk: Breaks existing deployments
**Likelihood:** High
**Impact:** Medium (easy fix: set env var)

**Mitigation:**
- Clear migration guide in release notes
- Version bump to indicate breaking change
- Warning period in release notes
- Easy rollback (explicit env var)

### Risk: Users set DEV_ALLOW_ANY in production
**Likelihood:** Low-Medium
**Impact:** High (security breach)

**Mitigation:**
- Prominent WARNING log makes it obvious
- Documentation emphasizes production vs dev modes
- Security audit checklist includes auth mode verification

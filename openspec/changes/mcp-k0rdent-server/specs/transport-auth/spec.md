# Transport & Auth (delta)

## ADDED Requirements

### Requirement: MCP transport (Streamable HTTP)
- The server SHALL expose an MCP **Streamable HTTP** endpoint.

#### Scenario: Startup
- WHEN the server starts
- THEN the MCP Streamable HTTP endpoint is available at the configured path

### Requirement: Bearer auth at the transport
- The server SHALL return **401** when an `Authorization: Bearer <token>` header is missing while `AUTH_MODE=OIDC_REQUIRED`.
- The server SHALL accept any `Authorization: Bearer <DEV_BEARER_TOKEN>` when `AUTH_MODE=DEV_ALLOW_ANY` (dev only).

#### Scenario: Reject missing bearer (prod)
- WHEN `AUTH_MODE=OIDC_REQUIRED` AND a client connects without the `Authorization` header
- THEN the server responds **401**

#### Scenario: Allow dev bearer (dev)
- WHEN `AUTH_MODE=DEV_ALLOW_ANY` AND the request includes `Authorization: Bearer $DEV_BEARER_TOKEN`
- THEN the server accepts the request (200/OK)

### Requirement: No Kubernetes impersonation
- The server SHALL NOT use Kubernetes impersonation.

#### Scenario: No impersonate verbs used
- WHEN the server performs Kubernetes calls
- THEN the request is made without impersonation headers or RBAC `impersonate` usage

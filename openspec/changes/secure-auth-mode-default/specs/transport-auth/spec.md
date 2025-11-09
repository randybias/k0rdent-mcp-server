# Transport & Auth Configuration (delta)

## MODIFIED Requirements

### Requirement: Bearer auth at the transport

The server SHALL require explicit `AUTH_MODE` configuration and SHALL return **401** when an `Authorization: Bearer <token>` header is missing while `AUTH_MODE=OIDC_REQUIRED`. The server SHALL accept any `Authorization: Bearer <token>` when `AUTH_MODE=DEV_ALLOW_ANY` is explicitly configured (development only). The server SHALL fail to start if `AUTH_MODE` is not explicitly set.

#### Scenario: Reject missing bearer (prod)

- **WHEN** `AUTH_MODE=OIDC_REQUIRED` AND a client connects without the `Authorization` header
- **THEN** the server responds **401**

#### Scenario: Allow any bearer in dev mode

- **WHEN** `AUTH_MODE=DEV_ALLOW_ANY` is explicitly configured AND the request includes `Authorization: Bearer <any-token>`
- **THEN** the server accepts the request (200/OK) AND logs a debug message indicating insecure mode

#### Scenario: Startup requires explicit AUTH_MODE

- **WHEN** the server starts without `AUTH_MODE` environment variable set
- **THEN** the server fails to start with an error explaining that `AUTH_MODE` must be explicitly set to `DEV_ALLOW_ANY` or `OIDC_REQUIRED`

#### Scenario: Security warning for development mode

- **WHEN** `AUTH_MODE=DEV_ALLOW_ANY` is configured
- **THEN** the server emits a prominent security warning at startup: "SECURITY WARNING: AUTH_MODE=DEV_ALLOW_ANY allows any bearer token. This mode is INSECURE and must not be used in production."

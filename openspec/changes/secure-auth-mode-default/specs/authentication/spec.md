# Authentication (delta)

## MODIFIED Requirements

### Requirement: Auth mode configuration
- The server **SHALL** default to `OIDC_REQUIRED` auth mode when `AUTH_MODE` environment variable is unset
- The server **SHALL** require explicit `AUTH_MODE=DEV_ALLOW_ANY` to enable development mode
- The server **SHALL** fail startup with clear error message if auth requirements cannot be satisfied

#### Scenario: Unset AUTH_MODE defaults to OIDC_REQUIRED
- GIVEN `AUTH_MODE` environment variable is not set
- WHEN the server starts
- THEN auth mode is set to `OIDC_REQUIRED`
- AND server expects OIDC bearer tokens on all requests

#### Scenario: Explicit DEV_ALLOW_ANY enables development mode
- GIVEN `AUTH_MODE=DEV_ALLOW_ANY` is set
- WHEN the server starts
- THEN auth mode is set to `DEV_ALLOW_ANY`
- AND bearer tokens are optional

#### Scenario: Invalid auth mode fails startup
- GIVEN `AUTH_MODE=INVALID_MODE` is set
- WHEN the server attempts to start
- THEN startup fails with error message listing valid modes

## ADDED Requirements

### Requirement: Development mode warning
- The server **SHALL** log a prominent WARNING message on every startup when `DEV_ALLOW_ANY` mode is active
- The warning **SHALL** clearly indicate that authentication is disabled
- The warning **SHALL** be logged at WARN level to ensure visibility

#### Scenario: Warning logged on startup in dev mode
- GIVEN `AUTH_MODE=DEV_ALLOW_ANY` is set
- WHEN the server starts
- THEN a WARN level log message is emitted stating authentication is disabled

#### Scenario: No warning in production mode
- GIVEN `AUTH_MODE=OIDC_REQUIRED` is set
- WHEN the server starts
- THEN no warning about disabled authentication is logged

### Requirement: Auth mode visibility
- The configuration loading **SHALL** log the resolved auth mode at startup
- Logs **SHALL** include whether the mode came from explicit config or default

#### Scenario: Explicit config logged
- GIVEN `AUTH_MODE=OIDC_REQUIRED` is explicitly set
- WHEN configuration is loaded
- THEN log message indicates "auth_mode=OIDC_REQUIRED (from environment)"

#### Scenario: Default config logged
- GIVEN `AUTH_MODE` is unset
- WHEN configuration is loaded
- THEN log message indicates "auth_mode=OIDC_REQUIRED (default)"

### Requirement: Migration documentation
- The project **SHALL** provide MIGRATION.md documenting the breaking change
- Migration guide **SHALL** include version information and upgrade steps
- All code examples **SHALL** show explicit AUTH_MODE configuration

#### Scenario: Migration guide covers all deployment types
- GIVEN a user reads MIGRATION.md
- THEN migration steps are provided for: local dev, Docker, Kubernetes
- AND rollback instructions are included

#### Scenario: Examples show explicit configuration
- GIVEN a user reads README.md or deployment examples
- THEN every example includes explicit `AUTH_MODE` configuration
- AND comments explain the difference between dev and production modes

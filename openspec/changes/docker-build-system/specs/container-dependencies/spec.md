# Capability: Container Dependencies Tracking

## ADDED Requirements

### Requirement: Document container image dependencies
The project MUST maintain a centralized record of all dependencies required in the container image beyond compiled Go code.

#### Scenario: Track Go module dependencies for container build
**Given** a new feature adds a Go module dependency (e.g., `helm.sh/helm/v3`)
**When** the dependency is added to `go.mod`
**Then** the dependency is documented in the `docker-build-system` proposal
**And** includes the module path, minimum version, and reason for inclusion
**And** any runtime requirements are noted (network access, filesystem, etc.)

#### Scenario: Track system package dependencies
**Given** a feature requires a system package in the container image (e.g., `ca-certificates`)
**When** the package is needed for runtime functionality
**Then** the package is documented in the `docker-build-system` proposal
**And** the reason for the package is explained
**And** the Dockerfile is updated to install the package

#### Scenario: Track external CLI tool dependencies
**Given** a feature requires an external CLI tool (e.g., `kubectl`, `helm`)
**When** the tool must be present in the container
**Then** the tool is documented in the `docker-build-system` proposal
**And** the installation method is specified (package manager, binary download, etc.)
**And** the Dockerfile is updated to include the tool

#### Scenario: Track runtime network requirements
**Given** a feature requires network access to external services (e.g., OCI registries)
**When** the service is required for normal operation
**Then** the network requirement is documented in the `docker-build-system` proposal
**And** includes the hostname/URL and purpose
**And** deployment documentation reflects the network requirement

### Requirement: Cross-reference dependencies in feature proposals
Feature proposals that introduce new container dependencies MUST reference the `docker-build-system` proposal.

#### Scenario: Feature proposal adds Go module dependency
**Given** a proposal (e.g., `use-kgst-for-catalog-installs`) adds Helm SDK dependency
**When** writing the proposal's Impact section
**Then** the proposal includes `docker-build-system` in "Related proposals"
**And** the proposal documents the specific dependency being added
**And** the `docker-build-system` proposal is updated with the dependency details

#### Scenario: Validate dependency consistency
**Given** multiple proposals have documented dependencies in `docker-build-system`
**When** building the container image
**Then** the Dockerfile includes all documented dependencies
**And** no undocumented dependencies are present in the Dockerfile
**And** `go.mod` includes all documented Go module dependencies

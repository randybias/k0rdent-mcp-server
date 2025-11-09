# Spec: Project Documentation

## Overview

This specification defines requirements for comprehensive project documentation that enables users to quickly understand, install, configure, and contribute to the k0rdent MCP Server.

## ADDED Requirements

### Requirement: README structure and completeness

The project README SHALL provide a comprehensive introduction and getting-started guide that includes:
- Project overview explaining what the MCP server does and why it exists
- Feature list organized by capability area with links to detailed documentation
- Installation instructions covering multiple methods (binary, source, container, Kubernetes)
- Quick Start guide enabling first successful tool invocation within 10 minutes
- Configuration reference with examples
- Tools reference table with categories and descriptions
- Documentation roadmap organizing existing docs into a learning path
- Architecture overview explaining security model and component interactions
- License and contribution information

The README SHALL link to detailed documentation in docs/ rather than including lengthy content inline, keeping the README focused on overview and quick start (target: 300-400 lines).

#### Scenario: New user discovers the project

**Given** a developer visits the GitHub repository for the first time
**When** they read the README
**Then** they SHALL understand:
- What the k0rdent MCP Server is and what problems it solves
- How it fits into the k0rdent ecosystem
- What capabilities it provides (cluster management, monitoring, troubleshooting)
- How to install it on their system
- How to complete a basic "hello world" operation (list namespaces or deploy a cluster)
- Where to find detailed documentation for advanced features

**And** the README SHALL answer these questions without requiring external research:
- "What is MCP and why does k0rdent need an MCP server?"
- "How do I install this?"
- "What can I do with this server?"
- "How do I configure authentication?"
- "Where do I go for help?"

#### Scenario: User needs to install the server

**Given** a user wants to run the k0rdent MCP Server
**When** they follow the Installation section
**Then** the documentation SHALL provide step-by-step instructions for:
- Installing from released binaries (with download links)
- Building from source (with Go version requirements)
- Running with Docker (with example docker run command)
- Deploying to Kubernetes (with example manifests)

**And** each installation method SHALL include:
- Prerequisites check list
- Expected output/success indicators
- Common troubleshooting steps

#### Scenario: User wants to configure the server

**Given** a user has installed the server
**When** they need to configure it for their environment
**Then** the README SHALL link to configuration documentation that covers:
- config.yaml structure with annotated examples
- Environment variable reference
- Authentication modes (kubeconfig dev mode vs OIDC production)
- Namespace filtering configuration
- TLS/certificate setup
- Logging configuration

**And** configuration examples SHALL be valid YAML that can be copy-pasted.

#### Scenario: User wants to understand available tools

**Given** a user has the server running
**When** they want to see what tools are available
**Then** the README SHALL include a Tools Reference section that:
- Lists all tools organized by category (Cluster Management, Service Deployment, Monitoring, Troubleshooting, Catalog)
- Provides one-line description for each tool
- Links to detailed documentation for complex tools
- Shows example invocations for common operations

**And** the tools SHALL be grouped logically:
- **Cluster Management**: list, deploy, delete, get status
- **Service Deployment**: apply services to running clusters
- **Monitoring**: subscribe to cluster provisioning progress
- **Troubleshooting**: stream events, tail pod logs
- **Catalog**: browse and install catalog entries

---

### Requirement: Contributing guidelines

The project SHALL provide a CONTRIBUTING.md file that enables developers to confidently set up a development environment, make changes, and submit contributions following project standards.

The CONTRIBUTING.md SHALL include:
- Welcome message and code of conduct reference
- Development environment setup (prerequisites, clone, build, run)
- Project structure explanation (directory layout, key packages)
- Change workflow (OpenSpec for features, direct PRs for bugs/docs)
- Testing requirements (unit tests, integration tests, coverage)
- Code style guidelines
- Commit message format (Conventional Commits)
- PR submission process with checklist
- OpenSpec workflow detailed guide

#### Scenario: New contributor wants to fix a bug

**Given** a developer wants to contribute a bug fix
**When** they read CONTRIBUTING.md
**Then** they SHALL be able to:
- Set up a working development environment
- Build and run the server locally
- Run existing tests to verify their environment works
- Make code changes following project style
- Write tests for their bug fix
- Submit a PR that passes CI checks

**And** the guide SHALL specify:
- Which Go version to use
- How to configure access to a test Kubernetes cluster
- How to run tests (`make test`, `go test ./...`)
- Where to put test files
- Commit message format expectations

#### Scenario: Contributor wants to propose a new feature

**Given** a developer wants to add a significant new feature
**When** they consult CONTRIBUTING.md
**Then** they SHALL understand:
- That new features require an OpenSpec proposal first
- How to create an OpenSpec proposal (`openspec` commands)
- What a proposal should include (problem, solution, design, tasks)
- How to get proposal feedback/approval
- When to start implementation

**And** the OpenSpec workflow section SHALL:
- Explain the philosophy (design before code)
- Show step-by-step proposal creation
- Link to openspec/AGENTS.md for detailed conventions
- Provide example proposals to reference

#### Scenario: Contributor needs to run integration tests

**Given** a developer has made changes affecting cluster operations
**When** they want to validate with integration tests
**Then** CONTRIBUTING.md SHALL explain:
- How to set up a test environment (kind/minikube)
- How to populate test fixtures (credentials, templates)
- How to run integration tests
- How to debug test failures
- Where integration test playbooks are documented

**And** SHALL provide:
- Commands to create test cluster
- Scripts to apply test resources
- Expected test duration
- How to clean up test resources

---

### Requirement: Documentation organization and discoverability

The project documentation SHALL be organized into a clear hierarchy that guides users from basic to advanced topics, with cross-references and a documented learning path.

The README SHALL include a "Documentation" section that maps existing docs/ files into categories:

**Getting Started**:
- Installation and configuration basics
- First tool invocations
- Quick wins

**Core Features**:
- Cluster provisioning workflows
- Service attachment to running clusters
- Catalog browsing and installation

**Advanced Topics**:
- Provider-specific deployment tools (Azure, AWS, GCP)
- Cluster deployment monitoring and subscriptions
- Event streaming and pod log tailing

**Reference**:
- Complete tools API reference
- OpenSpec proposals and specifications
- Architecture and security model

#### Scenario: User wants to learn cluster provisioning

**Given** a user wants to deploy Kubernetes clusters via the MCP server
**When** they navigate the documentation
**Then** they SHALL find a clear path:
1. README → "Cluster Management" feature overview
2. Link to docs/cluster-provisioning.md for detailed guide
3. Provider-specific details in docs/provider-specific-deployment.md
4. Monitoring progress via docs/features/cluster-monitoring.md

**And** each doc SHALL:
- State its scope and audience in the opening paragraph
- Link to prerequisites (e.g., credentials setup)
- Link to related docs (e.g., provider-specific tools)
- Include working examples

#### Scenario: Maintainer adds new feature documentation

**Given** a new feature has been implemented
**When** a maintainer writes documentation for it
**Then** the documentation structure SHALL make it obvious:
- Where to add the new doc file (docs/features/ or docs/)
- How to link it from README
- What template/structure to follow
- How to cross-reference from related docs

**And** the CONTRIBUTING.md SHALL specify:
- Documentation is required for new features
- Docs should be updated in the same PR as code
- OpenSpec proposals should link to planned doc structure

---

### Requirement: Architecture and security documentation

The project SHALL document its architecture, security model, and operational characteristics to build confidence for enterprise users and contributors.

Documentation SHALL cover:
- Component architecture (server, runtime, Kubernetes client, MCP transport)
- Security model (RBAC, no impersonation, OIDC authentication)
- Stateless operation and leader election for watchers
- Transport mechanism (Streamable HTTP)
- Namespace filtering and multi-tenancy

#### Scenario: Security team evaluates the server

**Given** an organization's security team evaluates the k0rdent MCP Server
**When** they review the architecture documentation
**Then** they SHALL find answers to:
- "Does this server impersonate users?" → No, explicit no-impersonation policy
- "How does authentication work?" → Kubeconfig (dev) or OIDC bearer token (prod)
- "What Kubernetes permissions does it need?" → RBAC requirements listed
- "Can it access resources outside configured namespaces?" → Namespace filtering enforced
- "How are secrets handled?" → Never logged, stored securely in Kubernetes

**And** the documentation SHALL:
- Include a security architecture diagram
- List minimum RBAC permissions required
- Explain the trust model (server trusts Kubernetes RBAC)
- Link to Kubernetes security best practices applied

#### Scenario: Operator wants to deploy in production

**Given** an operator wants to run the server in a production cluster
**When** they review deployment documentation
**Then** they SHALL understand:
- Whether the server needs to run in the management cluster (yes)
- How many replicas to run (stateless, multiple replicas supported)
- How leader election works for watch publishing
- What resource requirements are typical (CPU, memory)
- How to configure high availability
- Health check endpoints for probes

**And** SHALL find example manifests for:
- Deployment with multiple replicas
- Service definition
- RBAC ClusterRole and ClusterRoleBinding
- ConfigMap for configuration
- Liveness and readiness probes

---

### Requirement: Configuration examples and validation

Documentation SHALL provide complete, working configuration examples that users can copy and adapt, with explanations of each setting.

#### Scenario: User needs to configure OIDC authentication

**Given** a user wants to enable production OIDC authentication
**When** they consult the configuration documentation
**Then** they SHALL find:
- Complete config.yaml example with OIDC settings
- Explanation of each OIDC parameter (issuer, client ID, etc.)
- How the server validates bearer tokens
- How to test authentication is working
- Common configuration mistakes and how to avoid them

**And** the example SHALL be copy-pasteable valid YAML:
```yaml
auth:
  mode: oidc-required  # reject requests without bearer token
  oidc:
    issuer: https://dex.example.com
    audience: k0rdent-mcp-server
    # ... full working example
```

#### Scenario: User needs to restrict namespace access

**Given** a user wants to limit which namespaces the server can access
**When** they configure namespace filtering
**Then** the documentation SHALL explain:
- How to set allowed namespaces in config.yaml
- Whether this is enforced (yes, at runtime level)
- How it interacts with Kubernetes RBAC
- What happens when a tool tries to access a forbidden namespace (error returned)

**And** SHALL provide example configuration:
```yaml
namespaces:
  allowed:
    - production-*  # glob patterns supported
    - staging-*
    - kcm-system
```

---

### Requirement: Tools reference completeness

Documentation SHALL provide a complete reference of all available MCP tools, organized by category, with descriptions and example invocations.

#### Scenario: User wants to see all available tools

**Given** a user connects an MCP client to the server
**When** they want to understand what operations are possible
**Then** the README SHALL include a Tools Reference section listing:

**Cluster Management Tools**:
- `k0rdent.mgmt.clusterDeployments.list` - List all cluster deployments
- `k0rdent.mgmt.clusterDeployments.get` - Get cluster deployment details
- `k0rdent.mgmt.clusterTemplates.list` - List available cluster templates
- `k0rdent.mgmt.providers.listCredentials` - List provider credentials
- `k0rdent.provider.azure.clusterDeployments.deploy` - Deploy Azure cluster
- `k0rdent.provider.aws.clusterDeployments.deploy` - Deploy AWS cluster
- `k0rdent.provider.gcp.clusterDeployments.deploy` - Deploy GCP cluster
- `k0rdent.mgmt.clusterDeployments.delete` - Delete a cluster deployment

**Service Deployment Tools**:
- `k0rdent.mgmt.clusterDeployments.services.apply` - Apply services to cluster
- `k0rdent.mgmt.serviceTemplates.list` - List available service templates
- `k0rdent.catalog.serviceTemplates.list` - Browse catalog service templates
- `k0rdent.mgmt.serviceTemplates.install` - Install service template from catalog

**Monitoring & Troubleshooting Tools**:
- `k0rdent.mgmt.events.list` - List Kubernetes events
- `k0rdent.mgmt.podLogs.get` - Get pod logs
- `k0rdent.mgmt.namespaces.list` - List accessible namespaces

**Subscriptions**:
- `k0rdent://cluster-monitor/{namespace}/{name}` - Monitor cluster provisioning
- `k0rdent://events/{namespace}` - Stream namespace events
- `k0rdent://podlogs/{namespace}/{pod}/{container}` - Tail pod logs

**And** each tool SHALL link to detailed documentation when the tool requires complex parameters.

#### Scenario: User wants detailed help for a specific tool

**Given** a user wants to deploy an Azure cluster
**When** they click the link for `k0rdent.provider.azure.clusterDeployments.deploy`
**Then** they SHALL find documentation covering:
- Purpose and use cases
- Required parameters (name, credential, location, etc.)
- Optional parameters with defaults
- Example invocation with real values
- Expected output/success indicators
- Common errors and troubleshooting
- Related tools (list credentials, wait for ready, etc.)

**And** the documentation SHALL include:
- Link to Azure-specific deployment guide
- Link to credential setup instructions
- Link to cluster template documentation

---

## Implementation Notes

This is a documentation-only change requiring no code modifications.

**Files Created**:
- `CONTRIBUTING.md` (new, ~250-300 lines)
- `docs/ARCHITECTURE.md` (new, ~150 lines)
- `docs/CONFIGURATION.md` (new, ~200 lines)

**Files Modified**:
- `README.md` (complete rewrite, ~300-400 lines)
- Existing docs/ files (add frontmatter, update cross-references)

**Visual Assets**:
- ASCII art architecture diagram for README
- Detailed architecture diagram (PNG) for docs/ARCHITECTURE.md

**Validation**:
- All internal links resolve correctly
- All example commands/configurations are valid
- Documentation organization matches described learning path
- CONTRIBUTING.md setup steps produce working environment

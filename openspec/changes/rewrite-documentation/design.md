# Design: Project Documentation Rewrite

## Overview

This design document explains the rationale, structure, and organization principles for the comprehensive documentation rewrite. The goal is to create a documentation system that serves multiple audiences (new users, operators, contributors) while remaining maintainable as the project evolves.

## Documentation Architecture

### Multi-Tier Documentation Model

The documentation follows a three-tier model optimized for progressive disclosure:

```
┌─────────────────────────────────────────────────────────────┐
│                         README.md                            │
│  (Overview, Quick Start, Tool Categories, Links)             │
│                    Target: 300-400 lines                     │
└────────────┬────────────────────────────────────────────────┘
             │
    ┌────────┴────────┐
    │                 │
┌───▼────────┐    ┌──▼──────────┐
│ User Docs  │    │ Contributor │
│ (docs/)    │    │ Guide       │
│            │    │ (CONTRIB.md)│
└───┬────────┘    └─────────────┘
    │
    ├─── Getting Started
    │    ├─ CONFIGURATION.md (detailed config reference)
    │    ├─ AUTHENTICATION.md (dev & prod setup)
    │    └─ DEPLOYMENT.md (K8s manifests & operations)
    │
    ├─── Core Features
    │    ├─ cluster-provisioning.md (workflows)
    │    ├─ provider-specific-deployment.md (Azure/AWS/GCP)
    │    └─ features/cluster-monitoring.md (subscriptions)
    │
    ├─── Reference
    │    ├─ TOOLS_REFERENCE.md (complete API)
    │    ├─ ARCHITECTURE.md (internals)
    │    └─ catalog.md (catalog operations)
    │
    └─── Operations
         ├─ live-tests.md (testing playbooks)
         └─ release-notes.md (changelog)
```

### Tier 1: README.md (Entry Point)

**Purpose**: Answer "what is this" and "how do I get started" in <5 minutes.

**Content Strategy**:
- **Brief but complete**: Enough context to understand the project without overwhelming
- **Action-oriented**: Quick Start gets to first success ASAP
- **Hub model**: Links to detailed docs rather than inlining content
- **Visual**: ASCII diagram shows architecture at a glance
- **Scannable**: Tables, lists, clear sections

**Target Audience**:
- GitHub visitors evaluating the project
- New users wanting to try it quickly
- Evaluators checking capabilities and security model

**Length Target**: 300-400 lines (roughly 2-3 screens)

### Tier 2: User & Contributor Guides

**Purpose**: Provide detailed how-to guides for specific tasks.

**Content Strategy**:
- **Task-focused**: Organized around user goals
- **Step-by-step**: Clear procedures with expected outputs
- **Example-rich**: Working code/config samples
- **Troubleshooting**: Common issues and solutions
- **Cross-referenced**: Links to related docs

**Key Documents**:

1. **CONTRIBUTING.md**: Enables contributors to set up environment, understand workflow, submit changes
   - Standalone file (GitHub convention)
   - Complete setup guide
   - OpenSpec workflow detailed
   - Testing and PR requirements

2. **docs/CONFIGURATION.md**: Exhaustive configuration reference
   - All config options documented
   - Annotated examples
   - Environment variables
   - Authentication modes
   - Namespace filtering

3. **docs/DEPLOYMENT.md**: Production deployment guide
   - Kubernetes manifests
   - RBAC requirements
   - Resource sizing
   - High availability setup
   - Monitoring integration

4. **docs/AUTHENTICATION.md**: Auth setup for both dev and production
   - Kubeconfig mode (dev)
   - OIDC mode (production)
   - Provider setup examples
   - Token validation flow
   - Security best practices

### Tier 3: Reference & Internals

**Purpose**: Provide comprehensive reference and architectural understanding.

**Content Strategy**:
- **Complete coverage**: Every tool, every option documented
- **Explanatory**: Why things work the way they do
- **Architectural**: Internal design decisions
- **Searchable**: Clear structure for quick lookup

**Key Documents**:

1. **docs/TOOLS_REFERENCE.md**: Complete MCP tools API reference
   - Every tool documented
   - Parameters and types
   - Example invocations
   - Related operations

2. **docs/ARCHITECTURE.md**: System internals
   - Component diagram
   - Data flow
   - Security model
   - Concurrency model
   - Extension points

## Documentation Organization Principles

### Principle 1: Progressive Disclosure

**Rationale**: Users shouldn't need to read 100 pages to get started.

**Implementation**:
- README provides overview + quick start
- Links to detailed docs only when needed
- Each doc states its scope upfront
- Clear learning paths from basic to advanced

**Example**:
```
README → "Deploy Azure cluster" feature
  ↓
docs/cluster-provisioning.md → Basic workflow
  ↓
docs/provider-specific-deployment.md → Azure-specific parameters
  ↓
docs/TOOLS_REFERENCE.md → Complete API details
```

### Principle 2: Audience Segmentation

**Rationale**: Different users have different needs and prior knowledge.

**Audiences**:

| Audience | Needs | Entry Point |
|----------|-------|-------------|
| Evaluator | What is this? Is it secure? | README → ARCHITECTURE |
| New User | How do I install and use it? | README → Quick Start → cluster-provisioning.md |
| Operator | How do I deploy to production? | DEPLOYMENT.md → CONFIGURATION.md |
| Contributor | How do I add a feature? | CONTRIBUTING.md → OpenSpec workflow |
| Power User | Complete API reference | TOOLS_REFERENCE.md |

**Implementation**:
- README mentions all audiences and links to their docs
- Each doc identifies its target audience
- Different entry points for different goals

### Principle 3: Maintainability

**Rationale**: Documentation that isn't maintained becomes misleading and loses trust.

**Strategies**:

1. **Single Source of Truth**: Each concept documented in one place, referenced from others
   - Config schema → CONFIGURATION.md (not scattered)
   - Tools API → TOOLS_REFERENCE.md (not duplicated)
   - Architecture → ARCHITECTURE.md (not repeated)

2. **Example Validation**: Examples come from tested code
   - Config examples are valid YAML (linted)
   - Tool invocations come from test suites
   - Manifests tested in real clusters

3. **Update Process**: Documentation updates part of feature workflow
   - OpenSpec proposals include doc plan
   - PRs that add tools must update TOOLS_REFERENCE
   - CI checks for broken links

4. **Metadata**: Track freshness
   - "Last updated" dates on key docs
   - Version numbers where relevant
   - Deprecation notices clearly marked

### Principle 4: Discoverability

**Rationale**: Users can't use features they don't know exist.

**Strategies**:

1. **Centralized Index**: README lists all major features
2. **Categorization**: Tools grouped by purpose (not alphabetically)
3. **Cross-References**: Related docs link to each other
4. **Search-Friendly**: Clear headings, keywords, examples

**Example - Tool Discovery**:
```markdown
## Tools Reference

### Cluster Management
- `k0rdent.mgmt.clusterDeployments.list` - List all clusters
- `k0rdent.provider.azure.clusterDeployments.deploy` - Deploy Azure cluster
  - See: docs/provider-specific-deployment.md for Azure details
  - See: docs/cluster-provisioning.md for general workflow
```

### Principle 5: Actionability

**Rationale**: Documentation should enable action, not just describe.

**Implementation**:

1. **Working Examples**: Every example can be copy-pasted and run
2. **Expected Output**: Show what success looks like
3. **Troubleshooting**: Common errors and solutions included
4. **Prerequisites**: What you need before starting
5. **Validation Steps**: How to verify it worked

**Example Structure**:
```markdown
## Deploying an Azure Cluster

**Prerequisites**:
- Azure credential configured
- RBAC permissions on kcm-system namespace

**Steps**:
1. List available Azure credentials:
   ```
   k0rdent.mgmt.providers.listCredentials --provider azure
   ```
   Expected output: [...]

2. Deploy cluster:
   ```
   k0rdent.provider.azure.clusterDeployments.deploy \
     --name my-cluster \
     --credential azure-cred-001 \
     [...]
   ```

**Success Indicators**:
- Tool returns deployment ID
- Cluster appears in `clusterDeployments.list`

**Troubleshooting**:
- Error "credential not found" → Check credential name and namespace access
- [...]
```

## Content Templates

### Tool Documentation Template

```markdown
### Tool: k0rdent.category.resource.operation

**Purpose**: One-sentence description of what this tool does.

**Category**: Cluster Management | Service Deployment | Monitoring | Troubleshooting | Catalog

**Parameters**:
- `param1` (string, required): Description
- `param2` (string, optional, default: "value"): Description

**Example**:
<example invocation>

**Returns**:
<output structure>

**Related**:
- [Related Tool](#related-tool)
- [Guide](path/to/guide.md)

**Common Errors**:
- Error message → Solution
```

### Documentation File Template

```markdown
# Title: Clear, Action-Oriented

**Last Updated**: YYYY-MM-DD
**Audience**: Who should read this (e.g., "Operators deploying to production")
**Prerequisites**: What reader needs before starting

## Overview
Brief description of what this document covers and why it matters.

## [Section Title]
Content organized by task or concept.

## Examples
Working examples with explanations.

## Troubleshooting
Common issues and solutions.

## Related Documentation
- [Doc 1](path)
- [Doc 2](path)
```

## Visual Design

### ASCII Diagrams (README)

Use simple ASCII art for README to ensure it renders everywhere:

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│ MCP Client  │─────▶│ k0rdent MCP  │─────▶│ Kubernetes  │
│ (Claude)    │      │ Server       │      │ API Server  │
└─────────────┘      └──────────────┘      └─────────────┘
                            │
                            ▼
                     ┌─────────────┐
                     │ ClusterDep- │
                     │ loyments    │
                     │ Templates   │
                     │ Credentials │
                     └─────────────┘
```

### Detailed Diagrams (docs/)

Use tool-generated PNG diagrams for docs/ARCHITECTURE.md:
- Component architecture
- Request flow
- Authentication flow
- Leader election for watches

Tools: Mermaid, PlantUML, or similar (source committed, PNG rendered)

## Documentation Coverage Checklist

### User Journey Coverage

- [ ] **Discovery**: "What is this project?" → README overview
- [ ] **Evaluation**: "Should we use this?" → README features + ARCHITECTURE security
- [ ] **Installation**: "How do I install it?" → README installation + Quick Start
- [ ] **Configuration**: "How do I configure it?" → CONFIGURATION.md
- [ ] **First Use**: "How do I deploy a cluster?" → cluster-provisioning.md
- [ ] **Advanced Use**: "Azure-specific settings?" → provider-specific-deployment.md
- [ ] **Monitoring**: "How do I watch provisioning?" → features/cluster-monitoring.md
- [ ] **Troubleshooting**: "It's not working" → Each doc has troubleshooting section
- [ ] **Contributing**: "I want to help" → CONTRIBUTING.md

### Tool Coverage

- [ ] All MCP tools listed in README
- [ ] All tools documented in TOOLS_REFERENCE
- [ ] Provider-specific tools documented in provider-specific-deployment.md
- [ ] Subscription URIs documented with parameters
- [ ] Example invocations for common operations

### Operational Coverage

- [ ] Kubernetes deployment manifests
- [ ] RBAC requirements documented
- [ ] Resource requirements documented
- [ ] High availability setup documented
- [ ] Monitoring and observability guidance
- [ ] Backup/disaster recovery (if applicable)
- [ ] Upgrade procedures (when relevant)

### Contributor Coverage

- [ ] Development environment setup
- [ ] Build commands
- [ ] Test commands
- [ ] Code style guidelines
- [ ] OpenSpec workflow
- [ ] PR process
- [ ] Review checklist
- [ ] Debugging tips

## Migration Strategy

### Phase 1: Create New Structure
- Write new README alongside old
- Create new docs in docs/
- Don't delete old content yet

### Phase 2: Validation
- Have 2-3 people test using only new docs
- Identify gaps and confusing sections
- Fix issues

### Phase 3: Cutover
- Replace old README with new
- Archive old docs (if any)
- Update all internal links
- Announce in community channels

### Phase 4: Maintenance
- Monitor questions/issues for doc gaps
- Add "Was this helpful?" feedback mechanism
- Update as features evolve

## Success Metrics

**Quantitative**:
- New user time-to-first-success < 10 minutes
- Contributor setup success rate > 90%
- Documentation link click-through (vs search engine)
- Support questions about "how to" (should decrease)

**Qualitative**:
- User feedback: "docs were clear and helpful"
- Contributor feedback: "easy to get started"
- Maintainer feedback: "docs stay up-to-date"

## Open Design Questions

### Q1: Single TOOLS_REFERENCE vs Per-Category Files?

**Option A**: One big docs/TOOLS_REFERENCE.md
- Pros: Single search, complete reference
- Cons: Large file, harder to navigate

**Option B**: Separate files per category (docs/tools/cluster-management.md, etc.)
- Pros: Smaller files, easier to update
- Cons: Need index, more files to maintain

**Recommendation**: Start with Option A (single file with good TOC), split if it grows too large (>1000 lines).

### Q2: Authentication Examples: Which OIDC Providers?

**Options**:
- Generic OIDC setup (issuer, client ID, etc.)
- Dex example (popular in k8s)
- Keycloak example (enterprise common)
- Auth0/Okta examples (cloud providers)

**Recommendation**:
- AUTHENTICATION.md: Generic OIDC requirements + Dex example (most relevant for k8s users)
- Note that other providers work, link to provider docs
- Community can contribute additional provider examples

### Q3: Example Cluster Manifests: How Complete?

**Options**:
- Minimal (Deployment + Service only)
- Basic (+ RBAC + ConfigMap)
- Production-ready (+ ingress, monitoring, autoscaling)

**Recommendation**:
- DEPLOYMENT.md: Basic manifests (Deployment, Service, RBAC, ConfigMap)
- Note: Production deployments need monitoring, ingress, etc.
- Link to example production setups in community repos (when available)

## Dependencies & Requirements

**No Code Changes**: This is documentation-only, no code modifications required.

**Assets Needed**:
- Architecture diagrams (can be ASCII art initially)
- Example Kubernetes manifests (created from scratch)
- Screenshot of Claude Desktop integration (optional, nice-to-have)

**Validation Requirements**:
- All examples must be tested (run commands, apply manifests)
- All links must resolve (CI check)
- Configuration examples must be valid YAML (linter)
- Markdown formatting must be consistent (linter)

**Review Requirements**:
- Technical review: Ensure accuracy
- User review: Is it understandable?
- Contributor review: Can setup be followed?

# Proposal: Rewrite Project Documentation

## Problem

The current README.md doesn't accurately reflect the project's **early development state** and sets incorrect expectations. Key issues:

1. **Oversells Stability**: Doesn't make clear this is an **experimental development tool** with significant limitations and untested code paths.

2. **Missing Critical Warnings**: Doesn't warn about:
   - GCP deployments untested (may not work)
   - Azure requires subscription ID parameter
   - AWS minimally tested
   - Catalog sync may have issues
   - OIDC/non-admin not supported (admin kubeconfig required)
   - No TLS support (localhost only)
   - AI-assisted code with potential quality/security issues
   - Assumes existing k0rdent cluster (doesn't provision one)

3. **Unclear Target Audience**: Doesn't state this is for **k0rdent developers/early adopters** with deep k0rdent knowledge, not end users.

4. **No Contributing Guidelines**: No CONTRIBUTING.md explaining development workflow, testing, or OpenSpec process.

## Proposed Solution

Rewrite documentation to be **brutally honest** about current limitations while providing a clear path for experimentation and contribution.

### README.md Structure (Honest & Realistic)

**1. Header with Clear Status**
```
# k0rdent MCP Server

⚠️ **Experimental Development Tool** - Early stage, expect issues
🚧 **Localhost Only** - No TLS, admin kubeconfig required
🤖 **AI-Assisted** - Code quality and security not production-ready
```

**2. What This Is**
- Experimental MCP server for k0rdent cluster management
- For k0rdent developers and early adopters
- Runs on localhost with admin kubeconfig to existing k0rdent management cluster
- Built to explore MCP integration, not for production use

**3. Prerequisites (All Required)**
- **Existing k0rdent management cluster** (does NOT provision one for you)
- **Admin kubeconfig** to that cluster (RBAC limitations not tested)
- **Localhost deployment** (no TLS, not accessible externally)
- Go 1.24+ (to build from source)
- Claude Desktop or MCP-compatible client
- **Understanding of k0rdent** (ClusterDeployments, ServiceTemplates, etc.)

**4. Known Limitations & Issues**
- **GCP**: Not tested, may not work
- **Azure**: Requires subscription ID parameter (not auto-detected)
- **AWS**: Minimally tested, expect issues
- **Authentication**: Only admin kubeconfig (no OIDC, no RBAC enforcement)
- **Security**: AI-assisted code, not security-reviewed, localhost only
- **Catalog**: Synchronization may have bugs
- **TLS**: Not supported, runs on 127.0.0.1 only
- **Stability**: Experimental, expect crashes and bugs

**5. Quick Start (Experimental)**
- Clone repository
- Build: `go build -o server cmd/server/main.go`
- Create config.yaml pointing to your k0rdent cluster kubeconfig
- Run: `./server start`
- Connect Claude Desktop to `http://localhost:3000/mcp`
- Try: List namespaces (safest operation)
- **Warning**: Cluster deployments will create real cloud resources (costs apply)

**6. What Works (Tested Minimally)**
- **Azure Cluster Deployment**: Works if you provide subscription ID
- **Cluster Monitoring**: Subscribe to provisioning progress
- **Namespace Listing**: Basic K8s operations
- **Event Streaming**: Watch namespace events
- **Pod Logs**: Tail container logs

**7. What's Untested or Broken**
- GCP cluster deployments (not tested)
- AWS cluster deployments (minimally tested)
- Catalog operations (may have bugs)
- Service attachments to running clusters (needs testing)
- Non-admin access (RBAC filtering not implemented)
- Concurrent operations (race conditions possible)
- Error recovery (may leave orphaned resources)

**8. Configuration (Minimal)**
```yaml
# config.yaml - Minimal example
server:
  port: 3000

kube:
  kubeconfig: /path/to/admin-kubeconfig
  # Must be admin-level access

# No TLS configuration supported
# No auth modes other than kubeconfig
```

**9. Tools Overview**
- **Cluster Management**: list, deploy (Azure works, AWS minimal, GCP untested), delete
- **Monitoring**: Subscribe to cluster provisioning (tested on Azure)
- **Troubleshooting**: Events, pod logs
- **Catalog**: List/install (may have bugs)

Link to docs/ for what documentation exists.

**10. Documentation**
- This README (current limitations)
- docs/cluster-provisioning.md (Azure tested, others not)
- docs/provider-specific-deployment.md (Azure focus)
- docs/features/cluster-monitoring.md (tested on Azure)
- CONTRIBUTING.md (for developers)

**11. Contributing**
This is an experimental project built with AI assistance. Code quality and security need improvement. Contributions welcome, especially:
- Testing GCP and AWS deployment paths
- Fixing catalog synchronization bugs
- Improving error handling
- Adding proper RBAC support
- Security review and hardening
- Removing AI-generated code smells

See CONTRIBUTING.md for OpenSpec workflow.

**12. Security & Disclaimer**
- **Not production-ready**
- **AI-assisted code** - may contain security vulnerabilities
- **Admin access required** - no RBAC enforcement
- **Localhost only** - no TLS, not network-accessible
- **Creates real cloud resources** - costs apply, may leave orphans
- **No warranty** - experimental software, use at own risk

**13. Roadmap (Maybe)**
- Fix GCP deployment path
- Test and stabilize AWS deployments
- Fix catalog synchronization
- Add RBAC support (non-admin access)
- Add TLS support
- Security review and hardening
- Production deployment options

See `openspec list` for proposed changes.

### CONTRIBUTING.md Structure (Realistic)

**1. Welcome & Expectations**
- Experimental project, expect rough edges
- Built with AI assistance, code quality needs improvement
- Focus on testing and stabilization

**2. Prerequisites**
- Go 1.24+
- k0rdent management cluster (or ability to create one)
- Admin kubeconfig
- Understanding of k0rdent, Kubernetes, MCP protocol

**3. Development Setup**
- Clone and build
- Point to test cluster (NOT production)
- Run tests: `go test ./...` (may have gaps)
- Run server: `./server start`

**4. What Needs Work**
- **GCP deployment path** (completely untested)
- **AWS deployment** (minimally tested, needs validation)
- **Catalog sync** (has bugs)
- **Error handling** (crashes instead of graceful failures)
- **RBAC enforcement** (assumes admin, doesn't filter)
- **Security** (AI-generated code needs review)
- **Tests** (incomplete coverage)

**5. Making Changes**
- **New features**: OpenSpec proposal required
  - Explain why, what, how
  - Get approval before coding
  - See `openspec list` for examples
- **Bug fixes**: Direct PR with test
- **Testing improvements**: Always welcome
- **Documentation**: Update as you learn

**6. OpenSpec Workflow**
- Run `openspec list` to see existing proposals
- Create proposal: `openspec/changes/<change-id>/`
- Write proposal.md, specs/, tasks.md
- Validate: `openspec validate <change-id> --strict`
- Get feedback, then implement
- See openspec/AGENTS.md for full details

**7. Testing**
- Unit tests: `go test ./...`
- Integration tests: `go test -tags=integration ./...` (requires cluster)
- **Warning**: Integration tests create real cloud resources
- Test against non-production cluster only

**8. Code Quality**
- AI-assisted code needs human review
- Follow Go conventions
- Run `gofmt` before committing
- Fix linter warnings
- Add tests for new code

**9. Known Issues to Watch For**
- Race conditions (concurrent operations)
- Resource leaks (orphaned cloud resources)
- Error paths that panic
- Missing RBAC checks
- Hardcoded assumptions about cluster state

**10. PR Process**
- Fork and branch
- Commit messages: "type(scope): description"
- Ensure tests pass
- Update docs if behavior changes
- PR template checklist
- Expect review feedback

**11. Resources**
- MCP protocol: https://modelcontextprotocol.io
- k0rdent: https://docs.k0rdent.io
- OpenSpec: openspec/AGENTS.md in this repo

### Additional Files

**docs/DEVELOPMENT.md** (Keep It Simple)
- Setting up test k0rdent cluster (kind + k0rdent install)
- Getting admin kubeconfig
- Common dev tasks (build, test, run)
- Troubleshooting (connection issues, RBAC errors)
- Testing Azure deployment (safest path)
- Warning about costs for cloud resources

## Benefits

**Honesty Builds Trust**:
- Sets accurate expectations (experimental, not production)
- Warns about real issues (GCP untested, AI code, security concerns)
- Reduces support burden (people know what to expect)
- Attracts right audience (developers willing to test and contribute)

**Enables Contribution**:
- Clear about what needs work
- OpenSpec workflow prevents duplicate effort
- Testing gaps are explicit
- Code quality issues acknowledged

**Reduces Liability**:
- Explicit warnings about cloud resource costs
- Security disclaimers about AI-generated code
- No false promises about production-readiness
- Clear about localhost-only security model

## Implementation Notes

**Files to Create**:
- `CONTRIBUTING.md` (~150-200 lines with warnings)
- `docs/DEVELOPMENT.md` (~100 lines, test cluster setup)

**Files to Modify**:
- `README.md` (rewrite, ~250-300 lines with extensive warnings section)
- Update docs/ cross-references

**Tone**: Honest but not discouraging. "This is experimental, here's what works, here's what doesn't, contributions welcome."

## Success Criteria

1. **Honest**: No feature claims we can't back up
2. **Safe**: Clear warnings about costs, security, limitations
3. **Actionable**: Developer can still get started despite warnings
4. **Inviting**: Contributors understand what needs work
5. **Clear**: No ambiguity about experimental status

## Open Questions

1. **Should we add automated warnings in the code itself?**
   - e.g., "GCP deployment is untested, proceed at your own risk? (y/N)"
   - **Recommendation**: Yes, for untested providers and destructive operations

2. **Should we gate experimental features behind a flag?**
   - e.g., `--enable-experimental-gcp` flag required for GCP
   - **Recommendation**: Yes for untested paths, document in README

3. **Should we recommend insurance/cost alerts for cloud deployments?**
   - **Recommendation**: Yes, add note about setting up cloud provider cost alerts

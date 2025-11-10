# Change: Docker build system dependencies

## Why
- As the MCP server evolves, new Go dependencies are added (e.g., Helm v3 SDK for kgst integration) that must be included in the container image
- Currently, there's no centralized tracking of runtime dependencies required by the built Go binary beyond what `go.mod` captures for compilation
- Future iterations may require additional system dependencies, CLI tools, or libraries in the container image that aren't Go modules
- A placeholder proposal ensures dependency requirements are documented and tracked as features are added

## What Changes
- **This is a placeholder proposal** to track container build dependencies across feature implementations
- Other proposals (like `use-kgst-for-catalog-installs`) will reference this proposal when they introduce new dependencies
- Document dependencies required in the container image:
  - **Go module dependencies** from `go.mod` (e.g., `helm.sh/helm/v3`)
  - **System libraries** if needed (e.g., CA certificates, timezone data)
  - **CLI tools** if binary invocation is required (future: helm CLI if we shell out)
  - **Runtime requirements** (e.g., network access to OCI registries)

## Impact
- **Affected specs**: No new capability specs; this is a tracking/documentation proposal
- **Affected code**:
  - `Dockerfile` - Multi-stage build with appropriate base images and dependencies
  - `go.mod` - Go module dependencies (tracked by Go tooling)
  - `docs/` or `README.md` - Documentation of runtime requirements
- **Breaking**: No; purely additive documentation and tracking

## Out of Scope
- Implementing specific build optimizations (caching, layers, image size reduction)
- Container registry configuration or image distribution
- Kubernetes deployment manifests (helm charts, kustomize, etc.)
- CI/CD pipeline changes beyond build step requirements

## Acceptance
- `openspec validate docker-build-system --strict` passes
- Each feature proposal that adds container dependencies references this proposal
- Dockerfile includes all documented dependencies
- Documentation clearly lists runtime requirements (network, volumes, etc.)

## Dependencies Tracker

### From: use-kgst-for-catalog-installs
**Added Go modules:**
- `helm.sh/helm/v3` v3.14.0 or later - Helm SDK for kgst chart installation

**Runtime requirements:**
- Network access to `ghcr.io` (OCI registry for kgst chart)
- Network access to Helm chart repositories during verify-job execution

**No additional system packages required** - Helm SDK is pure Go, compiles into binary

---

### Template for Future Dependencies

```markdown
### From: <proposal-id>
**Added Go modules:**
- <module-path> <version> - <reason>

**Added system packages (if any):**
- <package-name> - <reason>

**Added CLI tools (if any):**
- <tool-name> - <reason>

**Runtime requirements:**
- <requirement> - <reason>
```

## Notes
- This proposal intentionally has no tasks or specs - it's a tracking document
- Update this proposal when adding new dependencies in other proposals
- Review Dockerfile periodically to ensure all tracked dependencies are present
- Consider security scanning for added dependencies

# Tasks: Rewrite Project Documentation (Realistic Scope)

## Phase 1: Planning (30 minutes)

### 1. Audit existing documentation
- [x] Review current README.md gaps
- [x] List existing docs/ files
- [x] Note what's missing vs what actually works
- [x] Identify tools that need brief descriptions
- **Output**: Documentation gaps checklist

## Phase 2: README Rewrite (2-3 hours)

### 2. Write new README.md (dev-focused)
- [x] Add header with "Development Tool" status badge
- [x] Write "What This Is (and Isn't)" section - set expectations
- [x] Document prerequisites (management cluster access, kubeconfig, Go, MCP client)
- [x] Write Quick Start (clone, build, config, run, connect)
- [x] List features that actually work now
- [x] Add minimal config.yaml example (kubeconfig mode)
- [x] Create tools overview table (categories only, link to docs)
- [x] Add "Current Limitations" section (no OIDC, no K8s deployment, etc.)
- [x] Add brief roadmap (3-4 bullets)
- [x] Link to CONTRIBUTING.md and existing docs
- **Output**: Complete README.md (~200-250 lines)
- **Validation**: Does it accurately represent dev-only state?

## Phase 3: Contributing Guide (2-3 hours)

### 3. Write CONTRIBUTING.md
- [x] Welcome section (dev tool, contributions wanted)
- [x] Development setup (Go, kubeconfig, clone, build)
- [x] Project structure overview (directories, key packages)
- [x] Making changes workflow:
  - New features → OpenSpec proposal first
  - Bug fixes → direct PR with tests
  - Docs → update alongside code
- [x] OpenSpec workflow section (what it is, how to use it)
- [x] Testing guide (unit tests, integration tests, where tests go)
- [x] Code style (Go conventions, gofmt, conventional commits)
- [x] PR submission process
- [x] Debugging tips (debug logs, troubleshooting)
- [x] Resources links (MCP, k0rdent, Go docs)
- **Output**: Complete CONTRIBUTING.md (~150 lines)
- **Validation**: Can a contributor set up environment?

## Phase 4: Development Guide (1-2 hours)

### 4. Create docs/DEVELOPMENT.md
- [x] Setting up local k0rdent cluster (kind + k0rdent install)
- [x] Getting valid kubeconfig with right permissions
- [x] Common development tasks (build, test, run)
- [x] Troubleshooting dev environment issues
- [x] Testing against different cluster configurations
- **Output**: docs/DEVELOPMENT.md (~100 lines)

## Phase 5: Update Existing Docs (1 hour)

### 5. Add frontmatter to existing docs
- [x] Add "Prerequisites" section to docs/cluster-provisioning.md
- [x] Add "Last Updated" to docs/provider-specific-deployment.md
- [x] Add context to docs/features/cluster-monitoring.md
- [x] Update cross-references to match new README structure
- **Output**: Updated existing docs with better context

### 6. Add troubleshooting sections
- [x] Add "MCP Client Compatibility" section (405 error, Streamable HTTP requirement)
- [x] Add "Kubeconfig Issues" section (wrong context, missing RBAC)
- [x] Add "Common Errors" to relevant docs
- **Output**: Troubleshooting guidance in docs (created comprehensive docs/TROUBLESHOOTING.md)

## Phase 6: Validation (30-60 minutes)

### 7. Test documentation accuracy
- [x] Follow Quick Start on clean machine (or VM)
- [x] Verify all commands work as documented
- [x] Check all internal links resolve
- [x] Verify config examples are valid YAML
- [x] Ensure limitations section is accurate
- **Validation**: Documentation matches reality

### 8. Review for tone and expectations
- [x] Verify "dev tool" messaging is clear throughout
- [x] Check that no production features are promised
- [x] Ensure prerequisites are explicit
- [x] Confirm OpenSpec workflow is explained
- **Validation**: Sets appropriate expectations

## Phase 7: Polish (30 minutes)

### 9. Final cleanup
- [x] Spellcheck all new content
- [x] Format code blocks consistently
- [x] Ensure markdown renders correctly
- [x] Add any missing cross-references
- **Output**: Polished documentation ready to merge

## Estimated Timeline

- **Phase 1 (Planning)**: 30 minutes
- **Phase 2 (README)**: 2-3 hours
- **Phase 3 (CONTRIBUTING)**: 2-3 hours
- **Phase 4 (DEVELOPMENT)**: 1-2 hours
- **Phase 5 (Update Docs)**: 1 hour
- **Phase 6 (Validation)**: 30-60 minutes
- **Phase 7 (Polish)**: 30 minutes

**Total**: 7-10 hours (roughly 1 day for one person)

## Success Criteria

- [x] README clearly states this is a dev tool (not production-ready)
- [x] Prerequisites are explicit (management cluster, kubeconfig, etc.)
- [x] New developer can clone, build, and run in <15 minutes
- [x] CONTRIBUTING.md enables successful PR submissions
- [x] Current limitations are clearly documented
- [x] No false promises about production features
- [x] All examples work as documented
- [x] OpenSpec workflow is explained

## What We're NOT Doing

- ❌ Production deployment documentation (feature doesn't exist)
- ❌ OIDC authentication setup (not implemented yet)
- ❌ Kubernetes deployment manifests (not ready)
- ❌ Architecture deep-dive docs (nice-to-have, not essential)
- ❌ Complete tools API reference (existing docs cover this)
- ❌ Aspirational feature documentation

## Dependencies

- None! This is documentation-only
- Need access to codebase to verify tool list
- Need kubeconfig to test Quick Start accuracy

## Parallelization Opportunities

If 2 people work in parallel:
- **Person 1**: README.md (Task 2) - 2-3 hours
- **Person 2**: CONTRIBUTING.md (Task 3) - 2-3 hours
- **Then both**: docs/DEVELOPMENT.md and updates (Tasks 4-6) - 2-3 hours

**Total with 2 people**: 4-6 hours

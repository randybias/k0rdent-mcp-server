# Contributing to k0rdent MCP Server

Thank you for your interest in contributing! This is an experimental development tool, and we welcome improvements, especially testing, bug fixes, and stability enhancements.

## Important Context

- This project is **experimental** and in early development
- Code was built with AI assistance and needs human review
- Focus on **testing and stabilization** over new features
- Expect rough edges and incomplete functionality

## Prerequisites

Before contributing, you should have:

- **Go 1.24+** installed
- **Access to a k0rdent management cluster** (or ability to create one)
- **Admin kubeconfig** for testing
- **Understanding of k0rdent**, Kubernetes, and the MCP protocol
- **Familiarity with Go development** and testing practices

## Development Setup

### 1. Clone and Build

```bash
git clone https://github.com/k0rdent/k0rdent-mcp-server.git
cd k0rdent-mcp-server
go build -o server cmd/server/main.go
```

### 2. Configure Test Environment

Set environment variables to point to a **non-production** k0rdent cluster:

```bash
# Required: kubeconfig for test cluster
export K0RDENT_MGMT_KUBECONFIG_PATH=/path/to/test-cluster-kubeconfig

# Optional: custom port (default is 6767)
export LISTEN_ADDR=:3000

# Optional: debug logging
export LOG_LEVEL=debug
```

**WARNING**: Do NOT use a production cluster for development. Bugs may corrupt cluster state or leave orphaned cloud resources.

### 3. Run Tests

```bash
# Unit tests
go test ./...

# Integration tests (requires cluster access)
go test -tags=integration ./...
```

**Note**: Test coverage is incomplete. Adding tests is highly valued.

### 4. Run the Server

```bash
./server start
```

The server will start on `http://localhost:6767` (or your custom port from `LISTEN_ADDR`).

## What Needs Work

Contributions are especially welcome in these areas:

### High Priority
- **GCP deployment path** – Completely untested, likely broken
- **AWS deployment** – Minimally tested, needs validation and fixes
- **Catalog synchronization** – Known bugs in sync operations
- **Error handling** – Crashes instead of graceful failures
- **Test coverage** – Many code paths untested

### Medium Priority
- **RBAC enforcement** – Currently assumes admin, doesn't filter by permissions
- **Security review** – AI-generated code needs security audit
- **Documentation** – Gaps in docs, outdated examples
- **Race conditions** – Concurrent operations may conflict

### Nice to Have
- **Performance optimization** – Some operations are slow
- **Code quality** – Refactoring AI-generated patterns
- **Observability** – Better logging and metrics

## Making Changes

### For Bug Fixes

1. **Create an issue** describing the bug (if one doesn't exist)
2. **Write a test** that reproduces the bug
3. **Fix the bug** and ensure the test passes
4. **Submit a PR** referencing the issue

### For New Features

**IMPORTANT**: New features require an OpenSpec proposal before implementation.

1. **Check existing proposals**: Run `openspec list` to see if someone is already working on it
2. **Create a proposal**: See "OpenSpec Workflow" section below
3. **Get feedback** from maintainers before implementing
4. **Implement** after proposal approval
5. **Submit PR** with tests and documentation

### For Documentation Improvements

Documentation updates are always welcome:

1. **Fix errors** you find while using the tool
2. **Add missing sections** or clarifications
3. **Update examples** to match current code
4. **Submit PR** with your changes

## OpenSpec Workflow

We use [OpenSpec](openspec/AGENTS.md) to manage change proposals. This prevents duplicate work and ensures alignment before coding.

### What is OpenSpec?

OpenSpec is a structured change proposal system. Each significant change gets a proposal directory under `openspec/changes/<change-id>/` with:

- `proposal.md` – Problem statement and proposed solution
- `specs/` – Detailed specifications (if needed)
- `tasks.md` – Implementation checklist

### When to Use OpenSpec

Use OpenSpec for:
- New features or capabilities
- Breaking changes or major refactoring
- Architecture changes
- Security or performance work

**Skip OpenSpec** for:
- Bug fixes (unless they require design decisions)
- Documentation updates
- Test additions
- Minor code cleanup

### Creating a Proposal

1. **Check existing proposals**:
   ```bash
   openspec list
   ```

2. **Create your proposal directory**:
   ```bash
   mkdir -p openspec/changes/<your-change-id>
   ```

3. **Write `proposal.md`**:
   - Problem: What issue are you solving?
   - Solution: How will you solve it?
   - Benefits: Why is this valuable?
   - Implementation Notes: Key technical details

4. **Create `tasks.md`**:
   - Break down implementation into concrete tasks
   - Add validation checkpoints
   - Note dependencies

5. **Validate your proposal**:
   ```bash
   openspec validate <your-change-id> --strict
   ```

6. **Get feedback** by opening a discussion or PR

7. **Implement** after approval

### Example Proposal Structure

```markdown
# Proposal: Fix GCP Deployment Support

## Problem
GCP deployments are untested and likely broken. Users cannot deploy clusters to GCP.

## Proposed Solution
Test and fix the GCP deployment path:
1. Create GCP test credentials
2. Test deployment end-to-end
3. Fix identified issues
4. Add integration tests

## Benefits
- GCP users can deploy clusters
- Better test coverage
- More provider parity

## Implementation Notes
- Need GCP test account
- May require changes to cluster-deploy tool
- Should add GCP-specific validation
```

See `openspec/AGENTS.md` for complete details and examples.

## Code Guidelines

### Go Conventions

- Follow standard Go style: `gofmt` your code
- Use meaningful variable names (avoid `x`, `y`, `tmp`)
- Write godoc comments for exported functions
- Keep functions small and focused

### Error Handling

- Return errors, don't panic (except for programmer errors)
- Wrap errors with context: `fmt.Errorf("doing thing: %w", err)`
- Log errors with appropriate severity
- Provide actionable error messages

### Testing

- Write tests for new code
- Use table-driven tests where appropriate
- Mock external dependencies (Kubernetes, cloud APIs)
- Test error paths, not just happy paths

### AI-Generated Code Issues to Watch For

This codebase contains AI-generated code. Watch for these patterns and fix them:

- Over-complicated logic that could be simpler
- Missing error checks or inadequate error handling
- Race conditions in concurrent code
- Hardcoded assumptions about cluster state
- Missing input validation
- Unclear variable names or lack of comments

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
type(scope): short description

Longer explanation if needed.

Fixes #123
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `chore`: Maintenance tasks

**Examples**:
```
fix(gcp): add missing project parameter validation

feat(monitoring): add cluster deployment progress subscription

docs(readme): clarify Azure subscription ID requirement

test(catalog): add integration test for sync operation
```

## Pull Request Process

1. **Fork and branch**:
   ```bash
   git checkout -b fix/your-bug-fix
   # or
   git checkout -b feat/your-feature
   ```

2. **Make your changes**:
   - Follow code guidelines above
   - Add or update tests
   - Update documentation if behavior changes

3. **Ensure tests pass**:
   ```bash
   go test ./...
   gofmt -w .
   ```

4. **Commit your changes**:
   ```bash
   git add .
   git commit -m "fix(scope): description"
   ```

5. **Push and create PR**:
   ```bash
   git push origin fix/your-bug-fix
   ```
   Then open a PR on GitHub.

6. **PR Checklist**:
   - [ ] Tests pass locally
   - [ ] Code is formatted (`gofmt`)
   - [ ] Documentation updated (if applicable)
   - [ ] Commit messages follow conventions
   - [ ] For features: OpenSpec proposal exists and is referenced
   - [ ] PR description explains the change

7. **Address review feedback**:
   - Respond to comments
   - Push additional commits to your branch
   - Request re-review when ready

## Testing Guidelines

### Unit Tests

- Test individual functions and methods
- Mock external dependencies
- Fast execution (no network calls)
- Run with: `go test ./...`

### Integration Tests

- Test against real k0rdent cluster
- Tag with `//go:build integration`
- Run with: `go test -tags=integration ./...`
- **WARNING**: May create real cloud resources

### Test Organization

```
pkg/
  tools/
    deploy.go
    deploy_test.go          # unit tests
    deploy_integration_test.go  # integration tests
```

### Running Specific Tests

```bash
# Single package
go test ./pkg/tools

# Specific test
go test ./pkg/tools -run TestDeployAzure

# With verbose output
go test -v ./...
```

## Debugging Tips

### Enable Debug Logging

```bash
./server start --debug
```

### Common Issues

**Connection refused**:
- Check kubeconfig path in config.yaml
- Verify cluster is accessible: `kubectl --kubeconfig=<path> get ns`
- Check firewall or network restrictions

**RBAC errors**:
- Ensure kubeconfig has admin permissions
- This server doesn't work with limited RBAC (known limitation)

**Provider deployment failures**:
- Check cloud credentials are valid
- Verify subscription ID (Azure) or project ID (GCP)
- Review event logs: `kubectl get events -n <namespace>`

### Useful Commands

```bash
# Check server logs
tail -f k0rdent-mcp-server.logs

# Watch cluster events
kubectl get events -n kcm-system --watch

# Get cluster deployment status
kubectl get clusterdeployment -n kcm-system <name> -o yaml

# Check pod logs
kubectl logs -n kcm-system <pod-name>
```

## Resources

- **MCP Protocol**: https://modelcontextprotocol.io
- **k0rdent Documentation**: https://docs.k0rdent.io
- **Go Documentation**: https://go.dev/doc/
- **OpenSpec Details**: See `openspec/AGENTS.md` in this repository

## Questions?

- Open a discussion: https://github.com/k0rdent/k0rdent-mcp-server/discussions
- Join k0rdent community channels (see k0rdent docs)
- Ask in your PR or issue

## Code of Conduct

Be respectful, constructive, and collaborative. We're all learning and improving together.

Thank you for contributing!

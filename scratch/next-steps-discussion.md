# Next Steps Discussion: Provider-Specific Deployment Tools

## Completed Work Summary

### Phase 1: Core Implementation ✅
- ✅ Template auto-selection logic (`SelectLatestTemplate`)
- ✅ AWS provider tool with AWS-specific parameters
- ✅ Azure provider tool with Azure-specific parameters
- ✅ GCP provider tool with GCP-specific parameters
- ✅ Common validation function (eliminates code duplication)
- ✅ Common constants for default values
- ✅ Three-state validation (0=default, negative=error, positive=use)
- ✅ Live testing with AWS (Singapore) and Azure (westus2)
- ✅ Implementation notes documentation

### Key Achievements
1. **AI Agent Discoverability**: Tools expose provider-specific parameters with clear descriptions
2. **Code Quality**: Single source of truth for validation, no duplication
3. **Validation**: Robust three-state validation with explicit error messages
4. **Testing**: Validated with live cluster deployments in multiple regions
5. **Template Selection**: Automatic selection of latest stable templates per provider

## Remaining Work

### Phase 1 Completion
- [ ] **Documentation** (tasks.md line 111-115)
  - Document migration path from generic to provider-specific tools
  - Document common validation function
  - Document default values and constants
  - Update API documentation

### Phase 2: Testing (Not Started)
- [ ] Unit tests for provider tools
- [ ] Unit tests for template selection
- [ ] Integration tests
- [ ] AI agent testing with Claude

### Phase 3: Documentation (Not Started)
- [ ] Create `docs/provider-specific-deployment.md`
- [ ] Update `docs/cluster-provisioning.md`
- [ ] Add usage examples
- [ ] Enhance tool descriptions

### Phase 4: Validation (Partially Done)
- [x] Manual testing with live clusters (AWS, Azure)
- [ ] Performance validation
- [ ] Code review preparation
- [ ] OpenSpec validation

### Optional: MCP Prompt Templates (Not Started)
- [ ] Create AWS deployment example prompt
- [ ] Create Azure deployment example prompt
- [ ] Create GCP deployment example prompt

## Discussion Topics

### 1. Testing Strategy
**Question**: What level of automated testing is required?

**Options**:
- **Minimal**: Keep current live testing approach, add basic unit tests
- **Standard**: Add unit tests + integration tests with mocks
- **Comprehensive**: Full test suite including AI agent testing

**Current State**: Live testing complete, no automated tests

### 2. Documentation Scope
**Question**: How comprehensive should the documentation be?

**Options**:
- **Essential**: Update implementation notes, add basic API docs
- **Standard**: Full documentation with examples for all providers
- **Comprehensive**: Include migration guide, troubleshooting, best practices

**Current State**: Implementation notes complete, API docs missing

### 3. MCP Prompt Templates
**Question**: Are prompt templates needed for this feature?

**Context**:
- MCP supports prompts for example usage
- Helps AI agents discover common patterns
- Not strictly required if tool schemas are clear

**Options**:
- Skip prompts (schemas should be enough)
- Add basic prompts for common use cases
- Add comprehensive prompts with variations

### 4. Generic Tool Future
**Question**: What should we do with the generic deploy tool?

**Current State**: Both generic and provider-specific tools coexist

**Options**:
- **Keep Both**: Maintain generic tool for advanced/custom scenarios
- **Deprecate Generic**: Phase out generic tool in favor of provider tools
- **Remove Generic**: Delete generic tool entirely (breaking change)

### 5. Future Provider Support
**Question**: Should we implement additional providers?

**Potential Providers**:
- vSphere (on-premises)
- OpenStack (on-premises)
- EKS/AKS/GKE (managed services)

**Considerations**:
- User demand
- Testing capabilities
- Maintenance burden

### 6. Template Version Pinning
**Question**: Should users be able to specify template versions?

**Current Behavior**: Always uses latest template matching provider

**Proposed**: Add optional `templateVersion` field
- Default: latest (current behavior)
- Specified: use exact version
- Use case: Reproducible deployments, testing specific versions

### 7. Validation Enhancement
**Question**: Should we add provider-specific credential validation?

**Current State**: Generic credential validation only

**Proposed**:
- AWS: Validate region against credential
- Azure: Validate subscription ID against credential
- GCP: Validate project against credential

**Benefit**: Earlier error detection
**Cost**: Additional API calls, slower validation

### 8. OpenSpec Integration
**Question**: Should we validate this change with OpenSpec?

**Context**: Project uses OpenSpec for change management

**Required**:
- Run `openspec validate add-provider-specific-deploy-tools --strict`
- Ensure all requirements have scenarios
- Fix any validation errors

### 9. Release Strategy
**Question**: How should this feature be released?

**Options**:
- **Feature Branch**: Keep separate until fully tested
- **Main Branch**: Merge core implementation, iterate on docs/tests
- **Release**: Tag as new version after all phases complete

**Considerations**:
- Feature is functional and tested
- Documentation incomplete
- Breaking changes: None (all additive)

## Recommended Next Steps

Based on priorities, here's a suggested order:

### Immediate (Must Do)
1. **Complete Basic Documentation** (1-2 hours)
   - Update IMPLEMENTATION_NOTES.md with final status
   - Add API documentation for provider tools
   - Document common validation function

2. **OpenSpec Validation** (30 minutes)
   - Run validation
   - Fix any issues
   - Document results

### Short Term (Should Do)
3. **Unit Tests** (2-3 hours)
   - Test template selection logic
   - Test validation function
   - Test default value application

4. **Integration Tests** (2-3 hours)
   - Test with mock Kubernetes client
   - Test tool schema generation
   - Test error handling

### Medium Term (Nice to Have)
5. **Comprehensive Documentation** (4-6 hours)
   - Full API documentation
   - Usage examples for all providers
   - Troubleshooting guide
   - Migration guide

6. **MCP Prompt Templates** (2-3 hours)
   - AWS deployment examples
   - Azure deployment examples
   - GCP deployment examples

### Long Term (Future Work)
7. **Additional Providers** (variable)
   - vSphere support
   - OpenStack support
   - Managed service support

8. **Advanced Features** (variable)
   - Template version pinning
   - Provider-specific credential validation
   - Schema generation from Helm charts

## Decision Points

Please provide direction on:

1. **Testing Priority**: Minimal, Standard, or Comprehensive?
2. **Documentation Scope**: Essential, Standard, or Comprehensive?
3. **MCP Prompts**: Skip, Basic, or Comprehensive?
4. **Generic Tool**: Keep, Deprecate, or Remove?
5. **Release Timing**: Feature branch, Main branch, or Wait for all phases?
6. **Future Providers**: Which ones, if any?
7. **Additional Features**: Version pinning, credential validation?

## Files Modified This Session

### Implementation Files
- `internal/tools/core/clusters.go` - Common validation and constants
- `internal/tools/core/clusters_aws.go` - AWS provider tool
- `internal/tools/core/clusters_azure.go` - Azure provider tool
- `internal/tools/core/clusters_gcp.go` - GCP provider tool

### Documentation Files
- `openspec/changes/add-provider-specific-deploy-tools/IMPLEMENTATION_NOTES.md` - Complete implementation notes
- `openspec/changes/add-provider-specific-deploy-tools/tasks.md` - Updated task status

### Test Results
- AWS cluster deployed successfully to ap-southeast-1
- Azure cluster deployed successfully to westus2
- Both clusters used refactored validation correctly
- Test clusters deleted successfully

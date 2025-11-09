# Implementation Tasks

## 1. API Layer Pagination
- [ ] 1.1 Add `PaginationOptions` struct with `Limit int64` and `Continue string` fields
- [ ] 1.2 Add `PaginatedResult` generic wrapper with `Continue string` field
- [ ] 1.3 Update `ListServiceTemplates` to accept pagination options and return paginated result
- [ ] 1.4 Update `ListClusterDeployments` to accept pagination options and return paginated result
- [ ] 1.5 Update `ListMultiClusterServices` to accept pagination options and return paginated result
- [ ] 1.6 Add helper function to apply default/max limits to user-provided limit

## 2. Configuration
- [ ] 2.1 Add `K0RDENT_LIST_PAGE_SIZE` environment variable (default: 100)
- [ ] 2.2 Add `K0RDENT_LIST_MAX_PAGE_SIZE` environment variable (default: 1000)
- [ ] 2.3 Load pagination config in runtime session initialization
- [ ] 2.4 Document pagination config in runtime-config spec

## 3. Tool Layer Updates
- [ ] 3.1 Update k0rdent tools to accept `limit` and `continue` input parameters
- [ ] 3.2 Update k0rdent tools to return `continue` token in response
- [ ] 3.3 Update namespace list tool to support pagination
- [ ] 3.4 Update events list tool to support pagination
- [ ] 3.5 Update MCP tool schemas with pagination parameter descriptions

## 4. Testing
- [ ] 4.1 Add unit tests for pagination options validation (negative limit, zero limit)
- [ ] 4.2 Add unit tests for default/max limit enforcement
- [ ] 4.3 Add integration test for single-page results (no continue token)
- [ ] 4.4 Add integration test for multi-page results (continue token present)
- [ ] 4.5 Add integration test for empty results (no continue token)
- [ ] 4.6 Add integration test for invalid continue token (clear error)
- [ ] 4.7 Add integration test for max page size enforcement

## 5. Documentation
- [ ] 5.1 Update tool documentation with pagination examples
- [ ] 5.2 Add pagination best practices to project documentation
- [ ] 5.3 Document continue token lifecycle (client responsibility, no server state)

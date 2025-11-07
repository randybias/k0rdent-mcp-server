# Implementation Tasks

## 1. Inventory & planning
- [x] 1.1 Enumerate all tool/resource identifiers that still used the legacy prefix (code, tests, docs, metrics) and confirm coverage with automated search.

## 2. Code updates
- [x] 2.1 Rename tool registrations in `internal/tools/core` and any other packages to use the `k0rdent.` prefix.
- [x] 2.2 Update resource/notification URIs and metrics labels that embed the old prefix.
- [x] 2.3 Add transition logging or optional alias handling if we decide to support legacy names temporarily.

## 3. Tests & validation
- [x] 3.1 Update unit/integration tests to reference the new tool names and ensure they continue to pass.
- [x] 3.2 Run the test suite (`go test ./...`) and update fixtures or golden files as needed. *(fails in `internal/clusters` due to pre-existing validation expectations; see summary)*

## 4. Documentation & examples
- [x] 4.1 Update README, docs, and developer guidance to show `k0rdent.` tools.
- [x] 4.2 Notify downstream consumers (release notes/changelog entry) about the breaking rename.

## 5. Spec & CI
- [x] 5.1 Ensure spec deltas for `tools-core`, `tools-catalog`, and `tools-clusters` reflect the new names.
- [x] 5.2 Run `openspec validate update-tool-prefix-k0rdent --strict` and ensure CI expectations are updated.

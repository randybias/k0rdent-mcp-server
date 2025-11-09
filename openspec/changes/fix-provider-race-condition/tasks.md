# Implementation Tasks

## 1. Add mutex to Provider struct
- [ ] 1.1 Add `sync.RWMutex` field to `Provider` struct in `internal/kube/events/provider.go`
- [ ] 1.2 Update Provider struct documentation to document thread-safety guarantees

## 2. Protect useEventsV1 flag access
- [ ] 2.1 Add read lock protection in `List()` method (line 111)
- [ ] 2.2 Add write lock protection in `listEventsV1()` method (line 187)
- [ ] 2.3 Add read lock protection in `startWatch()` method (line 277)
- [ ] 2.4 Add write lock protection in `startWatch()` method (line 286)
- [ ] 2.5 Review all other accesses to `useEventsV1` and add appropriate locking

## 3. Add concurrent test coverage
- [ ] 3.1 Create `TestConcurrentListOperations` test case in `provider_test.go`
- [ ] 3.2 Test exercises at least 10 parallel goroutines calling List() simultaneously
- [ ] 3.3 Test includes scenario where API version fallback is triggered concurrently

## 4. Verification
- [ ] 4.1 Run all tests with `-race` flag: `go test -race ./internal/kube/events/...`
- [ ] 4.2 Verify no data race warnings are reported
- [ ] 4.3 Run full test suite to ensure no regressions: `go test ./...`
- [ ] 4.4 Verify performance impact is negligible with benchmarks if needed

# Change: Fix data race in event provider API version fallback

## Why

The event provider in `internal/kube/events/provider.go` contains a data race condition on the `useEventsV1` boolean flag at line 187 and line 286. When multiple concurrent requests trigger API version fallback (due to `IsNotFound` or `IsForbidden` errors), the flag is mutated without synchronization. This violates Go's memory model and can lead to undefined behavior, including potential crashes or incorrect API routing under concurrent load.

The race occurs in two locations:
1. `listEventsV1()` at line 187: Sets `p.useEventsV1 = false` without mutex protection
2. `startWatch()` at line 286: Sets `p.useEventsV1 = false` without mutex protection

Both read and write operations on the flag lack proper synchronization, creating a classic data race that will be detected by Go's race detector.

## What Changes

- Add `sync.RWMutex` field to the `Provider` struct in `internal/kube/events/provider.go`
- Protect all reads of `useEventsV1` flag with read lock (`RLock`/`RUnlock`)
- Protect all writes to `useEventsV1` flag with write lock (`Lock`/`Unlock`)
- Add concurrent test case in `internal/kube/events/provider_test.go` that exercises parallel list operations to verify thread safety
- Document thread-safety guarantees in the Provider struct documentation
- Run all tests with `-race` flag to verify the fix eliminates the data race

## Impact

- **Affected specs**: event-provider (new capability spec)
- **Affected code**:
  - `internal/kube/events/provider.go` (lines 19-23, 111-114, 187, 277-286)
  - `internal/kube/events/provider_test.go` (add new concurrent test)
- **Breaking changes**: None - this is an internal implementation fix with no API changes
- **Performance impact**: Minimal - RWMutex allows concurrent reads, only writes are serialized

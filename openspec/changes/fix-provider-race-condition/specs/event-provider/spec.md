# Event Provider (delta)

## ADDED Requirements

### Requirement: Thread-safe API version fallback
The event Provider **SHALL** be safe for concurrent use by multiple goroutines. All internal state mutations, including the `useEventsV1` flag that controls API version selection, **SHALL** be protected by appropriate synchronization primitives. The Provider **SHALL** use `sync.RWMutex` to allow concurrent reads while serializing writes to shared state.

#### Scenario: Concurrent list operations
- **WHEN** multiple goroutines call `List()` simultaneously on the same Provider instance
- **THEN** all operations complete successfully without data races
- **AND** the Go race detector reports no race conditions

#### Scenario: Concurrent API version fallback
- **WHEN** multiple goroutines trigger API version fallback simultaneously (e.g., both encounter `IsNotFound` or `IsForbidden` errors)
- **THEN** the `useEventsV1` flag is updated atomically without race conditions
- **AND** subsequent operations correctly use the fallback API version

### Requirement: API version selection consistency
After the Provider falls back from `events.k8s.io/v1` to `core/v1` Events API, **all** subsequent event operations **SHALL** use the fallback API consistently. The Provider **SHALL** maintain this selection across all public methods (`List`, `WatchNamespace`) to avoid unnecessary API discovery attempts.

#### Scenario: Fallback persistence across operations
- **WHEN** `listEventsV1()` triggers a fallback to core/v1 due to API unavailability
- **THEN** subsequent calls to `List()` use core/v1 directly
- **AND** subsequent calls to `WatchNamespace()` also use core/v1 without retrying events.v1

#### Scenario: No spurious API retries after fallback
- **WHEN** the Provider has fallen back to core/v1
- **THEN** it does not attempt to use events.v1 API again
- **AND** no additional discovery or permission errors are logged

## MODIFIED Requirements

### Requirement: Event Provider initialization
The Provider **SHALL** be initialized via `NewProvider()` which discovers available Event APIs during construction. The returned Provider instance **SHALL** be safe for concurrent use without additional synchronization by callers. Internal state **SHALL** be protected by an embedded `sync.RWMutex` to ensure thread safety.

#### Scenario: Provider supports concurrent access after initialization
- **WHEN** a Provider is created via `NewProvider()`
- **THEN** the returned Provider can be called from multiple goroutines simultaneously
- **AND** all methods are safe for concurrent use

#### Scenario: Provider defaults to events.v1 when available
- **WHEN** `NewProvider()` discovers `events.k8s.io/v1` API is available
- **THEN** the Provider is initialized with `useEventsV1 = true`
- **AND** subsequent operations attempt events.v1 first

#### Scenario: Provider falls back to core/v1 when events.v1 unavailable
- **WHEN** `NewProvider()` discovers `events.k8s.io/v1` API is not available or forbidden
- **THEN** the Provider is initialized with `useEventsV1 = false`
- **AND** subsequent operations use core/v1 directly

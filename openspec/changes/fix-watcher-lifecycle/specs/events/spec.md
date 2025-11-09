# EventManager Lifecycle

## ADDED Requirements

### Requirement: EventManager context propagation
EventManager SHALL use the session's parent context for event watch operations instead of `context.Background()`.

#### Scenario: Event subscription uses parent context
- WHEN EventManager's `Subscribe()` method creates a watch context at line 88
- THEN it derives the context from the session's parent context (not `context.Background()`)
- AND this context is passed to `WatchNamespace()` and `streamEvents()`

#### Scenario: Watch context cancellation stops streaming
- WHEN the parent context is canceled
- THEN the watch context is canceled
- AND `streamEvents()` exits its select loop
- AND the event subscription terminates

### Requirement: EventManager WaitGroup tracking
EventManager SHALL maintain a `sync.WaitGroup` to track active event stream goroutines and provide a `Stop()` method for graceful shutdown.

#### Scenario: WaitGroup tracks subscription goroutines
- WHEN EventManager spawns `streamEvents()` and `sendInitialSnapshot()` goroutines
- THEN it calls `wg.Add(1)` for each goroutine before spawning
- AND each goroutine calls `defer wg.Done()` at its start

#### Scenario: Stop() method drains subscriptions
- WHEN `EventManager.Stop()` is called
- THEN it cancels all active subscription contexts
- AND waits on the WaitGroup with a timeout (default 10s)
- AND logs if timeout expires before all goroutines exit

### Requirement: EventManager exponential backoff
EventManager SHALL use `watchutil.Backoff` for event watch reconnection delays.

#### Scenario: Backoff on watch failure
- WHEN an event watch fails in `WatchNamespace()`
- THEN EventManager uses `backoff.Next()` to determine retry delay
- AND logs the delay duration at DEBUG level

#### Scenario: Backoff reset on success
- WHEN an event watch succeeds
- THEN EventManager calls `backoff.Reset()`

### Requirement: EventManager circuit breaker
EventManager SHALL use `watchutil.CircuitBreaker` to skip connection attempts during persistent Kubernetes API failures.

#### Scenario: Circuit breaker prevents subscription
- WHEN the circuit breaker is in "open" state
- THEN EventManager checks `circuitBreaker.Allow()` before calling `WatchNamespace()`
- AND if not allowed, returns an error to the subscriber
- AND logs that subscription is blocked due to circuit breaker state

#### Scenario: Circuit breaker records results
- WHEN an event watch succeeds
- THEN EventManager calls `circuitBreaker.RecordSuccess()`
- AND when a watch fails, calls `circuitBreaker.RecordFailure()`

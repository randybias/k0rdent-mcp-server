# GraphManager Lifecycle

## ADDED Requirements

### Requirement: GraphManager context propagation
GraphManager SHALL accept a parent context during initialization and use it for all watch operations. The manager MUST NOT use `context.Background()` for watch connections.

#### Scenario: GraphManager receives parent context
- WHEN GraphManager is created or bound to a session
- THEN it receives a context derived from the HTTP server lifecycle
- AND stores this context for use in watch operations

#### Scenario: Watch operations use parent context
- WHEN GraphManager starts watchers via `startWatchersLocked()`
- THEN it creates a child context from the parent context (not `context.Background()`)
- AND passes this child context to `watchResource()` calls

### Requirement: GraphManager WaitGroup tracking
GraphManager SHALL maintain a `sync.WaitGroup` to track active watcher goroutines and provide a `Stop()` method for graceful shutdown.

#### Scenario: WaitGroup tracks watch goroutines
- WHEN GraphManager starts a watch via `watchResource()`
- THEN it calls `wg.Add(1)` before spawning the goroutine
- AND the goroutine calls `defer wg.Done()` at the start of its function

#### Scenario: Stop() method drains watchers
- WHEN `GraphManager.Stop()` is called
- THEN it cancels the watch context
- AND waits on the WaitGroup with a timeout (default 10s)
- AND logs if timeout expires before all goroutines exit

### Requirement: GraphManager exponential backoff
GraphManager SHALL use `watchutil.Backoff` for watch reconnection delays instead of hardcoded sleeps.

#### Scenario: Backoff on watch failure
- WHEN a watch connection fails in `watchResource()`
- THEN GraphManager calls `backoff.Next()` to get the retry delay
- AND waits for the returned duration before reconnecting
- AND logs the delay duration at DEBUG level

#### Scenario: Backoff reset on success
- WHEN a watch connection succeeds
- THEN GraphManager calls `backoff.Reset()`
- AND future failures start with the initial 1-second delay

### Requirement: GraphManager circuit breaker
GraphManager SHALL use `watchutil.CircuitBreaker` to skip connection attempts during persistent Kubernetes API failures.

#### Scenario: Circuit breaker prevents connection attempts
- WHEN the circuit breaker is in "open" state
- THEN GraphManager calls `circuitBreaker.Allow()` which returns false
- AND GraphManager skips the watch connection attempt
- AND waits for the circuit breaker cooldown period

#### Scenario: Circuit breaker records results
- WHEN a watch connection succeeds
- THEN GraphManager calls `circuitBreaker.RecordSuccess()`
- AND when a connection fails, GraphManager calls `circuitBreaker.RecordFailure()`

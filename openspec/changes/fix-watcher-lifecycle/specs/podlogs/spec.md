# PodLogManager Lifecycle

## ADDED Requirements

### Requirement: PodLogManager context propagation
PodLogManager SHALL use the session's parent context for log stream operations instead of `context.Background()`.

#### Scenario: Log stream uses parent context
- WHEN PodLogManager's `ensureStream()` method creates a stream context at line 160
- THEN it derives the context from the session's parent context (not `context.Background()`)
- AND this context is passed to `Stream()` and `consumeLogs()`

#### Scenario: Stream context cancellation stops consumption
- WHEN the parent context is canceled
- THEN the stream context is canceled
- AND `consumeLogs()` exits its select loop
- AND the log stream terminates

### Requirement: PodLogManager WaitGroup tracking
PodLogManager SHALL maintain a `sync.WaitGroup` to track active log stream goroutines and provide a `Stop()` method for graceful shutdown.

#### Scenario: WaitGroup tracks stream goroutines
- WHEN PodLogManager spawns a `consumeLogs()` goroutine
- THEN it calls `wg.Add(1)` before spawning the goroutine
- AND the goroutine calls `defer wg.Done()` at its start

#### Scenario: Stop() method drains streams
- WHEN `PodLogManager.Stop()` is called
- THEN it cancels all active stream contexts
- AND waits on the WaitGroup with a timeout (default 10s)
- AND logs if timeout expires before all goroutines exit

### Requirement: PodLogManager exponential backoff
PodLogManager SHALL use `watchutil.Backoff` for log stream reconnection delays.

#### Scenario: Backoff on stream failure
- WHEN a log stream fails in `Stream()`
- THEN PodLogManager uses `backoff.Next()` to determine retry delay
- AND logs the delay duration at DEBUG level

#### Scenario: Backoff reset on success
- WHEN a log stream succeeds
- THEN PodLogManager calls `backoff.Reset()`

### Requirement: PodLogManager circuit breaker
PodLogManager SHALL use `watchutil.CircuitBreaker` to skip connection attempts during persistent Kubernetes API failures.

#### Scenario: Circuit breaker prevents stream creation
- WHEN the circuit breaker is in "open" state
- THEN PodLogManager checks `circuitBreaker.Allow()` before calling `Stream()`
- AND if not allowed, returns an error to the subscriber
- AND logs that stream is blocked due to circuit breaker state

#### Scenario: Circuit breaker records results
- WHEN a log stream succeeds
- THEN PodLogManager calls `circuitBreaker.RecordSuccess()`
- AND when a stream fails, calls `circuitBreaker.RecordFailure()`

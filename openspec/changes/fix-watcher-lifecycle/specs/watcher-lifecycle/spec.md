# Watcher Lifecycle Management

## ADDED Requirements

### Requirement: Context-Driven Watcher Lifecycle
The server SHALL propagate the HTTP server lifecycle context to all Kubernetes watchers (GraphManager, EventManager, PodLogManager). Watchers MUST NOT use `context.Background()` for watch operations.

#### Scenario: Watcher context cancellation on shutdown
- WHEN the HTTP server receives a shutdown signal
- THEN the server context is canceled
- AND all active watchers receive the cancellation signal
- AND watchers terminate their watch loops within 5 seconds

#### Scenario: Watcher inherits server context
- WHEN a watcher is initialized
- THEN the watcher receives a context derived from the HTTP server lifecycle
- AND the watcher uses this context for all Kubernetes API watch operations

### Requirement: Graceful Watcher Shutdown
The server SHALL wait for all active watchers to complete before exiting. Each watcher manager MUST track active goroutines via a `sync.WaitGroup` and provide a `Stop()` method that cancels the context and waits for all goroutines to finish.

#### Scenario: Server waits for watchers during shutdown
- WHEN the HTTP server initiates shutdown
- THEN the server calls `Stop()` on each watcher manager (GraphManager, EventManager, PodLogManager)
- AND the server waits for all active watcher goroutines to exit
- AND the server only completes HTTP shutdown after all watchers have stopped

#### Scenario: Stop() method drains active watchers
- WHEN a manager's `Stop()` method is called
- THEN the manager cancels its context
- AND the manager waits on its WaitGroup for all active goroutines to complete
- AND the method returns only after all goroutines have exited or a timeout occurs (default 10s)

#### Scenario: WaitGroup tracks watcher goroutines
- WHEN a watcher goroutine starts
- THEN the manager calls `wg.Add(1)`
- AND when the goroutine exits, it calls `wg.Done()`
- AND the WaitGroup count reflects the number of active watcher goroutines

### Requirement: Exponential Backoff for Watch Reconnection
Watchers SHALL implement exponential backoff with jitter when reconnecting after a watch failure. The backoff MUST start at 1 second, double on each retry, and cap at 60 seconds. Jitter MUST be 0-20% of the current delay.

#### Scenario: Initial reconnection delay
- WHEN a watch connection fails for the first time
- THEN the watcher waits 1 second (± 200ms jitter) before reconnecting

#### Scenario: Exponential increase on repeated failures
- WHEN a watch connection fails multiple times consecutively
- THEN the delay doubles on each failure: 1s → 2s → 4s → 8s → 16s → 32s → 60s
- AND each delay includes 0-20% random jitter

#### Scenario: Backoff cap at 60 seconds
- WHEN the backoff delay reaches 60 seconds
- THEN subsequent failures continue using 60 seconds (± 12s jitter) without further increase

#### Scenario: Backoff reset on success
- WHEN a watch connection succeeds after failures
- THEN the backoff delay resets to 1 second for future reconnections

### Requirement: Circuit Breaker for Persistent Failures
Watchers SHALL implement a circuit breaker that trips after 5 consecutive watch failures and enters a cooldown period of 30 seconds. During cooldown, no connection attempts SHALL be made.

#### Scenario: Circuit breaker trips after threshold
- WHEN a watcher experiences 5 consecutive watch failures
- THEN the circuit breaker trips to the "open" state
- AND the watcher enters a 30-second cooldown period
- AND no watch connection attempts are made during cooldown

#### Scenario: Circuit breaker cooldown period
- WHEN the circuit breaker is in the "open" state
- THEN the watcher waits for the full 30-second cooldown
- AND after cooldown expires, the circuit breaker transitions to "half-open"
- AND the watcher attempts one connection

#### Scenario: Circuit breaker reset on success
- WHEN the circuit breaker is in "half-open" state and a connection succeeds
- THEN the circuit breaker resets to "closed" state
- AND the failure counter resets to 0

#### Scenario: Circuit breaker reopens on half-open failure
- WHEN the circuit breaker is in "half-open" state and the connection fails
- THEN the circuit breaker returns to "open" state
- AND a new 30-second cooldown begins

### Requirement: Unified Backoff and Circuit Breaker Implementation
The server SHALL provide a shared `internal/kube/watchutil` package containing backoff and circuit breaker logic used by all watcher managers.

#### Scenario: watchutil.Backoff interface
- WHEN a watcher needs to calculate a retry delay
- THEN it calls `watchutil.Backoff.Next()` to get the next delay duration
- AND the backoff tracks consecutive failures and applies exponential growth with jitter

#### Scenario: watchutil.CircuitBreaker interface
- WHEN a watcher attempts a connection
- THEN it calls `watchutil.CircuitBreaker.Allow()` to check if a connection is permitted
- AND if not allowed, the watcher skips the connection attempt
- AND the watcher calls `RecordSuccess()` or `RecordFailure()` based on the result

#### Scenario: Shared logic across managers
- WHEN GraphManager, EventManager, or PodLogManager reconnect watches
- THEN all three managers use the same watchutil.Backoff and watchutil.CircuitBreaker implementations
- AND behavior is consistent across all watcher types

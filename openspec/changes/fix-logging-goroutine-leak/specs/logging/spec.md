# Logging (delta)

## MODIFIED Requirements

### Requirement: External sink hook
- The server **SHALL** provide a pluggable logging sink interface that forwards every log entry.
- The external sink **SHALL** be optional and disabled by default.
- The sink **MUST NOT** block the main execution path or spawn unbounded goroutines.
- When the sink buffer is full, the server **SHALL** drop new log entries rather than block or create additional goroutines.
- The server **SHALL** use a fixed-size worker pool to process sink entries, preventing resource exhaustion under high load.

#### Scenario: External sink enabled
- GIVEN `LOG_EXTERNAL_SINK_ENABLED=true`
- WHEN the server logs an event
- THEN the sink receives the log payload asynchronously without impacting request latency

#### Scenario: Sink buffer full under high load
- GIVEN the external sink is enabled and the buffer is full
- WHEN the server attempts to log additional entries
- THEN new log entries are dropped and a dropped log counter increments
- AND no additional goroutines are spawned

#### Scenario: Worker pool processes buffered logs
- GIVEN the external sink is enabled with a configured worker pool
- WHEN logs are sent to the sink channel
- THEN a fixed number of workers process the logs from the buffer
- AND the worker count remains constant regardless of load

## ADDED Requirements

### Requirement: Configurable sink buffer size
- The server **SHALL** accept `LOG_SINK_BUFFER_SIZE` to control the sink channel buffer size (default `128`).
- The value **MUST** be a positive integer; invalid values **SHALL** cause startup failure with a clear error message.

#### Scenario: Valid buffer size
- GIVEN `LOG_SINK_BUFFER_SIZE=512`
- WHEN the server starts
- THEN the sink channel is created with a buffer size of 512

#### Scenario: Invalid buffer size
- GIVEN `LOG_SINK_BUFFER_SIZE=-10` or `LOG_SINK_BUFFER_SIZE=abc`
- WHEN the server starts
- THEN startup fails with an error indicating invalid buffer size configuration

#### Scenario: Default buffer size
- GIVEN `LOG_SINK_BUFFER_SIZE` is not set
- WHEN the server starts
- THEN the sink channel is created with the default buffer size of 128

### Requirement: Dropped log metrics
- The server **SHALL** maintain a counter of dropped log entries when the sink buffer is full.
- The dropped log counter **SHALL** be exposed for monitoring and observability integration.
- The counter **SHALL** increment atomically to ensure accurate counts under concurrent load.

#### Scenario: Dropped logs are counted
- GIVEN the sink buffer is full
- WHEN 100 log entries are dropped
- THEN the dropped log counter increases by 100

#### Scenario: Metrics accessible for monitoring
- GIVEN the server is running with sink enabled
- WHEN an observability tool queries dropped log metrics
- THEN the current count of dropped logs is returned

### Requirement: Backpressure documentation
- The logging package **SHALL** document the backpressure behavior in godoc comments.
- Operational guidance **SHALL** be provided for tuning `LOG_SINK_BUFFER_SIZE` based on sink performance characteristics.
- Documentation **SHALL** explain when logs are dropped and how to monitor the dropped log counter.

#### Scenario: Developer reads backpressure behavior
- GIVEN a developer reviewing the logging package documentation
- WHEN they read the external sink implementation
- THEN clear documentation explains buffer size, worker pool, and drop behavior

#### Scenario: Operator tunes buffer size
- GIVEN operational documentation for the logging system
- WHEN an operator needs to adjust sink performance
- THEN guidance is available on how to calculate and set appropriate `LOG_SINK_BUFFER_SIZE` values

# Change: fix-logging-goroutine-leak

## Why
- The current logging sink handler (logger.go:156) spawns unbounded goroutines when the sink channel backs up under high-volume logging scenarios.
- Each blocked send triggers a new goroutine via `go func() { h.sinkCh <- payload }()`, leading to memory leaks and eventual OOM conditions in production.
- This pattern violates the design goal that external sinks "MUST NOT block the main execution path" (add-structured-logging spec) by introducing unbounded resource consumption.

## What Changes
- Replace the unbounded goroutine spawn pattern with a fixed-size worker pool that handles sink backpressure gracefully.
- Add configurable sink buffer size via `LOG_SINK_BUFFER_SIZE` environment variable (default: 128).
- Implement dropped log metrics when the buffer is full, providing visibility into sink performance issues.
- Document backpressure behavior and operational guidance for tuning buffer size.
- Add comprehensive tests covering high-volume logging scenarios and worker pool behavior under load.

## Impact
- **BREAKING**: Changes behavior when sink channel is full—logs will be dropped instead of spawning goroutines, but this is the correct behavior to prevent resource exhaustion.
- Affected specs: `logging` (modifies External sink hook requirement and adds new requirements for backpressure handling and metrics).
- Affected code:
  - `internal/logging/logger.go:60` — buffered channel size becomes configurable
  - `internal/logging/logger.go:150-158` — replaces goroutine spawn with worker pool pattern
  - `internal/logging/logger.go` — adds metrics tracking for dropped logs
  - New file: `internal/logging/worker_pool.go` — worker pool implementation
  - New file: `internal/logging/metrics.go` — dropped log counter
- Testing: Adds stress tests that simulate high-volume logging and verify no goroutine leaks occur.
- Documentation: Updates runtime-config.md with new `LOG_SINK_BUFFER_SIZE` environment variable.

## Acceptance
- Under high-volume logging (10k+ logs/sec), the server maintains stable goroutine count without leaks.
- When sink buffer is full, logs are dropped and a metric counter increments.
- `LOG_SINK_BUFFER_SIZE` environment variable controls buffer size (validates positive integer, defaults to 128).
- All existing logging tests continue to pass.
- New stress test verifies no goroutine leaks under sustained load.
- `openspec validate fix-logging-goroutine-leak --strict` passes.

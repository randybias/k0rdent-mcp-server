# Implementation Tasks

## 1. Configuration
- [ ] 1.1 Add `LOG_SINK_BUFFER_SIZE` environment variable to config parsing with validation (positive integer, default 128)
- [ ] 1.2 Update runtime-config.md documentation with new environment variable

## 2. Worker Pool Implementation
- [ ] 2.1 Create `internal/logging/worker_pool.go` with fixed-size worker pool pattern
- [ ] 2.2 Implement graceful worker lifecycle (startup, shutdown, drain)
- [ ] 2.3 Add configurable worker count based on buffer size

## 3. Metrics Implementation
- [ ] 3.1 Create `internal/logging/metrics.go` with dropped log counter
- [ ] 3.2 Integrate metrics collection into worker pool drop logic
- [ ] 3.3 Add metrics getter for observability integration

## 4. Refactor Sink Handler
- [ ] 4.1 Replace goroutine spawn logic (logger.go:156) with worker pool dispatch
- [ ] 4.2 Update Manager initialization to use configurable buffer size
- [ ] 4.3 Ensure worker pool is properly initialized and shut down with Manager lifecycle

## 5. Testing
- [ ] 5.1 Add unit tests for worker pool (creation, dispatch, shutdown)
- [ ] 5.2 Add stress test simulating 10k+ logs/sec to verify no goroutine leaks
- [ ] 5.3 Add test for metrics counter increment on dropped logs
- [ ] 5.4 Add test for buffer size configuration validation
- [ ] 5.5 Verify existing logging tests still pass

## 6. Documentation
- [ ] 6.1 Document backpressure behavior in internal/logging package godoc
- [ ] 6.2 Add operational guidance for tuning buffer size based on sink performance
- [ ] 6.3 Document dropped log metrics and how to monitor them

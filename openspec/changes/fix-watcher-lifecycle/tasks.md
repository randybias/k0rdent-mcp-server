## 1. Create watchutil package
- [ ] 1.1 Create `internal/kube/watchutil/backoff.go` with exponential backoff + jitter (1s→60s cap)
- [ ] 1.2 Create `internal/kube/watchutil/circuitbreaker.go` with trip threshold (5 failures) and cooldown (30s)
- [ ] 1.3 Add unit tests for backoff progression and circuit breaker state transitions
- [ ] 1.4 Document the backoff and circuit breaker algorithms in godoc comments

## 2. Update GraphManager lifecycle
- [ ] 2.1 Add `wg *sync.WaitGroup` field to GraphManager struct
- [ ] 2.2 Modify `startWatchersLocked()` to accept parent context instead of creating `context.Background()`
- [ ] 2.3 Update `watchResource()` to call `wg.Add(1)` at start and `wg.Done()` at end
- [ ] 2.4 Replace hardcoded 1-second sleep with watchutil backoff
- [ ] 2.5 Integrate watchutil circuit breaker to skip retries during persistent API failures
- [ ] 2.6 Add `Stop()` method that cancels context and waits on WaitGroup

## 3. Update EventManager lifecycle
- [ ] 3.1 Add `wg *sync.WaitGroup` field to EventManager struct
- [ ] 3.2 Modify `Subscribe()` to use parent context from server lifecycle (not `context.Background()`)
- [ ] 3.3 Update `WatchNamespace()` call and `streamEvents()` goroutine to track via WaitGroup
- [ ] 3.4 Replace any hardcoded retry sleeps with watchutil backoff
- [ ] 3.5 Integrate circuit breaker for event watch reconnection
- [ ] 3.6 Add `Stop()` method that cancels all subscriptions and waits on WaitGroup

## 4. Update PodLogManager lifecycle
- [ ] 4.1 Add `wg *sync.WaitGroup` field to PodLogManager struct
- [ ] 4.2 Modify `ensureStream()` to use parent context from server lifecycle (not `context.Background()`)
- [ ] 4.3 Update `consumeLogs()` goroutine to track via WaitGroup
- [ ] 4.4 Integrate watchutil backoff and circuit breaker for log stream reconnection
- [ ] 4.5 Add `Stop()` method that cancels all streams and waits on WaitGroup

## 5. Update server shutdown in main.go
- [ ] 5.1 Pass HTTP server context to all manager initialization (GraphManager, EventManager, PodLogManager)
- [ ] 5.2 Before `httpServer.Shutdown()`, call `Stop()` on each manager
- [ ] 5.3 Add timeout for watcher drain (default 10s, configurable via gracefulTimeout)
- [ ] 5.4 Log shutdown progress (watchers stopping, shutdown complete)

## 6. Integration tests
- [ ] 6.1 Add test that starts watchers, triggers shutdown, verifies all goroutines exit within 5s
- [ ] 6.2 Add test that simulates Kubernetes API failure and verifies exponential backoff behavior
- [ ] 6.3 Add test that verifies circuit breaker trips after 5 consecutive failures
- [ ] 6.4 Add test that verifies circuit breaker resets after cooldown period

## 7. Validation
- [ ] 7.1 Run `go test ./...` and verify all tests pass
- [ ] 7.2 Run `go vet ./...` and address any issues
- [ ] 7.3 Run `openspec validate fix-watcher-lifecycle --strict` and resolve any validation errors
- [ ] 7.4 Manually test server startup/shutdown cycle with active watchers

# Implementation Tasks

## 1. Logging Infrastructure
- [ ] 1.1 Create notification error logging helper in `internal/tools/core/logging.go`
- [ ] 1.2 Add structured fields: resource_type, uri, error_message, attempt_number
- [ ] 1.3 Replace `_ = server.ResourceUpdated()` with error logging in graph.go:424
- [ ] 1.4 Replace `_ = server.ResourceUpdated()` with error logging in events.go:210
- [ ] 1.5 Replace `_ = server.ResourceUpdated()` with error logging in podlogs.go:231

## 2. Metrics Infrastructure
- [ ] 2.1 Define Prometheus counter metric `mcp_notification_failures_total` with labels: resource_type, error_type
- [ ] 2.2 Register metric in metrics initialization
- [ ] 2.3 Increment counter on notification failures in all three locations
- [ ] 2.4 Add metric documentation in Prometheus format

## 3. Retry Logic
- [ ] 3.1 Create retry policy: 3 attempts, exponential backoff (100ms, 200ms, 400ms)
- [ ] 3.2 Classify errors as retryable (network, timeout) vs non-retryable (serialization, invalid params)
- [ ] 3.3 Implement retry wrapper for notification sending
- [ ] 3.4 Update graph.go sendDelta to use retry logic
- [ ] 3.5 Update events.go notification to use retry logic
- [ ] 3.6 Update podlogs.go notification to use retry logic

## 4. Testing
- [ ] 4.1 Add unit test for notification logging helper
- [ ] 4.2 Add test case: notification failure triggers error log with correct fields
- [ ] 4.3 Add test case: notification failure increments metrics counter
- [ ] 4.4 Add test case: retryable error triggers retry with backoff
- [ ] 4.5 Add test case: non-retryable error fails immediately without retry
- [ ] 4.6 Add integration test: verify end-to-end notification error handling

## 5. Documentation
- [ ] 5.1 Document notification error behavior in OpenSpec
- [ ] 5.2 Add operator guide for monitoring notification failures
- [ ] 5.3 Add alert recommendations for Prometheus metrics
- [ ] 5.4 Update tool specifications with failure semantics

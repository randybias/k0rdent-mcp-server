# Change: add-prometheus-metrics

## Why
- Server has no observability beyond logs
- Cannot answer operational questions: active subscriptions? watch lag? notification failures?
- No visibility into resource usage, API call latency, or error rates
- Difficult to debug performance issues or capacity plan
- Required by other proposals (handle-notification-errors depends on metrics)

## What Changes
- Add Prometheus client library (prometheus/client_golang)
- Add /metrics HTTP endpoint exposing Prometheus metrics
- Instrument key code paths: subscriptions, watchers, notifications, API calls
- Add standard Go runtime metrics (memory, goroutines, GC)
- Add configuration to enable/disable metrics endpoint
- Add tests for metric instrumentation

## Impact
- Enables operational monitoring and alerting
- Minimal performance overhead (atomic counters, periodic histograms)
- Exposes internal state for visibility
- Increases dependencies (Prometheus client library)
- Provides foundation for SRE practices

## Acceptance
- /metrics endpoint exposes Prometheus-formatted metrics
- Metrics include: active subscriptions, watcher restarts, notification failures, API latency
- Standard Go runtime metrics are included
- Metrics are disabled by default for security
- Tests verify metrics are incremented correctly
- `openspec validate add-prometheus-metrics --strict` passes

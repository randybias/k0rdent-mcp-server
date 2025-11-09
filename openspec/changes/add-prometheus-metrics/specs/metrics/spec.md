# Metrics (delta)

## ADDED Requirements

### Requirement: Prometheus metrics endpoint
- The server **SHALL** expose a /metrics HTTP endpoint when `METRICS_ENABLED=true`
- The endpoint **SHALL** return Prometheus text format metrics
- The endpoint **SHALL** be disabled by default for security
- The endpoint **SHALL NOT** require authentication (suitable for internal scraping)

#### Scenario: Metrics endpoint enabled
- GIVEN `METRICS_ENABLED=true`
- WHEN HTTP GET /metrics is requested
- THEN response status is 200
- AND response body contains Prometheus text format metrics

#### Scenario: Metrics endpoint disabled by default
- GIVEN `METRICS_ENABLED` is not set
- WHEN HTTP GET /metrics is requested
- THEN response status is 404

### Requirement: Subscription metrics
- The server **SHALL** expose gauge metric `mcp_subscriptions_active` tracking current active subscriptions
- The server **SHALL** expose counter metric `mcp_subscriptions_total` tracking total subscriptions created
- Metrics **SHALL** include label: resource_type (events/graph/podlogs)

#### Scenario: Active subscriptions tracked
- GIVEN a client creates 3 graph subscriptions and 2 event subscriptions
- WHEN /metrics is queried
- THEN `mcp_subscriptions_active{resource_type="graph"}` equals 3
- AND `mcp_subscriptions_active{resource_type="events"}` equals 2

#### Scenario: Subscriptions increment total counter
- GIVEN `mcp_subscriptions_total` starts at 0
- WHEN a client creates 5 subscriptions
- THEN `mcp_subscriptions_total` equals 5

#### Scenario: Unsubscribe decrements active gauge
- GIVEN `mcp_subscriptions_active` is 10
- WHEN a client unsubscribes from 3 resources
- THEN `mcp_subscriptions_active` equals 7

### Requirement: Watcher metrics
- The server **SHALL** expose counter metric `k8s_watcher_restarts_total` tracking watch reconnections
- The server **SHALL** expose counter metric `k8s_watcher_errors_total` tracking watch errors
- Metrics **SHALL** include labels: resource_type (servicetemplates/clusterdeployments/etc), namespace

#### Scenario: Watcher restart counted
- GIVEN Kubernetes API connection drops
- WHEN GraphManager restarts a ServiceTemplate watcher
- THEN `k8s_watcher_restarts_total{resource_type="servicetemplates"}` increments

#### Scenario: Watcher errors counted
- GIVEN Kubernetes API returns 403 Forbidden
- WHEN EventManager encounters RBAC error
- THEN `k8s_watcher_errors_total{resource_type="events"}` increments

### Requirement: Notification metrics
- The server **SHALL** expose counter metric `mcp_notifications_sent_total` tracking successful notifications
- The server **SHALL** expose counter metric `mcp_notification_errors_total` tracking failed notifications
- Metrics **SHALL** include labels: resource_type, error_type

#### Scenario: Successful notification counted
- GIVEN a graph delta is computed
- WHEN GraphManager sends notification successfully
- THEN `mcp_notifications_sent_total{resource_type="graph"}` increments

#### Scenario: Failed notification counted
- GIVEN a notification delivery fails
- WHEN error occurs during ResourceUpdated call
- THEN `mcp_notification_errors_total{resource_type="graph",error_type="delivery_failed"}` increments

### Requirement: API call metrics
- The server **SHALL** expose histogram metric `k8s_api_request_duration_seconds` tracking Kubernetes API call latency
- The histogram **SHALL** include labels: method (list/watch/get), resource_type, status (success/error)
- The histogram **SHALL** use standard Prometheus buckets suitable for API latency

#### Scenario: API call latency recorded
- GIVEN a ListServiceTemplates call takes 150ms
- WHEN the call completes
- THEN `k8s_api_request_duration_seconds{method="list",resource_type="servicetemplates",status="success"}` records 0.15

#### Scenario: API error recorded
- GIVEN a Watch call fails with 500 error
- WHEN the error occurs
- THEN `k8s_api_request_duration_seconds{method="watch",resource_type="events",status="error"}` records duration

### Requirement: Go runtime metrics
- The server **SHALL** expose standard Go runtime metrics when metrics enabled
- Metrics **SHALL** include: go_goroutines, go_memstats_alloc_bytes, go_gc_duration_seconds

#### Scenario: Runtime metrics exposed
- GIVEN `METRICS_ENABLED=true`
- WHEN /metrics is queried
- THEN response includes go_goroutines gauge
- AND response includes go_memstats_alloc_bytes gauge

### Requirement: Metric documentation
- The project **SHALL** document all custom metrics in README.md
- Documentation **SHALL** include metric name, type, labels, and purpose
- The project **SHALL** provide example Grafana dashboard JSON

#### Scenario: Metrics documented
- GIVEN a user reads README.md metrics section
- THEN all custom metrics are listed with descriptions
- AND example PromQL queries are provided

#### Scenario: Grafana dashboard provided
- GIVEN a user wants to visualize metrics
- THEN docs/grafana-dashboard.json exists
- AND dashboard includes panels for subscriptions, watchers, notifications, API latency

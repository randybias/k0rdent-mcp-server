# Rate Limiting (delta)

## ADDED Requirements

### Requirement: HTTP request rate limiting
- The server **SHALL** implement per-source-IP rate limiting for HTTP requests
- Rate limits **SHALL** be configurable via `RATE_LIMIT_REQUESTS_PER_SECOND` (default: 10) and `RATE_LIMIT_BURST` (default: 20)
- Requests exceeding the rate limit **SHALL** receive HTTP 429 Too Many Requests response

#### Scenario: Requests within limit succeed
- GIVEN rate limit is 10 req/s with burst of 20
- WHEN a client sends 15 requests in 1 second
- THEN all 15 requests are processed successfully

#### Scenario: Requests exceeding limit are rejected
- GIVEN rate limit is 10 req/s with burst of 20
- WHEN a client sends 25 requests in 1 second
- THEN first 20 requests succeed
- AND remaining 5 requests receive HTTP 429

#### Scenario: Rate limit includes Retry-After header
- GIVEN a request is rate limited
- WHEN HTTP 429 response is sent
- THEN Retry-After header indicates seconds until retry allowed

### Requirement: Subscription quota per session
- Each MCP session **SHALL** have a maximum number of active subscriptions
- The quota **SHALL** be configurable via `MAX_SUBSCRIPTIONS_PER_SESSION` (default: 50)
- Subscription requests exceeding the quota **SHALL** return an MCP error

#### Scenario: Subscriptions within quota succeed
- GIVEN session quota is 50 subscriptions
- WHEN a client subscribes to 40 resources
- THEN all subscriptions are created successfully

#### Scenario: Subscriptions exceeding quota fail
- GIVEN session quota is 50 subscriptions
- WHEN a client has 50 active subscriptions and requests one more
- THEN the new subscription request returns error "quota exceeded"

#### Scenario: Unsubscribe frees quota
- GIVEN a client has 50 active subscriptions (at quota)
- WHEN the client unsubscribes from 10 resources
- THEN the client can create 10 new subscriptions

### Requirement: Rate limit metrics
- The server **SHALL** expose Prometheus metrics for rate limiting events
- Metric **SHALL** include counter `rate_limit_hits_total` with labels: limit_type (http/subscription), source_ip

#### Scenario: HTTP rate limit hit recorded
- GIVEN Prometheus metrics are enabled
- WHEN a request is rate limited
- THEN `rate_limit_hits_total{limit_type="http",source_ip="192.0.2.1"}` increments

#### Scenario: Subscription quota hit recorded
- GIVEN Prometheus metrics are enabled
- WHEN a subscription exceeds quota
- THEN `rate_limit_hits_total{limit_type="subscription"}` increments

### Requirement: Configurable rate limits
- Rate limits **SHALL** be configurable at startup via environment variables
- Invalid rate limit values **SHALL** cause startup failure with clear error message
- Rate limits **SHALL** be logged at startup for operational visibility

#### Scenario: Valid configuration accepted
- GIVEN `RATE_LIMIT_REQUESTS_PER_SECOND=100` and `RATE_LIMIT_BURST=200`
- WHEN server starts
- THEN rate limits are applied as configured
- AND startup log shows "rate_limit_rps=100 burst=200"

#### Scenario: Invalid configuration fails startup
- GIVEN `RATE_LIMIT_REQUESTS_PER_SECOND=-10` (negative value)
- WHEN server attempts to start
- THEN startup fails with error "invalid rate limit: must be positive"

### Requirement: Per-IP tracking
- The rate limiter **SHALL** track limits independently per source IP address
- IP addresses **SHALL** be extracted using chi middleware for X-Forwarded-For support
- Rate limiter storage **SHALL** have automatic cleanup for stale entries

#### Scenario: Multiple IPs tracked independently
- GIVEN rate limit is 10 req/s
- WHEN IP 192.0.2.1 sends 10 requests and IP 192.0.2.2 sends 10 requests
- THEN both IPs' requests succeed (separate quotas)

#### Scenario: Stale IP entries cleaned up
- GIVEN an IP hasn't sent requests for 5 minutes
- WHEN cleanup runs
- THEN the IP's rate limit state is removed from memory

# Change: add-rate-limiting

## Why
- Server has no rate limiting: single client can spawn unlimited subscriptions
- Each subscription spawns multiple watchers (GraphManager: 3 goroutines)
- DoS attack vector: connect, subscribe 1000 times, overwhelm server and Kubernetes API
- No protection against accidental or malicious resource exhaustion

## What Changes
- Add rate limiting middleware for HTTP requests (per-IP limits)
- Add subscription quota per MCP session (max active subscriptions)
- Add configurable rate limits via environment variables
- Add metrics for rate limit hits
- Return HTTP 429 Too Many Requests when limits exceeded
- Add tests for rate limiting enforcement

## Impact
- Protects server from resource exhaustion attacks
- Protects Kubernetes API server from watcher spam
- May limit legitimate high-volume use cases (configurable)
- Adds small per-request overhead (token bucket algorithm)
- Improves operational stability

## Acceptance
- HTTP requests are rate-limited per source IP
- MCP sessions have maximum active subscription count
- Rate limit violations return HTTP 429 with Retry-After header
- Metrics track rate limit hits by type
- Configuration allows tuning limits for different environments
- Tests verify enforcement under load
- `openspec validate add-rate-limiting --strict` passes

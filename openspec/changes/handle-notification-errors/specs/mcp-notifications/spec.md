# MCP Notifications Specification

## ADDED Requirements

### Requirement: Notification Error Logging
The system SHALL log all MCP notification failures at ERROR level with structured context including resource type, URI, error message, and attempt number.

#### Scenario: Notification delivery fails
- **WHEN** `server.ResourceUpdated()` returns an error
- **THEN** the system logs the error with structured fields (resource_type, uri, error_message, attempt_number)
- **AND** the log level is ERROR

#### Scenario: Multiple notification failures
- **WHEN** notifications fail for different resource types
- **THEN** each failure is logged with the correct resource_type field
- **AND** logs are distinguishable by resource type

### Requirement: Notification Failure Metrics
The system SHALL expose a Prometheus counter metric `mcp_notification_failures_total` with labels for resource_type and error_type to track notification delivery failures.

#### Scenario: Notification failure increments counter
- **WHEN** a notification fails to deliver
- **THEN** the `mcp_notification_failures_total` counter is incremented
- **AND** the counter includes labels: resource_type (graph|events|podlogs), error_type (network|timeout|serialization|other)

#### Scenario: Metric labels distinguish failure types
- **WHEN** different notification failures occur
- **THEN** each failure type has its own counter series
- **AND** operators can query failures by resource type or error type

### Requirement: Notification Retry Logic
The system SHALL retry transient notification failures up to 3 times with exponential backoff (100ms, 200ms, 400ms) before logging as a permanent failure.

#### Scenario: Transient failure retries succeed
- **WHEN** a notification fails with a retryable error (network, timeout)
- **THEN** the system retries up to 3 times with exponential backoff
- **AND** if a retry succeeds, no error is logged
- **AND** the failure counter is not incremented

#### Scenario: Permanent failure after retries
- **WHEN** all 3 retry attempts fail
- **THEN** the system logs the error with attempt_number=3
- **AND** the failure counter is incremented once
- **AND** the notification is abandoned

#### Scenario: Non-retryable failure fails fast
- **WHEN** a notification fails with a non-retryable error (serialization, invalid params)
- **THEN** the system does not retry
- **AND** the error is logged immediately with attempt_number=1
- **AND** the failure counter is incremented

### Requirement: Error Classification
The system SHALL classify notification errors as retryable (network, timeout) or non-retryable (serialization, invalid parameters) to determine retry behavior.

#### Scenario: Network error classified as retryable
- **WHEN** a notification fails with a network connectivity error
- **THEN** the error is classified as retryable
- **AND** retry logic is applied

#### Scenario: Serialization error classified as non-retryable
- **WHEN** a notification fails with JSON marshaling error
- **THEN** the error is classified as non-retryable
- **AND** no retries are attempted

### Requirement: Notification Failure Documentation
Tool specifications SHALL document notification failure behavior, including retry semantics, observability, and operator guidance.

#### Scenario: Tool spec includes failure documentation
- **WHEN** a developer reviews a tool specification
- **THEN** the spec documents notification failure behavior
- **AND** the spec describes retry policy and metrics
- **AND** the spec provides operator guidance for monitoring

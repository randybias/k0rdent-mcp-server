package clusters

// ProviderType represents a cloud provider type
type ProviderType string

const (
	// ProviderAWS represents Amazon Web Services
	ProviderAWS ProviderType = "aws"

	// ProviderAzure represents Microsoft Azure
	ProviderAzure ProviderType = "azure"

	// ProviderGCP represents Google Cloud Platform
	ProviderGCP ProviderType = "gcp"

	// ProviderUnknown represents an unknown or unsupported provider
	ProviderUnknown ProviderType = "unknown"
)

// ValidationError represents a single configuration validation error
type ValidationError struct {
	// Field is the configuration field path that failed validation
	Field string `json:"field"`

	// Message is a human-readable error message
	Message string `json:"message"`

	// Code is a machine-readable error code (e.g., "azure.location.required")
	Code string `json:"code"`
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	// Provider is the detected cloud provider
	Provider ProviderType `json:"provider"`

	// Errors contains validation errors (configuration is invalid if non-empty)
	Errors []ValidationError `json:"errors,omitempty"`

	// Warnings contains non-blocking validation warnings
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// IsValid returns true if there are no validation errors
func (r ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// DetectProvider analyzes a template name and returns the corresponding ProviderType.
// It uses pattern matching against known cloud provider prefixes:
// - "aws-*" -> ProviderAWS
// - "azure-*" -> ProviderAzure
// - "gcp-*" -> ProviderGCP
// - anything else -> ProviderUnknown
func DetectProvider(templateName string) ProviderType {
	// Convert to lowercase for case-insensitive matching
	lower := ""
	for i := 0; i < len(templateName); i++ {
		c := templateName[i]
		if c >= 'A' && c <= 'Z' {
			lower += string(c + 32)
		} else {
			lower += string(c)
		}
	}

	// Match against known provider prefixes
	if len(lower) >= 4 && lower[:4] == "aws-" {
		return ProviderAWS
	}
	if len(lower) >= 6 && lower[:6] == "azure-" {
		return ProviderAzure
	}
	if len(lower) >= 4 && lower[:4] == "gcp-" {
		return ProviderGCP
	}

	return ProviderUnknown
}

// ValidateConfig performs provider-specific validation on cluster configuration.
// It detects the provider from the template name and dispatches to the appropriate
// validator function. Unknown providers pass validation (no validation performed).
func ValidateConfig(templateName string, config map[string]interface{}) ValidationResult {
	provider := DetectProvider(templateName)

	switch provider {
	case ProviderAWS:
		return ValidateAWSConfig(config)
	case ProviderAzure:
		return ValidateAzureConfig(config)
	case ProviderGCP:
		return ValidateGCPConfig(config)
	case ProviderUnknown:
		// No validation for unknown providers - allow them through
		return ValidationResult{
			Provider: provider,
		}
	default:
		// Should never reach here, but handle defensively
		return ValidationResult{
			Provider: provider,
		}
	}
}

// NewValidationError creates a ValidationError with the specified details.
func NewValidationError(field, message, code string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	}
}

// NewMissingFieldError creates a ValidationError for a missing required field.
func NewMissingFieldError(field, description string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: description + " is required but not provided",
		Code:    "required_field_missing",
	}
}

// hasNonEmptyString checks if a field exists in the config and is a non-empty string.
func hasNonEmptyString(config map[string]interface{}, field string) bool {
	val, ok := config[field]
	if !ok {
		return false
	}

	str, ok := val.(string)
	if !ok {
		return false
	}

	// Check if string is non-empty after trimming whitespace
	for _, ch := range str {
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			return true
		}
	}
	return false
}

// getNestedField retrieves a nested field value from a configuration map.
// It supports dot notation paths like "network.name".
// Returns the value and true if found, nil and false otherwise.
func getNestedField(config map[string]interface{}, path string) (interface{}, bool) {
	// Split path by dots
	parts := []string{}
	current := ""
	for _, ch := range path {
		if ch == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	currentMap := config
	for i, part := range parts {
		val, ok := currentMap[part]
		if !ok {
			return nil, false
		}

		// If this is the last part, return the value
		if i == len(parts)-1 {
			return val, true
		}

		// Otherwise, it should be a nested map
		nextMap, ok := val.(map[string]interface{})
		if !ok {
			return nil, false
		}
		currentMap = nextMap
	}

	return nil, false
}

// hasNonEmptyNestedString checks if a nested field exists and is a non-empty string.
// It supports dot notation paths like "network.name".
func hasNonEmptyNestedString(config map[string]interface{}, path string) bool {
	val, ok := getNestedField(config, path)
	if !ok {
		return false
	}

	str, ok := val.(string)
	if !ok {
		return false
	}

	// Check if string is non-empty after trimming whitespace
	for _, ch := range str {
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			return true
		}
	}
	return false
}

package clusters

import (
	"testing"
)

// TestProviderType tests the ProviderType constant values.
func TestProviderType(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{"AWS provider", ProviderAWS, "aws"},
		{"Azure provider", ProviderAzure, "azure"},
		{"GCP provider", ProviderGCP, "gcp"},
		{"Unknown provider", ProviderUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.provider))
			}
		})
	}
}

// TestValidationError tests the ValidationError structure.
func TestValidationError(t *testing.T) {
	err := ValidationError{
		Field:   "config.region",
		Message: "Region is required",
		Code:    "region.required",
	}

	if err.Field != "config.region" {
		t.Errorf("expected Field=%q, got Field=%q", "config.region", err.Field)
	}
	if err.Message != "Region is required" {
		t.Errorf("expected Message=%q, got Message=%q", "Region is required", err.Message)
	}
	if err.Code != "region.required" {
		t.Errorf("expected Code=%q, got Code=%q", "region.required", err.Code)
	}
}

// TestValidationResult tests the ValidationResult structure and its methods.
func TestValidationResult(t *testing.T) {
	tests := []struct {
		name          string
		result        ValidationResult
		expectIsValid bool
	}{
		{
			name: "valid result with no errors",
			result: ValidationResult{
				Provider: ProviderAWS,
				Errors:   []ValidationError{},
			},
			expectIsValid: true,
		},
		{
			name: "invalid result with one error",
			result: ValidationResult{
				Provider: ProviderAWS,
				Errors: []ValidationError{
					{Field: "config.region", Message: "Required", Code: "required"},
				},
			},
			expectIsValid: false,
		},
		{
			name: "invalid result with multiple errors",
			result: ValidationResult{
				Provider: ProviderAzure,
				Errors: []ValidationError{
					{Field: "config.location", Message: "Required", Code: "required"},
					{Field: "config.subscriptionID", Message: "Required", Code: "required"},
				},
			},
			expectIsValid: false,
		},
		{
			name: "valid result with warnings",
			result: ValidationResult{
				Provider: ProviderGCP,
				Errors:   []ValidationError{},
				Warnings: []ValidationError{
					{Field: "config.zone", Message: "Recommended", Code: "recommended"},
				},
			},
			expectIsValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.IsValid() != tt.expectIsValid {
				t.Errorf("expected IsValid()=%v, got IsValid()=%v", tt.expectIsValid, tt.result.IsValid())
			}
		})
	}
}

// TestDetectProvider tests provider detection from template names.
func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		expected     ProviderType
	}{
		// AWS templates
		{"AWS standalone", "aws-standalone-cp-1-0-16", ProviderAWS},
		{"AWS hosted", "aws-hosted-cp-1-0-15", ProviderAWS},
		{"AWS uppercase", "AWS-STANDALONE-CP", ProviderAWS},
		{"AWS mixed case", "Aws-Standalone-Cp", ProviderAWS},
		{"AWS prefix only", "aws-", ProviderAWS},

		// Azure templates
		{"Azure standalone", "azure-standalone-cp-1-0-17", ProviderAzure},
		{"Azure hosted", "azure-hosted-cp-1-0-16", ProviderAzure},
		{"Azure uppercase", "AZURE-STANDALONE-CP", ProviderAzure},
		{"Azure mixed case", "Azure-Standalone-Cp", ProviderAzure},
		{"Azure prefix only", "azure-", ProviderAzure},

		// GCP templates
		{"GCP standalone", "gcp-standalone-cp-1-0-15", ProviderGCP},
		{"GCP hosted", "gcp-hosted-cp-1-0-14", ProviderGCP},
		{"GCP uppercase", "GCP-STANDALONE-CP", ProviderGCP},
		{"GCP mixed case", "Gcp-Standalone-Cp", ProviderGCP},
		{"GCP prefix only", "gcp-", ProviderGCP},

		// Unknown/unsupported providers
		{"vSphere", "vsphere-standalone-cp", ProviderUnknown},
		{"OpenStack", "openstack-hosted-cp", ProviderUnknown},
		{"Custom", "my-custom-template", ProviderUnknown},
		{"Empty string", "", ProviderUnknown},
		{"No prefix", "standalone-cp", ProviderUnknown},
		{"Almost AWS", "aw-template", ProviderUnknown},
		{"Almost Azure", "azur-template", ProviderUnknown},
		{"Almost GCP", "gc-template", ProviderUnknown},
		{"Contains AWS but wrong position", "template-aws-something", ProviderUnknown},
		{"Contains azure but wrong position", "template-azure-something", ProviderUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProvider(tt.templateName)
			if result != tt.expected {
				t.Errorf("DetectProvider(%q) = %q, expected %q", tt.templateName, result, tt.expected)
			}
		})
	}
}

// TestValidateConfig tests the main validation dispatcher.
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		config       map[string]interface{}
		expectValid  bool
		expectErrors int
		provider     ProviderType
	}{
		// AWS validation
		{
			name:         "AWS valid config",
			templateName: "aws-standalone-cp-1-0-16",
			config: map[string]interface{}{
				"region": "us-west-2",
			},
			expectValid:  true,
			expectErrors: 0,
			provider:     ProviderAWS,
		},
		{
			name:         "AWS missing region",
			templateName: "aws-standalone-cp-1-0-16",
			config:       map[string]interface{}{},
			expectValid:  false,
			expectErrors: 1,
			provider:     ProviderAWS,
		},

		// Azure validation
		{
			name:         "Azure valid config",
			templateName: "azure-standalone-cp-1-0-17",
			config: map[string]interface{}{
				"location":       "westus2",
				"subscriptionID": "12345678-1234-1234-1234-123456789abc",
			},
			expectValid:  true,
			expectErrors: 0,
			provider:     ProviderAzure,
		},
		{
			name:         "Azure missing both fields",
			templateName: "azure-standalone-cp-1-0-17",
			config:       map[string]interface{}{},
			expectValid:  false,
			expectErrors: 2,
			provider:     ProviderAzure,
		},

		// GCP validation
		{
			name:         "GCP valid config",
			templateName: "gcp-standalone-cp-1-0-15",
			config: map[string]interface{}{
				"project": "my-project",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			expectValid:  true,
			expectErrors: 0,
			provider:     ProviderGCP,
		},
		{
			name:         "GCP missing all fields",
			templateName: "gcp-standalone-cp-1-0-15",
			config:       map[string]interface{}{},
			expectValid:  false,
			expectErrors: 3,
			provider:     ProviderGCP,
		},

		// Unknown provider (no validation)
		{
			name:         "Unknown provider passes through",
			templateName: "vsphere-standalone-cp",
			config:       map[string]interface{}{},
			expectValid:  true,
			expectErrors: 0,
			provider:     ProviderUnknown,
		},
		{
			name:         "Custom template passes through",
			templateName: "my-custom-template",
			config:       map[string]interface{}{},
			expectValid:  true,
			expectErrors: 0,
			provider:     ProviderUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfig(tt.templateName, tt.config)

			// Check provider detection
			if result.Provider != tt.provider {
				t.Errorf("expected Provider=%q, got Provider=%q", tt.provider, result.Provider)
			}

			// Check validity
			if result.IsValid() != tt.expectValid {
				t.Errorf("expected IsValid()=%v, got IsValid()=%v (errors: %v)", tt.expectValid, result.IsValid(), result.Errors)
			}

			// Check error count
			if len(result.Errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}
		})
	}
}

// TestNewValidationError tests the validation error constructor.
func TestNewValidationError(t *testing.T) {
	err := NewValidationError("config.field", "Field is invalid", "field.invalid")

	if err.Field != "config.field" {
		t.Errorf("expected Field=%q, got Field=%q", "config.field", err.Field)
	}
	if err.Message != "Field is invalid" {
		t.Errorf("expected Message=%q, got Message=%q", "Field is invalid", err.Message)
	}
	if err.Code != "field.invalid" {
		t.Errorf("expected Code=%q, got Code=%q", "field.invalid", err.Code)
	}
}

// TestNewMissingFieldError tests the missing field error constructor.
func TestNewMissingFieldError(t *testing.T) {
	err := NewMissingFieldError("config.region", "AWS region")

	if err.Field != "config.region" {
		t.Errorf("expected Field=%q, got Field=%q", "config.region", err.Field)
	}
	if !containsValidationString(err.Message, "AWS region") {
		t.Errorf("expected Message to contain 'AWS region', got: %q", err.Message)
	}
	if !containsValidationString(err.Message, "required") {
		t.Errorf("expected Message to contain 'required', got: %q", err.Message)
	}
	if err.Code != "required_field_missing" {
		t.Errorf("expected Code=%q, got Code=%q", "required_field_missing", err.Code)
	}
}

// containsValidationString is a helper function to check if a string contains a substring.
func containsValidationString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsValidationSubstring(s, substr))
}

// containsValidationSubstring performs substring search.
func containsValidationSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Note: Tests for hasNonEmptyString, getNestedField, and hasNonEmptyNestedString
// are located in validation_azure_test.go and validation_gcp_test.go to avoid duplication.

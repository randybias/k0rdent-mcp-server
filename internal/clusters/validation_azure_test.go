package clusters

import (
	"strings"
	"testing"
)

func TestValidateAzureConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		expectValid   bool
		expectedError string
	}{
		{
			name: "valid Azure config with all required fields",
			config: map[string]interface{}{
				"location":       "westus2",
				"subscriptionID": "12345678-1234-1234-1234-123456789abc",
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
				"worker": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectValid: true,
		},
		{
			name: "missing subscriptionID",
			config: map[string]interface{}{
				"location": "westus2",
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectValid:   false,
			expectedError: "subscriptionID",
		},
		{
			name: "missing location",
			config: map[string]interface{}{
				"subscriptionID": "12345678-1234-1234-1234-123456789abc",
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectValid:   false,
			expectedError: "location",
		},
		{
			name: "missing both required fields",
			config: map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"vmSize": "Standard_A4_v2",
				},
			},
			expectValid:   false,
			expectedError: "location", // Should include both errors
		},
		{
			name: "empty location string",
			config: map[string]interface{}{
				"location":       "",
				"subscriptionID": "12345678-1234-1234-1234-123456789abc",
			},
			expectValid:   false,
			expectedError: "location",
		},
		{
			name: "empty subscriptionID string",
			config: map[string]interface{}{
				"location":       "westus2",
				"subscriptionID": "",
			},
			expectValid:   false,
			expectedError: "subscriptionID",
		},
		{
			name: "location is not a string",
			config: map[string]interface{}{
				"location":       12345,
				"subscriptionID": "12345678-1234-1234-1234-123456789abc",
			},
			expectValid:   false,
			expectedError: "location",
		},
		{
			name: "subscriptionID is not a string",
			config: map[string]interface{}{
				"location":       "westus2",
				"subscriptionID": 12345,
			},
			expectValid:   false,
			expectedError: "subscriptionID",
		},
		{
			name: "valid config with additional optional fields",
			config: map[string]interface{}{
				"location":           "eastus",
				"subscriptionID":     "12345678-1234-1234-1234-123456789abc",
				"controlPlaneNumber": 3,
				"workersNumber":      5,
				"controlPlane": map[string]interface{}{
					"vmSize":         "Standard_D4s_v3",
					"rootVolumeSize": 50,
				},
				"worker": map[string]interface{}{
					"vmSize":         "Standard_D4s_v3",
					"rootVolumeSize": 100,
				},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateAzureConfig(tt.config)

			// Check validity
			if result.IsValid() != tt.expectValid {
				t.Errorf("expected IsValid=%v, got %v (errors: %v)", tt.expectValid, result.IsValid(), result.Errors)
			}

			// Check provider is set correctly
			if result.Provider != ProviderAzure {
				t.Errorf("expected provider=azure, got %s", result.Provider)
			}

			// Check error message contains expected text
			if !tt.expectValid && tt.expectedError != "" {
				foundError := false
				for _, err := range result.Errors {
					if strings.Contains(err.Field, tt.expectedError) || strings.Contains(err.Message, tt.expectedError) {
						foundError = true
						break
					}
				}
				if !foundError {
					t.Errorf("expected error containing %q, but got errors: %v", tt.expectedError, result.Errors)
				}
			}

			// Check that valid configs have no errors
			if tt.expectValid && len(result.Errors) > 0 {
				t.Errorf("expected no errors for valid config, got: %v", result.Errors)
			}
		})
	}
}

func TestFormatAzureValidationError(t *testing.T) {
	tests := []struct {
		name           string
		errors         []ValidationError
		expectContains []string
	}{
		{
			name:           "no errors returns empty string",
			errors:         []ValidationError{},
			expectContains: []string{},
		},
		{
			name: "single error includes field and message",
			errors: []ValidationError{
				{
					Field:   "config.location",
					Message: "Azure location is required",
					Code:    "azure.location.required",
				},
			},
			expectContains: []string{
				"config.location",
				"Azure location is required",
				"Example valid Azure configuration",
				"https://docs.k0rdent.io",
			},
		},
		{
			name: "multiple errors lists all",
			errors: []ValidationError{
				{
					Field:   "config.location",
					Message: "Azure location is required",
					Code:    "azure.location.required",
				},
				{
					Field:   "config.subscriptionID",
					Message: "Azure subscription ID is required",
					Code:    "azure.subscriptionID.required",
				},
			},
			expectContains: []string{
				"config.location",
				"config.subscriptionID",
				"Azure location is required",
				"Azure subscription ID is required",
				"westus2",
				"vmSize",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAzureValidationError(tt.errors)

			if len(tt.errors) == 0 {
				if result != "" {
					t.Errorf("expected empty string for no errors, got: %s", result)
				}
				return
			}

			for _, expected := range tt.expectContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected error message to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

func TestAzureValidationErrorFormat(t *testing.T) {
	// Test that error format is helpful for users
	config := map[string]interface{}{
		"controlPlane": map[string]interface{}{
			"vmSize": "Standard_A4_v2",
		},
	}

	result := ValidateAzureConfig(config)
	formatted := FormatAzureValidationError(result.Errors)

	// Should mention both missing fields
	if !strings.Contains(formatted, "location") {
		t.Error("error message should mention location")
	}
	if !strings.Contains(formatted, "subscriptionID") {
		t.Error("error message should mention subscriptionID")
	}

	// Should provide an example
	if !strings.Contains(formatted, "Example") || !strings.Contains(formatted, "westus2") {
		t.Error("error message should provide an example configuration")
	}

	// Should link to documentation
	if !strings.Contains(formatted, "https://docs.k0rdent.io") {
		t.Error("error message should link to documentation")
	}
}

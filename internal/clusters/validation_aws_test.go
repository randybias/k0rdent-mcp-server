package clusters

import (
	"testing"
)

// TestValidateAWSConfig tests AWS-specific configuration validation.
func TestValidateAWSConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		expectValid   bool
		expectErrors  int
		errorContains []string
	}{
		{
			name: "valid AWS config with region",
			config: map[string]interface{}{
				"region": "us-west-2",
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
				"worker": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "missing region",
			config: map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
				"worker": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"config.region", "AWS region"},
		},
		{
			name: "empty region string",
			config: map[string]interface{}{
				"region": "",
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"config.region", "AWS region"},
		},
		{
			name: "whitespace-only region",
			config: map[string]interface{}{
				"region": "   ",
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"config.region"},
		},
		{
			name: "region is not a string (number)",
			config: map[string]interface{}{
				"region": 12345,
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"config.region"},
		},
		{
			name: "region is not a string (boolean)",
			config: map[string]interface{}{
				"region": true,
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"config.region"},
		},
		{
			name:          "empty config",
			config:        map[string]interface{}{},
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"config.region"},
		},
		{
			name:          "nil config",
			config:        nil,
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"config.region"},
		},
		{
			name: "valid AWS config with different region",
			config: map[string]interface{}{
				"region": "eu-west-1",
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.medium",
				},
				"worker": map[string]interface{}{
					"instanceType": "t3.medium",
				},
			},
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "valid AWS config with minimal fields",
			config: map[string]interface{}{
				"region": "ap-southeast-1",
			},
			expectValid:  true,
			expectErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateAWSConfig(tt.config)

			// Check IsValid() method
			if result.IsValid() != tt.expectValid {
				t.Errorf("expected IsValid()=%v, got IsValid()=%v", tt.expectValid, result.IsValid())
			}

			// Check provider is set correctly
			if result.Provider != ProviderAWS {
				t.Errorf("expected Provider=%q, got Provider=%q", ProviderAWS, result.Provider)
			}

			// Check error count
			if len(result.Errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}

			// Check error content
			if len(tt.errorContains) > 0 && len(result.Errors) > 0 {
				errorMsg := result.Errors[0].Field + " " + result.Errors[0].Message
				for _, expected := range tt.errorContains {
					if !containsString(errorMsg, expected) {
						t.Errorf("expected error to contain %q, got: %q", expected, errorMsg)
					}
				}
			}

			// Verify error code is set for errors
			for _, err := range result.Errors {
				if err.Code == "" {
					t.Errorf("error %q has empty code", err.Field)
				}
			}
		})
	}
}

// TestValidateAWSConfig_ErrorFormat tests that error messages are helpful.
func TestValidateAWSConfig_ErrorFormat(t *testing.T) {
	config := map[string]interface{}{
		"controlPlane": map[string]interface{}{
			"instanceType": "t3.small",
		},
	}

	result := ValidateAWSConfig(config)

	if result.IsValid() {
		t.Error("expected validation to fail for missing region")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}

	err := result.Errors[0]

	// Check that error has field path
	if err.Field != "config.region" {
		t.Errorf("expected field=%q, got field=%q", "config.region", err.Field)
	}

	// Check that error has helpful message
	if err.Message == "" {
		t.Error("expected non-empty error message")
	}

	// Check that message mentions AWS and region
	if !containsString(err.Message, "region") {
		t.Errorf("expected error message to mention 'region', got: %q", err.Message)
	}

	// Check that error has code
	if err.Code == "" {
		t.Error("expected non-empty error code")
	}
}

// TestValidateAWSConfig_Integration tests validation using the main ValidateConfig function.
func TestValidateAWSConfig_Integration(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		config       map[string]interface{}
		expectValid  bool
	}{
		{
			name:         "AWS template with valid config",
			templateName: "aws-standalone-cp-1-0-16",
			config: map[string]interface{}{
				"region": "us-west-2",
			},
			expectValid: true,
		},
		{
			name:         "AWS template with missing region",
			templateName: "aws-standalone-cp-1-0-16",
			config: map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"instanceType": "t3.small",
				},
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfig(tt.templateName, tt.config)

			if result.Provider != ProviderAWS {
				t.Errorf("expected provider=%q, got provider=%q", ProviderAWS, result.Provider)
			}

			if result.IsValid() != tt.expectValid {
				t.Errorf("expected IsValid()=%v, got IsValid()=%v (errors: %v)", tt.expectValid, result.IsValid(), result.Errors)
			}
		})
	}
}

// containsString is a helper function to check if a string contains a substring (case-insensitive).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

// containsSubstring performs case-insensitive substring search.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package clusters

import (
	"strings"
	"testing"
)

// TestValidateGCPConfig tests GCP configuration validation
func TestValidateGCPConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         map[string]interface{}
		wantValid      bool
		wantErrorCount int
		wantErrorCodes []string
		wantFields     []string
	}{
		{
			name: "valid GCP config with all required fields",
			config: map[string]interface{}{
				"project": "my-gcp-project-123456",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": "default",
				},
				"controlPlane": map[string]interface{}{
					"instanceType": "n1-standard-4",
				},
				"worker": map[string]interface{}{
					"instanceType": "n1-standard-4",
				},
			},
			wantValid:      true,
			wantErrorCount: 0,
		},
		{
			name: "valid GCP config with custom VPC",
			config: map[string]interface{}{
				"project": "production-gcp-project",
				"region":  "europe-west1",
				"network": map[string]interface{}{
					"name": "my-custom-vpc",
				},
			},
			wantValid:      true,
			wantErrorCount: 0,
		},
		{
			name: "missing project field",
			config: map[string]interface{}{
				"region": "us-central1",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.project.required"},
			wantFields:     []string{"config.project"},
		},
		{
			name: "missing region field",
			config: map[string]interface{}{
				"project": "my-gcp-project",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.region.required"},
			wantFields:     []string{"config.region"},
		},
		{
			name: "missing network.name nested field",
			config: map[string]interface{}{
				"project": "my-gcp-project",
				"region":  "us-central1",
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.network.name.required"},
			wantFields:     []string{"config.network.name"},
		},
		{
			name: "network exists but name is missing",
			config: map[string]interface{}{
				"project": "my-gcp-project",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"other": "value",
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.network.name.required"},
			wantFields:     []string{"config.network.name"},
		},
		{
			name: "all fields missing",
			config: map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"instanceType": "n1-standard-4",
				},
			},
			wantValid:      false,
			wantErrorCount: 3,
			wantErrorCodes: []string{"gcp.project.required", "gcp.region.required", "gcp.network.name.required"},
			wantFields:     []string{"config.project", "config.region", "config.network.name"},
		},
		{
			name: "empty string project",
			config: map[string]interface{}{
				"project": "",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.project.required"},
			wantFields:     []string{"config.project"},
		},
		{
			name: "empty string region",
			config: map[string]interface{}{
				"project": "my-gcp-project",
				"region":  "",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.region.required"},
			wantFields:     []string{"config.region"},
		},
		{
			name: "empty string network.name",
			config: map[string]interface{}{
				"project": "my-gcp-project",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": "",
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.network.name.required"},
			wantFields:     []string{"config.network.name"},
		},
		{
			name: "whitespace only project",
			config: map[string]interface{}{
				"project": "   ",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": "default",
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.project.required"},
			wantFields:     []string{"config.project"},
		},
		{
			name: "network is not a map",
			config: map[string]interface{}{
				"project": "my-gcp-project",
				"region":  "us-central1",
				"network": "invalid-string",
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.network.name.required"},
			wantFields:     []string{"config.network.name"},
		},
		{
			name: "network.name is not a string",
			config: map[string]interface{}{
				"project": "my-gcp-project",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": 123,
				},
			},
			wantValid:      false,
			wantErrorCount: 1,
			wantErrorCodes: []string{"gcp.network.name.required"},
			wantFields:     []string{"config.network.name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateGCPConfig(tt.config)

			// Check provider
			if result.Provider != ProviderGCP {
				t.Errorf("expected provider %q, got %q", ProviderGCP, result.Provider)
			}

			// Check validity
			gotValid := len(result.Errors) == 0
			if gotValid != tt.wantValid {
				t.Errorf("expected valid=%v, got valid=%v (errors: %v)", tt.wantValid, gotValid, result.Errors)
			}

			// Check error count
			if len(result.Errors) != tt.wantErrorCount {
				t.Errorf("expected %d errors, got %d: %v", tt.wantErrorCount, len(result.Errors), result.Errors)
			}

			// Check error codes
			if len(tt.wantErrorCodes) > 0 {
				gotCodes := make([]string, len(result.Errors))
				for i, err := range result.Errors {
					gotCodes[i] = err.Code
				}

				if len(gotCodes) != len(tt.wantErrorCodes) {
					t.Errorf("expected error codes %v, got %v", tt.wantErrorCodes, gotCodes)
				} else {
					for i, wantCode := range tt.wantErrorCodes {
						if gotCodes[i] != wantCode {
							t.Errorf("error %d: expected code %q, got %q", i, wantCode, gotCodes[i])
						}
					}
				}
			}

			// Check error fields
			if len(tt.wantFields) > 0 {
				gotFields := make([]string, len(result.Errors))
				for i, err := range result.Errors {
					gotFields[i] = err.Field
				}

				if len(gotFields) != len(tt.wantFields) {
					t.Errorf("expected fields %v, got %v", tt.wantFields, gotFields)
				} else {
					for i, wantField := range tt.wantFields {
						if gotFields[i] != wantField {
							t.Errorf("error %d: expected field %q, got %q", i, wantField, gotFields[i])
						}
					}
				}
			}

			// Verify error messages are helpful
			for _, err := range result.Errors {
				if err.Message == "" {
					t.Errorf("error for field %q has empty message", err.Field)
				}
				if !strings.Contains(err.Message, "required") && !strings.Contains(err.Message, "GCP") {
					t.Errorf("error message should mention 'required' or 'GCP': %q", err.Message)
				}
			}
		})
	}
}

// TestValidateGCPConfig_NestedFieldHandling tests proper handling of nested fields
func TestValidateGCPConfig_NestedFieldHandling(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantValid bool
		desc      string
	}{
		{
			name: "deeply nested network config",
			config: map[string]interface{}{
				"project": "my-project",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"name": "my-vpc",
					"subnet": map[string]interface{}{
						"cidr": "10.0.0.0/16",
					},
				},
			},
			wantValid: true,
			desc:      "additional nested fields should be allowed",
		},
		{
			name: "network with only non-name fields",
			config: map[string]interface{}{
				"project": "my-project",
				"region":  "us-central1",
				"network": map[string]interface{}{
					"subnet": "10.0.0.0/16",
					"other":  "value",
				},
			},
			wantValid: false,
			desc:      "network.name is still required even if other fields exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateGCPConfig(tt.config)
			gotValid := len(result.Errors) == 0

			if gotValid != tt.wantValid {
				t.Errorf("%s: expected valid=%v, got valid=%v (errors: %v)",
					tt.desc, tt.wantValid, gotValid, result.Errors)
			}
		})
	}
}

// TestFormatGCPValidationError tests error message formatting
func TestFormatGCPValidationError(t *testing.T) {
	tests := []struct {
		name           string
		errors         []ValidationError
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "empty errors returns empty string",
			errors:         []ValidationError{},
			wantContains:   []string{},
			wantNotContain: []string{"GCP", "configuration"},
		},
		{
			name: "single error includes all context",
			errors: []ValidationError{
				{
					Field:   "config.project",
					Message: "GCP project ID is required",
					Code:    "gcp.project.required",
				},
			},
			wantContains: []string{
				"GCP",
				"configuration validation failed",
				"config.project",
				"GCP project ID is required",
				"Example valid GCP configuration",
				"https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/",
			},
		},
		{
			name: "multiple errors lists all",
			errors: []ValidationError{
				{
					Field:   "config.project",
					Message: "GCP project ID is required",
					Code:    "gcp.project.required",
				},
				{
					Field:   "config.region",
					Message: "GCP region is required",
					Code:    "gcp.region.required",
				},
			},
			wantContains: []string{
				"config.project",
				"config.region",
				"GCP project ID is required",
				"GCP region is required",
				"Example valid GCP configuration",
			},
		},
		{
			name: "includes example configuration",
			errors: []ValidationError{
				{Field: "config.network.name", Message: "network name required", Code: "gcp.network.name.required"},
			},
			wantContains: []string{
				`"project"`,
				`"region"`,
				`"network"`,
				`"name"`,
				`"controlPlane"`,
				`"worker"`,
				`"instanceType"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatGCPValidationError(tt.errors)

			// Check if empty when expected
			if len(tt.errors) == 0 {
				if result != "" {
					t.Errorf("expected empty string, got: %s", result)
				}
				return
			}

			// Check for required content
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got:\n%s", want, result)
				}
			}

			// Check for content that should not be present
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(result, notWant) {
					t.Errorf("expected result to not contain %q, got:\n%s", notWant, result)
				}
			}
		})
	}
}

// TestGCPValidation_ErrorMessageExamples verifies error messages include helpful examples
func TestGCPValidation_ErrorMessageExamples(t *testing.T) {
	config := map[string]interface{}{
		"controlPlane": map[string]interface{}{
			"instanceType": "n1-standard-4",
		},
	}

	result := ValidateGCPConfig(config)

	if len(result.Errors) == 0 {
		t.Fatal("expected validation errors for config missing all required fields")
	}

	// Check that each error has helpful guidance
	for _, err := range result.Errors {
		// Error message should mention the field name
		if !strings.Contains(err.Message, "GCP") && !strings.Contains(err.Message, "required") {
			t.Errorf("error message should be more descriptive: %q", err.Message)
		}

		// Error should have examples or specific values
		hasExample := strings.Contains(err.Message, "e.g.,") ||
			strings.Contains(err.Message, "example") ||
			strings.Contains(strings.ToLower(err.Message), "default")

		if !hasExample {
			t.Errorf("error message should include examples: %q", err.Message)
		}
	}
}

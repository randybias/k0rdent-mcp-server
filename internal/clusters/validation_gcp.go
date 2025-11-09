package clusters

import "fmt"

// ValidateGCPConfig validates GCP-specific cluster configuration.
// It checks for required fields per GCP ClusterDeployment documentation:
// https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/
func ValidateGCPConfig(config map[string]interface{}) ValidationResult {
	result := ValidationResult{
		Provider: ProviderGCP,
	}

	// Check for required project field
	if !hasNonEmptyString(config, "project") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "config.project",
			Message: "GCP project ID is required (e.g., 'my-gcp-project-123456')",
			Code:    "gcp.project.required",
		})
	}

	// Check for required region field
	if !hasNonEmptyString(config, "region") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "config.region",
			Message: "GCP region is required (e.g., 'us-central1', 'us-west1', 'europe-west1')",
			Code:    "gcp.region.required",
		})
	}

	// Check for required network.name nested field
	if !hasNonEmptyNestedString(config, "network.name") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "config.network.name",
			Message: "GCP network name is required (e.g., 'default' or custom VPC name)",
			Code:    "gcp.network.name.required",
		})
	}

	return result
}

// FormatGCPValidationError formats validation errors for GCP configurations.
// It provides helpful error messages with examples of correct configuration.
func FormatGCPValidationError(errors []ValidationError) string {
	if len(errors) == 0 {
		return ""
	}

	msg := "GCP cluster configuration validation failed:\n"
	for _, err := range errors {
		msg += fmt.Sprintf("  - %s: %s\n", err.Field, err.Message)
	}

	msg += "\nExample valid GCP configuration:\n"
	msg += `{
  "project": "my-gcp-project-123456",
  "region": "us-central1",
  "network": {
    "name": "default"
  },
  "controlPlane": {
    "instanceType": "n1-standard-4"
  },
  "worker": {
    "instanceType": "n1-standard-4"
  }
}

For more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-gcp/`

	return msg
}

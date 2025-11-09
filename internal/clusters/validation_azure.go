package clusters

import "fmt"

// ValidateAzureConfig validates Azure-specific cluster configuration.
// It checks for required fields per Azure ClusterDeployment documentation:
// https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/
func ValidateAzureConfig(config map[string]interface{}) ValidationResult {
	result := ValidationResult{
		Provider: ProviderAzure,
	}

	// Check for required location field
	if !hasNonEmptyString(config, "location") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "config.location",
			Message: "Azure location is required (e.g., 'westus2', 'eastus', 'centralus')",
			Code:    "azure.location.required",
		})
	}

	// Check for required subscriptionID field
	if !hasNonEmptyString(config, "subscriptionID") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "config.subscriptionID",
			Message: "Azure subscription ID is required (e.g., '12345678-1234-1234-1234-123456789abc')",
			Code:    "azure.subscriptionID.required",
		})
	}

	return result
}

// FormatAzureValidationError formats validation errors for Azure configurations.
// It provides helpful error messages with examples of correct configuration.
func FormatAzureValidationError(errors []ValidationError) string {
	if len(errors) == 0 {
		return ""
	}

	msg := "Azure cluster configuration validation failed:\n"
	for _, err := range errors {
		msg += fmt.Sprintf("  - %s: %s\n", err.Field, err.Message)
	}

	msg += "\nExample valid Azure configuration:\n"
	msg += `{
  "location": "westus2",
  "subscriptionID": "12345678-1234-1234-1234-123456789abc",
  "controlPlane": {
    "vmSize": "Standard_A4_v2"
  },
  "worker": {
    "vmSize": "Standard_A4_v2"
  }
}

For more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-azure/`

	return msg
}

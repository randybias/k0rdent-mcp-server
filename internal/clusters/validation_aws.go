package clusters

import "fmt"

// ValidateAWSConfig validates AWS-specific configuration requirements.
// It checks for required fields according to k0rdent AWS documentation:
// https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/
//
// Required fields:
//   - config.region (string, non-empty) - AWS region for the cluster
//
// Example valid config:
//
//	{
//	  "region": "us-west-2",
//	  "controlPlane": {"instanceType": "t3.small"},
//	  "worker": {"instanceType": "t3.small"}
//	}
func ValidateAWSConfig(config map[string]interface{}) ValidationResult {
	result := ValidationResult{
		Provider: ProviderAWS,
	}

	// Check required: region
	if !hasNonEmptyString(config, "region") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "config.region",
			Message: "AWS region is required (e.g., 'us-west-2', 'us-east-1', 'eu-west-1')",
			Code:    "aws.region.required",
		})
	}

	return result
}

// FormatAWSValidationError formats validation errors for AWS configurations.
// It provides helpful error messages with examples of correct configuration.
func FormatAWSValidationError(errors []ValidationError) string {
	if len(errors) == 0 {
		return ""
	}

	msg := "AWS cluster configuration validation failed:\n"
	for _, err := range errors {
		msg += fmt.Sprintf("  - %s: %s\n", err.Field, err.Message)
	}

	msg += "\nExample valid AWS configuration:\n"
	msg += `{
  "region": "us-west-2",
  "controlPlane": {
    "instanceType": "t3.small"
  },
  "worker": {
    "instanceType": "t3.small"
  }
}

For more information, see: https://docs.k0rdent.io/latest/quickstarts/quickstart-2-aws/`

	return msg
}

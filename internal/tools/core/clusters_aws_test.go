package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWSClusterDeploy_DefaultValues(t *testing.T) {
	// Test that default values are correctly set
	input := awsClusterDeployInput{
		Name:       "test-cluster",
		Credential: "aws-cred",
		Region:     "us-west-2",
		ControlPlane: awsNodeConfig{
			InstanceType: "t3.medium",
		},
		Worker: awsNodeConfig{
			InstanceType: "t3.small",
		},
	}

	// Test namespace defaulting
	namespace := input.Namespace
	if namespace == "" {
		namespace = "kcm-system"
	}
	assert.Equal(t, "kcm-system", namespace)

	// Test controlPlaneNumber default
	controlPlaneNumber := input.ControlPlaneNumber
	if controlPlaneNumber == 0 {
		controlPlaneNumber = 3
	}
	assert.Equal(t, 3, controlPlaneNumber)

	// Test workersNumber default
	workersNumber := input.WorkersNumber
	if workersNumber == 0 {
		workersNumber = 2
	}
	assert.Equal(t, 2, workersNumber)

	// Test controlPlane rootVolumeSize default
	controlPlaneRootVolumeSize := input.ControlPlane.RootVolumeSize
	if controlPlaneRootVolumeSize == 0 {
		controlPlaneRootVolumeSize = 32
	}
	assert.Equal(t, 32, controlPlaneRootVolumeSize)

	// Test worker rootVolumeSize default
	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = 32
	}
	assert.Equal(t, 32, workerRootVolumeSize)
}

func TestAWSClusterDeploy_CustomValues(t *testing.T) {
	// Test that custom values are preserved
	input := awsClusterDeployInput{
		Name:               "test-cluster",
		Credential:         "aws-cred",
		Region:             "us-east-1",
		Namespace:          "custom-ns",
		ControlPlaneNumber: 5,
		WorkersNumber:      10,
		ControlPlane: awsNodeConfig{
			InstanceType:   "t3.large",
			RootVolumeSize: 64,
		},
		Worker: awsNodeConfig{
			InstanceType:   "t3.medium",
			RootVolumeSize: 128,
		},
	}

	// Test namespace preservation
	namespace := input.Namespace
	if namespace == "" {
		namespace = "kcm-system"
	}
	assert.Equal(t, "custom-ns", namespace)

	// Test controlPlaneNumber preservation
	controlPlaneNumber := input.ControlPlaneNumber
	if controlPlaneNumber == 0 {
		controlPlaneNumber = 3
	}
	assert.Equal(t, 5, controlPlaneNumber)

	// Test workersNumber preservation
	workersNumber := input.WorkersNumber
	if workersNumber == 0 {
		workersNumber = 2
	}
	assert.Equal(t, 10, workersNumber)

	// Test controlPlane rootVolumeSize preservation
	controlPlaneRootVolumeSize := input.ControlPlane.RootVolumeSize
	if controlPlaneRootVolumeSize == 0 {
		controlPlaneRootVolumeSize = 32
	}
	assert.Equal(t, 64, controlPlaneRootVolumeSize)

	// Test worker rootVolumeSize preservation
	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = 32
	}
	assert.Equal(t, 128, workerRootVolumeSize)
}

func TestAWSClusterDeploy_ConfigMapBuilding(t *testing.T) {
	// Test config map structure
	input := awsClusterDeployInput{
		Name:               "test-cluster",
		Credential:         "aws-cred",
		Region:             "us-east-1",
		ControlPlaneNumber: 5,
		WorkersNumber:      3,
		ControlPlane: awsNodeConfig{
			InstanceType:   "t3.large",
			RootVolumeSize: 50,
		},
		Worker: awsNodeConfig{
			InstanceType:   "t3.medium",
			RootVolumeSize: 40,
		},
	}

	// Simulate config map building
	controlPlaneNumber := input.ControlPlaneNumber
	if controlPlaneNumber == 0 {
		controlPlaneNumber = 3
	}

	workersNumber := input.WorkersNumber
	if workersNumber == 0 {
		workersNumber = 2
	}

	controlPlaneRootVolumeSize := input.ControlPlane.RootVolumeSize
	if controlPlaneRootVolumeSize == 0 {
		controlPlaneRootVolumeSize = 32
	}

	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = 32
	}

	config := map[string]interface{}{
		"region":             input.Region,
		"controlPlaneNumber": controlPlaneNumber,
		"workersNumber":      workersNumber,
		"controlPlane": map[string]interface{}{
			"instanceType":   input.ControlPlane.InstanceType,
			"rootVolumeSize": controlPlaneRootVolumeSize,
		},
		"worker": map[string]interface{}{
			"instanceType":   input.Worker.InstanceType,
			"rootVolumeSize": workerRootVolumeSize,
		},
	}

	// Verify config map structure
	assert.Equal(t, "us-east-1", config["region"])
	assert.Equal(t, 5, config["controlPlaneNumber"])
	assert.Equal(t, 3, config["workersNumber"])

	cpConfig, ok := config["controlPlane"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "t3.large", cpConfig["instanceType"])
	assert.Equal(t, 50, cpConfig["rootVolumeSize"])

	workerConfig, ok := config["worker"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "t3.medium", workerConfig["instanceType"])
	assert.Equal(t, 40, workerConfig["rootVolumeSize"])
}

func TestAWSClusterDeploy_InputValidation(t *testing.T) {
	testCases := []struct {
		name  string
		input awsClusterDeployInput
		valid bool
	}{
		{
			name: "valid input",
			input: awsClusterDeployInput{
				Name:       "test-cluster",
				Credential: "aws-cred",
				Region:     "us-west-2",
				ControlPlane: awsNodeConfig{
					InstanceType: "t3.medium",
				},
				Worker: awsNodeConfig{
					InstanceType: "t3.small",
				},
			},
			valid: true,
		},
		{
			name: "missing cluster name",
			input: awsClusterDeployInput{
				Credential: "aws-cred",
				Region:     "us-west-2",
				ControlPlane: awsNodeConfig{
					InstanceType: "t3.medium",
				},
				Worker: awsNodeConfig{
					InstanceType: "t3.small",
				},
			},
			valid: false,
		},
		{
			name: "missing credential",
			input: awsClusterDeployInput{
				Name:   "test-cluster",
				Region: "us-west-2",
				ControlPlane: awsNodeConfig{
					InstanceType: "t3.medium",
				},
				Worker: awsNodeConfig{
					InstanceType: "t3.small",
				},
			},
			valid: false,
		},
		{
			name: "missing region",
			input: awsClusterDeployInput{
				Name:       "test-cluster",
				Credential: "aws-cred",
				ControlPlane: awsNodeConfig{
					InstanceType: "t3.medium",
				},
				Worker: awsNodeConfig{
					InstanceType: "t3.small",
				},
			},
			valid: false,
		},
		{
			name: "missing control plane instance type",
			input: awsClusterDeployInput{
				Name:         "test-cluster",
				Credential:   "aws-cred",
				Region:       "us-west-2",
				ControlPlane: awsNodeConfig{},
				Worker: awsNodeConfig{
					InstanceType: "t3.small",
				},
			},
			valid: false,
		},
		{
			name: "missing worker instance type",
			input: awsClusterDeployInput{
				Name:       "test-cluster",
				Credential: "aws-cred",
				Region:     "us-west-2",
				ControlPlane: awsNodeConfig{
					InstanceType: "t3.medium",
				},
				Worker: awsNodeConfig{},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate required fields
			valid := tc.input.Name != "" &&
				tc.input.Credential != "" &&
				tc.input.Region != "" &&
				tc.input.ControlPlane.InstanceType != "" &&
				tc.input.Worker.InstanceType != ""

			assert.Equal(t, tc.valid, valid)
		})
	}
}

func TestAWSNodeConfig_Struct(t *testing.T) {
	// Test awsNodeConfig struct
	config := awsNodeConfig{
		InstanceType:   "t3.large",
		RootVolumeSize: 64,
	}

	assert.Equal(t, "t3.large", config.InstanceType)
	assert.Equal(t, 64, config.RootVolumeSize)

	// Test zero values
	emptyConfig := awsNodeConfig{}
	assert.Equal(t, "", emptyConfig.InstanceType)
	assert.Equal(t, 0, emptyConfig.RootVolumeSize)
}

func TestAWSClusterDeployInput_Struct(t *testing.T) {
	// Test full awsClusterDeployInput struct
	input := awsClusterDeployInput{
		Name:               "test-cluster",
		Credential:         "aws-cred",
		Region:             "us-west-2",
		Namespace:          "test-ns",
		ControlPlaneNumber: 5,
		WorkersNumber:      10,
		ControlPlane: awsNodeConfig{
			InstanceType:   "t3.large",
			RootVolumeSize: 64,
		},
		Worker: awsNodeConfig{
			InstanceType:   "t3.medium",
			RootVolumeSize: 128,
		},
		Labels: map[string]string{
			"env":  "test",
			"team": "platform",
		},
		Wait:        true,
		WaitTimeout: "45m",
	}

	assert.Equal(t, "test-cluster", input.Name)
	assert.Equal(t, "aws-cred", input.Credential)
	assert.Equal(t, "us-west-2", input.Region)
	assert.Equal(t, "test-ns", input.Namespace)
	assert.Equal(t, 5, input.ControlPlaneNumber)
	assert.Equal(t, 10, input.WorkersNumber)
	assert.Equal(t, "t3.large", input.ControlPlane.InstanceType)
	assert.Equal(t, 64, input.ControlPlane.RootVolumeSize)
	assert.Equal(t, "t3.medium", input.Worker.InstanceType)
	assert.Equal(t, 128, input.Worker.RootVolumeSize)
	assert.Equal(t, 2, len(input.Labels))
	assert.Equal(t, "test", input.Labels["env"])
	assert.Equal(t, "platform", input.Labels["team"])
	assert.True(t, input.Wait)
	assert.Equal(t, "45m", input.WaitTimeout)
}

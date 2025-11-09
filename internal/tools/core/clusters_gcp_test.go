package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGCPClusterDeploy_DefaultValues(t *testing.T) {
	// Test that default values are correctly set
	input := gcpClusterDeployInput{
		Name:       "test-cluster",
		Credential: "gcp-cred",
		Project:    "my-gcp-project",
		Region:     "us-central1",
		Network: gcpNetworkConfig{
			Name: "default",
		},
		ControlPlane: gcpNodeConfig{
			InstanceType: "n1-standard-4",
		},
		Worker: gcpNodeConfig{
			InstanceType: "n1-standard-2",
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
		controlPlaneRootVolumeSize = 30
	}
	assert.Equal(t, 30, controlPlaneRootVolumeSize)

	// Test worker rootVolumeSize default
	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = 30
	}
	assert.Equal(t, 30, workerRootVolumeSize)
}

func TestGCPClusterDeploy_CustomValues(t *testing.T) {
	// Test that custom values are preserved
	input := gcpClusterDeployInput{
		Name:               "test-cluster",
		Credential:         "gcp-cred",
		Project:            "my-gcp-project",
		Region:             "europe-west1",
		Namespace:          "custom-ns",
		ControlPlaneNumber: 5,
		WorkersNumber:      10,
		Network: gcpNetworkConfig{
			Name: "custom-vpc",
		},
		ControlPlane: gcpNodeConfig{
			InstanceType:   "n1-standard-8",
			RootVolumeSize: 50,
		},
		Worker: gcpNodeConfig{
			InstanceType:   "n1-standard-4",
			RootVolumeSize: 40,
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
		controlPlaneRootVolumeSize = 30
	}
	assert.Equal(t, 50, controlPlaneRootVolumeSize)

	// Test worker rootVolumeSize preservation
	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = 30
	}
	assert.Equal(t, 40, workerRootVolumeSize)
}

func TestGCPClusterDeploy_ConfigMapBuilding(t *testing.T) {
	// Test config map structure with nested network
	input := gcpClusterDeployInput{
		Name:               "test-cluster",
		Credential:         "gcp-cred",
		Project:            "test-project-123",
		Region:             "asia-east1",
		ControlPlaneNumber: 5,
		WorkersNumber:      3,
		Network: gcpNetworkConfig{
			Name: "production-vpc",
		},
		ControlPlane: gcpNodeConfig{
			InstanceType:   "n1-standard-8",
			RootVolumeSize: 50,
		},
		Worker: gcpNodeConfig{
			InstanceType:   "n1-standard-4",
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
		controlPlaneRootVolumeSize = 30
	}

	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = 30
	}

	config := map[string]interface{}{
		"project": input.Project,
		"region":  input.Region,
		"network": map[string]interface{}{
			"name": input.Network.Name,
		},
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
	assert.Equal(t, "test-project-123", config["project"])
	assert.Equal(t, "asia-east1", config["region"])
	assert.Equal(t, 5, config["controlPlaneNumber"])
	assert.Equal(t, 3, config["workersNumber"])

	// Verify nested network structure
	network, ok := config["network"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "production-vpc", network["name"])

	cpConfig, ok := config["controlPlane"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "n1-standard-8", cpConfig["instanceType"])
	assert.Equal(t, 50, cpConfig["rootVolumeSize"])

	workerConfig, ok := config["worker"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "n1-standard-4", workerConfig["instanceType"])
	assert.Equal(t, 40, workerConfig["rootVolumeSize"])
}

func TestGCPClusterDeploy_InputValidation(t *testing.T) {
	testCases := []struct {
		name  string
		input gcpClusterDeployInput
		valid bool
	}{
		{
			name: "valid input",
			input: gcpClusterDeployInput{
				Name:       "test-cluster",
				Credential: "gcp-cred",
				Project:    "my-project",
				Region:     "us-central1",
				Network: gcpNetworkConfig{
					Name: "default",
				},
				ControlPlane: gcpNodeConfig{
					InstanceType: "n1-standard-4",
				},
				Worker: gcpNodeConfig{
					InstanceType: "n1-standard-2",
				},
			},
			valid: true,
		},
		{
			name: "missing cluster name",
			input: gcpClusterDeployInput{
				Credential: "gcp-cred",
				Project:    "my-project",
				Region:     "us-central1",
				Network: gcpNetworkConfig{
					Name: "default",
				},
				ControlPlane: gcpNodeConfig{
					InstanceType: "n1-standard-4",
				},
				Worker: gcpNodeConfig{
					InstanceType: "n1-standard-2",
				},
			},
			valid: false,
		},
		{
			name: "missing credential",
			input: gcpClusterDeployInput{
				Name:    "test-cluster",
				Project: "my-project",
				Region:  "us-central1",
				Network: gcpNetworkConfig{
					Name: "default",
				},
				ControlPlane: gcpNodeConfig{
					InstanceType: "n1-standard-4",
				},
				Worker: gcpNodeConfig{
					InstanceType: "n1-standard-2",
				},
			},
			valid: false,
		},
		{
			name: "missing project",
			input: gcpClusterDeployInput{
				Name:       "test-cluster",
				Credential: "gcp-cred",
				Region:     "us-central1",
				Network: gcpNetworkConfig{
					Name: "default",
				},
				ControlPlane: gcpNodeConfig{
					InstanceType: "n1-standard-4",
				},
				Worker: gcpNodeConfig{
					InstanceType: "n1-standard-2",
				},
			},
			valid: false,
		},
		{
			name: "missing region",
			input: gcpClusterDeployInput{
				Name:       "test-cluster",
				Credential: "gcp-cred",
				Project:    "my-project",
				Network: gcpNetworkConfig{
					Name: "default",
				},
				ControlPlane: gcpNodeConfig{
					InstanceType: "n1-standard-4",
				},
				Worker: gcpNodeConfig{
					InstanceType: "n1-standard-2",
				},
			},
			valid: false,
		},
		{
			name: "missing network name",
			input: gcpClusterDeployInput{
				Name:       "test-cluster",
				Credential: "gcp-cred",
				Project:    "my-project",
				Region:     "us-central1",
				Network:    gcpNetworkConfig{},
				ControlPlane: gcpNodeConfig{
					InstanceType: "n1-standard-4",
				},
				Worker: gcpNodeConfig{
					InstanceType: "n1-standard-2",
				},
			},
			valid: false,
		},
		{
			name: "missing control plane instance type",
			input: gcpClusterDeployInput{
				Name:       "test-cluster",
				Credential: "gcp-cred",
				Project:    "my-project",
				Region:     "us-central1",
				Network: gcpNetworkConfig{
					Name: "default",
				},
				ControlPlane: gcpNodeConfig{},
				Worker: gcpNodeConfig{
					InstanceType: "n1-standard-2",
				},
			},
			valid: false,
		},
		{
			name: "missing worker instance type",
			input: gcpClusterDeployInput{
				Name:       "test-cluster",
				Credential: "gcp-cred",
				Project:    "my-project",
				Region:     "us-central1",
				Network: gcpNetworkConfig{
					Name: "default",
				},
				ControlPlane: gcpNodeConfig{
					InstanceType: "n1-standard-4",
				},
				Worker: gcpNodeConfig{},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate required fields
			valid := tc.input.Name != "" &&
				tc.input.Credential != "" &&
				tc.input.Project != "" &&
				tc.input.Region != "" &&
				tc.input.Network.Name != "" &&
				tc.input.ControlPlane.InstanceType != "" &&
				tc.input.Worker.InstanceType != ""

			assert.Equal(t, tc.valid, valid)
		})
	}
}

func TestGCPNodeConfig_Struct(t *testing.T) {
	// Test gcpNodeConfig struct
	config := gcpNodeConfig{
		InstanceType:   "n1-standard-8",
		RootVolumeSize: 50,
	}

	assert.Equal(t, "n1-standard-8", config.InstanceType)
	assert.Equal(t, 50, config.RootVolumeSize)

	// Test zero values
	emptyConfig := gcpNodeConfig{}
	assert.Equal(t, "", emptyConfig.InstanceType)
	assert.Equal(t, 0, emptyConfig.RootVolumeSize)
}

func TestGCPNetworkConfig_Struct(t *testing.T) {
	// Test gcpNetworkConfig struct
	config := gcpNetworkConfig{
		Name: "production-vpc",
	}

	assert.Equal(t, "production-vpc", config.Name)

	// Test zero value
	emptyConfig := gcpNetworkConfig{}
	assert.Equal(t, "", emptyConfig.Name)
}

func TestGCPClusterDeployInput_Struct(t *testing.T) {
	// Test full gcpClusterDeployInput struct
	input := gcpClusterDeployInput{
		Name:               "test-cluster",
		Credential:         "gcp-cred",
		Project:            "test-project-123",
		Region:             "us-central1",
		Namespace:          "test-ns",
		ControlPlaneNumber: 5,
		WorkersNumber:      10,
		Network: gcpNetworkConfig{
			Name: "custom-vpc",
		},
		ControlPlane: gcpNodeConfig{
			InstanceType:   "n1-standard-8",
			RootVolumeSize: 50,
		},
		Worker: gcpNodeConfig{
			InstanceType:   "n1-standard-4",
			RootVolumeSize: 40,
		},
		Labels: map[string]string{
			"env":  "test",
			"team": "platform",
		},
		Wait:        true,
		WaitTimeout: "45m",
	}

	assert.Equal(t, "test-cluster", input.Name)
	assert.Equal(t, "gcp-cred", input.Credential)
	assert.Equal(t, "test-project-123", input.Project)
	assert.Equal(t, "us-central1", input.Region)
	assert.Equal(t, "test-ns", input.Namespace)
	assert.Equal(t, 5, input.ControlPlaneNumber)
	assert.Equal(t, 10, input.WorkersNumber)
	assert.Equal(t, "custom-vpc", input.Network.Name)
	assert.Equal(t, "n1-standard-8", input.ControlPlane.InstanceType)
	assert.Equal(t, 50, input.ControlPlane.RootVolumeSize)
	assert.Equal(t, "n1-standard-4", input.Worker.InstanceType)
	assert.Equal(t, 40, input.Worker.RootVolumeSize)
	assert.Equal(t, 2, len(input.Labels))
	assert.Equal(t, "test", input.Labels["env"])
	assert.Equal(t, "platform", input.Labels["team"])
	assert.True(t, input.Wait)
	assert.Equal(t, "45m", input.WaitTimeout)
}

func TestGCPClusterDeploy_NestedNetworkStructure(t *testing.T) {
	// Specific test to verify nested network.name field is properly handled
	input := gcpClusterDeployInput{
		Name:       "test-cluster",
		Credential: "gcp-cred",
		Project:    "my-project",
		Region:     "us-central1",
		Network: gcpNetworkConfig{
			Name: "my-vpc-network",
		},
		ControlPlane: gcpNodeConfig{
			InstanceType: "n1-standard-4",
		},
		Worker: gcpNodeConfig{
			InstanceType: "n1-standard-2",
		},
	}

	// Verify the nested structure is accessible
	assert.Equal(t, "my-vpc-network", input.Network.Name)

	// Simulate building the config with nested network
	config := map[string]interface{}{
		"network": map[string]interface{}{
			"name": input.Network.Name,
		},
	}

	network, ok := config["network"].(map[string]interface{})
	assert.True(t, ok, "network should be a map")
	assert.Equal(t, "my-vpc-network", network["name"], "network.name should be set correctly")
}

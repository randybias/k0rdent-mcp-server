package core

import (
	"context"
	"fmt"
	"time"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// awsClusterDeployTool handles AWS-specific cluster deployments
type awsClusterDeployTool struct {
	session *runtime.Session
}

// awsClusterDeployInput defines the input schema for AWS cluster deployment
type awsClusterDeployInput struct {
	Name               string            `json:"name" jsonschema:"Cluster deployment name"`
	Credential         string            `json:"credential" jsonschema:"AWS credential name"`
	Region             string            `json:"region" jsonschema:"AWS region (e.g. us-west-2, us-east-1, eu-west-1)"`
	ControlPlane       awsNodeConfig     `json:"controlPlane" jsonschema:"Control plane node configuration"`
	Worker             awsNodeConfig     `json:"worker" jsonschema:"Worker node configuration"`
	ControlPlaneNumber int               `json:"controlPlaneNumber,omitempty" jsonschema:"Number of control plane nodes (default: 3)"`
	WorkersNumber      int               `json:"workersNumber,omitempty" jsonschema:"Number of worker nodes (default: 2)"`
	Namespace          string            `json:"namespace,omitempty" jsonschema:"Deployment namespace (default: kcm-system)"`
	Labels             map[string]string `json:"labels,omitempty" jsonschema:"Labels for the cluster"`
	Wait               bool              `json:"wait,omitempty" jsonschema:"Wait for cluster to be ready before returning"`
	WaitTimeout        string            `json:"waitTimeout,omitempty" jsonschema:"Maximum time to wait for cluster ready (default: 30m)"`
}

// awsNodeConfig defines node configuration for AWS instances
type awsNodeConfig struct {
	InstanceType   string `json:"instanceType" jsonschema:"EC2 instance type (e.g. t3.small, t3.medium, m5.large)"`
	RootVolumeSize int    `json:"rootVolumeSize,omitempty" jsonschema:"Root volume size in GB (default: 32)"`
}

// awsClusterDeployResult is the result of an AWS cluster deployment
type awsClusterDeployResult clusters.DeployResult

// deploy handles the AWS cluster deployment request
func (t *awsClusterDeployTool) deploy(ctx context.Context, req *mcp.CallToolRequest, input awsClusterDeployInput) (*mcp.CallToolResult, awsClusterDeployResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters.aws")
	start := time.Now()

	logger.Info("deploying AWS cluster",
		"tool", name,
		"name", input.Name,
		"region", input.Region,
		"credential", input.Credential,
	)

	// Validate required fields (belt and suspenders - MCP should validate too)
	if input.Name == "" {
		return nil, awsClusterDeployResult{}, fmt.Errorf("cluster name is required")
	}
	if input.Credential == "" {
		return nil, awsClusterDeployResult{}, fmt.Errorf("credential is required")
	}
	if input.Region == "" {
		return nil, awsClusterDeployResult{}, fmt.Errorf("region is required")
	}
	if input.ControlPlane.InstanceType == "" {
		return nil, awsClusterDeployResult{}, fmt.Errorf("control plane instance type is required")
	}
	if input.Worker.InstanceType == "" {
		return nil, awsClusterDeployResult{}, fmt.Errorf("worker instance type is required")
	}

	// Default namespace to kcm-system if not specified
	namespace := input.Namespace
	if namespace == "" {
		namespace = "kcm-system"
	}

	// Auto-select latest AWS template
	// Note: This will call SelectLatestTemplate once it's implemented by the template selection agent
	template, err := t.session.Clusters.SelectLatestTemplate(ctx, "aws", namespace)
	if err != nil {
		logger.Error("failed to select AWS template", "tool", name, "error", err)
		return nil, awsClusterDeployResult{}, fmt.Errorf("select template: %w", err)
	}

	logger.Debug("selected AWS template", "tool", name, "template", template)

	// Validate and apply defaults for node counts
	controlPlaneNumber, workersNumber, err := validateAndDefaultNodeCounts(input.ControlPlaneNumber, input.WorkersNumber)
	if err != nil {
		return nil, awsClusterDeployResult{}, err
	}

	// Apply defaults for volume sizes
	controlPlaneRootVolumeSize := input.ControlPlane.RootVolumeSize
	if controlPlaneRootVolumeSize == 0 {
		controlPlaneRootVolumeSize = defaultAWSRootVolumeSize
	}

	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = defaultAWSRootVolumeSize
	}

	// Build config map from structured input
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

	logger.Debug("built AWS config",
		"tool", name,
		"region", input.Region,
		"controlPlaneNumber", controlPlaneNumber,
		"workersNumber", workersNumber,
	)

	// Create generic deploy request
	deployReq := clusters.DeployRequest{
		Name:       input.Name,
		Template:   template,
		Credential: input.Credential,
		Namespace:  namespace,
		Labels:     input.Labels,
		Config:     config,
	}

	// Call existing deploy logic (reuses validation!)
	result, err := t.session.Clusters.DeployCluster(ctx, namespace, deployReq)
	if err != nil {
		logger.Error("failed to deploy AWS cluster", "tool", name, "error", err)
		return nil, awsClusterDeployResult{}, err
	}

	awsResult := awsClusterDeployResult(result)

	// If wait is requested, monitor the cluster until ready or timeout
	if input.Wait {
		logger.Info("waiting for AWS cluster to be ready",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", namespace,
		)

		// Parse wait timeout with default
		waitTimeout := 30 * time.Minute
		if input.WaitTimeout != "" {
			if d, err := time.ParseDuration(input.WaitTimeout); err == nil {
				waitTimeout = d
			} else {
				logger.Warn("invalid waitTimeout, using default", "input", input.WaitTimeout, "default", waitTimeout)
			}
		}

		// Use the shared wait helper
		waitHelper := &clusterWaitHelper{session: t.session}
		ready, err := waitHelper.waitForClusterReady(
			ctx,
			namespace,
			input.Name,
			30*time.Second, // pollInterval
			waitTimeout,    // provisionTimeout
			10*time.Minute, // stallThreshold
			logger,
		)
		if err != nil {
			logger.Error("error while waiting for AWS cluster", "tool", name, "error", err)
			return nil, awsClusterDeployResult{}, fmt.Errorf("wait for cluster ready: %w", err)
		}

		if !ready {
			logger.Warn("AWS cluster did not become ready within timeout",
				"tool", name,
				"cluster_name", input.Name,
				"timeout", waitTimeout,
			)
			return nil, awsClusterDeployResult{}, fmt.Errorf("cluster %s did not become ready within %v", input.Name, waitTimeout)
		}

		logger.Info("AWS cluster is ready",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", namespace,
		)
	}

	logger.Info("AWS cluster deployment completed",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", namespace,
		"status", awsResult.Status,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, awsResult, nil
}

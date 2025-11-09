package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// gcpClusterDeployTool implements GCP-specific cluster deployment
type gcpClusterDeployTool struct {
	session *runtime.Session
}

// gcpClusterDeployInput defines the input parameters for GCP cluster deployment
type gcpClusterDeployInput struct {
	Name               string            `json:"name" jsonschema:"Cluster deployment name"`
	Credential         string            `json:"credential" jsonschema:"GCP credential name"`
	Project            string            `json:"project" jsonschema:"GCP project ID"`
	Region             string            `json:"region" jsonschema:"GCP region (e.g. us-central1, us-west1, europe-west1)"`
	Network            gcpNetworkConfig  `json:"network" jsonschema:"VPC network configuration"`
	ControlPlane       gcpNodeConfig     `json:"controlPlane" jsonschema:"Control plane node configuration"`
	Worker             gcpNodeConfig     `json:"worker" jsonschema:"Worker node configuration"`
	ControlPlaneNumber int               `json:"controlPlaneNumber,omitempty" jsonschema:"Number of control plane nodes (default: 3)"`
	WorkersNumber      int               `json:"workersNumber,omitempty" jsonschema:"Number of worker nodes (default: 2)"`
	Namespace          string            `json:"namespace,omitempty" jsonschema:"Deployment namespace (default: kcm-system)"`
	Labels             map[string]string `json:"labels,omitempty" jsonschema:"Labels for the cluster"`
	Wait               bool              `json:"wait,omitempty" jsonschema:"Wait for cluster to be ready before returning"`
	WaitTimeout        string            `json:"waitTimeout,omitempty" jsonschema:"Maximum time to wait for cluster ready (default: 30m)"`
}

// gcpNodeConfig defines GCP-specific node configuration
type gcpNodeConfig struct {
	InstanceType   string `json:"instanceType" jsonschema:"GCE instance type (e.g. n1-standard-4, n1-standard-8, n2-standard-4)"`
	RootVolumeSize int    `json:"rootVolumeSize,omitempty" jsonschema:"Root volume size in GB (default: 30)"`
}

// gcpNetworkConfig defines GCP network configuration
type gcpNetworkConfig struct {
	Name string `json:"name" jsonschema:"VPC network name (e.g. default, custom-vpc)"`
}

// gcpClusterDeployResult is the result of a GCP cluster deployment
type gcpClusterDeployResult clusters.DeployResult

// deploy handles the GCP cluster deployment
func (t *gcpClusterDeployTool) deploy(ctx context.Context, req *mcp.CallToolRequest, input gcpClusterDeployInput) (*mcp.CallToolResult, gcpClusterDeployResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters.gcp")
	start := time.Now()

	logger.Debug("deploying GCP cluster",
		"tool", name,
		"cluster_name", input.Name,
		"project", input.Project,
		"region", input.Region,
		"credential", input.Credential,
		"namespace", input.Namespace,
	)

	// Validate required fields
	if input.Name == "" {
		return nil, gcpClusterDeployResult{}, fmt.Errorf("cluster name is required")
	}
	if input.Credential == "" {
		return nil, gcpClusterDeployResult{}, fmt.Errorf("credential is required")
	}
	if input.Project == "" {
		return nil, gcpClusterDeployResult{}, fmt.Errorf("project is required")
	}
	if input.Region == "" {
		return nil, gcpClusterDeployResult{}, fmt.Errorf("region is required")
	}
	if input.Network.Name == "" {
		return nil, gcpClusterDeployResult{}, fmt.Errorf("network.name is required")
	}
	if input.ControlPlane.InstanceType == "" {
		return nil, gcpClusterDeployResult{}, fmt.Errorf("controlPlane.instanceType is required")
	}
	if input.Worker.InstanceType == "" {
		return nil, gcpClusterDeployResult{}, fmt.Errorf("worker.instanceType is required")
	}

	// Validate and apply defaults for node counts
	controlPlaneNumber, workersNumber, err := validateAndDefaultNodeCounts(input.ControlPlaneNumber, input.WorkersNumber)
	if err != nil {
		return nil, gcpClusterDeployResult{}, err
	}
	input.ControlPlaneNumber = controlPlaneNumber
	input.WorkersNumber = workersNumber

	// Apply defaults for volume sizes
	if input.ControlPlane.RootVolumeSize == 0 {
		input.ControlPlane.RootVolumeSize = defaultGCPRootVolumeSize
	}
	if input.Worker.RootVolumeSize == 0 {
		input.Worker.RootVolumeSize = defaultGCPRootVolumeSize
	}

	// Resolve target namespace
	targetNamespace, err := t.resolveDeployNamespace(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve deploy namespace", "tool", name, "error", err)
		return nil, gcpClusterDeployResult{}, fmt.Errorf("resolve namespace: %w", err)
	}

	logger.Debug("resolved deploy namespace", "tool", name, "namespace", targetNamespace)

	// Select latest GCP template
	template, err := t.session.Clusters.SelectLatestTemplate(ctx, "gcp", targetNamespace)
	if err != nil {
		logger.Error("failed to select GCP template", "tool", name, "error", err)
		return nil, gcpClusterDeployResult{}, fmt.Errorf("select GCP template: %w", err)
	}

	logger.Debug("selected GCP template", "tool", name, "template", template, "namespace", targetNamespace)

	// Build config map with GCP-specific fields including nested network structure
	config := map[string]any{
		"project": input.Project,
		"region":  input.Region,
		"network": map[string]any{
			"name": input.Network.Name,
		},
		"controlPlane": map[string]any{
			"instanceType":   input.ControlPlane.InstanceType,
			"rootVolumeSize": input.ControlPlane.RootVolumeSize,
		},
		"worker": map[string]any{
			"instanceType":   input.Worker.InstanceType,
			"rootVolumeSize": input.Worker.RootVolumeSize,
		},
		"controlPlaneNumber": input.ControlPlaneNumber,
		"workersNumber":      input.WorkersNumber,
	}

	// Build deploy request
	deployReq := clusters.DeployRequest{
		Name:       input.Name,
		Template:   template,
		Credential: input.Credential,
		Namespace:  targetNamespace,
		Labels:     input.Labels,
		Config:     config,
	}

	// Deploy cluster using cluster manager
	deployResult, err := t.session.Clusters.DeployCluster(ctx, targetNamespace, deployReq)
	if err != nil {
		logger.Error("failed to deploy GCP cluster", "tool", name, "error", err)
		return nil, gcpClusterDeployResult{}, fmt.Errorf("deploy cluster: %w", err)
	}

	result := gcpClusterDeployResult(deployResult)

	// If wait is requested, monitor the cluster until ready or timeout
	if input.Wait {
		logger.Info("waiting for GCP cluster to be ready",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", targetNamespace,
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

		// Use default poll interval and stall threshold
		pollInterval := 30 * time.Second
		stallThreshold := 10 * time.Minute

		// Create a helper deploy tool to use the wait logic
		waitHelper := &clusterWaitHelper{session: t.session}

		// Wait for cluster to be ready
		ready, err := waitHelper.waitForClusterReady(ctx, targetNamespace, input.Name, pollInterval, waitTimeout, stallThreshold, logger)
		if err != nil {
			logger.Error("error while waiting for GCP cluster", "tool", name, "error", err)
			return nil, gcpClusterDeployResult{}, fmt.Errorf("wait for cluster ready: %w", err)
		}

		if !ready {
			logger.Warn("GCP cluster did not become ready within timeout",
				"tool", name,
				"cluster_name", input.Name,
				"timeout", waitTimeout,
			)
			return nil, gcpClusterDeployResult{}, fmt.Errorf("cluster %s did not become ready within %v", input.Name, waitTimeout)
		}

		logger.Info("GCP cluster is ready",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", targetNamespace,
		)
	}

	logger.Info("GCP cluster deployment completed",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", targetNamespace,
		"template", template,
		"status", result.Status,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

// resolveDeployNamespace determines the target namespace for GCP cluster deployment
func (t *gcpClusterDeployTool) resolveDeployNamespace(ctx context.Context, namespace string, logger *slog.Logger) (string, error) {
	// If specific namespace provided, validate it
	if namespace != "" {
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(namespace) {
			return "", fmt.Errorf("namespace %q not allowed by namespace filter", namespace)
		}
		return namespace, nil
	}

	// No namespace specified - determine default behavior
	// DEV_ALLOW_ANY mode (no filter or matches all): default to kcm-system
	// OIDC_REQUIRED mode (restricted filter): require explicit namespace
	if t.session.NamespaceFilter == nil || t.session.NamespaceFilter.MatchString("kcm-system") {
		// DEV_ALLOW_ANY mode - default to kcm-system
		logger.Debug("defaulting to kcm-system namespace (DEV_ALLOW_ANY mode)")
		return "kcm-system", nil
	}

	// OIDC_REQUIRED mode - require explicit namespace
	return "", fmt.Errorf("namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter)")
}

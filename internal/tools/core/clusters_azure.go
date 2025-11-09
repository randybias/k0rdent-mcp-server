package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

// azureClusterDeployTool handles Azure-specific cluster deployments
type azureClusterDeployTool struct {
	session *runtime.Session
}

// azureClusterDeployInput defines Azure-specific cluster deployment parameters
type azureClusterDeployInput struct {
	Name               string            `json:"name" jsonschema:"Name of the cluster deployment"`
	Credential         string            `json:"credential" jsonschema:"Azure credential name"`
	Location           string            `json:"location" jsonschema:"Azure location (e.g. westus2, eastus, westeurope)"`
	SubscriptionID     string            `json:"subscriptionID" jsonschema:"Azure subscription ID (GUID format)"`
	ControlPlane       azureNodeConfig   `json:"controlPlane" jsonschema:"Control plane node configuration"`
	Worker             azureNodeConfig   `json:"worker" jsonschema:"Worker node configuration"`
	ControlPlaneNumber int               `json:"controlPlaneNumber,omitempty" jsonschema:"Number of control plane nodes (default: 3)"`
	WorkersNumber      int               `json:"workersNumber,omitempty" jsonschema:"Number of worker nodes (default: 2)"`
	Namespace          string            `json:"namespace,omitempty" jsonschema:"Target namespace for deployment (default: kcm-system)"`
	Labels             map[string]string `json:"labels,omitempty" jsonschema:"Additional labels to apply to the cluster deployment"`
	Wait               bool              `json:"wait,omitempty" jsonschema:"Wait for cluster to be ready before returning"`
	WaitTimeout        string            `json:"waitTimeout,omitempty" jsonschema:"Maximum time to wait for provisioning (default: 30m)"`
}

// azureNodeConfig defines Azure-specific node configuration
type azureNodeConfig struct {
	VMSize         string `json:"vmSize" jsonschema:"Azure VM size (e.g. Standard_A4_v2, Standard_D2s_v3)"`
	RootVolumeSize int    `json:"rootVolumeSize,omitempty" jsonschema:"Root volume size in GB (default: 30)"`
}

// azureClusterDeployResult wraps the deployment result
type azureClusterDeployResult clusters.DeployResult

// deploy handles Azure cluster deployment with auto-template selection
func (t *azureClusterDeployTool) deploy(ctx context.Context, req *mcp.CallToolRequest, input azureClusterDeployInput) (*mcp.CallToolResult, azureClusterDeployResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters.azure")
	start := time.Now()

	logger.Debug("deploying Azure cluster",
		"tool", name,
		"cluster_name", input.Name,
		"location", input.Location,
		"subscription_id", input.SubscriptionID,
		"namespace", input.Namespace,
	)

	// Validate required fields
	if input.Name == "" {
		return nil, azureClusterDeployResult{}, fmt.Errorf("cluster name is required")
	}
	if input.Credential == "" {
		return nil, azureClusterDeployResult{}, fmt.Errorf("credential is required")
	}
	if input.Location == "" {
		return nil, azureClusterDeployResult{}, fmt.Errorf("location is required")
	}
	if input.SubscriptionID == "" {
		return nil, azureClusterDeployResult{}, fmt.Errorf("subscriptionID is required")
	}
	if input.ControlPlane.VMSize == "" {
		return nil, azureClusterDeployResult{}, fmt.Errorf("controlPlane.vmSize is required")
	}
	if input.Worker.VMSize == "" {
		return nil, azureClusterDeployResult{}, fmt.Errorf("worker.vmSize is required")
	}

	// Resolve target namespace
	targetNamespace, err := t.resolveDeployNamespace(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve deploy namespace", "tool", name, "error", err)
		return nil, azureClusterDeployResult{}, fmt.Errorf("resolve namespace: %w", err)
	}

	logger.Debug("resolved deploy namespace", "tool", name, "namespace", targetNamespace)

	// Auto-select latest Azure template
	template, err := t.session.Clusters.SelectLatestTemplate(ctx, "azure", targetNamespace)
	if err != nil {
		logger.Error("failed to select Azure template", "tool", name, "namespace", targetNamespace, "error", err)
		return nil, azureClusterDeployResult{}, fmt.Errorf("select Azure template: %w", err)
	}

	logger.Info("selected Azure template", "tool", name, "template", template, "namespace", targetNamespace)

	// Validate and apply defaults for node counts
	controlPlaneNumber, workersNumber, err := validateAndDefaultNodeCounts(input.ControlPlaneNumber, input.WorkersNumber)
	if err != nil {
		return nil, azureClusterDeployResult{}, err
	}

	// Apply defaults for volume sizes
	controlPlaneRootVolumeSize := input.ControlPlane.RootVolumeSize
	if controlPlaneRootVolumeSize == 0 {
		controlPlaneRootVolumeSize = defaultAzureRootVolumeSize
	}

	workerRootVolumeSize := input.Worker.RootVolumeSize
	if workerRootVolumeSize == 0 {
		workerRootVolumeSize = defaultAzureRootVolumeSize
	}

	// Build Azure-specific config map
	config := map[string]any{
		"location":       input.Location,
		"subscriptionID": input.SubscriptionID,
		"controlPlane": map[string]any{
			"vmSize":         input.ControlPlane.VMSize,
			"rootVolumeSize": controlPlaneRootVolumeSize,
		},
		"worker": map[string]any{
			"vmSize":         input.Worker.VMSize,
			"rootVolumeSize": workerRootVolumeSize,
		},
		"controlPlaneNumber": controlPlaneNumber,
		"workersNumber":      workersNumber,
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
		logger.Error("failed to deploy Azure cluster", "tool", name, "error", err)
		return nil, azureClusterDeployResult{}, fmt.Errorf("deploy cluster: %w", err)
	}

	result := azureClusterDeployResult(deployResult)

	// If wait is requested, monitor the cluster until ready or timeout
	if input.Wait {
		logger.Info("waiting for Azure cluster to be ready",
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

		// Use standard polling parameters for wait
		pollInterval := 30 * time.Second
		stallThreshold := 10 * time.Minute

		// Create a helper deploy tool to use the wait logic
		waitHelper := &clusterWaitHelper{session: t.session}

		// Wait for cluster to be ready
		ready, err := waitHelper.waitForClusterReady(ctx, targetNamespace, input.Name, pollInterval, waitTimeout, stallThreshold, logger)
		if err != nil {
			logger.Error("error while waiting for Azure cluster", "tool", name, "error", err)
			return nil, azureClusterDeployResult{}, fmt.Errorf("wait for cluster ready: %w", err)
		}

		if !ready {
			logger.Warn("Azure cluster did not become ready within timeout",
				"tool", name,
				"cluster_name", input.Name,
				"timeout", waitTimeout,
			)
			return nil, azureClusterDeployResult{}, fmt.Errorf("cluster %s did not become ready within %v", input.Name, waitTimeout)
		}

		logger.Info("Azure cluster is ready",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", targetNamespace,
		)
	}

	logger.Info("Azure cluster deployment completed",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", targetNamespace,
		"template", template,
		"status", result.Status,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

// resolveDeployNamespace determines the target namespace for Azure cluster deployment
func (t *azureClusterDeployTool) resolveDeployNamespace(ctx context.Context, namespace string, logger *slog.Logger) (string, error) {
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

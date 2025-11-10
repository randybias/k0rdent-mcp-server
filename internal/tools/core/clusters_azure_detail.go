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

// azureClusterDetailTool provides detailed Azure infrastructure inspection for a ClusterDeployment
type azureClusterDetailTool struct {
	session *runtime.Session
}

// azureClusterDetailInput defines the input parameters for Azure cluster detail retrieval
type azureClusterDetailInput struct {
	Name      string `json:"name" jsonschema:"Name of the ClusterDeployment"`
	Namespace string `json:"namespace,omitempty" jsonschema:"Namespace of the ClusterDeployment (optional, resolved per auth mode)"`
}

// azureClusterDetailResult wraps the detailed Azure cluster information
type azureClusterDetailResult clusters.AzureClusterDetail

// detail retrieves detailed Azure infrastructure information for a ClusterDeployment
func (t *azureClusterDetailTool) detail(ctx context.Context, req *mcp.CallToolRequest, input azureClusterDetailInput) (*mcp.CallToolResult, azureClusterDetailResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters.azure.detail")
	start := time.Now()

	logger.Debug("retrieving Azure cluster detail",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", input.Namespace,
	)

	// Validate required fields
	if input.Name == "" {
		return nil, azureClusterDetailResult{}, fmt.Errorf("cluster name is required")
	}

	// Resolve target namespace
	targetNamespace, err := t.resolveNamespace(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve namespace", "tool", name, "error", err)
		return nil, azureClusterDetailResult{}, fmt.Errorf("resolve namespace: %w", err)
	}

	logger.Debug("resolved namespace", "tool", name, "namespace", targetNamespace)

	// Get Azure cluster detail from cluster manager
	detail, err := t.session.Clusters.GetAzureClusterDetail(ctx, targetNamespace, input.Name)
	if err != nil {
		logger.Error("failed to get Azure cluster detail", "tool", name, "error", err)
		return nil, azureClusterDetailResult{}, fmt.Errorf("get Azure cluster detail: %w", err)
	}

	result := azureClusterDetailResult(*detail)

	logger.Info("Azure cluster detail retrieved",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", targetNamespace,
		"resource_group", detail.Azure.ResourceGroup,
		"location", detail.Azure.Location,
		"subscription_id", detail.Azure.SubscriptionID,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

// resolveNamespace determines the target namespace for Azure cluster detail retrieval
func (t *azureClusterDetailTool) resolveNamespace(ctx context.Context, namespace string, logger *slog.Logger) (string, error) {
	// If specific namespace provided, validate it
	if namespace != "" {
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(namespace) {
			return "", fmt.Errorf("namespace %q not allowed by namespace filter", namespace)
		}
		return namespace, nil
	}

	// No namespace specified - in OIDC mode, require explicit namespace
	// In DEV mode, we'll need the user to specify since we can't infer which cluster they mean
	if t.session.NamespaceFilter == nil || t.session.NamespaceFilter.MatchString("kcm-system") {
		// DEV_ALLOW_ANY mode - default to kcm-system
		logger.Debug("defaulting to kcm-system namespace (DEV_ALLOW_ANY mode)")
		return "kcm-system", nil
	}

	// OIDC_REQUIRED mode - require explicit namespace
	return "", fmt.Errorf("namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter)")
}

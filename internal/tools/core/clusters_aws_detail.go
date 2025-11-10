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

// awsClusterDetailTool handles AWS-specific cluster detail retrieval
type awsClusterDetailTool struct {
	session *runtime.Session
}

// awsClusterDetailInput defines the input schema for AWS cluster detail retrieval
type awsClusterDetailInput struct {
	Name      string `json:"name" jsonschema:"Cluster deployment name"`
	Namespace string `json:"namespace,omitempty" jsonschema:"Deployment namespace (optional, follows standard patterns)"`
}

// awsClusterDetailResult is the result of an AWS cluster detail request
type awsClusterDetailResult clusters.AWSClusterDetail

// detail handles the AWS cluster detail retrieval request
func (t *awsClusterDetailTool) detail(ctx context.Context, req *mcp.CallToolRequest, input awsClusterDetailInput) (*mcp.CallToolResult, awsClusterDetailResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters.aws.detail")
	start := time.Now()

	logger.Debug("fetching AWS cluster detail",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", input.Namespace,
	)

	// Validate required fields
	if input.Name == "" {
		return nil, awsClusterDetailResult{}, fmt.Errorf("cluster name is required")
	}

	// Resolve target namespace
	targetNamespace, err := t.resolveNamespace(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve namespace", "tool", name, "error", err)
		return nil, awsClusterDetailResult{}, fmt.Errorf("resolve namespace: %w", err)
	}

	logger.Debug("resolved namespace", "tool", name, "namespace", targetNamespace)

	// Fetch AWS cluster detail using cluster manager
	detail, err := t.session.Clusters.GetAWSClusterDetail(ctx, targetNamespace, input.Name)
	if err != nil {
		logger.Error("failed to fetch AWS cluster detail", "tool", name, "error", err)
		return nil, awsClusterDetailResult{}, fmt.Errorf("fetch AWS cluster detail: %w", err)
	}

	result := awsClusterDetailResult(detail)

	vpcID := ""
	if detail.AWS.VPC != nil {
		vpcID = detail.AWS.VPC.ID
	}

	logger.Info("AWS cluster detail fetched successfully",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", targetNamespace,
		"vpc_id", vpcID,
		"region", detail.Region,
		"subnet_count", len(detail.AWS.Subnets),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

// resolveNamespace determines the target namespace for AWS cluster detail retrieval
func (t *awsClusterDetailTool) resolveNamespace(ctx context.Context, namespace string, logger *slog.Logger) (string, error) {
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

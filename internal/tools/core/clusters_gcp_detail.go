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

// gcpClusterDetailTool implements GCP-specific cluster detail inspection
type gcpClusterDetailTool struct {
	session *runtime.Session
}

// gcpClusterDetailInput defines the input parameters for GCP cluster detail retrieval
type gcpClusterDetailInput struct {
	Name      string `json:"name" jsonschema:"Cluster deployment name"`
	Namespace string `json:"namespace,omitempty" jsonschema:"Deployment namespace (optional, follows standard patterns)"`
}

// gcpClusterDetailResult is the result of a GCP cluster detail query
type gcpClusterDetailResult clusters.GCPClusterDetail

// detail handles the GCP cluster detail retrieval
func (t *gcpClusterDetailTool) detail(ctx context.Context, req *mcp.CallToolRequest, input gcpClusterDetailInput) (*mcp.CallToolResult, gcpClusterDetailResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters.gcp.detail")
	start := time.Now()

	logger.Debug("fetching GCP cluster detail",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", input.Namespace,
	)

	// Validate required fields
	if input.Name == "" {
		return nil, gcpClusterDetailResult{}, fmt.Errorf("cluster name is required")
	}

	// Resolve target namespace
	targetNamespace, err := t.resolveNamespace(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve namespace", "tool", name, "error", err)
		return nil, gcpClusterDetailResult{}, fmt.Errorf("resolve namespace: %w", err)
	}

	logger.Debug("resolved namespace", "tool", name, "namespace", targetNamespace)

	// Fetch GCP cluster detail using cluster manager
	detail, err := t.session.Clusters.GetGCPClusterDetail(ctx, targetNamespace, input.Name)
	if err != nil {
		logger.Error("failed to fetch GCP cluster detail", "tool", name, "error", err)
		return nil, gcpClusterDetailResult{}, fmt.Errorf("fetch GCP cluster detail: %w", err)
	}

	result := gcpClusterDetailResult(*detail)

	logger.Info("GCP cluster detail fetched successfully",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", targetNamespace,
		"project", detail.GCP.Project,
		"region", detail.GCP.Region,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

// resolveNamespace determines the target namespace for GCP cluster detail retrieval
func (t *gcpClusterDetailTool) resolveNamespace(ctx context.Context, namespace string, logger *slog.Logger) (string, error) {
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

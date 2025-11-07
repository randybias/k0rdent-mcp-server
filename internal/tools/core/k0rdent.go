package core

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/k0rdent/mcp-k0rdent-server/internal/k0rdent/api"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

type serviceTemplatesTool struct {
	session *runtime.Session
}

type serviceTemplatesResult struct {
	Items []api.ServiceTemplateSummary `json:"items"`
}

type clusterDeploymentsTool struct {
	session *runtime.Session
}

type clusterDeploymentsInput struct {
	Selector string `json:"selector,omitempty"`
}

type clusterDeploymentsResult struct {
	Items []api.ClusterDeploymentSummary `json:"items"`
}

type multiClusterServicesTool struct {
	session *runtime.Session
}

type multiClusterServicesInput struct {
	Selector string `json:"selector,omitempty"`
}

type multiClusterServicesResult struct {
	Items []api.MultiClusterServiceSummary `json:"items"`
}

func registerK0rdentTools(server *mcp.Server, session *runtime.Session) error {
	if session == nil {
		return fmt.Errorf("session is required")
	}

	stTool := &serviceTemplatesTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.k0rdent.serviceTemplates.list",
		Description: "List K0rdent ServiceTemplates",
	}, stTool.list)

	cdTool := &clusterDeploymentsTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.k0rdent.clusterDeployments.list",
		Description: "List K0rdent ClusterDeployments",
	}, cdTool.list)

	msTool := &multiClusterServicesTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.k0rdent.multiClusterServices.list",
		Description: "List K0rdent MultiClusterServices",
	}, msTool.list)

	return nil
}

func (t *serviceTemplatesTool) list(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, serviceTemplatesResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.k0rdent")
	start := time.Now()
	if filter := t.session.NamespaceFilter; filter != nil {
		logger.Debug("listing service templates", "tool", name, "namespace_filter", filter.String())
	} else {
		logger.Debug("listing service templates", "tool", name)
	}

	items, err := api.ListServiceTemplates(ctx, t.session.Clients.Dynamic)
	if err != nil {
		logger.Error("list service templates failed", "tool", name, "error", err)
		return nil, serviceTemplatesResult{}, err
	}
	filtered := filterServiceTemplatesByNamespace(items, t.session.NamespaceFilter)
	logger.Info("service templates listed", "tool", name, "count", len(filtered), "duration_ms", time.Since(start).Milliseconds())
	return nil, serviceTemplatesResult{Items: filtered}, nil
}

func (t *clusterDeploymentsTool) list(ctx context.Context, req *mcp.CallToolRequest, input clusterDeploymentsInput) (*mcp.CallToolResult, clusterDeploymentsResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.k0rdent")
	start := time.Now()
	if input.Selector != "" {
		if _, err := labels.Parse(input.Selector); err != nil {
			logger.Error("invalid selector", "tool", name, "selector", input.Selector, "error", err)
			return nil, clusterDeploymentsResult{}, fmt.Errorf("invalid selector: %w", err)
		}
	}
	logger.Debug("listing cluster deployments", "tool", name, "selector", input.Selector)
	items, err := api.ListClusterDeployments(ctx, t.session.Clients.Dynamic, input.Selector)
	if err != nil {
		logger.Error("list cluster deployments failed", "tool", name, "selector", input.Selector, "error", err)
		return nil, clusterDeploymentsResult{}, err
	}
	filtered := filterClusterDeploymentsByNamespace(items, t.session.NamespaceFilter)
	logger.Info("cluster deployments listed", "tool", name, "count", len(filtered), "duration_ms", time.Since(start).Milliseconds())
	return nil, clusterDeploymentsResult{Items: filtered}, nil
}

func (t *multiClusterServicesTool) list(ctx context.Context, req *mcp.CallToolRequest, input multiClusterServicesInput) (*mcp.CallToolResult, multiClusterServicesResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.k0rdent")
	start := time.Now()
	if input.Selector != "" {
		if _, err := labels.Parse(input.Selector); err != nil {
			logger.Error("invalid selector", "tool", name, "selector", input.Selector, "error", err)
			return nil, multiClusterServicesResult{}, fmt.Errorf("invalid selector: %w", err)
		}
	}
	logger.Debug("listing multi-cluster services", "tool", name, "selector", input.Selector)
	items, err := api.ListMultiClusterServices(ctx, t.session.Clients.Dynamic, input.Selector)
	if err != nil {
		logger.Error("list multi-cluster services failed", "tool", name, "selector", input.Selector, "error", err)
		return nil, multiClusterServicesResult{}, err
	}
	filtered := filterMultiClusterServicesByNamespace(items, t.session.NamespaceFilter)
	logger.Info("multi-cluster services listed", "tool", name, "count", len(filtered), "duration_ms", time.Since(start).Milliseconds())
	return nil, multiClusterServicesResult{Items: filtered}, nil
}

func filterServiceTemplatesByNamespace(items []api.ServiceTemplateSummary, filter *regexp.Regexp) []api.ServiceTemplateSummary {
	if filter == nil {
		return items
	}
	filtered := make([]api.ServiceTemplateSummary, 0, len(items))
	for _, item := range items {
		if filter.MatchString(item.Namespace) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterClusterDeploymentsByNamespace(items []api.ClusterDeploymentSummary, filter *regexp.Regexp) []api.ClusterDeploymentSummary {
	if filter == nil {
		return items
	}
	filtered := make([]api.ClusterDeploymentSummary, 0, len(items))
	for _, item := range items {
		if filter.MatchString(item.Namespace) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterMultiClusterServicesByNamespace(items []api.MultiClusterServiceSummary, filter *regexp.Regexp) []api.MultiClusterServiceSummary {
	if filter == nil {
		return items
	}
	filtered := make([]api.MultiClusterServiceSummary, 0, len(items))
	for _, item := range items {
		if filter.MatchString(item.Namespace) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

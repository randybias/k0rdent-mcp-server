package core

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

type namespacesTool struct {
	session *runtime.Session
}

type namespaceListInput struct{}

type namespaceListResult struct {
	Namespaces []namespaceInfo `json:"namespaces"`
}

type namespaceInfo struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Status string            `json:"status"`
}

func registerNamespaces(server *mcp.Server, session *runtime.Session) error {
	tool := &namespacesTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.namespaces.list",
		Description: "List namespaces with their labels and phase status",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "namespaces",
			"action":   "list",
		},
	}, tool.handle)
	return nil
}

func (t *namespacesTool) handle(ctx context.Context, req *mcp.CallToolRequest, _ namespaceListInput) (*mcp.CallToolResult, namespaceListResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.namespaces")
	start := time.Now()

	client := t.session.Clients.Kubernetes.CoreV1().Namespaces()
	list, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("list namespaces failed", "tool", name, "error", err)
		return nil, namespaceListResult{}, fmt.Errorf("list namespaces: %w", err)
	}

	filter := t.session.NamespaceFilter
	if logger != nil {
		if filter != nil {
			logger.Debug("filtering namespaces", "tool", name, "filter", filter.String())
		} else {
			logger.Debug("no namespace filter applied", "tool", name)
		}
	}

	out := namespaceListResult{
		Namespaces: make([]namespaceInfo, 0, len(list.Items)),
	}
	for _, item := range list.Items {
		if filter != nil && !filter.MatchString(item.Name) {
			continue
		}
		out.Namespaces = append(out.Namespaces, namespaceInfo{
			Name:   item.Name,
			Labels: copyMap(item.Labels),
			Status: string(item.Status.Phase),
		})
	}

	sort.Slice(out.Namespaces, func(i, j int) bool {
		return out.Namespaces[i].Name < out.Namespaces[j].Name
	})

	if logger != nil {
		logger.Info("namespaces listed",
			"tool", name,
			"count", len(out.Namespaces),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	return nil, out, nil
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(in))
	for k, v := range in {
		cloned[k] = v
	}
	return cloned
}

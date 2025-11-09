package core

import (
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/catalog"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

// Context keys for per-session infrastructure.
const (
	ContextKeyEventManager          = "core:eventManager"
	ContextKeyPodLogManager         = "core:podLogManager"
	ContextKeyClusterMonitorManager = "core:clusterMonitorManager"
)

// Options control which tool groups are registered for a session.
type Options struct {
	EventManager          *EventManager
	PodLogManager         *PodLogManager
	ClusterMonitorManager *ClusterMonitorManager
	CatalogManager        *catalog.Manager
}

// Register installs the core tool suite on the provided MCP server.
func Register(server *mcp.Server, session *runtime.Session, opts Options) error {
	if server == nil {
		return errors.New("server is required")
	}
	if session == nil {
		return errors.New("session is required")
	}

	if err := registerNamespaces(server, session); err != nil {
		return err
	}

	if err := registerEvents(server, session, opts.EventManager); err != nil {
		return err
	}

	if err := registerClusterMonitor(server, session, opts.ClusterMonitorManager); err != nil {
		return err
	}

	if err := registerPodLogs(server, session, opts.PodLogManager); err != nil {
		return err
	}

	if err := registerK0rdentTools(server, session); err != nil {
		return err
	}

	if err := registerCatalog(server, session, opts.CatalogManager); err != nil {
		return err
	}

	if err := registerClusters(server, session); err != nil {
		return err
	}

	return nil
}

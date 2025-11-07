package runtime

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/kube"
	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
	logsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/logs"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/metrics"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Runtime wires global dependencies that are required to service MCP sessions.
type Runtime struct {
	settings         *config.Settings
	factory          *kube.ClientFactory
	logger           *slog.Logger
	newEventProvider func(context.Context, kubernetes.Interface) (*eventsprovider.Provider, error)
	newLogProvider   func(kubernetes.Interface) (*logsprovider.Provider, error)
}

// Session represents the per-connection runtime state.
type Session struct {
	Token           string
	Logger          *slog.Logger
	NamespaceFilter *regexp.Regexp
	Events          *eventsprovider.Provider
	Logs            *logsprovider.Provider
	Clients         Clients
	Clusters        *clusters.Manager
	ClusterMetrics  *metrics.ClusterMetrics
	settings        *config.Settings
}

// Clients bundles the Kubernetes clients used by the tools.
type Clients struct {
	Kubernetes kubernetes.Interface
	Dynamic    dynamic.Interface
}

// New creates a Runtime from the shared configuration.
func New(settings *config.Settings, factory *kube.ClientFactory, logger *slog.Logger) (*Runtime, error) {
	if settings == nil {
		return nil, errors.New("settings are required")
	}
	if factory == nil {
		return nil, errors.New("client factory is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Runtime{
		settings:         settings,
		factory:          factory,
		logger:           logging.WithComponent(logger, "runtime"),
		newEventProvider: eventsprovider.NewProvider,
		newLogProvider: func(client kubernetes.Interface) (*logsprovider.Provider, error) {
			return logsprovider.NewProvider(client)
		},
	}, nil
}

// NewSession spawns a session scoped view of the runtime, binding Kubernetes clients
// to the provided bearer token.
func (r *Runtime) NewSession(ctx context.Context, token string) (*Session, error) {
	if r == nil {
		return nil, errors.New("runtime is not configured")
	}

	log := logging.WithContext(ctx, r.logger)
	if log != nil {
		log.Info("creating runtime session", "has_token", token != "")
	}

	kubeClient, err := r.factory.KubernetesClient(token)
	if err != nil {
		if log != nil {
			log.Error("failed to create kubernetes client", "error", err)
		}
		return nil, err
	}
	dynamicClient, err := r.factory.DynamicClient(token)
	if err != nil {
		if log != nil {
			log.Error("failed to create dynamic client", "error", err)
		}
		return nil, err
	}

	eventProvider, err := r.newEventProvider(ctx, kubeClient)
	if err != nil {
		if log != nil {
			log.Error("failed to create event provider", "error", err)
		}
		return nil, err
	}

	logProvider, err := r.newLogProvider(kubeClient)
	if err != nil {
		if log != nil {
			log.Error("failed to create log provider", "error", err)
		}
		return nil, err
	}

	clusterManager, err := clusters.NewManager(clusters.Options{
		DynamicClient:   dynamicClient,
		NamespaceFilter: r.settings.NamespaceFilter,
		GlobalNamespace: r.settings.Cluster.GlobalNamespace,
		FieldOwner:      r.settings.Cluster.DeployFieldOwner,
		Logger:          r.logger,
	})
	if err != nil {
		if log != nil {
			log.Error("failed to create cluster manager", "error", err)
		}
		return nil, err
	}

	if log != nil {
		log.Info("runtime session ready")
	}

	clusterMetrics := metrics.NewClusterMetrics()

	return &Session{
		Token:           token,
		Logger:          r.logger,
		NamespaceFilter: r.settings.NamespaceFilter,
		Events:          eventProvider,
		Logs:            logProvider,
		Clients: Clients{
			Kubernetes: kubeClient,
			Dynamic:    dynamicClient,
		},
		Clusters:       clusterManager,
		ClusterMetrics: clusterMetrics,
		settings:       r.settings,
	}, nil
}

// IsDevMode returns true if the session is running in dev mode (DEV_ALLOW_ANY).
func (s *Session) IsDevMode() bool {
	if s == nil || s.settings == nil {
		return false
	}
	return s.settings.AuthMode == config.AuthModeDevAllowAny
}

// GlobalNamespace returns the configured global namespace for cluster resources.
func (s *Session) GlobalNamespace() string {
	if s == nil || s.settings == nil {
		return "kcm-system"
	}
	return s.settings.Cluster.GlobalNamespace
}

// DefaultNamespaceDev returns the default namespace to use in dev mode.
func (s *Session) DefaultNamespaceDev() string {
	if s == nil || s.settings == nil {
		return "kcm-system"
	}
	return s.settings.Cluster.DefaultNamespaceDev
}

// DeployFieldOwner returns the field owner to use for cluster deployments.
func (s *Session) DeployFieldOwner() string {
	if s == nil || s.settings == nil {
		return "mcp.clusters"
	}
	return s.settings.Cluster.DeployFieldOwner
}

// ResolveNamespaces returns the list of namespaces accessible to this session.
// In dev mode, it includes the global namespace. In production mode, it respects
// the namespace filter regex.
func (s *Session) ResolveNamespaces(ctx context.Context, requestedNamespace string) ([]string, error) {
	if s == nil {
		return nil, errors.New("session is nil")
	}

	// If a specific namespace is requested, validate it
	if requestedNamespace != "" {
		if s.IsDevMode() {
			// In dev mode, allow any namespace
			return []string{requestedNamespace}, nil
		}
		// In production, check against filter
		if s.NamespaceFilter != nil && !s.NamespaceFilter.MatchString(requestedNamespace) {
			return nil, errors.New("requested namespace not allowed by filter")
		}
		return []string{requestedNamespace}, nil
	}

	// No specific namespace requested, return allowed namespaces
	namespaces := []string{}

	// Always include global namespace
	globalNs := s.GlobalNamespace()
	namespaces = append(namespaces, globalNs)

	// In production mode, add namespaces matching the filter
	if !s.IsDevMode() && s.NamespaceFilter != nil {
		// TODO: Query actual namespaces from cluster and filter
		// For now, we just return the global namespace in production mode
		// unless a specific namespace is requested
	}

	return namespaces, nil
}

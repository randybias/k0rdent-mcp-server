package runtime

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/kube"
	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
	logsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/logs"

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
		logger:           logger,
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

	kubeClient, err := r.factory.KubernetesClient(token)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := r.factory.DynamicClient(token)
	if err != nil {
		return nil, err
	}

	eventProvider, err := r.newEventProvider(ctx, kubeClient)
	if err != nil {
		return nil, err
	}

	logProvider, err := r.newLogProvider(kubeClient)
	if err != nil {
		return nil, err
	}

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
	}, nil
}

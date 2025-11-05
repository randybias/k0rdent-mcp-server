package kube

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

// ClientFactory produces Kubernetes clients derived from a shared base rest.Config.
type ClientFactory struct {
	baseConfig    *rest.Config
	newKubernetes func(*rest.Config) (kubernetes.Interface, error)
	newDynamic    func(*rest.Config) (dynamic.Interface, error)
	logger        *slog.Logger
}

// NewClientFactory constructs a ClientFactory. The provided base configuration is copied to avoid mutation.
func NewClientFactory(base *rest.Config, logger *slog.Logger) (*ClientFactory, error) {
	if base == nil {
		return nil, errors.New("base config is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &ClientFactory{
		baseConfig: rest.CopyConfig(base),
		newKubernetes: func(cfg *rest.Config) (kubernetes.Interface, error) {
			return kubernetes.NewForConfig(cfg)
		},
		newDynamic: func(cfg *rest.Config) (dynamic.Interface, error) {
			return dynamic.NewForConfig(cfg)
		},
		logger: logging.WithComponent(logger, "kube.clientfactory"),
	}, nil
}

// WithConstructors allows tests to inject client constructors.
func (f *ClientFactory) WithConstructors(
	newKube func(*rest.Config) (kubernetes.Interface, error),
	newDynamic func(*rest.Config) (dynamic.Interface, error),
) *ClientFactory {
	if newKube != nil {
		f.newKubernetes = newKube
	}
	if newDynamic != nil {
		f.newDynamic = newDynamic
	}
	return f
}

// RESTConfigForToken returns a copy of the base rest.Config optionally overriding the bearer token.
func (f *ClientFactory) RESTConfigForToken(token string) (*rest.Config, error) {
	if f == nil || f.baseConfig == nil {
		return nil, errors.New("client factory is not configured")
	}

	if f.logger != nil {
		f.logger.Debug("preparing rest config", "has_token", token != "")
	}

	cfg := rest.CopyConfig(f.baseConfig)
	if token != "" {
		cfg.BearerToken = token
		cfg.BearerTokenFile = ""
		cfg.Username = ""
		cfg.Password = ""
	}
	return cfg, nil
}

// KubernetesClient returns a typed Kubernetes client for the provided bearer token.
func (f *ClientFactory) KubernetesClient(token string) (kubernetes.Interface, error) {
	ctx := context.Background()
	cfg, err := f.RESTConfigForToken(token)
	if err != nil {
		if f.logger != nil {
			f.logger.ErrorContext(ctx, "failed to prepare rest config for kubernetes client", "error", err)
		}
		return nil, err
	}
	client, err := f.newKubernetes(cfg)
	if err != nil {
		err = fmt.Errorf("create kubernetes client: %w", err)
		if f.logger != nil {
			f.logger.ErrorContext(ctx, "failed to create kubernetes client", "error", err)
		}
		return nil, err
	}
	if f.logger != nil {
		f.logger.DebugContext(ctx, "created kubernetes client", "has_token", token != "")
	}
	return client, nil
}

// DynamicClient returns a dynamic client for the provided bearer token.
func (f *ClientFactory) DynamicClient(token string) (dynamic.Interface, error) {
	ctx := context.Background()
	cfg, err := f.RESTConfigForToken(token)
	if err != nil {
		if f.logger != nil {
			f.logger.ErrorContext(ctx, "failed to prepare rest config for dynamic client", "error", err)
		}
		return nil, err
	}
	client, err := f.newDynamic(cfg)
	if err != nil {
		err = fmt.Errorf("create dynamic client: %w", err)
		if f.logger != nil {
			f.logger.ErrorContext(ctx, "failed to create dynamic client", "error", err)
		}
		return nil, err
	}
	if f.logger != nil {
		f.logger.DebugContext(ctx, "created dynamic client", "has_token", token != "")
	}
	return client, nil
}

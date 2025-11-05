package kube

import (
	"errors"
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ClientFactory produces Kubernetes clients derived from a shared base rest.Config.
type ClientFactory struct {
	baseConfig    *rest.Config
	newKubernetes func(*rest.Config) (kubernetes.Interface, error)
	newDynamic    func(*rest.Config) (dynamic.Interface, error)
}

// NewClientFactory constructs a ClientFactory. The provided base configuration is copied to avoid mutation.
func NewClientFactory(base *rest.Config) (*ClientFactory, error) {
	if base == nil {
		return nil, errors.New("base config is nil")
	}

	return &ClientFactory{
		baseConfig:    rest.CopyConfig(base),
		newKubernetes: func(cfg *rest.Config) (kubernetes.Interface, error) {
			return kubernetes.NewForConfig(cfg)
		},
		newDynamic: func(cfg *rest.Config) (dynamic.Interface, error) {
			return dynamic.NewForConfig(cfg)
		},
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
	cfg, err := f.RESTConfigForToken(token)
	if err != nil {
		return nil, err
	}
	client, err := f.newKubernetes(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return client, nil
}

// DynamicClient returns a dynamic client for the provided bearer token.
func (f *ClientFactory) DynamicClient(token string) (dynamic.Interface, error) {
	cfg, err := f.RESTConfigForToken(token)
	if err != nil {
		return nil, err
	}
	client, err := f.newDynamic(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	return client, nil
}

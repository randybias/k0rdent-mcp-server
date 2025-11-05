package config

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	envKubeconfigPath = "K0RDENT_MGMT_KUBECONFIG_PATH"
	envKubeconfigB64  = "K0RDENT_MGMT_KUBECONFIG_B64"
	envKubeconfigText = "K0RDENT_MGMT_KUBECONFIG_TEXT"
	envContext        = "K0RDENT_MGMT_CONTEXT"
	envNamespaceExpr  = "K0RDENT_NAMESPACE_FILTER"
	envAuthMode       = "AUTH_MODE"
)

// AuthMode determines how incoming requests are authenticated.
type AuthMode string

const (
	// AuthModeDevAllowAny accepts any bearer token and is intended for local development only.
	AuthModeDevAllowAny AuthMode = "DEV_ALLOW_ANY"
	// AuthModeOIDCRequired requires a bearer token to be present and forwards it to Kubernetes.
	AuthModeOIDCRequired AuthMode = "OIDC_REQUIRED"
)

var validAuthModes = map[AuthMode]struct{}{
	AuthModeDevAllowAny:  {},
	AuthModeOIDCRequired: {},
}

// SourceType indicates how the kubeconfig was provided.
type SourceType string

const (
	SourcePath SourceType = "path"
	SourceB64  SourceType = "base64"
	SourceText SourceType = "text"
)

// Settings captures runtime configuration derived from the environment.
type Settings struct {
	RestConfig      *rest.Config
	ContextName     string
	NamespaceFilter *regexp.Regexp
	AuthMode        AuthMode
	Source          SourceType
	RawConfig       *clientcmdapi.Config
}

// Loader loads runtime configuration from the environment and validates cluster access.
type Loader struct {
	envLookup func(string) (string, bool)
	readFile  func(string) ([]byte, error)
	ping      func(context.Context, *rest.Config) error
}

// NewLoader creates a Loader that reads from the real environment and performs a live discovery ping.
func NewLoader() *Loader {
	return &Loader{
		envLookup: os.LookupEnv,
		readFile:  os.ReadFile,
		ping:      defaultDiscoveryPing,
	}
}

// Load reads configuration from the environment, constructs a Kubernetes rest.Config, and verifies connectivity.
func (l *Loader) Load(ctx context.Context) (*Settings, error) {
	if l.envLookup == nil {
		l.envLookup = os.LookupEnv
	}
	if l.readFile == nil {
		l.readFile = os.ReadFile
	}
	if l.ping == nil {
		l.ping = defaultDiscoveryPing
	}

	source, kubeconfigBytes, err := l.readKubeconfig()
	if err != nil {
		return nil, err
	}

	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	contextName, err := l.resolveContext(cfg)
	if err != nil {
		return nil, err
	}

	namespaceFilter, err := l.compileNamespaceFilter()
	if err != nil {
		return nil, err
	}

	authMode, err := l.resolveAuthMode()
	if err != nil {
		return nil, err
	}

	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*cfg, overrides)

	restCfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("create kubernetes rest config: %w", err)
	}

	if err := l.ping(ctx, restCfg); err != nil {
		return nil, fmt.Errorf("kubernetes discovery ping failed: %w", err)
	}

	return &Settings{
		RestConfig:      restCfg,
		ContextName:     contextName,
		NamespaceFilter: namespaceFilter,
		AuthMode:        authMode,
		Source:          source,
		RawConfig:       cfg,
	}, nil
}

func (l *Loader) readKubeconfig() (SourceType, []byte, error) {
	path, hasPath := l.envLookup(envKubeconfigPath)
	b64, hasB64 := l.envLookup(envKubeconfigB64)
	text, hasText := l.envLookup(envKubeconfigText)

	sourcesSet := 0
	if hasPath && path != "" {
		sourcesSet++
	}
	if hasB64 && b64 != "" {
		sourcesSet++
	}
	if hasText && text != "" {
		sourcesSet++
	}

	if sourcesSet == 0 {
		return "", nil, errors.New("one of K0RDENT_MGMT_KUBECONFIG_PATH, _B64, or _TEXT must be provided")
	}
	if sourcesSet > 1 {
		return "", nil, errors.New("only one of K0RDENT_MGMT_KUBECONFIG_PATH, _B64, or _TEXT may be provided")
	}

	switch {
	case hasPath && path != "":
		data, err := l.readFile(path)
		if err != nil {
			return "", nil, fmt.Errorf("read kubeconfig path: %w", err)
		}
		return SourcePath, data, nil
	case hasB64 && b64 != "":
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", nil, fmt.Errorf("decode K0RDENT_MGMT_KUBECONFIG_B64: %w", err)
		}
		return SourceB64, data, nil
	default:
		return SourceText, []byte(text), nil
	}
}

func (l *Loader) resolveContext(cfg *clientcmdapi.Config) (string, error) {
	if cfg == nil {
		return "", errors.New("kubeconfig is nil")
	}

	if v, ok := l.envLookup(envContext); ok && v != "" {
		if _, exists := cfg.Contexts[v]; !exists {
			return "", fmt.Errorf("requested context %q not found in kubeconfig", v)
		}
		return v, nil
	}

	if cfg.CurrentContext == "" {
		return "", errors.New("kubeconfig has no current-context and K0RDENT_MGMT_CONTEXT is not set")
	}
	if _, exists := cfg.Contexts[cfg.CurrentContext]; !exists {
		return "", fmt.Errorf("current-context %q not found in kubeconfig", cfg.CurrentContext)
	}
	return cfg.CurrentContext, nil
}

func (l *Loader) compileNamespaceFilter() (*regexp.Regexp, error) {
	value, ok := l.envLookup(envNamespaceExpr)
	if !ok || value == "" {
		return nil, nil
	}
	re, err := regexp.Compile(value)
	if err != nil {
		return nil, fmt.Errorf("compile namespace filter regex: %w", err)
	}
	return re, nil
}

func (l *Loader) resolveAuthMode() (AuthMode, error) {
	value, ok := l.envLookup(envAuthMode)
	if !ok || value == "" {
		return AuthModeDevAllowAny, nil
	}
	mode := AuthMode(value)
	if _, valid := validAuthModes[mode]; !valid {
		return "", fmt.Errorf("invalid AUTH_MODE %q", value)
	}
	return mode, nil
}

func defaultDiscoveryPing(ctx context.Context, baseCfg *rest.Config) error {
	if baseCfg == nil {
		return errors.New("rest config is nil")
	}

	cfg := rest.CopyConfig(baseCfg)
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}

	req := clientset.Discovery().RESTClient().Get().AbsPath("/version")
	return req.Do(ctx).Error()
}

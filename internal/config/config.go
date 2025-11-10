package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

const (
	envKubeconfigPath = "K0RDENT_MGMT_KUBECONFIG_PATH"
	envContext        = "K0RDENT_MGMT_CONTEXT"
	envNamespaceExpr  = "K0RDENT_NAMESPACE_FILTER"
	envAuthMode       = "AUTH_MODE"
	envLogLevel       = "LOG_LEVEL"
	envLogSinkEnabled = "LOG_EXTERNAL_SINK_ENABLED"

	envClusterGlobalNamespace       = "CLUSTER_GLOBAL_NAMESPACE"
	envClusterDefaultNamespaceDev   = "CLUSTER_DEFAULT_NAMESPACE_DEV"
	envClusterDeployFieldOwner      = "CLUSTER_DEPLOY_FIELD_OWNER"
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
)

// Settings captures runtime configuration derived from the environment.
type Settings struct {
	RestConfig      *rest.Config
	ContextName     string
	NamespaceFilter *regexp.Regexp
	AuthMode        AuthMode
	Source          SourceType
	RawConfig       *clientcmdapi.Config
	Logging         LoggingSettings
	Cluster         ClusterSettings
}

// LoggingSettings describe how structured logging is configured.
type LoggingSettings struct {
	Level               slog.Level
	ExternalSinkEnabled bool
}

// ClusterSettings describe cluster provisioning configuration.
type ClusterSettings struct {
	GlobalNamespace       string
	DefaultNamespaceDev   string
	DeployFieldOwner      string
}

// Loader loads runtime configuration from the environment and validates cluster access.
type Loader struct {
	envLookup func(string) (string, bool)
	readFile  func(string) ([]byte, error)
	ping      func(context.Context, *rest.Config) error

	logger *slog.Logger
}

// NewLoader creates a Loader that reads from the real environment and performs a live discovery ping.
func NewLoader(logger *slog.Logger) *Loader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Loader{
		envLookup: os.LookupEnv,
		readFile:  os.ReadFile,
		ping:      defaultDiscoveryPing,
		logger:    logging.WithComponent(logger, "config.loader"),
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
	log := logging.WithContext(ctx, l.logger)
	log.Info("loading configuration")

	source, kubeconfigBytes, err := l.readKubeconfig()
	if err != nil {
		log.Error("failed to read kubeconfig source", "error", err)
		return nil, err
	}

	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		err = fmt.Errorf("parse kubeconfig: %w", err)
		log.Error("failed to parse kubeconfig", "error", err)
		return nil, err
	}

	contextName, err := l.resolveContext(cfg)
	if err != nil {
		log.Error("failed to resolve context", "error", err)
		return nil, err
	}

	namespaceFilter, err := l.compileNamespaceFilter()
	if err != nil {
		log.Error("failed to compile namespace filter", "error", err)
		return nil, err
	}

	authMode, err := l.resolveAuthMode()
	if err != nil {
		log.Error("failed to resolve auth mode", "error", err)
		return nil, err
	}

	loggingSettings := l.resolveLogging(log)
	clusterSettings := l.resolveCluster()

	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*cfg, overrides)

	restCfg, err := clientConfig.ClientConfig()
	if err != nil {
		err = fmt.Errorf("create kubernetes rest config: %w", err)
		log.Error("failed to create kubernetes rest config", "error", err)
		return nil, err
	}

	if err := l.ping(ctx, restCfg); err != nil {
		err = fmt.Errorf("kubernetes discovery ping failed: %w", err)
		log.Error("kubernetes discovery ping failed", "error", err)
		return nil, err
	}

	log.Info("configuration loaded",
		"context", contextName,
		"auth_mode", authMode,
		"source", source)

	return &Settings{
		RestConfig:      restCfg,
		ContextName:     contextName,
		NamespaceFilter: namespaceFilter,
		AuthMode:        authMode,
		Source:          source,
		RawConfig:       cfg,
		Logging:         loggingSettings,
		Cluster:         clusterSettings,
	}, nil
}

func (l *Loader) readKubeconfig() (SourceType, []byte, error) {
	path, hasPath := l.envLookup(envKubeconfigPath)

	if !hasPath || path == "" {
		return "", nil, errors.New("K0RDENT_MGMT_KUBECONFIG_PATH must be provided")
	}

	data, err := l.readFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read kubeconfig path: %w", err)
	}
	return SourcePath, data, nil
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

func (l *Loader) resolveLogging(logger *slog.Logger) LoggingSettings {
	settings := LoggingSettings{Level: slog.LevelInfo}

	if raw, ok := l.envLookup(envLogLevel); ok && strings.TrimSpace(raw) != "" {
		lvl, err := logging.ParseLevel(raw)
		if err != nil {
			if logger != nil {
				logger.Warn("invalid LOG_LEVEL value; defaulting to INFO", "value", raw, "error", err)
			}
		} else {
			settings.Level = lvl
		}
	}

	if raw, ok := l.envLookup(envLogSinkEnabled); ok && strings.TrimSpace(raw) != "" {
		enabled, err := parseBoolEnv(raw)
		if err != nil {
			if logger != nil {
				logger.Warn("invalid LOG_EXTERNAL_SINK_ENABLED value", "value", raw)
			}
		} else {
			settings.ExternalSinkEnabled = enabled
		}
	}

	if logger != nil {
		logger.Info("logging configuration resolved",
			"level", settings.Level.String(),
			"external_sink_enabled", settings.ExternalSinkEnabled,
		)
	}

	return settings
}

func (l *Loader) resolveCluster() ClusterSettings {
	settings := ClusterSettings{
		GlobalNamespace:     "kcm-system",
		DefaultNamespaceDev: "kcm-system",
		DeployFieldOwner:    "mcp.clusters",
	}

	if raw, ok := l.envLookup(envClusterGlobalNamespace); ok && strings.TrimSpace(raw) != "" {
		settings.GlobalNamespace = strings.TrimSpace(raw)
	}

	if raw, ok := l.envLookup(envClusterDefaultNamespaceDev); ok && strings.TrimSpace(raw) != "" {
		settings.DefaultNamespaceDev = strings.TrimSpace(raw)
	}

	if raw, ok := l.envLookup(envClusterDeployFieldOwner); ok && strings.TrimSpace(raw) != "" {
		settings.DeployFieldOwner = strings.TrimSpace(raw)
	}

	return settings
}

func parseBoolEnv(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
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

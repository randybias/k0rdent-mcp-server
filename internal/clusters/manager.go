package clusters

import (
	"log/slog"
	"regexp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"k8s.io/client-go/dynamic"
)

// Manager handles cluster provisioning operations using the k0rdent API.
type Manager struct {
	dynamicClient   dynamic.Interface
	namespaceFilter *regexp.Regexp
	globalNamespace string
	fieldOwner      string
	logger          *slog.Logger
}

// Options configure the cluster Manager.
type Options struct {
	// DynamicClient is the Kubernetes dynamic client for CRD operations (required)
	DynamicClient dynamic.Interface

	// NamespaceFilter restricts operations to namespaces matching this regex (nil = no filter)
	NamespaceFilter *regexp.Regexp

	// GlobalNamespace is the namespace where global resources reside (default: "kcm-system")
	GlobalNamespace string

	// FieldOwner is the identifier for server-side apply operations (default: "mcp.clusters")
	FieldOwner string

	// Logger is used for structured logging (optional, defaults to slog.Default())
	Logger *slog.Logger
}

// NewManager constructs a Manager with the provided options.
func NewManager(opts Options) (*Manager, error) {
	if opts.DynamicClient == nil {
		return nil, ErrDynamicClientRequired
	}

	if opts.GlobalNamespace == "" {
		opts.GlobalNamespace = "kcm-system"
	}

	if opts.FieldOwner == "" {
		opts.FieldOwner = "mcp.clusters"
	}

	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	return &Manager{
		dynamicClient:   opts.DynamicClient,
		namespaceFilter: opts.NamespaceFilter,
		globalNamespace: opts.GlobalNamespace,
		fieldOwner:      opts.FieldOwner,
		logger:          logging.WithComponent(opts.Logger, "clusters.manager"),
	}, nil
}

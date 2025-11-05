package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/auth"
	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/kube"
	"github.com/k0rdent/mcp-k0rdent-server/internal/mcpserver"
	"github.com/k0rdent/mcp-k0rdent-server/internal/version"
)

type contextKey string

const (
	bearerTokenKey contextKey = "bearer-token"
	serverHolderKey contextKey = "mcp-server-holder"
)

// Dependencies contains the external components required by the App.
type Dependencies struct {
	Settings     *config.Settings
	ClientFactory *kube.ClientFactory
	MCPFactory   *mcpserver.Factory
}

// Options configure HTTP surface behavior.
type Options struct {
	StreamPath    string
	HealthPath    string
	Logger        *slog.Logger
	StreamOptions *mcp.StreamableHTTPOptions
}

// App wires HTTP transport, authentication, and MCP session handling.
type App struct {
	deps          Dependencies
	gate          *auth.Gate
	logger        *slog.Logger
	streamHandler *mcp.StreamableHTTPHandler
	router        chi.Router
}

// NewApp constructs the HTTP application with sane defaults.
func NewApp(deps Dependencies, opts Options) (*App, error) {
	if deps.Settings == nil {
		return nil, errors.New("settings are required")
	}
	if deps.MCPFactory == nil {
		return nil, errors.New("MCP factory is required")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	streamOptions := &mcp.StreamableHTTPOptions{}
	if opts.StreamOptions != nil {
		*streamOptions = *opts.StreamOptions
	}
	if streamOptions.Logger == nil {
		streamOptions.Logger = logger
	}

	app := &App{
		deps:          deps,
		gate:          auth.NewGate(deps.Settings.AuthMode),
		logger:        logger,
		streamHandler: nil, // assigned below
	}

	streamFactory := func(req *http.Request) *mcp.Server {
		holder, _ := req.Context().Value(serverHolderKey).(*sessionHolder)
		if holder == nil {
			holder = &sessionHolder{
				factory: deps.MCPFactory,
				token:   "",
				logger:  logger,
			}
		}
		return holder.serverInstance(req.Context())
	}
	app.streamHandler = mcp.NewStreamableHTTPHandler(streamFactory, streamOptions)

	streamPath := opts.StreamPath
	if streamPath == "" {
		streamPath = "/mcp"
	}
	healthPath := opts.HealthPath
	if healthPath == "" {
		healthPath = "/healthz"
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	router.Method(http.MethodGet, healthPath, http.HandlerFunc(app.handleHealth))
	router.Method(http.MethodHead, healthPath, http.HandlerFunc(app.handleHealth))

	// The MCP transport accepts GET/POST/DELETE on the same path.
	streamHandler := http.HandlerFunc(app.handleStream)
	router.Method(http.MethodGet, streamPath, streamHandler)
	router.Method(http.MethodPost, streamPath, streamHandler)
	router.Method(http.MethodDelete, streamPath, streamHandler)

	// Many clients expect the trailing-slash variant to route as well.
	router.Method(http.MethodGet, streamPath+"/", streamHandler)
	router.Method(http.MethodPost, streamPath+"/", streamHandler)
	router.Method(http.MethodDelete, streamPath+"/", streamHandler)

	app.router = router
	return app, nil
}

// Router exposes the configured HTTP handler.
func (a *App) Router() http.Handler {
	return a.router
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	info := version.Get()
	resp := map[string]any{
		"status":  "ok",
		"version": info,
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *App) handleStream(w http.ResponseWriter, r *http.Request) {
	token, err := a.gate.ExtractBearer(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, err.Error(), status)
		return
	}

	holder := &sessionHolder{
		factory: a.deps.MCPFactory,
		token:   token,
		logger:  a.logger,
	}

	ctx := context.WithValue(r.Context(), bearerTokenKey, token)
	ctx = context.WithValue(ctx, serverHolderKey, holder)
	r = r.WithContext(ctx)

	// Establish the session eagerly when the client hasn't provided an ID.
	if r.Header.Get("Mcp-Session-Id") == "" {
		if holder.serverInstance(ctx) == nil {
			http.Error(w, "failed to initialize MCP session", http.StatusInternalServerError)
			return
		}
	}

	a.streamHandler.ServeHTTP(w, r)
}

type sessionHolder struct {
	once    sync.Once
	factory *mcpserver.Factory
	token   string
	logger  *slog.Logger

	server *mcp.Server
	err    error
}

func (h *sessionHolder) serverInstance(ctx context.Context) *mcp.Server {
	h.once.Do(func() {
		if h.factory == nil {
			h.err = errors.New("MCP factory is not configured")
			if h.logger != nil {
				h.logger.Error("mcp factory missing")
			}
			return
		}

		server, err := h.factory.NewSession(mcpserver.SessionContext{
			BearerToken: h.token,
		})
		if err != nil {
			h.err = err
			if h.logger != nil {
				h.logger.Error("failed to create MCP session", "error", err)
			}
			return
		}
		h.server = server
	})
	return h.server
}

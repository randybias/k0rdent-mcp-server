package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/kube"
	"github.com/k0rdent/mcp-k0rdent-server/internal/mcpserver"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
	"github.com/k0rdent/mcp-k0rdent-server/internal/server"
	"github.com/k0rdent/mcp-k0rdent-server/internal/tools/core"
	"github.com/k0rdent/mcp-k0rdent-server/internal/version"
)

const (
	defaultListenAddr = ":8080"
	gracefulTimeout   = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	buildInfo := version.Get()
	logger.Info("starting k0rdent MCP server", "version", buildInfo.Version, "commit", buildInfo.GitCommit)

	settings, err := config.NewLoader().Load(ctx)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	factory, err := kube.NewClientFactory(settings.RestConfig)
	if err != nil {
		logger.Error("failed to initialize Kubernetes client factory", "error", err)
		os.Exit(1)
	}

	rt, err := runtime.New(settings, factory, logger)
	if err != nil {
		logger.Error("failed to prepare runtime", "error", err)
		os.Exit(1)
	}

	sessionOptions := func(ctx *mcpserver.SessionContext) (*mcp.ServerOptions, error) {
		if ctx.Values == nil {
			ctx.Values = make(map[string]any)
		}

		router := core.NewSubscriptionRouter()
		eventManager := core.NewEventManager()
		podLogManager := core.NewPodLogManager()
		graphManager := core.NewGraphManager()

		router.Register("events", eventManager)
		router.Register("podlogs", podLogManager)
		router.Register("graph", graphManager)

		ctx.Values[core.ContextKeyEventManager] = eventManager
		ctx.Values[core.ContextKeyPodLogManager] = podLogManager
		ctx.Values[core.ContextKeyGraphManager] = graphManager

		return &mcp.ServerOptions{
			HasTools:           true,
			HasResources:       true,
			SubscribeHandler:   router.Subscribe,
			UnsubscribeHandler: router.Unsubscribe,
		}, nil
	}

	sessionInitializer := func(s *mcp.Server, ctx *mcpserver.SessionContext) error {
		session, err := rt.NewSession(context.Background(), ctx.BearerToken)
		if err != nil {
			return err
		}
		var (
			eventManager  *core.EventManager
			podLogManager *core.PodLogManager
			graphManager  *core.GraphManager
		)
		if ctx != nil && ctx.Values != nil {
			if mgr, ok := ctx.Values[core.ContextKeyEventManager].(*core.EventManager); ok {
				eventManager = mgr
			}
			if mgr, ok := ctx.Values[core.ContextKeyPodLogManager].(*core.PodLogManager); ok {
				podLogManager = mgr
			}
			if mgr, ok := ctx.Values[core.ContextKeyGraphManager].(*core.GraphManager); ok {
				graphManager = mgr
			}
		}
		return core.Register(s, session, core.Options{
			EventManager:  eventManager,
			PodLogManager: podLogManager,
			GraphManager:  graphManager,
		})
	}

	mcpFactory, err := mcpserver.NewFactory(&mcp.Implementation{
		Name:    "k0rdent-mcp-server",
		Version: buildInfo.Version,
	}, sessionOptions, sessionInitializer)
	if err != nil {
		logger.Error("failed to create MCP factory", "error", err)
		os.Exit(1)
	}

	app, err := server.NewApp(server.Dependencies{
		Settings:      settings,
		ClientFactory: factory,
		MCPFactory:    mcpFactory,
	}, server.Options{
		Logger: logger,
	})
	if err != nil {
		logger.Error("failed to configure HTTP server", "error", err)
		os.Exit(1)
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = defaultListenAddr
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: app.Router(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info("http server listening", "addr", addr, "auth_mode", settings.AuthMode)

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}

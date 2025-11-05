package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/k0rdent/mcp-k0rdent-server/internal/cli"
	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/kube"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/mcpserver"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
	"github.com/k0rdent/mcp-k0rdent-server/internal/server"
	"github.com/k0rdent/mcp-k0rdent-server/internal/tools/core"
	"github.com/k0rdent/mcp-k0rdent-server/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultListenAddr = ":8080"
	gracefulTimeout   = 10 * time.Second
	defaultPIDFile    = "k0rdent-mcp.pid"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "start":
		if err := runStart(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "stop":
		if err := runStop(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `k0rdent MCP Server

Usage:
  k0rdent-mcp start [flags]
  k0rdent-mcp stop [flags]

Commands:
  start   Launch the MCP server and write a PID file for lifecycle management (use --debug to turn on debug logging).
  stop    Send a graceful termination signal to the running server referenced by the PID file.

Use "k0rdent-mcp <command> --help" for more information about a command.
`)
}

type envOverrides []string

func (e *envOverrides) String() string {
	return strings.Join(*e, ",")
}

func (e *envOverrides) Set(value string) error {
	if !strings.Contains(value, "=") {
		return fmt.Errorf("invalid env override %q (expected KEY=VALUE)", value)
	}
	*e = append(*e, value)
	return nil
}

type startFlagValues struct {
	pidFile    *string
	logLevel   *string
	listen     *string
	envs       envOverrides
	debug      *bool
	debugAlias *bool
}

func registerStartFlags(fs *flag.FlagSet) startFlagValues {
	values := startFlagValues{}
	values.pidFile = fs.String("pid-file", defaultPIDFile, "Path to the PID file written by the running server")
	values.logLevel = fs.String("log-level", "", "Override LOG_LEVEL (debug, info, warn, error)")
	values.listen = fs.String("listen", "", "Override LISTEN_ADDR used by the HTTP server")
	fs.Var(&values.envs, "env", "Set additional environment variables (KEY=VALUE). May be specified multiple times.")
	values.debug = fs.Bool("debug", false, "Enable debug logging (overrides --log-level/LOG_LEVEL)")
	values.debugAlias = fs.Bool("d", false, "Alias for --debug")
	return values
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	values := registerStartFlags(fs)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `Usage: k0rdent-mcp start [flags]

Starts the MCP server with the provided configuration options.

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if err := cli.ApplyEnvOverrides([]string(values.envs)); err != nil {
		return err
	}

	if *values.listen != "" {
		if err := os.Setenv("LISTEN_ADDR", *values.listen); err != nil {
			return fmt.Errorf("set LISTEN_ADDR: %w", err)
		}
	}

	debugEnabled := false
	if values.debug != nil && *values.debug {
		debugEnabled = true
	}
	if values.debugAlias != nil && *values.debugAlias {
		debugEnabled = true
	}
	if err := applyLogLevelFlags(debugEnabled, *values.logLevel, os.Stderr); err != nil {
		return err
	}

	if err := ensurePIDDir(*values.pidFile); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	setup, err := initializeServer(ctx)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
		defer cancel()
		_ = setup.logManager.Close(closeCtx)
	}()

	logger := logging.WithComponent(setup.logger, "bootstrap")
	slog.SetDefault(logger)

	if err := cli.WritePID(*values.pidFile, os.Getpid()); err != nil {
		return err
	}
	defer func() {
		_ = cli.RemovePID(*values.pidFile)
	}()

	printStartupSummary(os.Stdout, setup.settings, setup.httpServer.Addr, *values.pidFile)
	logStartupConfiguration(logger, setup.settings, setup.httpServer.Addr, *values.pidFile)

	logger.Info("http server listening", "addr", setup.httpServer.Addr, "auth_mode", setup.authMode)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
		defer cancel()
		if err := setup.httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	if err := setup.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	logger.Info("server stopped")
	return nil
}

func runStop(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `Usage: k0rdent-mcp stop [flags]

Stops the MCP server referenced by the PID file.

Flags:
`)
		fs.PrintDefaults()
	}

	pidFile := fs.String("pid-file", defaultPIDFile, "Path to the PID file created by the running server")
	timeout := fs.Duration("timeout", 10*time.Second, "Maximum time to wait for the server to stop")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	pid, err := cli.ReadPID(*pidFile)
	if err != nil {
		return err
	}

	if err := cli.SignalProcess(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			_ = cli.RemovePID(*pidFile)
			return nil
		}
		return err
	}

	if err := cli.WaitForExit(pid, *timeout); err != nil {
		return err
	}

	return cli.RemovePID(*pidFile)
}

type serverSetup struct {
	httpServer *http.Server
	logger     *slog.Logger
	logManager *logging.Manager
	authMode   config.AuthMode
	settings   *config.Settings
}

func initializeServer(ctx context.Context) (*serverSetup, error) {
	bootstrapManager := logging.NewManager(logging.Options{Level: slog.LevelInfo})
	bootstrapLogger := logging.WithComponent(bootstrapManager.Logger(), "bootstrap")

	buildInfo := version.Get()
	bootstrapLogger.Info("starting k0rdent MCP server", "version", buildInfo.Version, "commit", buildInfo.GitCommit)

	settings, err := config.NewLoader(bootstrapLogger).Load(ctx)
	if err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
		defer cancel()
		_ = bootstrapManager.Close(closeCtx)
		return nil, err
	}

	logOptions := logging.Options{Level: settings.Logging.Level}
	if settings.Logging.ExternalSinkEnabled {
		logOptions.Sink = logging.NewJSONSink(os.Stderr)
	}

	logManager := logging.NewManager(logOptions)
	closeCtx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer cancel()
	_ = bootstrapManager.Close(closeCtx)

	logger := logging.WithComponent(logManager.Logger(), "bootstrap")
	slog.SetDefault(logger)

	factory, err := kube.NewClientFactory(settings.RestConfig, logger)
	if err != nil {
		_ = logManager.Close(context.Background())
		return nil, err
	}

	rt, err := runtime.New(settings, factory, logger)
	if err != nil {
		_ = logManager.Close(context.Background())
		return nil, err
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
		_ = logManager.Close(context.Background())
		return nil, err
	}

	app, err := server.NewApp(server.Dependencies{
		Settings:      settings,
		ClientFactory: factory,
		MCPFactory:    mcpFactory,
	}, server.Options{
		Logger: logger,
	})
	if err != nil {
		_ = logManager.Close(context.Background())
		return nil, err
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = defaultListenAddr
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: app.Router(),
	}

	return &serverSetup{
		httpServer: httpServer,
		logManager: logManager,
		logger:     logger,
		authMode:   settings.AuthMode,
		settings:   settings,
	}, nil
}

func ensurePIDDir(pidFile string) error {
	dir := filepath.Dir(pidFile)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func applyLogLevelFlags(debug bool, logLevelFlag string, stderr io.Writer) error {
	if debug {
		if logLevelFlag != "" && stderr != nil {
			fmt.Fprintln(stderr, "warning: --debug overrides --log-level; using DEBUG log level")
		}
		return os.Setenv("LOG_LEVEL", "DEBUG")
	}
	if logLevelFlag != "" {
		return os.Setenv("LOG_LEVEL", logLevelFlag)
	}
	return nil
}

func printStartupSummary(w io.Writer, settings *config.Settings, listenAddr, pidFile string) {
	if w == nil || settings == nil {
		return
	}

	namespaceFilter := "<none>"
	if settings.NamespaceFilter != nil {
		namespaceFilter = settings.NamespaceFilter.String()
	}

	level := settings.Logging.Level.String()

	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w, "K0rdent MCP Server Startup Summary")
	fmt.Fprintf(w, "  Listen Address:       %s\n", listenAddr)
	fmt.Fprintf(w, "  Auth Mode:            %s\n", settings.AuthMode)
	fmt.Fprintf(w, "  Kubeconfig Source:    %s\n", settings.Source)
	fmt.Fprintf(w, "  Kubeconfig Context:   %s\n", settings.ContextName)
	fmt.Fprintf(w, "  Namespace Filter:     %s\n", namespaceFilter)
	fmt.Fprintf(w, "  Log Level:            %s\n", level)
	fmt.Fprintf(w, "  External Sink:        %t\n", settings.Logging.ExternalSinkEnabled)
	fmt.Fprintf(w, "  PID File:             %s\n", pidFile)
	fmt.Fprintln(w, "========================================")
}

func logStartupConfiguration(logger *slog.Logger, settings *config.Settings, listenAddr, pidFile string) {
	if logger == nil || settings == nil {
		return
	}
	logger.Info("startup configuration", startupSummaryAttributes(settings, listenAddr, pidFile)...)
}

func startupSummaryAttributes(settings *config.Settings, listenAddr, pidFile string) []any {
	namespaceFilter := ""
	if settings.NamespaceFilter != nil {
		namespaceFilter = settings.NamespaceFilter.String()
	}

	level := settings.Logging.Level.String()

	return []any{
		"listen_addr", listenAddr,
		"auth_mode", settings.AuthMode,
		"kubeconfig_source", settings.Source,
		"kubeconfig_context", settings.ContextName,
		"namespace_filter", namespaceFilter,
		"log_level", level,
		"external_sink_enabled", settings.Logging.ExternalSinkEnabled,
		"pid_file", pidFile,
	}
}

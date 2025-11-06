package main

import (
	"bytes"
	"context"
	"flag"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
)

type recordingHandler struct {
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, rec slog.Record) error {
	h.records = append(h.records, rec.Clone())
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordingHandler) WithGroup(string) slog.Handler { return h }

func TestPrintStartupSummary(t *testing.T) {
	buf := &bytes.Buffer{}
	settings := &config.Settings{
		AuthMode:        config.AuthModeDevAllowAny,
		Source:          config.SourcePath,
		ContextName:     "dev",
		NamespaceFilter: regexp.MustCompile(`^team-`),
		Logging: config.LoggingSettings{
			Level:               slog.LevelDebug,
			ExternalSinkEnabled: true,
		},
	}

	printStartupSummary(buf, settings, "127.0.0.1:6767", "/tmp/pid")
	output := buf.String()

	checks := []string{
		"K0rdent MCP Server Startup Summary",
		"Listen Address:       127.0.0.1:6767",
		"Auth Mode:            DEV_ALLOW_ANY",
		"Kubeconfig Source:    path",
		"Namespace Filter:     ^team-",
		"Log Level:            DEBUG",
		"External Sink:        true",
		"PID File:             /tmp/pid",
	}

	for _, expected := range checks {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestStartupSummaryAttributes(t *testing.T) {
	settings := &config.Settings{
		AuthMode:    config.AuthModeOIDCRequired,
		Source:      config.SourceText,
		ContextName: "prod",
		Logging: config.LoggingSettings{
			Level:               slog.LevelWarn,
			ExternalSinkEnabled: false,
		},
	}

	attrs := startupSummaryAttributes(settings, ":8443", "pidfile")
	if len(attrs)%2 != 0 {
		t.Fatalf("expected even number of attribute entries, got %d", len(attrs))
	}

	attrMap := make(map[string]any)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok {
			t.Fatalf("expected string key at index %d", i)
		}
		attrMap[key] = attrs[i+1]
	}

	cases := map[string]any{
		"listen_addr":           ":8443",
		"auth_mode":             config.AuthModeOIDCRequired,
		"kubeconfig_source":     config.SourceText,
		"kubeconfig_context":    "prod",
		"namespace_filter":      "",
		"log_level":             slog.LevelWarn.String(),
		"external_sink_enabled": false,
		"pid_file":              "pidfile",
	}

	for key, want := range cases {
		if got := attrMap[key]; got != want {
			t.Fatalf("attribute %s mismatch: got %v want %v", key, got, want)
		}
	}
}

func TestLogStartupConfiguration(t *testing.T) {
	settings := &config.Settings{
		AuthMode: config.AuthModeDevAllowAny,
		Source:   config.SourceB64,
		Logging:  config.LoggingSettings{Level: slog.LevelInfo},
	}

	handler := &recordingHandler{}
	logger := slog.New(handler)

	logStartupConfiguration(logger, settings, ":9090", "pid")

	if len(handler.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(handler.records))
	}

	if msg := handler.records[0].Message; msg != "startup configuration" {
		t.Fatalf("unexpected log message %q", msg)
	}
}

func TestRegisterStartFlagsDebugAlias(t *testing.T) {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	values := registerStartFlags(fs)

	if err := fs.Parse([]string{"-d"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if values.debugAlias == nil || !*values.debugAlias {
		t.Fatalf("expected -d alias to enable debug")
	}
}

func TestApplyLogLevelFlagsDebugOverrides(t *testing.T) {
	t.Setenv("LOG_LEVEL", "info")
	var buf bytes.Buffer
	if err := applyLogLevelFlags(true, "warn", &buf); err != nil {
		t.Fatalf("applyLogLevelFlags returned error: %v", err)
	}
	if got := os.Getenv("LOG_LEVEL"); got != "DEBUG" {
		t.Fatalf("expected LOG_LEVEL=DEBUG, got %q", got)
	}
	if !strings.Contains(buf.String(), "warning: --debug overrides --log-level") {
		t.Fatalf("expected warning about debug overriding log level, got %q", buf.String())
	}
}

func TestApplyLogLevelFlagsSetsFlagLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	var buf bytes.Buffer
	if err := applyLogLevelFlags(false, "warn", &buf); err != nil {
		t.Fatalf("applyLogLevelFlags returned error: %v", err)
	}
	if got := os.Getenv("LOG_LEVEL"); got != "warn" {
		t.Fatalf("expected LOG_LEVEL=warn, got %q", got)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no warning output when debug disabled, got %q", buf.String())
	}
}

func TestRegisterStartFlagsHelpIncludesDebug(t *testing.T) {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	values := registerStartFlags(fs)
	_ = values // ensure we touch the return value so the compiler keeps the registration

	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.PrintDefaults()

	output := buf.String()
	if !strings.Contains(output, "--debug") {
		t.Fatalf("expected help output to mention --debug, got %q", output)
	}
}

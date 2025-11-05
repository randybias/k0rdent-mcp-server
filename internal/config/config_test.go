package config

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/rest"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

func TestLoadFromPath(t *testing.T) {
	env := map[string]string{
		envKubeconfigPath: "/tmp/kubeconfig",
		envContext:        "dev",
		envNamespaceExpr:  "^team-",
		envAuthMode:       string(AuthModeOIDCRequired),
	}

	kubeconfigYAML := strings.TrimSpace(`
apiVersion: v1
clusters:
- cluster:
    server: https://example.com
  name: prod
contexts:
- context:
    cluster: prod
    user: default
  name: prod
- context:
    cluster: prod
    user: dev-user
  name: dev
current-context: prod
users:
- name: default
  user:
    token: prod-token
- name: dev-user
  user:
    token: dev-token
`)

	loader := NewLoader(testLogger())
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(path string) ([]byte, error) {
		if path != "/tmp/kubeconfig" {
			t.Fatalf("unexpected path %q", path)
		}
		return []byte(kubeconfigYAML), nil
	}
	loader.ping = func(ctx context.Context, cfg *rest.Config) error {
		if cfg == nil {
			t.Fatal("rest config is nil")
		}
		if got, want := cfg.Host, "https://example.com"; got != want {
			t.Fatalf("unexpected host: got %q want %q", got, want)
		}
		return nil
	}

	settings, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if settings.Source != SourcePath {
		t.Fatalf("expected SourcePath, got %q", settings.Source)
	}
	if settings.ContextName != "dev" {
		t.Fatalf("expected context %q, got %q", "dev", settings.ContextName)
	}
	if settings.AuthMode != AuthModeOIDCRequired {
		t.Fatalf("expected auth mode %q, got %q", AuthModeOIDCRequired, settings.AuthMode)
	}
	if settings.NamespaceFilter == nil {
		t.Fatal("expected namespace filter regex to be compiled")
	}
	if !settings.NamespaceFilter.MatchString("team-alpha") {
		t.Fatal("compiled namespace filter does not match expected value")
	}
	if settings.NamespaceFilter.MatchString("other") {
		t.Fatal("compiled namespace filter should not match 'other'")
	}
	if settings.Logging.Level != slog.LevelInfo {
		t.Fatalf("expected default log level INFO, got %s", settings.Logging.Level.String())
	}
	if settings.Logging.ExternalSinkEnabled {
		t.Fatalf("expected external sink disabled by default")
	}
}

func TestLoadRejectsInvalidBase64(t *testing.T) {
	env := map[string]string{
		envKubeconfigB64: "!!!notbase64!!!",
	}

	loader := NewLoader(testLogger())
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(string) ([]byte, error) {
		return nil, errors.New("should not be called")
	}
	loader.ping = func(context.Context, *rest.Config) error {
		return errors.New("should not be called")
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid base64 input")
	}
	if !strings.Contains(err.Error(), "decode K0RDENT_MGMT_KUBECONFIG_B64") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestLoadRejectsMultipleSources(t *testing.T) {
	env := map[string]string{
		envKubeconfigPath: "/tmp/config",
		envKubeconfigText: "fake",
	}

	loader := NewLoader(testLogger())
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(string) ([]byte, error) {
		return nil, errors.New("should not be called")
	}
	loader.ping = func(context.Context, *rest.Config) error {
		return errors.New("should not be called")
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error when multiple kubeconfig sources are set")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected only one source error, got %v", err)
	}
}

func TestLoadRejectsInvalidNamespaceRegex(t *testing.T) {
	env := map[string]string{
		envKubeconfigText: minimalKubeconfig(),
		envNamespaceExpr:  "(",
	}

	loader := NewLoader(testLogger())
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(string) ([]byte, error) {
		return nil, errors.New("should not be called")
	}
	loader.ping = func(context.Context, *rest.Config) error {
		return nil
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid namespace regex")
	}
	if !strings.Contains(err.Error(), "compile namespace filter regex") {
		t.Fatalf("expected namespace regex error, got %v", err)
	}
}

func TestLoadRejectsInvalidAuthMode(t *testing.T) {
	env := map[string]string{
		envKubeconfigText: minimalKubeconfig(),
		envAuthMode:       "NOT_VALID",
	}

	loader := NewLoader(testLogger())
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(string) ([]byte, error) {
		return nil, errors.New("should not be called")
	}
	loader.ping = func(context.Context, *rest.Config) error {
		return nil
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid AUTH_MODE")
	}
	if !strings.Contains(err.Error(), "invalid AUTH_MODE") {
		t.Fatalf("expected invalid AUTH_MODE error, got %v", err)
	}
}

func TestLoadEmitsStructuredLogs(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	sink := &recordingSink{}
	mgr := logging.NewManager(logging.Options{
		Level:       slog.LevelDebug,
		Sink:        sink,
		Destination: &buf,
	})
	logger := mgr.Logger()
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = mgr.Close(closeCtx)
	})

	env := map[string]string{
		envKubeconfigText: minimalKubeconfig(),
	}

	loader := NewLoader(logger)
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(string) ([]byte, error) {
		return nil, errors.New("should not be called")
	}
	loader.ping = func(context.Context, *rest.Config) error {
		return nil
	}

	if _, err := loader.Load(ctx); err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	waitUntil(func() bool { return sink.Len() >= 2 }, 500*time.Millisecond)

	if sink.Len() == 0 {
		t.Fatalf("expected sink to receive log entries")
	}
	if !strings.Contains(buf.String(), `"component":"config.loader"`) {
		t.Fatalf("expected stdout log to include component attribute, got %s", buf.String())
	}
}

func TestResolveLoggingLevelInvalid(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	sink := &recordingSink{}
	mgr := logging.NewManager(logging.Options{
		Level:       slog.LevelDebug,
		Sink:        sink,
		Destination: &buf,
	})
	logger := mgr.Logger()
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = mgr.Close(closeCtx)
	})

	env := map[string]string{
		envKubeconfigText: minimalKubeconfig(),
		envLogLevel:       "LOUD",
	}

	loader := NewLoader(logger)
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(string) ([]byte, error) { return nil, errors.New("should not be called") }
	loader.ping = func(context.Context, *rest.Config) error { return nil }

	settings, err := loader.Load(ctx)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if settings.Logging.Level != slog.LevelInfo {
		t.Fatalf("expected fallback INFO level, got %s", settings.Logging.Level.String())
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for sink.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if sink.Len() == 0 {
		t.Fatalf("expected log warning for invalid level")
	}
}

func TestResolveLoggingSinkEnabled(t *testing.T) {
	ctx := context.Background()

	loader := NewLoader(testLogger())
	env := map[string]string{
		envKubeconfigText: minimalKubeconfig(),
		envLogSinkEnabled: "true",
		envLogLevel:       "debug",
	}
	loader.envLookup = func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
	loader.readFile = func(string) ([]byte, error) { return nil, errors.New("should not be called") }
	loader.ping = func(context.Context, *rest.Config) error { return nil }

	settings, err := loader.Load(ctx)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if !settings.Logging.ExternalSinkEnabled {
		t.Fatalf("expected external sink to be enabled")
	}
	if settings.Logging.Level != slog.LevelDebug {
		t.Fatalf("expected debug level, got %s", settings.Logging.Level.String())
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

type recordingSink struct {
	mu      sync.Mutex
	entries []logging.Entry
}

func (s *recordingSink) Write(_ context.Context, e logging.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	return nil
}

func (s *recordingSink) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func waitUntil(fn func() bool, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for !fn() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
}

func minimalKubeconfig() string {
	return strings.TrimSpace(`
apiVersion: v1
clusters:
- cluster:
    server: https://example.com
  name: main
contexts:
- context:
    cluster: main
    user: default
  name: main
current-context: main
users:
- name: default
  user:
    token: token
`)
}

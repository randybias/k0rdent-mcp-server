package kube

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

func TestRESTConfigForToken(t *testing.T) {
	base := &rest.Config{
		Host:        "https://example.com",
		BearerToken: "base-token",
		Username:    "user",
		Password:    "pass",
	}

	factory, err := NewClientFactory(base, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewClientFactory returned error: %v", err)
	}

	cfg, err := factory.RESTConfigForToken("override")
	if err != nil {
		t.Fatalf("RESTConfigForToken returned error: %v", err)
	}

	if cfg == base {
		t.Fatal("expected REST config to be copied, but references match")
	}
	if got, want := cfg.BearerToken, "override"; got != want {
		t.Fatalf("unexpected bearer token: got %q want %q", got, want)
	}
	if cfg.BearerTokenFile != "" {
		t.Fatalf("expected bearer token file to be cleared, got %q", cfg.BearerTokenFile)
	}
	if cfg.Username != "" || cfg.Password != "" {
		t.Fatalf("expected basic auth credentials to be cleared, got user=%q pass=%q", cfg.Username, cfg.Password)
	}

	if base.BearerToken != "base-token" {
		t.Fatalf("expected base config bearer token to remain unchanged, got %q", base.BearerToken)
	}
	if base.Username != "user" || base.Password != "pass" {
		t.Fatalf("expected base basic auth credentials to remain unchanged, got user=%q pass=%q", base.Username, base.Password)
	}
}

func TestKubernetesClientDelegatesToConstructor(t *testing.T) {
	base := &rest.Config{
		Host: "https://example.com",
	}

	factory, err := NewClientFactory(base, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewClientFactory returned error: %v", err)
	}

	var capturedToken string
	stub := func(cfg *rest.Config) (kubernetes.Interface, error) {
		capturedToken = cfg.BearerToken
		return nil, nil
	}

	factory.WithConstructors(stub, nil)

	if _, err := factory.KubernetesClient("from-request"); err != nil {
		t.Fatalf("KubernetesClient returned error: %v", err)
	}

	if capturedToken != "from-request" {
		t.Fatalf("expected constructor to receive overridden bearer token, got %q", capturedToken)
	}
}

func TestDynamicClientDelegatesToConstructor(t *testing.T) {
	base := &rest.Config{
		Host: "https://example.com",
	}

	factory, err := NewClientFactory(base, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewClientFactory returned error: %v", err)
	}

	var capturedHost string
	stub := func(cfg *rest.Config) (dynamic.Interface, error) {
		capturedHost = cfg.Host
		return nil, errors.New("stop here")
	}

	factory.WithConstructors(nil, stub)

	if _, err := factory.DynamicClient(""); err == nil || err.Error() != "create dynamic client: stop here" {
		t.Fatalf("expected wrapped constructor error, got %v", err)
	}

	if capturedHost != "https://example.com" {
		t.Fatalf("expected constructor to receive copied config, got host %q", capturedHost)
	}
}

func TestFactoryEmitsLogs(t *testing.T) {
	var buf bytes.Buffer
	sink := &recordingSink{}
	mgr := logging.NewManager(logging.Options{
		Level:       slog.LevelDebug,
		Sink:        sink,
		Destination: &buf,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = mgr.Close(ctx)
	})

	base := &rest.Config{Host: "https://example.com"}
	factory, err := NewClientFactory(base, mgr.Logger())
	if err != nil {
		t.Fatalf("NewClientFactory error: %v", err)
	}

	var once sync.Once
	factory.WithConstructors(func(cfg *rest.Config) (kubernetes.Interface, error) {
		once.Do(func() {
			if cfg.BearerToken != "token" {
				t.Errorf("expected token override, got %q", cfg.BearerToken)
			}
		})
		return nil, errors.New("kube fail")
	}, func(_ *rest.Config) (dynamic.Interface, error) {
		return nil, errors.New("dynamic fail")
	})

	_, _ = factory.KubernetesClient("token")
	_, _ = factory.DynamicClient("")

	waitUntil(func() bool { return sink.Len() >= 2 }, 500*time.Millisecond)
	if sink.Len() == 0 {
		t.Fatalf("expected sink to capture logs")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"component":"kube.clientfactory"`)) {
		t.Fatalf("expected stdout log to include component attribute, got %s", buf.String())
	}
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

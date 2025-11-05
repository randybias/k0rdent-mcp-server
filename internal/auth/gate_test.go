package auth

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

func TestExtractBearerModes(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	g := NewGate(config.AuthModeOIDCRequired, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	if _, err := g.ExtractBearer(req); err == nil {
		t.Fatal("expected missing bearer token to error in OIDC mode")
	}

	req.Header.Set("Authorization", "Bearer token")
	token, err := g.ExtractBearer(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "token" {
		t.Fatalf("unexpected token %q", token)
	}

	devGate := NewGate(config.AuthModeDevAllowAny, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	req.Header.Del("Authorization")
	if token, err := devGate.ExtractBearer(req); err != nil || token != "" {
		t.Fatalf("expected dev mode to allow missing bearer, got token=%q err=%v", token, err)
	}
}

func TestExtractBearerMalformed(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Basic foo")
	g := NewGate(config.AuthModeDevAllowAny, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if _, err := g.ExtractBearer(req); err == nil {
		t.Fatal("expected error for non-Bearer scheme")
	}

	req.Header.Set("Authorization", "Bearer   ")
	if _, err := g.ExtractBearer(req); err == nil {
		t.Fatal("expected error for empty bearer token")
	}
}

func TestExtractBearerLogs(t *testing.T) {
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

	gate := NewGate(config.AuthModeOIDCRequired, mgr.Logger())

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)
	_, _ = gate.ExtractBearer(req)

	waitUntil(func() bool { return sink.Len() > 0 }, 500*time.Millisecond)
	if sink.Len() == 0 {
		t.Fatalf("expected sink to capture gate log")
	}
	if !stringsContains(buf.String(), `"component":"auth.gate"`) {
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

func stringsContains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/mcpserver"
)

func newTestFactory(t *testing.T) *mcpserver.Factory {
	t.Helper()
	impl := &mcp.Implementation{Name: "test", Version: "1.0.0"}
	factory, err := mcpserver.NewFactory(impl, nil, func(*mcp.Server, *mcpserver.SessionContext) error {
		return nil
	})
	if err != nil {
		t.Fatalf("NewFactory returned error: %v", err)
	}
	return factory
}

func TestNewAppMissingDeps(t *testing.T) {
	_, err := NewApp(Dependencies{}, Options{})
	if err == nil {
		t.Fatal("expected error when required dependencies missing")
	}
}

func TestHandleHealth(t *testing.T) {
	app, err := NewApp(Dependencies{
		Settings:   &config.Settings{AuthMode: config.AuthModeDevAllowAny},
		MCPFactory: newTestFactory(t),
	}, Options{})
	if err != nil {
		t.Fatalf("NewApp returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	app.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %#v", body["status"])
	}
}

func TestHandleStreamUnauthorized(t *testing.T) {
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

	app, err := NewApp(Dependencies{
		Settings:   &config.Settings{AuthMode: config.AuthModeOIDCRequired},
		MCPFactory: newTestFactory(t),
	}, Options{Logger: mgr.Logger()})
	if err != nil {
		t.Fatalf("NewApp returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()

	app.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	deadline := time.Now().Add(time.Second)
	var (
		entry logging.Entry
		ok    bool
	)
	for time.Now().Before(deadline) {
		if entry, ok = sink.Find("handled mcp request"); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ok {
		t.Fatalf("expected handled mcp request log, captured entries: %v\nstdout: %s", sink.Messages(), buf.String())
	}
	statusAttr, exists := entry.Attributes["status"]
	if !exists {
		t.Fatalf("expected status attribute, got %#v", entry.Attributes)
	}
	var status int
	switch v := statusAttr.(type) {
	case int:
		status = v
	case int64:
		status = int(v)
	case float64:
		status = int(v)
	default:
		t.Fatalf("unexpected status attribute type %T", statusAttr)
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("expected status attribute 401, got %#v", statusAttr)
	}
	if entry.Attributes["duration_ms"] == nil {
		t.Fatalf("expected duration_ms attribute")
	}
	if _, ok := entry.Attributes["request_id"]; !ok {
		t.Fatalf("expected request_id attribute in log entry, got %#v", entry.Attributes)
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

func (s *recordingSink) Find(msg string) (logging.Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		if entry.Message == msg {
			return entry, true
		}
	}
	return logging.Entry{}, false
}

func (s *recordingSink) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := make([]string, 0, len(s.entries))
	for _, entry := range s.entries {
		msgs = append(msgs, entry.Message)
	}
	return msgs
}

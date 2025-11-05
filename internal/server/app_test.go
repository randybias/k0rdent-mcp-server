package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
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
	app, err := NewApp(Dependencies{
		Settings:   &config.Settings{AuthMode: config.AuthModeOIDCRequired},
		MCPFactory: newTestFactory(t),
	}, Options{})
	if err != nil {
		t.Fatalf("NewApp returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()

	app.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

package auth

import (
	"net/http"
	"testing"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
)

func TestExtractBearerModes(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	g := NewGate(config.AuthModeOIDCRequired)

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

	devGate := NewGate(config.AuthModeDevAllowAny)
	req.Header.Del("Authorization")
	if token, err := devGate.ExtractBearer(req); err != nil || token != "" {
		t.Fatalf("expected dev mode to allow missing bearer, got token=%q err=%v", token, err)
	}
}

func TestExtractBearerMalformed(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Basic foo")
	g := NewGate(config.AuthModeDevAllowAny)
	if _, err := g.ExtractBearer(req); err == nil {
		t.Fatal("expected error for non-Bearer scheme")
	}

	req.Header.Set("Authorization", "Bearer   ")
	if _, err := g.ExtractBearer(req); err == nil {
		t.Fatal("expected error for empty bearer token")
	}
}

package mcpserver

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFactoryNewSession(t *testing.T) {
	impl := &mcp.Implementation{
		Name:    "test",
		Version: "v0.0.0",
	}

	var receivedToken string
	builder := func(ctx *SessionContext) (*mcp.ServerOptions, error) {
		return &mcp.ServerOptions{
			Instructions: "hello",
			HasTools:     true,
		}, nil
	}
	init := func(server *mcp.Server, ctx *SessionContext) error {
		if server == nil {
			t.Fatal("expected server instance")
		}
		receivedToken = ctx.BearerToken
		return nil
	}

	factory, err := NewFactory(impl, builder, init)
	if err != nil {
		t.Fatalf("NewFactory returned error: %v", err)
	}

	server, err := factory.NewSession(SessionContext{BearerToken: "token"})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if server == nil {
		t.Fatal("expected server instance")
	}
	if receivedToken != "token" {
		t.Fatalf("initializer did not receive token, got %q", receivedToken)
	}
}

func TestNewFactoryRejectsNilImplementation(t *testing.T) {
	if _, err := NewFactory(nil, nil, nil); err == nil {
		t.Fatal("expected error for nil implementation")
	}
}

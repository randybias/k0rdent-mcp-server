package mcpserver

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SessionContext carries connection specific metadata that tool handlers may require.
type SessionContext struct {
	BearerToken string
	Values      map[string]any
}

// SessionInitializer populates an MCP server instance with tools, resources, and subscriptions.
type SessionInitializer func(server *mcp.Server, ctx *SessionContext) error

// OptionsBuilder constructs server options for an incoming session.
type OptionsBuilder func(ctx *SessionContext) (*mcp.ServerOptions, error)

// Factory creates fully configured MCP server instances per incoming session.
type Factory struct {
	impl       *mcp.Implementation
	buildOpts  OptionsBuilder
	initialize SessionInitializer
}

// NewFactory constructs a Factory. The implementation argument must not be nil.
func NewFactory(impl *mcp.Implementation, builder OptionsBuilder, init SessionInitializer) (*Factory, error) {
	if impl == nil {
		return nil, fmt.Errorf("mcp implementation is required")
	}
	return &Factory{
		impl:       impl,
		buildOpts:  builder,
		initialize: init,
	}, nil
}

// NewSession creates a new MCP server for the given session context.
func (f *Factory) NewSession(ctx SessionContext) (*mcp.Server, error) {
	if f == nil {
		return nil, fmt.Errorf("factory is nil")
	}

	var opts *mcp.ServerOptions
	if f.buildOpts != nil {
		if ctx.Values == nil {
			ctx.Values = make(map[string]any)
		}
		var err error
		opts, err = f.buildOpts(&ctx)
		if err != nil {
			return nil, err
		}
	}

	server := mcp.NewServer(f.impl, opts)
	if f.initialize != nil {
		if err := f.initialize(server, &ctx); err != nil {
			return nil, err
		}
	}
	return server, nil
}

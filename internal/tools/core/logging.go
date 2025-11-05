package core

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

func toolContext(ctx context.Context, session *runtime.Session, toolName, component string) (context.Context, *slog.Logger) {
	ctx = logging.WithToolName(ctx, toolName)
	var base *slog.Logger
	if session != nil && session.Logger != nil {
		base = session.Logger
	} else {
		base = slog.Default()
	}

	logger := logging.WithContext(ctx, base)
	if component != "" {
		logger = logging.WithComponent(logger, component)
	}
	if toolName != "" {
		logger = logger.With("tool", toolName)
	}
	return ctx, logger
}

func toolName(req *mcp.CallToolRequest) string {
	if req == nil || req.Params == nil {
		return ""
	}
	return req.Params.Name
}

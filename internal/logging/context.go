package logging

import (
	"context"
	"log/slog"
)

type contextKey string

const (
	requestIDKey contextKey = "logging-request-id"
	sessionIDKey contextKey = "logging-session-id"
	toolNameKey  contextKey = "logging-tool-name"
	namespaceKey contextKey = "logging-namespace"
)

// WithRequestID stores an HTTP request identifier in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID extracts an HTTP request identifier from the context, if present.
func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// WithSessionID stores an MCP session identifier in the context.
func WithSessionID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionIDKey, id)
}

// SessionID returns the MCP session identifier from the context.
func SessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return ""
}

// WithToolName annotates the context with the active tool name.
func WithToolName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, toolNameKey, name)
}

// ToolName fetches the tool name from the context when available.
func ToolName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(toolNameKey).(string); ok {
		return v
	}
	return ""
}

// WithNamespace tracks the namespace under execution.
func WithNamespace(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, namespaceKey, name)
}

// Namespace retrieves the namespace from the context.
func Namespace(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(namespaceKey).(string); ok {
		return v
	}
	return ""
}

// WithComponent returns a logger annotated with the provided component name.
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	if logger == nil || component == "" {
		return logger
	}
	return logger.With("component", component)
}

// WithContext enriches a logger with correlation identifiers found in ctx.
func WithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return nil
	}

	attrs := make([]any, 0, 4)
	if id := RequestID(ctx); id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	if id := SessionID(ctx); id != "" {
		attrs = append(attrs, slog.String("session_id", id))
	}
	if tool := ToolName(ctx); tool != "" {
		attrs = append(attrs, slog.String("tool", tool))
	}
	if ns := Namespace(ctx); ns != "" {
		attrs = append(attrs, slog.String("namespace", ns))
	}
	if len(attrs) == 0 {
		return logger
	}
	return logger.With(attrs...)
}

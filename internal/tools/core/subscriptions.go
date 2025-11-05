package core

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SubscriptionHandler defines the contract for streamable resources.
type SubscriptionHandler interface {
	Subscribe(ctx context.Context, req *mcp.SubscribeRequest) error
	Unsubscribe(ctx context.Context, req *mcp.UnsubscribeRequest) error
}

// SubscriptionRouter routes subscribe/unsubscribe requests to host-specific handlers.
type SubscriptionRouter struct {
	mu       sync.RWMutex
	handlers map[string]SubscriptionHandler
}

// NewSubscriptionRouter creates a router with no handlers.
func NewSubscriptionRouter() *SubscriptionRouter {
	return &SubscriptionRouter{handlers: make(map[string]SubscriptionHandler)}
}

// Register associates the given host with a handler.
func (r *SubscriptionRouter) Register(host string, handler SubscriptionHandler) {
	if r == nil || handler == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[strings.ToLower(host)] = handler
}

// Subscribe routes the request to the appropriate handler.
func (r *SubscriptionRouter) Subscribe(ctx context.Context, req *mcp.SubscribeRequest) error {
	handler, err := r.lookup(req.Params.URI)
	if err != nil {
		return err
	}
	return handler.Subscribe(ctx, req)
}

// Unsubscribe routes the request to the appropriate handler.
func (r *SubscriptionRouter) Unsubscribe(ctx context.Context, req *mcp.UnsubscribeRequest) error {
	handler, err := r.lookup(req.Params.URI)
	if err != nil {
		return err
	}
	return handler.Unsubscribe(ctx, req)
}

func (r *SubscriptionRouter) lookup(rawURI string) (SubscriptionHandler, error) {
	if r == nil {
		return nil, fmt.Errorf("subscription router not configured")
	}
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription URI: %w", err)
	}
	if parsed.Scheme != eventsScheme { // scheme shared across handlers ('k0')
		return nil, fmt.Errorf("unsupported subscription scheme %q", parsed.Scheme)
	}

	host := strings.ToLower(parsed.Host)
	r.mu.RLock()
	handler, ok := r.handlers[host]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no subscription handler registered for host %q", parsed.Host)
	}
	return handler, nil
}

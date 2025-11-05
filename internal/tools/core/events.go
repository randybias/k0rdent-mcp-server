package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/watch"

	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

const (
	eventsURITemplate = "k0://events/{namespace}"
	eventsScheme      = "k0"
	eventsHost        = "events"
	eventsMIMEType    = "application/json"
)

// EventManager coordinates namespace event subscriptions across a session.
type EventManager struct {
	mu            sync.Mutex
	server        *mcp.Server
	session       *runtime.Session
	subscriptions map[string]*eventSubscription
}

// eventSubscription tracks the lifecycle of a namespace watch.
type eventSubscription struct {
	namespace string
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewEventManager creates an EventManager ready to be bound to a session.
func NewEventManager() *EventManager {
	return &EventManager{
		subscriptions: make(map[string]*eventSubscription),
	}
}

// Bind attaches runtime dependencies to the manager. Safe to call multiple times.
func (m *EventManager) Bind(server *mcp.Server, session *runtime.Session) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.server = server
	m.session = session
}

// Subscribe begins streaming events for the namespace represented by the URI.
func (m *EventManager) Subscribe(ctx context.Context, req *mcp.SubscribeRequest) error {
	if m == nil {
		return fmt.Errorf("event manager not configured")
	}
	namespace, err := parseEventsURI(req.Params.URI)
	if err != nil {
		return err
	}

	ctx = logging.WithNamespace(ctx, namespace)
	ctx, logger := toolContext(ctx, m.session, "k0.events.subscribe", "tool.events")
	logger = logger.With("namespace", namespace)
	logger.Info("subscribing to namespace events")

	m.mu.Lock()
	if m.session == nil || m.session.Events == nil || m.server == nil {
		m.mu.Unlock()
		logger.Error("event manager not bound to session")
		return fmt.Errorf("event manager not bound to session")
	}
	if _, exists := m.subscriptions[namespace]; exists {
		m.mu.Unlock()
		logger.Debug("event subscription already active")
		return nil
	}

	watchCtx, cancel := context.WithCancel(context.Background())
	deltaCh, errCh, err := m.session.Events.WatchNamespace(watchCtx, namespace, eventsprovider.WatchOptions{})
	if err != nil {
		cancel()
		m.mu.Unlock()
		logger.Error("failed to start event watch", "error", err)
		return fmt.Errorf("start event watch: %w", err)
	}

	sub := &eventSubscription{
		namespace: namespace,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	m.subscriptions[namespace] = sub
	server := m.server
	provider := m.session.Events
	m.mu.Unlock()

	go m.streamEvents(watchCtx, server, namespace, deltaCh, errCh, sub)

	// Send an initial snapshot so subscribers have immediate context.
	go m.sendInitialSnapshot(watchCtx, server, namespace, provider)

	logger.Info("event subscription started")
	return nil
}

// Unsubscribe terminates the active watch for the provided namespace URI.
func (m *EventManager) Unsubscribe(ctx context.Context, req *mcp.UnsubscribeRequest) error {
	if m == nil {
		return fmt.Errorf("event manager not configured")
	}
	namespace, err := parseEventsURI(req.Params.URI)
	if err != nil {
		return err
	}

	ctx = logging.WithNamespace(ctx, namespace)
	ctx, logger := toolContext(ctx, m.session, "k0.events.unsubscribe", "tool.events")
	logger = logger.With("namespace", namespace)
	logger.Info("unsubscribing from namespace events")

	m.mu.Lock()
	sub, ok := m.subscriptions[namespace]
	if ok {
		delete(m.subscriptions, namespace)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}

	sub.cancel()
	<-sub.done
	logger.Info("event subscription terminated")
	return nil
}

func (m *EventManager) streamEvents(ctx context.Context, server *mcp.Server, namespace string, deltaCh <-chan eventsprovider.Delta, errCh <-chan error, sub *eventSubscription) {
	defer close(sub.done)

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if ok && err != nil {
				// Surface the error as a synthetic log entry to subscribers.
				m.publishEvent(server, namespace, watch.Error, eventsprovider.Event{Message: err.Error(), Namespace: namespace})
			}
			return
		case delta, ok := <-deltaCh:
			if !ok {
				return
			}
			m.publishEvent(server, namespace, delta.Type, delta.Event)
		}
	}
}

func (m *EventManager) sendInitialSnapshot(ctx context.Context, server *mcp.Server, namespace string, provider *eventsprovider.Provider) {
	events, err := provider.List(ctx, namespace, eventsprovider.ListOptions{})
	if err != nil {
		m.publishEvent(server, namespace, watch.Error, eventsprovider.Event{Message: err.Error(), Namespace: namespace})
		return
	}
	for _, evt := range events {
		select {
		case <-ctx.Done():
			return
		default:
			m.publishEvent(server, namespace, watch.Added, evt)
		}
	}
}

func (m *EventManager) publishEvent(server *mcp.Server, namespace string, eventType watch.EventType, event eventsprovider.Event) {
	if server == nil {
		return
	}
	payload := struct {
		Action string               `json:"action"`
		Event  eventsprovider.Event `json:"event"`
	}{
		Action: string(eventType),
		Event:  event,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	uri := buildEventsURI(namespace)
	params := &mcp.ResourceUpdatedNotificationParams{
		URI: uri,
		Meta: mcp.Meta{
			"delta": json.RawMessage(data),
		},
	}
	_ = server.ResourceUpdated(context.Background(), params)
}

type eventsTool struct {
	session *runtime.Session
}

type eventsListInput struct {
	Namespace    string   `json:"namespace" jsonschema:"Namespace to query"`
	Types        []string `json:"types,omitempty"`
	ForKind      string   `json:"forKind,omitempty"`
	ForName      string   `json:"forName,omitempty"`
	SinceSeconds *int64   `json:"sinceSeconds,omitempty"`
	Limit        *int     `json:"limit,omitempty"`
}

type eventsListResult struct {
	Events []eventsprovider.Event `json:"events"`
}

func registerEvents(server *mcp.Server, session *runtime.Session, manager *EventManager) error {
	if session == nil || session.Events == nil {
		return errors.New("session events provider is not configured")
	}

	if manager != nil {
		manager.Bind(server, session)
	}

	tool := &eventsTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0.events.list",
		Description: "List Kubernetes events for a namespace",
	}, tool.list)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "k0.events",
		Title:       "Kubernetes namespace events",
		Description: "Streaming events scoped to a Kubernetes namespace",
		URITemplate: eventsURITemplate,
		MIMEType:    eventsMIMEType,
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		namespace, err := parseEventsURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		events, err := session.Events.List(ctx, namespace, eventsprovider.ListOptions{})
		if err != nil {
			return nil, err
		}
		payload := eventsListResult{Events: events}
		blob, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: eventsMIMEType,
				Blob:     blob,
			}},
		}, nil
	})

	return nil
}

func (t *eventsTool) list(ctx context.Context, req *mcp.CallToolRequest, input eventsListInput) (*mcp.CallToolResult, eventsListResult, error) {
	if input.Namespace == "" {
		return nil, eventsListResult{}, fmt.Errorf("namespace is required")
	}

	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.events")
	start := time.Now()

	options := eventsprovider.ListOptions{
		Types:        input.Types,
		ForKind:      input.ForKind,
		ForName:      input.ForName,
		SinceSeconds: input.SinceSeconds,
		Limit:        input.Limit,
	}

	logger.Debug("listing namespace events",
		"tool", name,
		"namespace", input.Namespace,
		"types", input.Types,
		"for_kind", input.ForKind,
		"for_name", input.ForName,
		"since_seconds", derefInt64(input.SinceSeconds),
		"limit", derefInt(input.Limit),
	)

	events, err := t.session.Events.List(ctx, input.Namespace, options)
	if err != nil {
		logger.Error("list events failed", "tool", name, "namespace", input.Namespace, "error", err)
		return nil, eventsListResult{}, err
	}

	logger.Info("events listed",
		"tool", name,
		"namespace", input.Namespace,
		"count", len(events),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, eventsListResult{Events: events}, nil
}

func parseEventsURI(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("subscription URI is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid events URI: %w", err)
	}
	if parsed.Scheme != eventsScheme {
		return "", fmt.Errorf("unexpected events scheme %q", parsed.Scheme)
	}
	if !strings.EqualFold(parsed.Host, eventsHost) {
		return "", fmt.Errorf("unexpected events host %q", parsed.Host)
	}
	namespace := strings.Trim(parsed.Path, "/")
	if namespace == "" {
		return "", fmt.Errorf("events namespace is required")
	}
	return namespace, nil
}

func buildEventsURI(namespace string) string {
	return fmt.Sprintf("%s://%s/%s", eventsScheme, eventsHost, namespace)
}

func derefInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func derefInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

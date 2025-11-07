package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	logsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/logs"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

const (
	podLogsScheme      = "k0rdent"
	podLogsHost        = "podlogs"
	podLogsURITemplate = "k0rdent://podlogs/{namespace}/{pod}/{container}"
	podLogsMIMEType    = "text/plain"
)

type podLogKey struct {
	Namespace    string
	Pod          string
	Container    string
	Previous     bool
	TailLines    *int64
	SinceSeconds *int64
}

// PodLogManager manages streaming pod log subscriptions.
type PodLogManager struct {
	mu      sync.Mutex
	server  *mcp.Server
	session *runtime.Session
	streams map[string]*logSubscription
}

type logSubscription struct {
	key    podLogKey
	cancel context.CancelFunc
	done   chan struct{}
	seq    int64
}

// NewPodLogManager returns a manager ready for binding.
func NewPodLogManager() *PodLogManager {
	return &PodLogManager{streams: make(map[string]*logSubscription)}
}

// Bind associates the underlying runtime dependencies.
func (m *PodLogManager) Bind(server *mcp.Server, session *runtime.Session) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.server = server
	m.session = session
}

// Subscribe ensures a streaming tail is active for the requested pod logs.
func (m *PodLogManager) Subscribe(ctx context.Context, req *mcp.SubscribeRequest) error {
	if m == nil {
		return fmt.Errorf("pod log manager not configured")
	}
	key, uri, err := parsePodLogURI(req.Params.URI)
	if err != nil {
		return err
	}
	ctx = logging.WithNamespace(ctx, key.Namespace)
	ctx, logger := toolContext(ctx, m.session, "k0rdent.podLogs.follow", "tool.podlogs")
	logger = logger.With("tool", "k0rdent.podLogs.follow", "pod", key.Pod, "container", key.Container, "uri", uri)
	logger.Info("subscribing to pod log stream")
	_, err = m.ensureStream(uri, key)
	if err != nil {
		logger.Error("failed to subscribe to pod logs", "error", err)
		return err
	}
	logger.Info("pod log stream active")
	return err
}

// Unsubscribe terminates the tail for the requested pod logs.
func (m *PodLogManager) Unsubscribe(ctx context.Context, req *mcp.UnsubscribeRequest) error {
	if m == nil {
		return fmt.Errorf("pod log manager not configured")
	}
	key, uri, err := parsePodLogURI(req.Params.URI)
	if err != nil {
		return err
	}

	ctx = logging.WithNamespace(ctx, key.Namespace)
	ctx, logger := toolContext(ctx, m.session, "k0rdent.podLogs.unfollow", "tool.podlogs")
	logger = logger.With("tool", "k0rdent.podLogs.unfollow", "pod", key.Pod, "container", key.Container, "uri", uri)
	logger.Info("unsubscribing from pod log stream")

	m.mu.Lock()
	sub, ok := m.streams[uri]
	if ok {
		delete(m.streams, uri)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}

	sub.cancel()
	<-sub.done
	logger.Info("pod log stream terminated")
	return nil
}

// EnsureStream starts a stream for the given configuration if one is not already running.
func (m *PodLogManager) EnsureStream(key podLogKey) (string, error) {
	var logger *slog.Logger
	if m != nil && m.session != nil && m.session.Logger != nil {
		logger = logging.WithComponent(m.session.Logger, "tool.podlogs")
		logger = logger.With(
			"tool", "k0rdent.podLogs.follow",
			"namespace", key.Namespace,
			"pod", key.Pod,
			"container", key.Container,
		)
		logger.Info("ensuring pod log stream")
	}
	uri := buildURIFromKey(key)
	if _, err := m.ensureStream(uri, key); err != nil {
		if logger != nil {
			logger.Error("failed to ensure pod log stream", "error", err)
		}
		return "", err
	}
	if logger != nil {
		logger.Info("pod log stream ensured", "uri", uri)
	}
	return uri, nil
}

func (m *PodLogManager) ensureStream(uri string, key podLogKey) (*logSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil || m.session.Logs == nil || m.server == nil {
		return nil, fmt.Errorf("pod log manager not bound to session")
	}
	if existing, ok := m.streams[uri]; ok {
		return existing, nil
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	lines, errCh, err := m.session.Logs.Stream(streamCtx, key.Namespace, key.Pod, logsprovider.StreamOptions{
		Options: logsprovider.Options{
			Container:    key.Container,
			TailLines:    key.TailLines,
			SinceSeconds: key.SinceSeconds,
			Previous:     key.Previous,
		},
	})
	if err != nil {
		cancel()
		return nil, err
	}

	sub := &logSubscription{
		key:    key,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.streams[uri] = sub
	server := m.server

	go m.consumeLogs(streamCtx, server, uri, sub, lines, errCh)

	return sub, nil
}

func (m *PodLogManager) consumeLogs(ctx context.Context, server *mcp.Server, uri string, sub *logSubscription, lines <-chan string, errCh <-chan error) {
	defer close(sub.done)

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if ok && err != nil {
				m.publish(server, uri, map[string]any{
					"type":  "error",
					"error": err.Error(),
				})
			}
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			sub.seq++
			m.publish(server, uri, map[string]any{
				"type":      "line",
				"sequence":  sub.seq,
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				"line":      line,
			})
		}
	}
}

func (m *PodLogManager) publish(server *mcp.Server, uri string, payload map[string]any) {
	if server == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	params := &mcp.ResourceUpdatedNotificationParams{
		URI: uri,
		Meta: mcp.Meta{
			"delta": json.RawMessage(data),
		},
	}
	_ = server.ResourceUpdated(context.Background(), params)
}

type podLogsTool struct {
	session *runtime.Session
	manager *PodLogManager
}

type podLogsInput struct {
	Namespace    string `json:"namespace" jsonschema:"Namespace of the pod"`
	Pod          string `json:"pod" jsonschema:"Pod name"`
	Container    string `json:"container,omitempty"`
	TailLines    *int   `json:"tailLines,omitempty"`
	SinceSeconds *int64 `json:"sinceSeconds,omitempty"`
	Previous     bool   `json:"previous,omitempty"`
	Follow       bool   `json:"follow,omitempty"`
}

type podLogsResult struct {
	Logs      string `json:"logs"`
	FollowURI string `json:"followUri,omitempty"`
	Following bool   `json:"following"`
}

func registerPodLogs(server *mcp.Server, session *runtime.Session, manager *PodLogManager) error {
	if session == nil || session.Logs == nil {
		return errors.New("session log provider is not configured")
	}

	if manager != nil {
		manager.Bind(server, session)
	}

	tool := &podLogsTool{session: session, manager: manager}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.podLogs.get",
		Description: "Get Kubernetes pod logs",
	}, tool.get)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "k0rdent.podLogs",
		Title:       "Kubernetes pod logs",
		Description: "Streaming pod logs for troubleshooting",
		URITemplate: podLogsURITemplate,
		MIMEType:    podLogsMIMEType,
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		key, uri, err := parsePodLogURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		logs, err := session.Logs.Get(ctx, key.Namespace, key.Pod, logsprovider.Options{
			Container:    key.Container,
			TailLines:    key.TailLines,
			SinceSeconds: key.SinceSeconds,
			Previous:     key.Previous,
		})
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: podLogsMIMEType,
				Text:     logs,
			}},
		}, nil
	})

	return nil
}

func (t *podLogsTool) get(ctx context.Context, req *mcp.CallToolRequest, input podLogsInput) (*mcp.CallToolResult, podLogsResult, error) {
	if input.Namespace == "" {
		return nil, podLogsResult{}, fmt.Errorf("namespace is required")
	}
	if input.Pod == "" {
		return nil, podLogsResult{}, fmt.Errorf("pod is required")
	}

	name := toolName(req)
	ctx = logging.WithNamespace(ctx, input.Namespace)
	ctx, logger := toolContext(ctx, t.session, name, "tool.podlogs")
	logger = logger.With(
		"tool", name,
		"pod", input.Pod,
		"container", input.Container,
		"follow", input.Follow,
		"previous", input.Previous,
		"tail_lines", derefInt(input.TailLines),
		"since_seconds", derefInt64(input.SinceSeconds),
	)
	start := time.Now()
	logger.Info("retrieving pod logs")

	opts := logsprovider.Options{
		Container: input.Container,
		Previous:  input.Previous,
	}
	if input.TailLines != nil {
		opts.TailLines = logsprovider.ToPointer(*input.TailLines)
	}
	if input.SinceSeconds != nil {
		opts.SinceSeconds = input.SinceSeconds
	}

	logs, err := t.session.Logs.Get(ctx, input.Namespace, input.Pod, opts)
	if err != nil {
		logger.Error("failed to get pod logs", "tool", name, "error", err)
		return nil, podLogsResult{}, err
	}

	result := podLogsResult{Logs: logs}
	if input.Follow {
		if t.manager == nil {
			logger.Error("follow requested but manager not available", "tool", name)
			return nil, podLogsResult{}, fmt.Errorf("follow not available")
		}
		key := podLogKey{
			Namespace:    input.Namespace,
			Pod:          input.Pod,
			Container:    input.Container,
			Previous:     input.Previous,
			TailLines:    opts.TailLines,
			SinceSeconds: opts.SinceSeconds,
		}
		followURI, err := t.manager.EnsureStream(key)
		if err != nil {
			logger.Error("failed to ensure follow stream", "tool", name, "error", err)
			return nil, podLogsResult{}, err
		}
		result.FollowURI = followURI
		result.Following = true
		logger.Info("follow stream prepared", "tool", name, "follow_uri", followURI)
	}

	logger.Info("pod logs retrieved",
		"tool", name,
		"bytes", len(result.Logs),
		"following", result.Following,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

func parsePodLogURI(raw string) (podLogKey, string, error) {
	if raw == "" {
		return podLogKey{}, "", fmt.Errorf("subscription URI is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return podLogKey{}, "", fmt.Errorf("invalid pod log URI: %w", err)
	}
	if parsed.Scheme != podLogsScheme {
		return podLogKey{}, "", fmt.Errorf("unexpected pod log scheme %q", parsed.Scheme)
	}
	if !strings.EqualFold(parsed.Host, podLogsHost) {
		return podLogKey{}, "", fmt.Errorf("unexpected pod log host %q", parsed.Host)
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return podLogKey{}, "", fmt.Errorf("pod log URI must include namespace and pod")
	}
	key := podLogKey{
		Namespace: parts[0],
		Pod:       parts[1],
	}
	if len(parts) > 2 {
		key.Container = parts[2]
	}

	query := parsed.Query()
	if prev := query.Get("previous"); prev != "" {
		parsedPrev, err := strconv.ParseBool(prev)
		if err != nil {
			return podLogKey{}, "", fmt.Errorf("invalid previous value: %w", err)
		}
		key.Previous = parsedPrev
	}
	if tail := query.Get("tailLines"); tail != "" {
		v, err := strconv.ParseInt(tail, 10, 64)
		if err != nil {
			return podLogKey{}, "", fmt.Errorf("invalid tailLines value: %w", err)
		}
		key.TailLines = &v
	}
	if since := query.Get("sinceSeconds"); since != "" {
		v, err := strconv.ParseInt(since, 10, 64)
		if err != nil {
			return podLogKey{}, "", fmt.Errorf("invalid sinceSeconds value: %w", err)
		}
		key.SinceSeconds = &v
	}

	return key, buildURIFromKey(key), nil
}

func buildURIFromKey(key podLogKey) string {
	path := fmt.Sprintf("%s://%s/%s/%s", podLogsScheme, podLogsHost, key.Namespace, key.Pod)
	if key.Container != "" {
		path = fmt.Sprintf("%s/%s", path, key.Container)
	}
	params := url.Values{}
	if key.Previous {
		params.Set("previous", "true")
	}
	if key.TailLines != nil {
		params.Set("tailLines", strconv.FormatInt(*key.TailLines, 10))
	}
	if key.SinceSeconds != nil {
		params.Set("sinceSeconds", strconv.FormatInt(*key.SinceSeconds, 10))
	}
	if encoded := params.Encode(); encoded != "" {
		path = fmt.Sprintf("%s?%s", path, encoded)
	}
	return path
}

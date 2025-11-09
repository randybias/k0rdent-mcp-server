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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	clustermonitor "github.com/k0rdent/mcp-k0rdent-server/internal/kube/cluster_monitor"
	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

const (
	clusterMonitorScheme      = "k0rdent"
	clusterMonitorHost        = "cluster-monitor"
	clusterMonitorURITemplate = "k0rdent://cluster-monitor/{namespace}/{name}"
	clusterMonitorMIMEType    = "application/json"

	defaultClusterMonitorTimeout = time.Hour
	timeoutWarningLead           = 5 * time.Minute
	maxClusterMonitorPerSession  = 10
	maxClusterMonitorGlobal      = 100
	recentEventLimit             = 50
	eventRetentionWindow         = 2 * time.Minute
)

var (
	globalClusterMonitorMu     sync.Mutex
	globalClusterMonitorActive int
)

// ClusterMonitorManager coordinates streaming subscriptions for ClusterDeployment progress.
type ClusterMonitorManager struct {
	mu            sync.Mutex
	server        *mcp.Server
	session       *runtime.Session
	subscriptions map[string]*clusterSubscription
	clock         func() time.Time
}

type clusterSubscription struct {
	namespace    string
	name         string
	uri          string
	cancel       context.CancelFunc
	done         chan struct{}
	clusterCh    <-chan clusterDelta
	clusterErr   <-chan error
	eventCh      <-chan eventsprovider.Delta
	eventErr     <-chan error
	eventFilter  *clustermonitor.EventFilter
	recentEvents []eventsprovider.Event

	currentPhase clustermonitor.ProvisioningPhase
	lastMessage  string
	lastReason   string

	timeout       time.Duration
	deadline      time.Time
	timeoutWarned bool
	logger        *slog.Logger
}

type clusterDelta struct {
	Object *unstructured.Unstructured
	Type   watch.EventType
}

type clusterMonitorTarget struct {
	Namespace string
	Name      string
	Timeout   time.Duration
}

type clusterMonitorTool struct {
	session *runtime.Session
}

type clusterMonitorStateInput struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

type clusterMonitorStateResult struct {
	Update clustermonitor.ProgressUpdate `json:"update"`
}

// NewClusterMonitorManager constructs a manager ready to bind to a session.
func NewClusterMonitorManager() *ClusterMonitorManager {
	return &ClusterMonitorManager{
		subscriptions: make(map[string]*clusterSubscription),
		clock:         time.Now,
	}
}

// Bind attaches runtime dependencies to the manager.
func (m *ClusterMonitorManager) Bind(server *mcp.Server, session *runtime.Session) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.server = server
	m.session = session
}

// Subscribe creates (or reuses) a monitoring stream for the requested cluster.
func (m *ClusterMonitorManager) Subscribe(ctx context.Context, req *mcp.SubscribeRequest) error {
	if m == nil {
		return errors.New("cluster monitor manager not configured")
	}
	target, err := parseClusterMonitorURI(req.Params.URI)
	if err != nil {
		return err
	}
	if err := m.authorizeNamespace(target.Namespace); err != nil {
		return err
	}

	ctx = logging.WithNamespace(ctx, target.Namespace)
	ctx, logger := toolContext(ctx, m.session, "k0rdent.cluster.monitor.subscribe", "tool.cluster-monitor")
	logger = logger.With("namespace", target.Namespace, "cluster", target.Name)
	logger.Info("subscribing to cluster monitor stream")

	key := subscriptionKey(target.Namespace, target.Name)

	m.mu.Lock()
	if err := m.ensureReady(); err != nil {
		m.mu.Unlock()
		logger.Error("cluster monitor manager not bound", "error", err)
		return err
	}
	if len(m.subscriptions) >= maxClusterMonitorPerSession {
		m.mu.Unlock()
		return fmt.Errorf("per-client subscription limit exceeded (max: %d)", maxClusterMonitorPerSession)
	}
	if _, exists := m.subscriptions[key]; exists {
		m.mu.Unlock()
		logger.Debug("cluster monitor subscription already active")
		return nil
	}
	m.mu.Unlock()

	if !acquireClusterMonitorSlot() {
		return fmt.Errorf("server subscription limit exceeded (max: %d)", maxClusterMonitorGlobal)
	}

	sub, err := m.newSubscription(ctx, req.Params.URI, target, logger)
	if err != nil {
		releaseClusterMonitorSlot()
		return err
	}

	m.mu.Lock()
	m.subscriptions[key] = sub
	m.mu.Unlock()

	go m.runSubscription(sub)
	logger.Info("cluster monitor subscription started")
	return nil
}

// Unsubscribe terminates the monitoring stream for the provided URI.
func (m *ClusterMonitorManager) Unsubscribe(ctx context.Context, req *mcp.UnsubscribeRequest) error {
	if m == nil {
		return errors.New("cluster monitor manager not configured")
	}
	target, err := parseClusterMonitorURI(req.Params.URI)
	if err != nil {
		return err
	}

	ctx = logging.WithNamespace(ctx, target.Namespace)
	ctx, logger := toolContext(ctx, m.session, "k0rdent.cluster.monitor.unsubscribe", "tool.cluster-monitor")
	logger = logger.With("namespace", target.Namespace, "cluster", target.Name)
	logger.Info("unsubscribing from cluster monitor stream")

	key := subscriptionKey(target.Namespace, target.Name)

	m.mu.Lock()
	sub, ok := m.subscriptions[key]
	if ok {
		delete(m.subscriptions, key)
	}
	m.mu.Unlock()
	if !ok || sub == nil {
		return nil
	}

	sub.cancel()
	<-sub.done
	logger.Info("cluster monitor subscription terminated")
	return nil
}

func (m *ClusterMonitorManager) newSubscription(ctx context.Context, uri string, target clusterMonitorTarget, logger *slog.Logger) (*clusterSubscription, error) {
	session := m.session
	client := session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).Namespace(target.Namespace)
	obj, err := client.Get(ctx, target.Name, v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("clusterdeployment %s/%s not found", target.Namespace, target.Name)
		}
		return nil, fmt.Errorf("get clusterdeployment: %w", err)
	}

	timeout := target.Timeout
	if timeout <= 0 {
		timeout = defaultClusterMonitorTimeout
	}

	watchCtx, cancel := context.WithCancel(context.Background())
	clusterCh, clusterErr, err := watchClusterDeployment(watchCtx, session.Clients.Dynamic, target.Namespace, target.Name)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watch clusterdeployment: %w", err)
	}
	eventCh, eventErr, err := session.Events.WatchNamespace(watchCtx, target.Namespace, eventsprovider.WatchOptions{})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watch namespace events: %w", err)
	}

	sub := &clusterSubscription{
		namespace:    target.Namespace,
		name:         target.Name,
		uri:          uri,
		cancel:       cancel,
		done:         make(chan struct{}),
		clusterCh:    clusterCh,
		clusterErr:   clusterErr,
		eventCh:      eventCh,
		eventErr:     eventErr,
		eventFilter:  clustermonitor.NewEventFilter(target.Name, target.Namespace),
		recentEvents: make([]eventsprovider.Event, 0, 16),
		currentPhase: clustermonitor.PhaseUnknown,
		timeout:      timeout,
		deadline:     m.clock().Add(timeout),
		logger:       logger,
	}
	sub.eventFilter.WithClock(m.clock)

	// Emit initial snapshot immediately.
	m.processClusterDelta(sub, clusterDelta{Object: obj.DeepCopy(), Type: watch.Added})
	m.publishRecentEventsSnapshot(ctx, sub)
	return sub, nil
}

func (m *ClusterMonitorManager) runSubscription(sub *clusterSubscription) {
	defer func() {
		releaseClusterMonitorSlot()
		m.forgetSubscription(sub)
		close(sub.done)
	}()
	defer sub.cancel()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case delta, ok := <-sub.clusterCh:
			if !ok {
				m.publishSystemMessage(sub, clustermonitor.SeverityWarning, "Cluster watch closed", true)
				return
			}
			if m.processClusterDelta(sub, delta) {
				return
			}
		case err, ok := <-sub.clusterErr:
			if ok && err != nil {
				m.publishSystemMessage(sub, clustermonitor.SeverityError, fmt.Sprintf("Cluster watch error: %v", err), true)
			}
			return
		case delta, ok := <-sub.eventCh:
			if !ok {
				sub.eventCh = nil
				continue
			}
			m.handleEventDelta(sub, delta.Event)
		case err, ok := <-sub.eventErr:
			if ok && err != nil {
				m.publishSystemMessage(sub, clustermonitor.SeverityWarning, fmt.Sprintf("Event watch error: %v", err), false)
			}
			sub.eventErr = nil
		case <-ticker.C:
			if m.checkTimeout(sub) {
				return
			}
		}
	}
}

func (m *ClusterMonitorManager) processClusterDelta(sub *clusterSubscription, delta clusterDelta) bool {
	if delta.Object == nil {
		return false
	}
	update := buildClusterProgress(delta.Object, sub.recentEvents)
	update.Timestamp = m.clock().UTC()

	if delta.Type == watch.Deleted {
		update.Terminal = true
		update.Severity = clustermonitor.SeverityWarning
		update.Message = fmt.Sprintf("Cluster %s deleted", sub.name)
		update.Source = clustermonitor.SourceSystem
	}

	phaseChanged := update.Phase != clustermonitor.PhaseUnknown && update.Phase != sub.currentPhase
	if phaseChanged {
		sub.currentPhase = update.Phase
	}

	if update.Message == "" {
		update.Message = fmt.Sprintf("Cluster phase: %s", update.Phase)
	}

	if m.shouldPublishClusterUpdate(sub, update, phaseChanged) {
		m.publishUpdate(sub.uri, update)
		sub.lastMessage = update.Message
		sub.lastReason = update.Reason
	}

	return update.Terminal
}

func (m *ClusterMonitorManager) shouldPublishClusterUpdate(sub *clusterSubscription, update clustermonitor.ProgressUpdate, phaseChanged bool) bool {
	if sub.lastMessage == "" && sub.lastReason == "" {
		return true
	}
	if update.Terminal {
		return true
	}
	if phaseChanged {
		return true
	}
	if update.Message != "" && update.Message != sub.lastMessage {
		return true
	}
	if update.Reason != "" && update.Reason != sub.lastReason {
		return true
	}
	return false
}

func (m *ClusterMonitorManager) handleEventDelta(sub *clusterSubscription, event eventsprovider.Event) {
	now := m.clock()
	if sub.eventFilter.InScope(event) {
		sub.appendEvent(event, now)
		if result, ok := sub.eventFilter.Evaluate(event); ok {
			update := result.Update
			if update.Timestamp.IsZero() {
				update.Timestamp = now.UTC()
			}
			m.publishUpdate(sub.uri, update)
			if update.Phase != clustermonitor.PhaseUnknown && update.Phase != sub.currentPhase {
				sub.currentPhase = update.Phase
			}
			if update.Terminal {
				// Allow run loop to exit once cluster watch observes terminal phase
				sub.cancel()
			}
			return
		}
	} else {
		sub.appendEvent(event, now)
	}
}

func (s *clusterSubscription) appendEvent(event eventsprovider.Event, now time.Time) {
	s.recentEvents = append(s.recentEvents, event)
	cutoff := now.Add(-eventRetentionWindow)
	filtered := s.recentEvents[:0]
	for _, evt := range s.recentEvents {
		ts := monitorEventTimestamp(evt)
		if ts.IsZero() || ts.After(cutoff) {
			filtered = append(filtered, evt)
		}
	}
	if len(filtered) > recentEventLimit {
		filtered = filtered[len(filtered)-recentEventLimit:]
	}
	clone := make([]eventsprovider.Event, len(filtered))
	copy(clone, filtered)
	s.recentEvents = clone
}

func (m *ClusterMonitorManager) publishSystemMessage(sub *clusterSubscription, severity clustermonitor.SeverityLevel, message string, terminal bool) {
	update := clustermonitor.ProgressUpdate{
		Timestamp: m.clock().UTC(),
		Phase:     sub.currentPhase,
		Message:   message,
		Source:    clustermonitor.SourceSystem,
		Severity:  severity,
		Terminal:  terminal,
	}
	m.publishUpdate(sub.uri, update)
}

func (m *ClusterMonitorManager) publishRecentEventsSnapshot(ctx context.Context, sub *clusterSubscription) {
	if m.session == nil || m.session.Events == nil {
		return
	}
	events, err := m.session.Events.List(ctx, sub.namespace, eventsprovider.ListOptions{})
	if err != nil {
		if sub.logger != nil {
			sub.logger.Warn("failed to list namespace events for snapshot", "error", err)
		}
		return
	}
	if len(events) == 0 {
		return
	}
	cutoff := m.clock().Add(-eventRetentionWindow)
	selected := make([]eventsprovider.Event, 0, 5)
	for i := len(events) - 1; i >= 0; i-- {
		evt := events[i]
		if !sub.eventFilter.InScope(evt) {
			continue
		}
		ts := monitorEventTimestamp(evt)
		if !ts.IsZero() && ts.Before(cutoff) {
			continue
		}
		selected = append(selected, evt)
		if len(selected) >= 5 {
			break
		}
	}
	if len(selected) == 0 {
		return
	}
	now := m.clock()
	for i := len(selected) - 1; i >= 0; i-- {
		evt := selected[i]
		sub.appendEvent(evt, now)
		if result, ok := sub.eventFilter.Evaluate(evt); ok {
			update := result.Update
			if update.Timestamp.IsZero() {
				update.Timestamp = now.UTC()
			}
			m.publishUpdate(sub.uri, update)
			if update.Phase != clustermonitor.PhaseUnknown && update.Phase != sub.currentPhase {
				sub.currentPhase = update.Phase
			}
		}
	}
}

func (m *ClusterMonitorManager) checkTimeout(sub *clusterSubscription) bool {
	if sub.timeout <= 0 {
		return false
	}
	now := m.clock()
	remaining := sub.deadline.Sub(now)
	if !sub.timeoutWarned && remaining > 0 && remaining <= timeoutWarningLead {
		message := fmt.Sprintf("Provisioning timeout approaching (%s remaining)", remaining.Truncate(time.Minute))
		m.publishSystemMessage(sub, clustermonitor.SeverityWarning, message, false)
		sub.timeoutWarned = true
	}
	if remaining <= 0 {
		m.publishSystemMessage(sub, clustermonitor.SeverityWarning, "Monitoring timeout exceeded, subscription terminated", true)
		return true
	}
	return false
}

func (m *ClusterMonitorManager) publishUpdate(uri string, update clustermonitor.ProgressUpdate) {
	if m == nil || m.server == nil {
		return
	}
	payload, err := json.Marshal(update)
	if err != nil {
		return
	}
	params := &mcp.ResourceUpdatedNotificationParams{
		URI: uri,
		Meta: mcp.Meta{
			"delta": json.RawMessage(payload),
		},
	}
	_ = m.server.ResourceUpdated(context.Background(), params)
}

func (m *ClusterMonitorManager) authorizeNamespace(namespace string) error {
	if namespace == "" {
		return errors.New("namespace is required")
	}
	if m.session == nil {
		return errors.New("session not bound")
	}
	if m.session.IsDevMode() || m.session.NamespaceFilter == nil {
		return nil
	}
	if !m.session.NamespaceFilter.MatchString(namespace) {
		return fmt.Errorf("namespace %q not allowed by session filter", namespace)
	}
	return nil
}

func (m *ClusterMonitorManager) ensureReady() error {
	switch {
	case m.session == nil:
		return errors.New("session not bound")
	case m.session.Clients.Dynamic == nil:
		return errors.New("dynamic client not configured")
	case m.session.Events == nil:
		return errors.New("events provider not configured")
	case m.server == nil:
		return errors.New("server not bound")
	default:
		return nil
	}
}

func (m *ClusterMonitorManager) forgetSubscription(sub *clusterSubscription) {
	if sub == nil {
		return
	}
	key := subscriptionKey(sub.namespace, sub.name)
	m.mu.Lock()
	if existing, ok := m.subscriptions[key]; ok && existing == sub {
		delete(m.subscriptions, key)
	}
	m.mu.Unlock()
}

func acquireClusterMonitorSlot() bool {
	globalClusterMonitorMu.Lock()
	defer globalClusterMonitorMu.Unlock()
	if globalClusterMonitorActive >= maxClusterMonitorGlobal {
		return false
	}
	globalClusterMonitorActive++
	return true
}

func releaseClusterMonitorSlot() {
	globalClusterMonitorMu.Lock()
	defer globalClusterMonitorMu.Unlock()
	if globalClusterMonitorActive > 0 {
		globalClusterMonitorActive--
	}
}

func parseClusterMonitorURI(raw string) (clusterMonitorTarget, error) {
	var target clusterMonitorTarget
	parsed, err := url.Parse(raw)
	if err != nil {
		return target, fmt.Errorf("invalid cluster monitor URI: %w", err)
	}
	if parsed.Scheme != clusterMonitorScheme {
		return target, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if !strings.EqualFold(parsed.Host, clusterMonitorHost) {
		return target, fmt.Errorf("unsupported host %q", parsed.Host)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return target, errors.New("cluster monitor URI must be in the form k0rdent://cluster-monitor/{namespace}/{name}")
	}
	target.Namespace = parts[0]
	target.Name = parts[1]
	target.Timeout = defaultClusterMonitorTimeout

	if timeoutStr := parsed.Query().Get("timeout"); timeoutStr != "" {
		seconds, err := strconv.Atoi(timeoutStr)
		if err != nil || seconds <= 0 {
			return target, fmt.Errorf("invalid timeout %q", timeoutStr)
		}
		target.Timeout = time.Duration(seconds) * time.Second
	}
	return target, nil
}

func subscriptionKey(namespace, name string) string {
	return namespace + "/" + name
}

func watchClusterDeployment(ctx context.Context, client dynamic.Interface, namespace, name string) (<-chan clusterDelta, <-chan error, error) {
	if client == nil {
		return nil, nil, errors.New("dynamic client is nil")
	}
	watcher, err := client.Resource(clusters.ClusterDeploymentsGVR).Namespace(namespace).Watch(ctx, v1.ListOptions{
		FieldSelector:       fields.OneTermEqualSelector("metadata.name", name).String(),
		AllowWatchBookmarks: true,
	})
	if err != nil {
		return nil, nil, err
	}

	out := make(chan clusterDelta)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					errCh <- errors.New("cluster watch channel closed")
					return
				}
				if event.Type == watch.Bookmark {
					continue
				}
				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- clusterDelta{Object: obj.DeepCopy(), Type: event.Type}:
				}
				if event.Type == watch.Deleted {
					return
				}
			}
		}
	}()

	return out, errCh, nil
}

func (t *clusterMonitorTool) state(ctx context.Context, req *mcp.CallToolRequest, input clusterMonitorStateInput) (*mcp.CallToolResult, clusterMonitorStateResult, error) {
	if t == nil || t.session == nil {
		return nil, clusterMonitorStateResult{}, fmt.Errorf("cluster monitor tool not configured")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, clusterMonitorStateResult{}, fmt.Errorf("cluster name is required")
	}
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		namespace = t.session.GlobalNamespace()
	}
	if namespace == "" {
		return nil, clusterMonitorStateResult{}, fmt.Errorf("namespace is required")
	}
	if !t.session.IsDevMode() && t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(namespace) {
		return nil, clusterMonitorStateResult{}, fmt.Errorf("namespace %q not allowed by filter", namespace)
	}
	if t.session.Clients.Dynamic == nil {
		return nil, clusterMonitorStateResult{}, fmt.Errorf("dynamic client not configured")
	}

	ctx = logging.WithNamespace(ctx, namespace)
	toolID := toolName(req)
	ctx, logger := toolContext(ctx, t.session, toolID, "tool.cluster-monitor")
	logger = logger.With("namespace", namespace, "cluster", name)
	logger.Info("fetching cluster monitor state")

	obj, err := t.session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).
		Namespace(namespace).
		Get(ctx, name, v1.GetOptions{})
	if err != nil {
		logger.Error("failed to fetch cluster deployment", "error", err)
		return nil, clusterMonitorStateResult{}, err
	}

	update := buildClusterProgress(obj, nil)
	update.Timestamp = time.Now().UTC()

	logger.Info("cluster monitor state fetched",
		"phase", update.Phase,
		"terminal", update.Terminal,
	)

	return nil, clusterMonitorStateResult{Update: update}, nil
}

func buildClusterProgress(obj *unstructured.Unstructured, events []eventsprovider.Event) clustermonitor.ProgressUpdate {
	summary := clusters.SummarizeClusterDeployment(obj)
	phase := clustermonitor.DetectPhase(obj, events)
	progress := clustermonitor.EstimateProgress(phase, summary.Conditions)
	message := summary.Message
	if message == "" {
		message = fmt.Sprintf("Cluster phase: %s", phase)
	}
	severity := clustermonitor.SeverityInfo
	if phase == clustermonitor.PhaseFailed {
		severity = clustermonitor.SeverityError
	}
	return clustermonitor.ProgressUpdate{
		Phase:      phase,
		Progress:   progress,
		Message:    message,
		Reason:     summary.Phase,
		Source:     clustermonitor.SourceCondition,
		Severity:   severity,
		Conditions: summary.Conditions,
		Terminal:   phase.IsTerminal(),
	}
}

func monitorEventTimestamp(event eventsprovider.Event) time.Time {
	switch {
	case event.EventTime != nil && !event.EventTime.IsZero():
		return *event.EventTime
	case event.LastTimestamp != nil && !event.LastTimestamp.IsZero():
		return *event.LastTimestamp
	case event.FirstTimestamp != nil && !event.FirstTimestamp.IsZero():
		return *event.FirstTimestamp
	default:
		return time.Time{}
	}
}

func registerClusterMonitor(server *mcp.Server, session *runtime.Session, manager *ClusterMonitorManager) error {
	if session == nil {
		return errors.New("session is required")
	}
	if session.Clients.Dynamic == nil {
		return errors.New("dynamic client is not configured")
	}

	if manager != nil {
		manager.Bind(server, session)
	}

	tool := &clusterMonitorTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.clusterDeployments.getState",
		Description: "Fetch the latest ClusterDeployment monitoring state",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "clusterDeployments",
			"action":   "get",
		},
	}, tool.state)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "k0rdent.cluster.monitor",
		Title:       "Cluster deployment monitoring",
		Description: "Streaming progress updates for ClusterDeployment resources",
		URITemplate: clusterMonitorURITemplate,
		MIMEType:    clusterMonitorMIMEType,
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		target, err := parseClusterMonitorURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		obj, err := session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).
			Namespace(target.Namespace).
			Get(ctx, target.Name, v1.GetOptions{})
		if err != nil {
			return nil, err
		}
		update := buildClusterProgress(obj, nil)
		update.Timestamp = time.Now().UTC()
		payload, err := json.Marshal(update)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: clusterMonitorMIMEType,
				Blob:     payload,
			}},
		}, nil
	})

	return nil
}

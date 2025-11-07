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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/k0rdent/mcp-k0rdent-server/internal/k0rdent/api"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

const (
	graphScheme      = "k0rdent"
	graphHost        = "graph"
	graphURITemplate = "k0rdent://graph"
	graphMIMEType    = "application/json"
)

type GraphNode struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Summary   map[string]any    `json:"summary,omitempty"`
}

type GraphEdge struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

type GraphSnapshot struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type graphDelta struct {
	Action string      `json:"action"`
	Nodes  []GraphNode `json:"nodes,omitempty"`
	Edges  []GraphEdge `json:"edges,omitempty"`
}

type graphFilter struct {
	namespaces map[string]struct{}
	kinds      map[string]struct{}
}

func (f graphFilter) allowsNode(node GraphNode) bool {
	if len(f.namespaces) > 0 {
		if _, ok := f.namespaces[node.Namespace]; !ok {
			return false
		}
	}
	if len(f.kinds) > 0 {
		if _, ok := f.kinds[node.Type]; !ok {
			return false
		}
	}
	return true
}

type graphSubscription struct {
	uri    string
	filter graphFilter
}

type graphState struct {
	serviceTemplates     map[string]api.ServiceTemplateSummary
	clusterDeployments   map[string]api.ClusterDeploymentSummary
	multiClusterServices map[string]api.MultiClusterServiceSummary
}

type GraphManager struct {
	mu          sync.Mutex
	server      *mcp.Server
	session     *runtime.Session
	subscribers map[string]graphSubscription
	cancelWatch context.CancelFunc
	state       graphState
	watchersRun bool
}

func NewGraphManager() *GraphManager {
	return &GraphManager{
		subscribers: make(map[string]graphSubscription),
		state: graphState{
			serviceTemplates:     make(map[string]api.ServiceTemplateSummary),
			clusterDeployments:   make(map[string]api.ClusterDeploymentSummary),
			multiClusterServices: make(map[string]api.MultiClusterServiceSummary),
		},
	}
}

func (m *GraphManager) Bind(server *mcp.Server, session *runtime.Session) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.server = server
	m.session = session
}

func (m *GraphManager) Subscribe(ctx context.Context, req *mcp.SubscribeRequest) error {
	if m == nil {
		return fmt.Errorf("graph manager not configured")
	}
	filter, err := parseGraphURI(req.Params.URI)
	if err != nil {
		return err
	}

	ctx, logger := toolContext(ctx, m.session, "k0rdent.graph.subscribe", "tool.graph")
	logger = logger.With(
		"uri", req.Params.URI,
		"namespaces", mapKeys(filter.namespaces),
		"kinds", mapKeys(filter.kinds),
	)
	logger.Info("subscribing to graph stream")

	m.mu.Lock()
	if m.session == nil || m.session.Clients.Dynamic == nil || m.server == nil {
		m.mu.Unlock()
		logger.Error("graph manager not bound to session")
		return fmt.Errorf("graph manager not bound to session")
	}
	key := fmt.Sprintf("%s:%s", req.Session.ID(), req.Params.URI)
	m.subscribers[key] = graphSubscription{uri: req.Params.URI, filter: filter}
	if !m.watchersRun {
		if err := m.startWatchersLocked(); err != nil {
			delete(m.subscribers, key)
			m.mu.Unlock()
			logger.Error("failed to start graph watchers", "error", err)
			return err
		}
		m.watchersRun = true
	}
	m.mu.Unlock()

	snapshot, err := m.Snapshot(ctx, filter)
	if err != nil {
		logger.Error("failed to generate initial graph snapshot", "error", err)
		return err
	}
	m.sendDelta(graphDelta{Action: "snapshot", Nodes: snapshot.Nodes, Edges: snapshot.Edges}, filter, req.Params.URI)
	logger.Info("graph subscription active", "nodes", len(snapshot.Nodes), "edges", len(snapshot.Edges))
	return nil
}

func (m *GraphManager) Unsubscribe(ctx context.Context, req *mcp.UnsubscribeRequest) error {
	if m == nil {
		return fmt.Errorf("graph manager not configured")
	}
	ctx, logger := toolContext(ctx, m.session, "k0rdent.graph.unsubscribe", "tool.graph")
	logger = logger.With("uri", req.Params.URI)
	logger.Info("unsubscribing from graph stream")

	m.mu.Lock()
	key := fmt.Sprintf("%s:%s", req.Session.ID(), req.Params.URI)
	delete(m.subscribers, key)
	stopWatchers := m.watchersRun && len(m.subscribers) == 0
	cancel := m.cancelWatch
	if stopWatchers {
		m.watchersRun = false
		m.cancelWatch = nil
	}
	m.mu.Unlock()

	if stopWatchers && cancel != nil {
		cancel()
	}
	if stopWatchers {
		logger.Info("graph watchers stopped")
	} else {
		logger.Info("graph subscription canceled")
	}
	return nil
}

func (m *GraphManager) Snapshot(ctx context.Context, filter graphFilter) (GraphSnapshot, error) {
	if m.session == nil || m.session.Clients.Dynamic == nil {
		return GraphSnapshot{}, errors.New("graph manager not bound")
	}
	logger := logging.WithComponent(logging.WithContext(ctx, m.session.Logger), "tool.graph")
	if logger != nil {
		logger.Info("building graph snapshot",
			"namespaces", mapKeys(filter.namespaces),
			"kinds", mapKeys(filter.kinds),
		)
	}
	st, err := api.ListServiceTemplates(ctx, m.session.Clients.Dynamic)
	if err != nil {
		if logger != nil {
			logger.Error("list service templates failed", "error", err)
		}
		return GraphSnapshot{}, err
	}
	cds, err := api.ListClusterDeployments(ctx, m.session.Clients.Dynamic, "")
	if err != nil {
		if logger != nil {
			logger.Error("list cluster deployments failed", "error", err)
		}
		return GraphSnapshot{}, err
	}
	mcs, err := api.ListMultiClusterServices(ctx, m.session.Clients.Dynamic, "")
	if err != nil {
		if logger != nil {
			logger.Error("list multi-cluster services failed", "error", err)
		}
		return GraphSnapshot{}, err
	}

	stMap := make(map[string]api.ServiceTemplateSummary, len(st))
	for _, item := range st {
		stMap[keyFor(item.Namespace, item.Name)] = item
	}
	cdMap := make(map[string]api.ClusterDeploymentSummary, len(cds))
	for _, item := range cds {
		cdMap[keyFor(item.Namespace, item.Name)] = item
	}
	mcsMap := make(map[string]api.MultiClusterServiceSummary, len(mcs))
	for _, item := range mcs {
		mcsMap[keyFor(item.Namespace, item.Name)] = item
	}

	nodes := make([]GraphNode, 0, len(st)+len(cds)+len(mcs))
	allowedNodes := make(map[string]struct{})

	for _, item := range st {
		node := serviceTemplateNode(item)
		if filter.allowsNode(node) {
			nodes = append(nodes, node)
			allowedNodes[node.ID] = struct{}{}
		}
	}
	for _, item := range cds {
		node := clusterDeploymentNode(item)
		if filter.allowsNode(node) {
			nodes = append(nodes, node)
			allowedNodes[node.ID] = struct{}{}
		}
	}
	for _, item := range mcs {
		node := multiClusterServiceNode(item)
		if filter.allowsNode(node) {
			nodes = append(nodes, node)
			allowedNodes[node.ID] = struct{}{}
		}
	}

	edges := buildGraphEdges(stMap, cdMap, mcsMap, filter, allowedNodes)
	return GraphSnapshot{Nodes: nodes, Edges: edges}, nil
}

func (m *GraphManager) startWatchersLocked() error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelWatch = cancel

	go m.watchResource(ctx, api.ServiceTemplateGVR(), m.handleServiceTemplateEvent)
	go m.watchResource(ctx, api.ClusterDeploymentGVR(), m.handleClusterDeploymentEvent)
	go m.watchResource(ctx, api.MultiClusterServiceGVR(), m.handleMultiClusterServiceEvent)
	return nil
}

func (m *GraphManager) watchResource(ctx context.Context, gvr schema.GroupVersionResource, handler func(watch.Event)) {
	for {
		if ctx.Err() != nil {
			return
		}
		watcher, err := m.session.Clients.Dynamic.Resource(gvr).Namespace(metav1.NamespaceAll).Watch(ctx, metav1.ListOptions{AllowWatchBookmarks: true})
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		for event := range watcher.ResultChan() {
			if event.Type == watch.Bookmark {
				continue
			}
			handler(event)
		}
		watcher.Stop()
		time.Sleep(time.Second)
	}
}

func (m *GraphManager) handleServiceTemplateEvent(event watch.Event) {
	obj := toUnstructured(event.Object)
	key := keyFor(obj.GetNamespace(), obj.GetName())

	var delta graphDelta
	delta.Action = strings.ToLower(string(event.Type))

	m.mu.Lock()
	switch event.Type {
	case watch.Deleted:
		summary, ok := m.state.serviceTemplates[key]
		if ok {
			delete(m.state.serviceTemplates, key)
			delta.Nodes = []GraphNode{serviceTemplateNode(summary)}
		}
	default:
		summary := api.SummarizeServiceTemplate(obj)
		m.state.serviceTemplates[key] = summary
		delta.Nodes = []GraphNode{serviceTemplateNode(summary)}
	}
	m.mu.Unlock()

	if len(delta.Nodes) > 0 {
		m.broadcastDelta(delta)
	}
}

func (m *GraphManager) handleClusterDeploymentEvent(event watch.Event) {
	obj := toUnstructured(event.Object)
	key := keyFor(obj.GetNamespace(), obj.GetName())

	var delta graphDelta
	delta.Action = strings.ToLower(string(event.Type))

	m.mu.Lock()
	switch event.Type {
	case watch.Deleted:
		summary, ok := m.state.clusterDeployments[key]
		if ok {
			delete(m.state.clusterDeployments, key)
			delta.Nodes = []GraphNode{clusterDeploymentNode(summary)}
			delta.Edges = append(delta.Edges, clusterDeploymentEdges(summary, m.state.serviceTemplates)...)
			delta.Edges = append(delta.Edges, edgesForCluster(summary, m.state.multiClusterServices)...)
		}
	default:
		summary := api.SummarizeClusterDeployment(obj)
		m.state.clusterDeployments[key] = summary
		delta.Nodes = []GraphNode{clusterDeploymentNode(summary)}
		delta.Edges = append(delta.Edges, clusterDeploymentEdges(summary, m.state.serviceTemplates)...)
		delta.Edges = append(delta.Edges, edgesForCluster(summary, m.state.multiClusterServices)...)
	}
	m.mu.Unlock()

	if len(delta.Nodes) > 0 || len(delta.Edges) > 0 {
		m.broadcastDelta(dedupeDelta(delta))
	}
}

func (m *GraphManager) handleMultiClusterServiceEvent(event watch.Event) {
	obj := toUnstructured(event.Object)
	key := keyFor(obj.GetNamespace(), obj.GetName())

	var delta graphDelta
	delta.Action = strings.ToLower(string(event.Type))

	m.mu.Lock()
	switch event.Type {
	case watch.Deleted:
		summary, ok := m.state.multiClusterServices[key]
		if ok {
			delete(m.state.multiClusterServices, key)
			delta.Nodes = []GraphNode{multiClusterServiceNode(summary)}
			delta.Edges = multiClusterServiceEdges(summary, m.state.clusterDeployments)
		}
	default:
		summary := api.SummarizeMultiClusterService(obj)
		m.state.multiClusterServices[key] = summary
		delta.Nodes = []GraphNode{multiClusterServiceNode(summary)}
		delta.Edges = multiClusterServiceEdges(summary, m.state.clusterDeployments)
	}
	m.mu.Unlock()

	if len(delta.Nodes) > 0 || len(delta.Edges) > 0 {
		m.broadcastDelta(dedupeDelta(delta))
	}
}

func (m *GraphManager) broadcastDelta(delta graphDelta) {
	m.mu.Lock()
	subs := make([]graphSubscription, 0, len(m.subscribers))
	for _, sub := range m.subscribers {
		subs = append(subs, sub)
	}
	m.mu.Unlock()

	if len(subs) == 0 {
		return
	}
	for _, sub := range subs {
		filtered, ok := filterDelta(delta, sub.filter)
		if !ok {
			continue
		}
		m.sendDelta(filtered, sub.filter, sub.uri)
	}
}

func (m *GraphManager) sendDelta(delta graphDelta, filter graphFilter, uri string) {
	payload := map[string]any{
		"action": delta.Action,
		"nodes":  delta.Nodes,
		"edges":  delta.Edges,
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
	_ = m.server.ResourceUpdated(context.Background(), params)
}

func dedupeDelta(delta graphDelta) graphDelta {
	edgeMap := make(map[string]GraphEdge, len(delta.Edges))
	for _, edge := range delta.Edges {
		edgeMap[edge.ID] = edge
	}
	deduped := make([]GraphEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		deduped = append(deduped, edge)
	}
	delta.Edges = deduped
	return delta
}

func buildGraphEdges(st map[string]api.ServiceTemplateSummary, cds map[string]api.ClusterDeploymentSummary, mcs map[string]api.MultiClusterServiceSummary, filter graphFilter, allowed map[string]struct{}) []GraphEdge {
	edges := make([]GraphEdge, 0)
	for _, cd := range cds {
		edges = append(edges, clusterDeploymentEdges(cd, st)...)
	}
	for _, svc := range mcs {
		edges = append(edges, multiClusterServiceEdges(svc, cds)...)
	}

	filtered := make([]GraphEdge, 0, len(edges))
	for _, edge := range edges {
		if edgeMatchesFilter(edge, filter, allowed) {
			filtered = append(filtered, edge)
		}
	}
	return dedupeEdges(filtered)
}

func clusterDeploymentEdges(cd api.ClusterDeploymentSummary, st map[string]api.ServiceTemplateSummary) []GraphEdge {
	edges := make([]GraphEdge, 0, len(cd.ServiceTemplates))
	from := fmt.Sprintf("ClusterDeployment:%s/%s", cd.Namespace, cd.Name)
	for _, tmpl := range cd.ServiceTemplates {
		key := keyFor(cd.Namespace, tmpl)
		if _, ok := st[key]; !ok {
			continue
		}
		edgeID := fmt.Sprintf("%s->ServiceTemplate:%s/%s", from, cd.Namespace, tmpl)
		edges = append(edges, GraphEdge{
			ID:   edgeID,
			From: from,
			To:   fmt.Sprintf("ServiceTemplate:%s/%s", cd.Namespace, tmpl),
			Type: "uses-template",
		})
	}
	return edges
}

func multiClusterServiceEdges(svc api.MultiClusterServiceSummary, cds map[string]api.ClusterDeploymentSummary) []GraphEdge {
	edges := make([]GraphEdge, 0)
	from := fmt.Sprintf("MultiClusterService:%s/%s", svc.Namespace, svc.Name)
	for _, cd := range cds {
		if len(svc.MatchLabels) > 0 {
			match := true
			for k, v := range svc.MatchLabels {
				if cd.Labels[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		edgeID := fmt.Sprintf("%s->ClusterDeployment:%s/%s", from, cd.Namespace, cd.Name)
		edges = append(edges, GraphEdge{
			ID:   edgeID,
			From: from,
			To:   fmt.Sprintf("ClusterDeployment:%s/%s", cd.Namespace, cd.Name),
			Type: "targets-cluster",
		})
	}
	return edges
}

func edgesForCluster(cd api.ClusterDeploymentSummary, services map[string]api.MultiClusterServiceSummary) []GraphEdge {
	cds := map[string]api.ClusterDeploymentSummary{keyFor(cd.Namespace, cd.Name): cd}
	edges := make([]GraphEdge, 0)
	for _, svc := range services {
		edges = append(edges, multiClusterServiceEdges(svc, cds)...)
	}
	return edges
}

func dedupeEdges(edges []GraphEdge) []GraphEdge {
	if len(edges) == 0 {
		return edges
	}
	uniq := make(map[string]GraphEdge, len(edges))
	for _, edge := range edges {
		uniq[edge.ID] = edge
	}
	out := make([]GraphEdge, 0, len(uniq))
	for _, edge := range uniq {
		out = append(out, edge)
	}
	return out
}

func edgeMatchesFilter(edge GraphEdge, filter graphFilter, allowed map[string]struct{}) bool {
	if len(filter.namespaces) == 0 && len(filter.kinds) == 0 {
		return true
	}
	if !nodeMatches(edge.From, filter, allowed) {
		return false
	}
	if !nodeMatches(edge.To, filter, allowed) {
		return false
	}
	return true
}

func nodeMatches(id string, filter graphFilter, allowed map[string]struct{}) bool {
	if len(filter.namespaces) == 0 && len(filter.kinds) == 0 {
		return true
	}
	if _, ok := allowed[id]; ok {
		return true
	}
	kind, namespace := parseNodeID(id)
	if len(filter.kinds) > 0 {
		if _, ok := filter.kinds[kind]; !ok {
			return false
		}
	}
	if len(filter.namespaces) > 0 {
		if _, ok := filter.namespaces[namespace]; !ok {
			return false
		}
	}
	return true
}

func filterDelta(delta graphDelta, filter graphFilter) (graphDelta, bool) {
	if len(filter.namespaces) == 0 && len(filter.kinds) == 0 {
		return delta, len(delta.Nodes) > 0 || len(delta.Edges) > 0
	}
	allowed := make(map[string]struct{})
	filteredNodes := make([]GraphNode, 0, len(delta.Nodes))
	for _, node := range delta.Nodes {
		if filter.allowsNode(node) {
			filteredNodes = append(filteredNodes, node)
			allowed[node.ID] = struct{}{}
		}
	}
	filteredEdges := make([]GraphEdge, 0, len(delta.Edges))
	for _, edge := range delta.Edges {
		if edgeMatchesFilter(edge, filter, allowed) {
			filteredEdges = append(filteredEdges, edge)
		}
	}
	if len(filteredNodes) == 0 && len(filteredEdges) == 0 {
		return graphDelta{}, false
	}
	return graphDelta{Action: delta.Action, Nodes: filteredNodes, Edges: filteredEdges}, true
}

func parseGraphURI(raw string) (graphFilter, error) {
	filter := graphFilter{}
	if raw == "" {
		return filter, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return filter, fmt.Errorf("invalid graph URI: %w", err)
	}
	if parsed.Scheme != graphScheme {
		return filter, fmt.Errorf("unexpected graph scheme %q", parsed.Scheme)
	}
	if !strings.EqualFold(parsed.Host, graphHost) {
		return filter, fmt.Errorf("unexpected graph host %q", parsed.Host)
	}
	values := parsed.Query()
	if namespaces := values.Get("namespace"); namespaces != "" {
		filter.namespaces = make(map[string]struct{})
		for _, ns := range strings.Split(namespaces, ",") {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				filter.namespaces[ns] = struct{}{}
			}
		}
	}
	if kinds := values.Get("kinds"); kinds != "" {
		filter.kinds = make(map[string]struct{})
		for _, kind := range strings.Split(kinds, ",") {
			kind = strings.TrimSpace(kind)
			if kind != "" {
				filter.kinds[kind] = struct{}{}
			}
		}
	}
	return filter, nil
}

func toUnstructured(obj any) *unstructured.Unstructured {
	switch v := obj.(type) {
	case *unstructured.Unstructured:
		return v
	case unstructured.Unstructured:
		return &v
	case *metav1.PartialObjectMetadata:
		u := &unstructured.Unstructured{}
		u.SetName(v.GetName())
		u.SetNamespace(v.GetNamespace())
		u.SetLabels(v.GetLabels())
		return u
	default:
		return &unstructured.Unstructured{}
	}
}

func parseNodeID(id string) (string, string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	kind := parts[0]
	nsName := parts[1]
	if idx := strings.Index(nsName, "/"); idx != -1 {
		return kind, nsName[:idx]
	}
	return kind, nsName
}

func keyFor(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", namespace, name)
}

func serviceTemplateNode(summary api.ServiceTemplateSummary) GraphNode {
	summaryMap := map[string]any{}
	if summary.Version != "" {
		summaryMap["version"] = summary.Version
	}
	if summary.ChartName != "" {
		summaryMap["chart"] = map[string]string{
			"name": summary.ChartName,
			"kind": summary.ChartKind,
		}
	}
	if summary.Description != "" {
		summaryMap["description"] = summary.Description
	}
	return GraphNode{
		ID:        fmt.Sprintf("ServiceTemplate:%s/%s", summary.Namespace, summary.Name),
		Type:      "ServiceTemplate",
		Name:      summary.Name,
		Namespace: summary.Namespace,
		Labels:    summary.Labels,
		Summary:   summaryMap,
	}
}

func clusterDeploymentNode(summary api.ClusterDeploymentSummary) GraphNode {
	summaryMap := map[string]any{}
	if summary.TemplateRef != "" {
		summaryMap["template"] = summary.TemplateRef
	}
	if summary.CredentialRef != "" {
		summaryMap["credential"] = summary.CredentialRef
	}
	if len(summary.ServiceTemplates) > 0 {
		summaryMap["serviceTemplates"] = summary.ServiceTemplates
	}
	return GraphNode{
		ID:        fmt.Sprintf("ClusterDeployment:%s/%s", summary.Namespace, summary.Name),
		Type:      "ClusterDeployment",
		Name:      summary.Name,
		Namespace: summary.Namespace,
		Labels:    summary.Labels,
		Summary:   summaryMap,
	}
}

func multiClusterServiceNode(summary api.MultiClusterServiceSummary) GraphNode {
	summaryMap := map[string]any{}
	if len(summary.MatchLabels) > 0 {
		summaryMap["matchLabels"] = summary.MatchLabels
	}
	summaryMap["serviceCount"] = summary.ServiceCount
	return GraphNode{
		ID:        fmt.Sprintf("MultiClusterService:%s/%s", summary.Namespace, summary.Name),
		Type:      "MultiClusterService",
		Name:      summary.Name,
		Namespace: summary.Namespace,
		Labels:    summary.Labels,
		Summary:   summaryMap,
	}
}

// Graph tool

type graphTool struct {
	manager *GraphManager
}

type graphSnapshotInput struct {
	Namespace string   `json:"namespace,omitempty"`
	Kinds     []string `json:"kinds,omitempty"`
}

type graphSnapshotResult struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

func registerGraph(server *mcp.Server, session *runtime.Session, manager *GraphManager) error {
	if manager == nil {
		return errors.New("graph manager is required")
	}

	manager.Bind(server, session)

	tool := &graphTool{manager: manager}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.graph.snapshot",
		Description: "Return a graph snapshot of K0rdent resources",
	}, tool.snapshot)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "k0rdent.graph",
		Title:       "K0rdent graph stream",
		Description: "Streaming graph deltas for K0rdent resources",
		URITemplate: graphURITemplate,
		MIMEType:    graphMIMEType,
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		filter, err := parseGraphURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		snapshot, err := manager.Snapshot(ctx, filter)
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(snapshot)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: graphMIMEType,
			Blob:     payload,
		}}}, nil
	})

	return nil
}

func (t *graphTool) snapshot(ctx context.Context, req *mcp.CallToolRequest, input graphSnapshotInput) (*mcp.CallToolResult, graphSnapshotResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.manager.session, name, "tool.graph")
	logger = logger.With("tool", name, "namespace", input.Namespace, "kinds", input.Kinds)
	start := time.Now()
	logger.Info("handling graph snapshot request")

	filter := graphFilter{}
	if input.Namespace != "" {
		filter.namespaces = map[string]struct{}{input.Namespace: {}}
	}
	if len(input.Kinds) > 0 {
		filter.kinds = make(map[string]struct{}, len(input.Kinds))
		for _, kind := range input.Kinds {
			kind = strings.TrimSpace(kind)
			if kind != "" {
				filter.kinds[kind] = struct{}{}
			}
		}
	}
	snapshot, err := t.manager.Snapshot(ctx, filter)
	if err != nil {
		logger.Error("graph snapshot failed", "tool", name, "error", err)
		return nil, graphSnapshotResult{}, err
	}
	logger.Info("graph snapshot ready", "tool", name, "nodes", len(snapshot.Nodes), "edges", len(snapshot.Edges), "duration_ms", time.Since(start).Milliseconds())
	return nil, graphSnapshotResult{Nodes: snapshot.Nodes, Edges: snapshot.Edges}, nil
}

func mapKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}

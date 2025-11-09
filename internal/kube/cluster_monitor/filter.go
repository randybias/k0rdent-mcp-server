package clustermonitor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
)

// EventFilter evaluates raw Kubernetes events and emits high-value progress updates.
type EventFilter struct {
	clusterName string
	namespace   string
	deduper     *eventDeduplicator
	now         func() time.Time
}

// EventPattern represents a high-signal event signature.
type EventPattern struct {
	Reason          string
	InvolvedKind    string
	MessageContains []string
	Message         string
	Phase           ProvisioningPhase
	Severity        SeverityLevel
	Terminal        bool
}

// eventDeduplicator enforces per-pattern emission windows.
type eventDeduplicator struct {
	now      func() time.Time
	mu       sync.Mutex
	lastSeen map[string]time.Time
}

type deduplicationRule struct {
	Reason string
	Window time.Duration
}

// EventFilterResult captures the outcome of a successful match.
type EventFilterResult struct {
	Update ProgressUpdate
}

// NewEventFilter builds an EventFilter for a specific cluster namespace/name.
func NewEventFilter(clusterName, namespace string) *EventFilter {
	return &EventFilter{
		clusterName: strings.ToLower(clusterName),
		namespace:   namespace,
		deduper:     newEventDeduplicator(time.Now),
		now:         time.Now,
	}
}

// WithClock overrides the time source, primarily for testing.
func (f *EventFilter) WithClock(clock func() time.Time) {
	if clock == nil {
		return
	}
	f.now = clock
	f.deduper.now = clock
}

// Evaluate returns a ProgressUpdate if the event passes all filters.
func (f *EventFilter) Evaluate(event eventsprovider.Event) (*EventFilterResult, bool) {
	if f == nil {
		return nil, false
	}
	if !f.matchesScope(event) {
		return nil, false
	}
	if isSuppressedEvent(event) {
		return nil, false
	}
	pattern, ok := matchSignificantPattern(event)
	if !ok {
		return nil, false
	}
	if !f.deduper.shouldEmit(event) {
		return nil, false
	}

	update := ProgressUpdate{
		Timestamp: eventTimestampOrNow(event, f.now),
		Phase:     pattern.Phase,
		Message:   resolveMessage(pattern, event),
		Reason:    event.Reason,
		Source:    SourceEvent,
		Severity:  pattern.Severity,
		RelatedObject: &ObjectReference{
			Kind:      event.InvolvedObject.Kind,
			Name:      event.InvolvedObject.Name,
			Namespace: effectiveNamespace(event),
			UID:       event.InvolvedObject.UID,
		},
		Terminal: pattern.Terminal || pattern.Phase.IsTerminal(),
	}
	return &EventFilterResult{Update: update}, true
}

// InScope reports whether the event references resources related to the filter's cluster.
func (f *EventFilter) InScope(event eventsprovider.Event) bool {
	if f == nil {
		return false
	}
	return f.matchesScope(event)
}

func (f *EventFilter) matchesScope(event eventsprovider.Event) bool {
	name := strings.ToLower(event.InvolvedObject.Name)
	cluster := f.clusterName
	if cluster == "" {
		return false
	}
	if ns := effectiveNamespace(event); f.namespace != "" && ns != "" && ns != f.namespace {
		return false
	}
	if name == cluster || strings.HasPrefix(name, cluster+"-") {
		return true
	}
	return strings.Contains(name, cluster) || strings.Contains(strings.ToLower(event.Message), cluster)
}

func effectiveNamespace(event eventsprovider.Event) string {
	if event.InvolvedObject.Namespace != "" {
		return event.InvolvedObject.Namespace
	}
	return event.Namespace
}

func resolveMessage(pattern EventPattern, event eventsprovider.Event) string {
	if pattern.Message != "" {
		return pattern.Message
	}
	return event.Message
}

func eventTimestampOrNow(event eventsprovider.Event, now func() time.Time) time.Time {
	if ts := eventTimestamp(event); !ts.IsZero() {
		return ts
	}
	return now()
}

func newEventDeduplicator(clock func() time.Time) *eventDeduplicator {
	if clock == nil {
		clock = time.Now
	}
	return &eventDeduplicator{
		now:      clock,
		lastSeen: make(map[string]time.Time),
	}
}

func (d *eventDeduplicator) shouldEmit(event eventsprovider.Event) bool {
	if d == nil {
		return true
	}
	key := fmt.Sprintf("%s/%s/%s", strings.ToLower(event.Reason), strings.ToLower(event.InvolvedObject.Kind), strings.ToLower(event.InvolvedObject.Name))
	window := deduplicationWindow(event.Reason)
	if window == 0 {
		return true
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if last, ok := d.lastSeen[key]; ok {
		if d.now().Sub(last) < window {
			return false
		}
	}
	d.lastSeen[key] = d.now()
	return true
}

func deduplicationWindow(reason string) time.Duration {
	for _, rule := range defaultDeduplicationRules {
		if strings.EqualFold(rule.Reason, reason) {
			return rule.Window
		}
	}
	return time.Second * 30
}

func isSuppressedEvent(event eventsprovider.Event) bool {
	reasonLower := strings.ToLower(event.Reason)
	for _, reason := range suppressedReasons {
		if strings.Contains(reasonLower, reason) {
			return true
		}
	}
	messageLower := strings.ToLower(event.Message)
	for _, fragment := range suppressedMessageFragments {
		if strings.Contains(messageLower, fragment) {
			return true
		}
	}
	return false
}

func matchSignificantPattern(event eventsprovider.Event) (EventPattern, bool) {
	for _, pattern := range significantEventPatterns {
		if !pattern.matches(event) {
			continue
		}
		return pattern, true
	}
	return EventPattern{}, false
}

func (p EventPattern) matches(event eventsprovider.Event) bool {
	if p.Reason != "" && !strings.EqualFold(p.Reason, event.Reason) {
		return false
	}
	if p.InvolvedKind != "" && !strings.EqualFold(p.InvolvedKind, event.InvolvedObject.Kind) {
		return false
	}
	messageLower := strings.ToLower(event.Message)
	for _, fragment := range p.MessageContains {
		if !strings.Contains(messageLower, strings.ToLower(fragment)) {
			return false
		}
	}
	return true
}

var defaultDeduplicationRules = []deduplicationRule{
	{Reason: "CAPIClusterIsProvisioning", Window: time.Minute},
	{Reason: "ServiceSetCollectServiceStatusesFailed", Window: 5 * time.Minute},
	{Reason: "ClusterReconcilerNormalFailed", Window: 2 * time.Minute},
	{Reason: "InfrastructureReady", Window: 30 * time.Second},
}

var suppressedReasons = []string{
	"artifactuptodate",
	"ownerrefnotset",
	"vmidentitynone",
}

var suppressedMessageFragments = []string{
	"waitingforcontrolplaneinitialization",
	"waitingforclusterinfrastructure",
	"machine controller dependency not yet met",
}

var significantEventPatterns = []EventPattern{
	{
		Reason:       "HelmReleaseCreated",
		InvolvedKind: "ClusterDeployment",
		Message:      "Helm release created for cluster deployment",
		Phase:        PhaseInitializing,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "CAPIClusterIsProvisioning",
		InvolvedKind: "ClusterDeployment",
		Message:      "Cluster provisioning started",
		Phase:        PhaseProvisioning,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "InstallSucceeded",
		InvolvedKind: "HelmRelease",
		Message:      "Helm chart installation succeeded",
		Phase:        PhaseInitializing,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "BeginCreateOrUpdate",
		InvolvedKind: "VirtualNetwork",
		Message:      "Virtual network creation started",
		Phase:        PhaseProvisioning,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "BeginCreateOrUpdate",
		InvolvedKind: "Subnet",
		Message:      "Subnet creation started",
		Phase:        PhaseProvisioning,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "SuccessfulCreate",
		InvolvedKind: "MachineDeployment",
		Message:      "Worker node deployment created",
		Phase:        PhaseProvisioning,
		Severity:     SeverityInfo,
	},
	{
		Reason:          "MachineReady",
		InvolvedKind:    "Machine",
		MessageContains: []string{"control plane"},
		Message:         "Control plane machine ready",
		Phase:           PhaseBootstrapping,
		Severity:        SeverityInfo,
	},
	{
		Reason:          "MachineReady",
		InvolvedKind:    "Machine",
		MessageContains: []string{"worker"},
		Message:         "Worker machine ready",
		Phase:           PhaseScaling,
		Severity:        SeverityInfo,
	},
	{
		Reason:       "NodeJoined",
		InvolvedKind: "Machine",
		Message:      "Node joined the cluster",
		Phase:        PhaseScaling,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "ServiceInstalling",
		InvolvedKind: "ServiceSet",
		Message:      "Installing service template",
		Phase:        PhaseInstalling,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "ServiceReady",
		InvolvedKind: "ServiceSet",
		Message:      "Service ready",
		Phase:        PhaseInstalling,
		Severity:     SeverityInfo,
	},
	{
		Reason:       "CAPIClusterIsReady",
		InvolvedKind: "ClusterDeployment",
		Message:      "Cluster fully provisioned and operational",
		Phase:        PhaseReady,
		Severity:     SeverityInfo,
		Terminal:     true,
	},
	{
		Reason:       "ProvisioningFailed",
		InvolvedKind: "ClusterDeployment",
		Message:      "Cluster provisioning failed",
		Phase:        PhaseFailed,
		Severity:     SeverityError,
		Terminal:     true,
	},
	{
		Reason:       "ServiceSetCollectServiceStatusesFailed",
		InvolvedKind: "ServiceSet",
		Message:      "Service installation encountering issues",
		Phase:        PhaseInstalling,
		Severity:     SeverityWarning,
	},
	{
		Reason:       "ClusterReconcilerNormalFailed",
		InvolvedKind: "AzureCluster",
		Message:      "Azure cluster reconciliation warning",
		Phase:        PhaseProvisioning,
		Severity:     SeverityWarning,
	},
}

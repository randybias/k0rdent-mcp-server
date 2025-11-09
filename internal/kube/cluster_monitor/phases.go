package clustermonitor

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
)

const (
	eventRecencyWindow = 2 * time.Minute
)

var timeNow = time.Now

// DetectPhase infers the current provisioning phase for the ClusterDeployment by
// examining its status conditions and, when ambiguous, falling back to recent events.
func DetectPhase(cd *unstructured.Unstructured, recentEvents []eventsprovider.Event) ProvisioningPhase {
	if cd == nil {
		return PhaseInitializing
	}

	summary := clusters.SummarizeClusterDeployment(cd)
	conditions := summary.Conditions

	if summary.Ready {
		return PhaseReady
	}

	if isFailed(summary, conditions) {
		return PhaseFailed
	}

	if phase := phaseFromStatus(summary); phase != PhaseUnknown {
		return phase
	}

	phaseFromConditions := detectPhaseFromConditions(conditions)
	if phaseFromConditions != PhaseUnknown {
		return phaseFromConditions
	}

	if phase := detectPhaseFromEvents(recentEvents); phase != PhaseUnknown {
		return phase
	}

	if len(conditions) == 0 {
		return PhaseInitializing
	}

	return PhaseProvisioning
}

// EstimateProgress attempts to assign an approximate completion percentage based on the
// detected phase and any reinforcing condition signals.
func EstimateProgress(phase ProvisioningPhase, conditions []clusters.ConditionSummary) *int {
	base, ok := defaultProgressByPhase[phase]
	if !ok {
		return nil
	}

	switch phase {
	case PhaseProvisioning:
		if isConditionTrue(conditions, "InfrastructureReady") {
			base = 40
		}
	case PhaseBootstrapping:
		if isConditionTrue(conditions, "ControlPlaneInitialized") {
			base = 60
		}
	case PhaseScaling:
		if isConditionTrue(conditions, "WorkersAvailable") || isConditionTrue(conditions, "WorkerMachinesReady") {
			base = 85
		}
	case PhaseInstalling:
		if isConditionTrue(conditions, "ServicesInReadyState") || isConditionTrue(conditions, "ServicesReady") {
			base = 95
		}
	}

	return ptr(base)
}

func detectPhaseFromConditions(conditions []clusters.ConditionSummary) ProvisioningPhase {
	switch {
	case isConditionFalse(conditions, "InfrastructureReady"):
		return PhaseProvisioning
	case isConditionFalse(conditions, "ControlPlaneInitialized"),
		isConditionFalse(conditions, "ControlPlaneAvailable"):
		return PhaseBootstrapping
	case isConditionFalse(conditions, "WorkersAvailable"),
		isConditionFalse(conditions, "WorkerMachinesReady"):
		return PhaseScaling
	case isConditionFalse(conditions, "ServicesInReadyState"),
		isConditionFalse(conditions, "ServicesInstalled"):
		return PhaseInstalling
	}
	return PhaseUnknown
}

func detectPhaseFromEvents(events []eventsprovider.Event) ProvisioningPhase {
	now := timeNow()
	for _, evt := range events {
		if !isRecentEvent(evt, now) {
			continue
		}
		if phase := phaseFromEvent(evt); phase != PhaseUnknown {
			return phase
		}
	}
	return PhaseUnknown
}

func phaseFromEvent(event eventsprovider.Event) ProvisioningPhase {
	reason := strings.ToLower(event.Reason)
	message := strings.ToLower(event.Message)

	switch {
	case strings.Contains(reason, "ready") && strings.Contains(message, "cluster"):
		return PhaseReady
	case strings.Contains(reason, "failed") || (event.Type == corev1.EventTypeWarning && strings.Contains(message, "failed")):
		return PhaseFailed
	case strings.Contains(reason, "provision") || strings.Contains(message, "infrastructure"):
		return PhaseProvisioning
	case strings.Contains(reason, "bootstrap") || strings.Contains(message, "control plane"):
		return PhaseBootstrapping
	case strings.Contains(reason, "machine") || strings.Contains(message, "node joined"):
		return PhaseScaling
	case strings.Contains(reason, "service") && (strings.Contains(message, "install") || strings.Contains(message, "ready")):
		return PhaseInstalling
	default:
		return PhaseUnknown
	}
}

func phaseFromStatus(summary clusters.ClusterDeploymentSummary) ProvisioningPhase {
	switch strings.ToLower(summary.Phase) {
	case "ready":
		return PhaseReady
	case "failed":
		return PhaseFailed
	case "provisioning":
		return PhaseProvisioning
	case "bootstrapping":
		return PhaseBootstrapping
	case "scaling":
		return PhaseScaling
	case "installing":
		return PhaseInstalling
	default:
		return PhaseUnknown
	}
}

func isFailed(summary clusters.ClusterDeploymentSummary, conditions []clusters.ConditionSummary) bool {
	if strings.EqualFold(summary.Message, "failed") {
		return true
	}
	if strings.Contains(strings.ToLower(summary.Message), "failed") {
		return true
	}
	cond, ok := findCondition(conditions, "Ready")
	if ok && strings.EqualFold(cond.Status, string(corev1.ConditionFalse)) {
		if strings.EqualFold(cond.Reason, "Failed") || strings.Contains(strings.ToLower(cond.Message), "failed") {
			return true
		}
	}
	return false
}

func findCondition(conditions []clusters.ConditionSummary, condType string) (clusters.ConditionSummary, bool) {
	for _, cond := range conditions {
		if strings.EqualFold(cond.Type, condType) {
			return cond, true
		}
	}
	return clusters.ConditionSummary{}, false
}

func isConditionTrue(conditions []clusters.ConditionSummary, condType string) bool {
	cond, ok := findCondition(conditions, condType)
	if !ok {
		return false
	}
	return strings.EqualFold(cond.Status, string(corev1.ConditionTrue))
}

func isConditionFalse(conditions []clusters.ConditionSummary, condType string) bool {
	cond, ok := findCondition(conditions, condType)
	if !ok {
		return false
	}
	return !strings.EqualFold(cond.Status, string(corev1.ConditionTrue))
}

func isRecentEvent(event eventsprovider.Event, now time.Time) bool {
	ts := eventTimestamp(event)
	if ts.IsZero() {
		return true
	}
	return now.Sub(ts) <= eventRecencyWindow
}

func eventTimestamp(event eventsprovider.Event) time.Time {
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

func ptr(value int) *int {
	return &value
}

var defaultProgressByPhase = map[ProvisioningPhase]int{
	PhaseUnknown:       0,
	PhaseInitializing:  5,
	PhaseProvisioning:  25,
	PhaseBootstrapping: 50,
	PhaseScaling:       75,
	PhaseInstalling:    90,
	PhaseReady:         100,
	PhaseFailed:        0,
}

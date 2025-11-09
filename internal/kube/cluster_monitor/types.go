package clustermonitor

import (
	"time"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
)

// ProvisioningPhase describes the coarse-grained lifecycle stage of a ClusterDeployment.
type ProvisioningPhase string

// Enumerated provisioning phases reported to subscribers.
const (
	PhaseUnknown       ProvisioningPhase = "Unknown"
	PhaseInitializing  ProvisioningPhase = "Initializing"
	PhaseProvisioning  ProvisioningPhase = "Provisioning"
	PhaseBootstrapping ProvisioningPhase = "Bootstrapping"
	PhaseScaling       ProvisioningPhase = "Scaling"
	PhaseInstalling    ProvisioningPhase = "Installing"
	PhaseReady         ProvisioningPhase = "Ready"
	PhaseFailed        ProvisioningPhase = "Failed"
)

func (p ProvisioningPhase) String() string {
	if p == "" {
		return string(PhaseUnknown)
	}
	return string(p)
}

// SeverityLevel indicates the importance of a progress update.
type SeverityLevel string

const (
	SeverityInfo    SeverityLevel = "info"
	SeverityWarning SeverityLevel = "warning"
	SeverityError   SeverityLevel = "error"
)

// UpdateSource identifies the origin of a progress update.
type UpdateSource string

const (
	SourceCondition UpdateSource = "condition"
	SourceEvent     UpdateSource = "event"
	SourceLog       UpdateSource = "log"
	SourceSystem    UpdateSource = "system"
)

// ObjectReference mirrors a subset of the Kubernetes ObjectReference fields for transport.
type ObjectReference struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	UID       string `json:"uid,omitempty"`
}

// ProgressUpdate encapsulates a single streaming delta published to clients.
type ProgressUpdate struct {
	Timestamp     time.Time                   `json:"timestamp"`
	Phase         ProvisioningPhase           `json:"phase"`
	Progress      *int                        `json:"progress,omitempty"`
	Message       string                      `json:"message,omitempty"`
	Reason        string                      `json:"reason,omitempty"`
	Source        UpdateSource                `json:"source,omitempty"`
	Severity      SeverityLevel               `json:"severity,omitempty"`
	RelatedObject *ObjectReference            `json:"relatedObject,omitempty"`
	Conditions    []clusters.ConditionSummary `json:"conditions,omitempty"`
	Terminal      bool                        `json:"terminal,omitempty"`
}

// IsTerminal reports whether the supplied phase represents a terminal lifecycle state.
func (p ProvisioningPhase) IsTerminal() bool {
	return p == PhaseReady || p == PhaseFailed
}

// Copy returns a deep copy of the progress update, safe for mutation before publication.
func (u *ProgressUpdate) Copy() ProgressUpdate {
	if u == nil {
		return ProgressUpdate{}
	}
	clone := *u
	if u.RelatedObject != nil {
		ref := *u.RelatedObject
		clone.RelatedObject = &ref
	}
	if len(u.Conditions) > 0 {
		clone.Conditions = make([]clusters.ConditionSummary, len(u.Conditions))
		copy(clone.Conditions, u.Conditions)
	}
	if u.Progress != nil {
		val := *u.Progress
		clone.Progress = &val
	}
	return clone
}

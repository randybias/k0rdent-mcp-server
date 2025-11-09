package clustermonitor

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stretchr/testify/require"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
)

func TestDetectPhase_Initializing(t *testing.T) {
	cd := newClusterDeployment(nil, "", "")
	phase := DetectPhase(cd, nil)
	require.Equal(t, PhaseInitializing, phase)
}

func TestDetectPhase_Provisioning(t *testing.T) {
	conditions := []map[string]any{
		newCondition("InfrastructureReady", string(corev1.ConditionFalse), "Creating", "Creating infra"),
	}
	cd := newClusterDeployment(conditions, "", "")
	phase := DetectPhase(cd, nil)
	require.Equal(t, PhaseProvisioning, phase)
}

func TestDetectPhase_Bootstrapping(t *testing.T) {
	conditions := []map[string]any{
		newCondition("InfrastructureReady", string(corev1.ConditionTrue), "Done", ""),
		newCondition("ControlPlaneInitialized", string(corev1.ConditionFalse), "Waiting", "Control plane starting"),
	}
	cd := newClusterDeployment(conditions, "", "")
	phase := DetectPhase(cd, nil)
	require.Equal(t, PhaseBootstrapping, phase)
}

func TestDetectPhase_Scaling(t *testing.T) {
	conditions := []map[string]any{
		newCondition("InfrastructureReady", string(corev1.ConditionTrue), "", ""),
		newCondition("ControlPlaneInitialized", string(corev1.ConditionTrue), "", ""),
		newCondition("WorkersAvailable", string(corev1.ConditionFalse), "ScalingUp", "Waiting for workers"),
	}
	cd := newClusterDeployment(conditions, "", "")
	phase := DetectPhase(cd, nil)
	require.Equal(t, PhaseScaling, phase)
}

func TestDetectPhase_Ready(t *testing.T) {
	conditions := []map[string]any{
		newCondition("Ready", string(corev1.ConditionTrue), "Succeeded", "Cluster ready"),
	}
	cd := newClusterDeployment(conditions, "Ready", "")
	phase := DetectPhase(cd, nil)
	require.Equal(t, PhaseReady, phase)
}

func TestDetectPhase_Failed(t *testing.T) {
	conditions := []map[string]any{
		newCondition("Ready", string(corev1.ConditionFalse), "Failed", "Provisioning failed"),
	}
	cd := newClusterDeployment(conditions, "Failed", "Provisioning failed")
	phase := DetectPhase(cd, nil)
	require.Equal(t, PhaseFailed, phase)
}

func TestDetectPhase_UsesRecentEvents(t *testing.T) {
	origNow := timeNow
	defer func() { timeNow = origNow }()

	base := time.Unix(1_700_000_000, 0).UTC()
	timeNow = func() time.Time { return base.Add(30 * time.Second) }

	eventTime := base
	event := eventsprovider.Event{
		Reason:    "ServiceInstalling",
		Message:   "Installing service template",
		EventTime: &eventTime,
		InvolvedObject: eventsprovider.InvolvedObject{
			Kind:      "ServiceSet",
			Name:      "demo-cluster-services",
			Namespace: "kcm-system",
		},
	}
	cd := newClusterDeployment(nil, "", "")
	phase := DetectPhase(cd, []eventsprovider.Event{event})
	require.Equal(t, PhaseInstalling, phase)
}

func TestEstimateProgress(t *testing.T) {
	progress := EstimateProgress(PhaseScaling, []clusters.ConditionSummary{
		{
			Type:   "WorkersAvailable",
			Status: string(corev1.ConditionTrue),
		},
	})
	require.NotNil(t, progress)
	require.Equal(t, 85, *progress)

	require.Nil(t, EstimateProgress(ProvisioningPhase("Unrecognized"), nil))
}

func newClusterDeployment(conditions []map[string]any, phase, message string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterDeployment",
			"metadata": map[string]any{
				"name":      "demo-cluster",
				"namespace": "kcm-system",
			},
		},
	}
	status := map[string]any{}
	if phase != "" {
		status["phase"] = phase
	}
	if message != "" {
		status["message"] = message
	}
	if len(conditions) > 0 {
		values := make([]interface{}, len(conditions))
		for i := range conditions {
			values[i] = conditions[i]
		}
		status["conditions"] = values
	}
	if len(status) > 0 {
		obj.Object["status"] = status
	}
	return obj
}

func newCondition(condType, status, reason, message string) map[string]any {
	cond := map[string]any{
		"type":   condType,
		"status": status,
	}
	if reason != "" {
		cond["reason"] = reason
	}
	if message != "" {
		cond["message"] = message
	}
	cond["lastTransitionTime"] = time.Now().UTC().Format(time.RFC3339)
	return cond
}

package clustermonitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
)

func TestFilterSignificantEventsInclude(t *testing.T) {
	filter := NewEventFilter("demo-cluster", "kcm-system")
	event := eventsprovider.Event{
		Reason: "BeginCreateOrUpdate",
		InvolvedObject: eventsprovider.InvolvedObject{
			Kind:      "VirtualNetwork",
			Name:      "demo-cluster-vnet",
			Namespace: "kcm-system",
		},
		Message: "Virtual network creation started",
	}

	result, ok := filter.Evaluate(event)
	require.True(t, ok)
	require.Equal(t, PhaseProvisioning, result.Update.Phase)
	require.Equal(t, "Virtual network creation started", result.Update.Message)
	require.Equal(t, "kcm-system", result.Update.RelatedObject.Namespace)
}

func TestFilterSignificantEventsExclude(t *testing.T) {
	filter := NewEventFilter("demo-cluster", "kcm-system")
	event := eventsprovider.Event{
		Reason: "ArtifactUpToDate",
		InvolvedObject: eventsprovider.InvolvedObject{
			Kind:      "HelmChart",
			Name:      "demo-cluster-helm",
			Namespace: "kcm-system",
		},
		Message: "artifact up-to-date",
	}

	_, ok := filter.Evaluate(event)
	require.False(t, ok)
}

func TestFilterSignificantEventsDeduplication(t *testing.T) {
	filter := NewEventFilter("demo-cluster", "kcm-system")
	now := time.Unix(1_700_000_000, 0).UTC()
	filter.WithClock(func() time.Time { return now })

	event := eventsprovider.Event{
		Reason: "CAPIClusterIsProvisioning",
		InvolvedObject: eventsprovider.InvolvedObject{
			Kind:      "ClusterDeployment",
			Name:      "demo-cluster",
			Namespace: "kcm-system",
		},
		Message: "Cluster provisioning started",
	}

	if _, ok := filter.Evaluate(event); !ok {
		t.Fatalf("expected first event to emit")
	}

	// Advance 10 seconds, still within dedup window.
	filter.WithClock(func() time.Time { return now.Add(10 * time.Second) })
	if _, ok := filter.Evaluate(event); ok {
		t.Fatalf("expected duplicate event to be suppressed")
	}

	// Move past window to allow emission.
	filter.WithClock(func() time.Time { return now.Add(2 * time.Minute) })
	if _, ok := filter.Evaluate(event); !ok {
		t.Fatalf("expected event to emit after window")
	}
}

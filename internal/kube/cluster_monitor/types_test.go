package clustermonitor

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
)

func TestProgressUpdateSerialization(t *testing.T) {
	timestamp := time.Unix(1_700_000_000, 0).UTC()
	progress := 75
	update := ProgressUpdate{
		Timestamp: timestamp,
		Phase:     PhaseScaling,
		Progress:  &progress,
		Message:   "Worker machine ready",
		Reason:    "MachineReady",
		Source:    SourceEvent,
		Severity:  SeverityInfo,
		RelatedObject: &ObjectReference{
			Kind:      "Machine",
			Name:      "demo-cluster-md-abc",
			Namespace: "kcm-system",
			UID:       "1234",
		},
		Conditions: []clusters.ConditionSummary{{
			Type:    "WorkersAvailable",
			Status:  "False",
			Reason:  "ScalingUp",
			Message: "Rolling out replicas",
		}},
	}

	data, err := json.Marshal(update)
	require.NoError(t, err)

	var decoded ProgressUpdate
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Equal(t, update.Timestamp, decoded.Timestamp)
	require.Equal(t, update.Phase, decoded.Phase)
	require.NotNil(t, decoded.Progress)
	require.Equal(t, *update.Progress, *decoded.Progress)
	require.Equal(t, update.Message, decoded.Message)
	require.Equal(t, update.Reason, decoded.Reason)
	require.Equal(t, update.Source, decoded.Source)
	require.Equal(t, update.Severity, decoded.Severity)
	require.Equal(t, update.RelatedObject, decoded.RelatedObject)
	require.Len(t, decoded.Conditions, 1)
	require.Equal(t, update.Conditions[0].Type, decoded.Conditions[0].Type)
	require.False(t, decoded.Terminal)
}

package events

import (
	"testing"
	"time"
)

func TestFilterEventsByTypeAndKind(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	provider := &Provider{
		serverTimeSource: func() time.Time { return now },
	}

	events := []Event{
		{
			Name: "a",
			Type: "Warning",
			EventTime: func() *time.Time {
				ts := now.Add(-30 * time.Second)
				return &ts
			}(),
			InvolvedObject: InvolvedObject{
				Kind: "Pod",
				Name: "demo",
			},
		},
		{
			Name: "b",
			Type: "Normal",
			EventTime: func() *time.Time {
				ts := now.Add(-10 * time.Second)
				return &ts
			}(),
			InvolvedObject: InvolvedObject{
				Kind: "Deployment",
				Name: "demo",
			},
		},
	}

    since := int64(40)
	filtered := provider.filterEvents(events, ListOptions{
		Types:        []string{"warning"},
		ForKind:      "pod",
		ForName:      "demo",
		SinceSeconds: &since,
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 event after filtering, got %d", len(filtered))
	}
	if filtered[0].Name != "a" {
		t.Fatalf("expected event 'a', got %q", filtered[0].Name)
	}
}

func TestEnforceLimit(t *testing.T) {
	provider := &Provider{}
	events := []Event{{Name: "1"}, {Name: "2"}, {Name: "3"}}
	limit := 2
	limited := provider.enforceLimit(events, &limit)
	if len(limited) != 2 {
		t.Fatalf("expected limit to apply, got %d", len(limited))
	}
}

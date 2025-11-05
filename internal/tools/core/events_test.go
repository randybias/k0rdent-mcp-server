package core

import (
	"context"
	"testing"
)

func TestParseEventsURI(t *testing.T) {
	ns, err := parseEventsURI("k0://events/team-alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns != "team-alpha" {
		t.Fatalf("expected namespace team-alpha, got %s", ns)
	}

	if _, err := parseEventsURI("http://events/foo"); err == nil {
		t.Fatalf("expected error for invalid scheme")
	}
}

func TestEventsToolRequiresNamespace(t *testing.T) {
	tool := &eventsTool{}
	if _, _, err := tool.list(context.Background(), nil, eventsListInput{}); err == nil {
		t.Fatalf("expected error when namespace missing")
	}
}

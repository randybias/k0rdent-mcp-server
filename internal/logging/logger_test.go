package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingSink struct {
	mu      sync.Mutex
	entries []Entry
}

func (s *recordingSink) Write(_ context.Context, e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	return nil
}

func (s *recordingSink) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func TestManagerForwardsSink(t *testing.T) {
	var buf bytes.Buffer
	sink := &recordingSink{}

	mgr := NewManager(Options{
		Level:       slog.LevelInfo,
		Sink:        sink,
		Destination: &buf,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := mgr.Close(ctx); err != nil {
			t.Fatalf("Close() error: %v", err)
		}
	})

	logger := mgr.Logger()
	logger.InfoContext(context.Background(), "test log", "foo", "bar")

	deadline := time.Now().Add(500 * time.Millisecond)
	for sink.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if sink.Len() == 0 {
		t.Fatalf("expected sink to receive entry")
	}

	if buf.Len() == 0 {
		t.Fatalf("expected JSON log written to destination")
	}
}

func TestWithContextEnrichment(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithSessionID(ctx, "sess-456")
	ctx = WithToolName(ctx, "k0.namespaces.list")
	ctx = WithNamespace(ctx, "default")

	enriched := WithContext(ctx, logger)
	// The enriched logger should not be the same pointer when attributes are added.
	if enriched == logger {
		t.Fatalf("expected logger to be enriched")
	}

	// Ensure helper getters return expected values.
	if got := RequestID(ctx); got != "req-123" {
		t.Fatalf("RequestID() = %q, want %q", got, "req-123")
	}
	if got := SessionID(ctx); got != "sess-456" {
		t.Fatalf("SessionID() = %q, want %q", got, "sess-456")
	}
	if got := ToolName(ctx); got != "k0.namespaces.list" {
		t.Fatalf("ToolName() = %q, want %q", got, "k0.namespaces.list")
	}
	if got := Namespace(ctx); got != "default" {
		t.Fatalf("Namespace() = %q, want %q", got, "default")
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"":        slog.LevelInfo,
		"INFO":    slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"Warn":    slog.LevelWarn,
		"WARNING": slog.LevelWarn,
		"error":   slog.LevelError,
		"TRACE":   slog.LevelDebug - 4,
		"fatal":   slog.LevelError + 4,
	}

	for input, want := range tests {
		got, err := ParseLevel(input)
		if err != nil {
			t.Fatalf("ParseLevel(%q) unexpected error: %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}

	if _, err := ParseLevel("LOUD"); err == nil {
		t.Fatalf("ParseLevel should error on invalid input")
	}
}

func TestJSONSinkWrites(t *testing.T) {
	var buf bytes.Buffer
	sink := NewJSONSink(&buf)
	entry := Entry{Time: time.Unix(0, 0), Level: slog.LevelInfo, Message: "hello", Attributes: map[string]any{"foo": "bar"}}
	if err := sink.Write(context.Background(), entry); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "\"msg\":\"hello\"") {
		t.Fatalf("expected encoded message, got %s", buf.String())
	}
}

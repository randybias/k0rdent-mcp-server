package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Sink receives log entries for forwarding to an external system.
type Sink interface {
	Write(context.Context, Entry) error
}

// Entry captures the structured representation of a log record.
type Entry struct {
	Time       time.Time
	Level      slog.Level
	Message    string
	Attributes map[string]any
	Source     *slog.Source
}

// Options configure the logging manager.
type Options struct {
	Level       slog.Leveler
	Sink        Sink
	Destination io.Writer
}

// Manager owns the process-wide logger and optional sink worker.
type Manager struct {
	logger *slog.Logger

	sink Sink
	ch   chan sinkPayload
	wg   sync.WaitGroup
}

// NewManager constructs a structured JSON logger wired to stdout (or a provided writer)
// and, when configured, dispatches a copy of each record to an external sink asynchronously.
func NewManager(opts Options) *Manager {
	dest := opts.Destination
	if dest == nil {
		dest = os.Stdout
	}

	handler := slog.NewJSONHandler(dest, &slog.HandlerOptions{
		Level: opts.Level,
	})

	var (
		payloads chan sinkPayload
		logger   *slog.Logger
	)

	if opts.Sink != nil {
		payloads = make(chan sinkPayload, 128)
		logger = slog.New(&sinkHandler{
			primary: handler,
			sinkCh:  payloads,
		})
	} else {
		logger = slog.New(handler)
	}

	mgr := &Manager{
		logger: logger,
		sink:   opts.Sink,
		ch:     payloads,
	}
	if opts.Sink != nil {
		mgr.wg.Add(1)
		go mgr.drain()
	}
	return mgr
}

// Logger exposes the configured slog.Logger instance.
func (m *Manager) Logger() *slog.Logger {
	if m == nil {
		return nil
	}
	return m.logger
}

// Close flushes sink workers. If ctx expires before the worker stops, ctx.Err() is returned.
func (m *Manager) Close(ctx context.Context) error {
	if m == nil || m.ch == nil {
		return nil
	}
	close(m.ch)

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) drain() {
	defer m.wg.Done()
	for payload := range m.ch {
		if m.sink == nil {
			continue
		}
		if err := m.sink.Write(payload.ctx, payload.entry); err != nil {
			// Best-effort: log sink failure locally without recursion via m.logger.
			// The sink should not panic or block even if forwarding fails.
			slog.Default().Error("external sink write failed", "error", err)
		}
	}
}

type sinkPayload struct {
	ctx   context.Context
	entry Entry
}

type sinkHandler struct {
	primary slog.Handler
	sinkCh  chan<- sinkPayload
}

func (h *sinkHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.primary.Enabled(ctx, level)
}

func (h *sinkHandler) Handle(ctx context.Context, rec slog.Record) error {
	if err := h.primary.Handle(ctx, rec); err != nil {
		return err
	}
	if h.sinkCh == nil {
		return nil
	}

	recCopy := rec.Clone()
	payload := sinkPayload{
		ctx:   ctx,
		entry: recordToEntry(recCopy),
	}

	select {
	case h.sinkCh <- payload:
	default:
		// Fallback to a goroutine so logging never blocks the caller.
		go func() { h.sinkCh <- payload }()
	}
	return nil
}

func (h *sinkHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &sinkHandler{
		primary: h.primary.WithAttrs(attrs),
		sinkCh:  h.sinkCh,
	}
}

func (h *sinkHandler) WithGroup(name string) slog.Handler {
	return &sinkHandler{
		primary: h.primary.WithGroup(name),
		sinkCh:  h.sinkCh,
	}
}

func recordToEntry(rec slog.Record) Entry {
	attrs := make(map[string]any, rec.NumAttrs())
	rec.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = attrToAny(a)
		return true
	})

	return Entry{
		Time:       rec.Time,
		Level:      rec.Level,
		Message:    rec.Message,
		Attributes: attrs,
		Source:     rec.Source(),
	}
}

func attrToAny(attr slog.Attr) any {
	switch attr.Value.Kind() {
	case slog.KindGroup:
		children := attr.Value.Group()
		group := make(map[string]any, len(children))
		for _, child := range children {
			group[child.Key] = attrToAny(child)
		}
		return group
	default:
		return attr.Value.Any()
	}
}

package logging

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

type SinkFunc func(context.Context, Entry) error

func (f SinkFunc) Write(ctx context.Context, e Entry) error {
	if f == nil {
		return nil
	}
	return f(ctx, e)
}

func NewNoopSink() Sink {
	return SinkFunc(func(context.Context, Entry) error { return nil })
}

func NewJSONSink(w io.Writer) Sink {
	if w == nil {
		return NewNoopSink()
	}
	encoder := json.NewEncoder(w)
	return SinkFunc(func(_ context.Context, entry Entry) error {
		return encoder.Encode(struct {
			Time       time.Time      `json:"time"`
			Level      string         `json:"level"`
			Message    string         `json:"msg"`
			Attributes map[string]any `json:"attributes"`
		}{
			Time:       entry.Time,
			Level:      entry.Level.String(),
			Message:    entry.Message,
			Attributes: entry.Attributes,
		})
	})
}

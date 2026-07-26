package logger

import (
	"context"
	"log/slog"

	"github.com/compozy/agh/internal/redact"
)

type redactingHandler struct {
	next   slog.Handler
	engine *redact.Engine
}

func newRedactingHandler(next slog.Handler) slog.Handler {
	return &redactingHandler{next: next, engine: redact.New(redact.Options{Disabled: !redact.Enabled()})}
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, h.engine.RedactString(record.Message), record.PC)
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	redacted.AddAttrs(h.engine.RedactLogAttrs(attrs)...)
	return h.next.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &redactingHandler{next: h.next.WithAttrs(h.engine.RedactLogAttrs(attrs)), engine: h.engine}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name), engine: h.engine}
}

var _ slog.Handler = (*redactingHandler)(nil)

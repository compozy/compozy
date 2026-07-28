package session

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"sync"
	"testing"
	"time"
)

type capturedLogRecord struct {
	Message string
	Level   slog.Level
	Attrs   map[string]string
}

type captureLogHandler struct {
	mu      *sync.RWMutex
	records *[]capturedLogRecord
	attrs   []boundLogAttr
	groups  []string
}

var _ slog.Handler = (*captureLogHandler)(nil)

type boundLogAttr struct {
	attr   slog.Attr
	groups []string
}

func newCaptureLogHandler() *captureLogHandler {
	records := make([]capturedLogRecord, 0, 8)
	return &captureLogHandler{
		mu:      &sync.RWMutex{},
		records: &records,
	}
}

func (h *captureLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureLogHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]string, len(h.attrs)+record.NumAttrs())

	var add func(groups []string, attr slog.Attr)
	add = func(groups []string, attr slog.Attr) {
		attr.Value = attr.Value.Resolve()
		if attr.Key == "" && attr.Value.Kind() == slog.KindGroup {
			for _, child := range attr.Value.Group() {
				add(groups, child)
			}
			return
		}
		if attr.Value.Kind() == slog.KindGroup {
			nextGroups := append(append([]string(nil), groups...), attr.Key)
			for _, child := range attr.Value.Group() {
				add(nextGroups, child)
			}
			return
		}
		key := attr.Key
		if len(groups) > 0 {
			key = groups[0]
			for _, group := range groups[1:] {
				key += "." + group
			}
			key += "." + attr.Key
		}
		attrs[key] = slogValueString(attr.Value)
	}

	for _, attr := range h.attrs {
		add(attr.groups, attr.attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		add(h.groups, attr)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, capturedLogRecord{
		Message: record.Message,
		Level:   record.Level,
		Attrs:   attrs,
	})
	return nil
}

func (h *captureLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	clone := *h
	clone.attrs = append([]boundLogAttr(nil), h.attrs...)
	for _, attr := range attrs {
		clone.attrs = append(clone.attrs, boundLogAttr{
			attr:   attr,
			groups: append([]string(nil), h.groups...),
		})
	}
	return &clone
}

func (h *captureLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	clone := *h
	clone.groups = append(append([]string(nil), h.groups...), name)
	return &clone
}

func TestCaptureLogHandlerBindsAttrsToTheirCurrentGroups(t *testing.T) {
	t.Parallel()

	handler := newCaptureLogHandler()
	logger := slog.New(handler).WithGroup("outer").With("pre", 1).WithGroup("inner")

	logger.Info("grouped", "post", 2)

	record, ok := handler.FindByMessage("grouped")
	if !ok {
		t.Fatalf("FindByMessage(grouped) = false, records = %#v", handler.Records())
	}
	if got, want := record.Attrs["outer.pre"], "1"; got != want {
		t.Fatalf("Attrs[outer.pre] = %q, want %q", got, want)
	}
	if _, ok := record.Attrs["outer.inner.pre"]; ok {
		t.Fatalf("Attrs unexpectedly regrouped bound attr: %#v", record.Attrs)
	}
	if got, want := record.Attrs["outer.inner.post"], "2"; got != want {
		t.Fatalf("Attrs[outer.inner.post] = %q, want %q", got, want)
	}
}

func TestCaptureLogHandlerWithEmptyGroupReturnsReceiver(t *testing.T) {
	t.Parallel()

	handler := newCaptureLogHandler()
	if got := handler.WithGroup(""); got != handler {
		t.Fatalf("WithGroup(\"\") returned %T %p, want original handler %p", got, got, handler)
	}
}

func (h *captureLogHandler) Records() []capturedLogRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	records := make([]capturedLogRecord, 0, len(*h.records))
	for _, record := range *h.records {
		records = append(records, cloneCapturedLogRecord(record))
	}
	return records
}

func (h *captureLogHandler) FindByMessage(message string) (capturedLogRecord, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, record := range *h.records {
		if record.Message == message {
			return cloneCapturedLogRecord(record), true
		}
	}
	return capturedLogRecord{}, false
}

func cloneCapturedLogRecord(record capturedLogRecord) capturedLogRecord {
	record.Attrs = cloneCapturedLogAttrs(record.Attrs)
	return record
}

func cloneCapturedLogAttrs(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(attrs))
	maps.Copy(cloned, attrs)
	return cloned
}

func slogValueString(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'f', -1, 64)
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		return fmt.Sprint(value.Any())
	default:
		return value.String()
	}
}

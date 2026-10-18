package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
)

func TestWriteSSEComment(t *testing.T) {
	t.Run("Should write comment-only keepalive frames", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		if err := core.WriteSSEComment(writer, "keepalive"); err != nil {
			t.Fatalf("WriteSSEComment() error = %v", err)
		}
		if got, want := writer.String(), ": keepalive\n\n"; got != want {
			t.Fatalf("WriteSSEComment() = %q, want %q", got, want)
		}
		for _, forbidden := range []string{"id:", "event:", "data:"} {
			if strings.Contains(writer.String(), forbidden) {
				t.Fatalf("WriteSSEComment() body contains %q: %q", forbidden, writer.String())
			}
		}
	})
}

func TestEmitLogsSessionReconnect(t *testing.T) {
	t.Run("Should resume session events with stable ID ordering when sequences are absent", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
		events := []store.EventSummary{
			{
				ID:        "session-00000000000000000001",
				Type:      "session.created",
				Summary:   "first session event",
				Timestamp: timestamp,
			},
			{
				ID:        "session-00000000000000000002",
				Type:      "session.stopped",
				Summary:   "second session event",
				Timestamp: timestamp,
			},
		}
		writer := &bufferFlusher{}
		next := core.EmitLogs(writer, events, core.LogsCursor{
			Timestamp: timestamp,
			ID:        "session-00000000000000000001",
		})
		body := writer.String()
		if strings.Contains(body, "first session event") {
			t.Fatalf("EmitLogs replayed cursor event: %s", body)
		}
		if !strings.Contains(body, "second session event") {
			t.Fatalf("EmitLogs body = %s, want second event", body)
		}
		if next.ID != "session-00000000000000000002" {
			t.Fatalf("EmitLogs cursor = %#v, want second event ID", next)
		}
	})
}

package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/store"
)

func TestWriteSSEScrubsMemoryContext(t *testing.T) {
	t.Run("Should scrub memory context from JSON encoded SSE payloads", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		err := core.WriteSSE(writer, core.SSEMessage{
			ID:   "memory-1",
			Name: "agent_message",
			Data: map[string]string{
				"text": "before <memory-context>secret prompt bytes</memory-context> after",
			},
		})
		if err != nil {
			t.Fatalf("WriteSSE() error = %v", err)
		}
		body := writer.String()
		if strings.Contains(body, "secret prompt bytes") {
			t.Fatalf("SSE body leaked memory context: %s", body)
		}
		if !strings.Contains(body, "[memory-context redacted]") {
			t.Fatalf("SSE body = %s, want redaction marker", body)
		}
	})

	t.Run("Should scrub memory context from raw SSE payloads", func(t *testing.T) {
		t.Parallel()

		writer := &bufferFlusher{}
		err := core.WriteSSERaw(
			writer,
			"memory-raw",
			`{"text":"<memory-context>raw secret</memory-context>"}`,
			"agent_message",
		)
		if err != nil {
			t.Fatalf("WriteSSERaw() error = %v", err)
		}
		body := writer.String()
		if strings.Contains(body, "raw secret") {
			t.Fatalf("raw SSE body leaked memory context: %s", body)
		}
		if !strings.Contains(body, "[memory-context redacted]") {
			t.Fatalf("raw SSE body = %s, want redaction marker", body)
		}
	})
}

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

func TestLogEventPayloadScrubsMemoryContext(t *testing.T) {
	t.Run("Should scrub observe summaries and content before response shaping", func(t *testing.T) {
		t.Parallel()

		payload := core.LogEventPayloadFromSummary(store.EventSummary{
			ID:        "memevt-workspace-1",
			Type:      "memory.recall.executed",
			Content:   []byte(`{"text":"<memory-context>content secret</memory-context>"}`),
			Summary:   "summary <memory-context>summary secret</memory-context>",
			Timestamp: time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC),
		})
		if strings.Contains(string(payload.Content), "content secret") {
			t.Fatalf("LogEventPayload.Content leaked memory context: %s", payload.Content)
		}
		if strings.Contains(payload.Summary, "summary secret") {
			t.Fatalf("LogEventPayload.Summary leaked memory context: %s", payload.Summary)
		}
		if !strings.Contains(string(payload.Content), "[memory-context redacted]") ||
			!strings.Contains(payload.Summary, "[memory-context redacted]") {
			t.Fatalf("LogEventPayload = %#v, want redaction markers", payload)
		}
	})
}

func TestEmitLogsMemoryReconnect(t *testing.T) {
	t.Run("Should resume memory events with stable ID ordering when sequences are absent", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
		events := []store.EventSummary{
			{
				ID:        "memevt-global-00000000000000000001",
				Type:      "memory.write.committed",
				Summary:   "first memory event",
				Timestamp: timestamp,
			},
			{
				ID:        "memevt-workspace-00000000000000000002",
				Type:      "memory.recall.executed",
				Summary:   "second memory event",
				Timestamp: timestamp,
			},
		}
		writer := &bufferFlusher{}
		next := core.EmitLogs(writer, events, core.LogsCursor{
			Timestamp: timestamp,
			ID:        "memevt-global-00000000000000000001",
		})
		body := writer.String()
		if strings.Contains(body, "first memory event") {
			t.Fatalf("EmitLogs replayed cursor event: %s", body)
		}
		if !strings.Contains(body, "second memory event") {
			t.Fatalf("EmitLogs body = %s, want second event", body)
		}
		if next.ID != "memevt-workspace-00000000000000000002" {
			t.Fatalf("EmitLogs cursor = %#v, want second event ID", next)
		}
	})
}

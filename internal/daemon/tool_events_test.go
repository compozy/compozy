package daemon

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestDaemonToolEventSink(t *testing.T) {
	t.Parallel()

	t.Run("Should persist tool dispatch events as registered global summaries", func(t *testing.T) {
		t.Parallel()

		writer := &recordingToolEventSummaryStore{}
		now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
		sink := &daemonToolEventSink{
			writer: writer,
			now:    func() time.Time { return now },
		}
		event := toolspkg.ToolCallEvent{
			Kind:        toolspkg.ToolCallFailed,
			ToolID:      toolspkg.ToolIDConfigSet,
			SourceKind:  toolspkg.SourceBuiltin,
			WorkspaceID: "ws-1",
			SessionID:   "sess-1",
			AgentName:   "agent-1",
		}

		if err := sink.EmitToolEvent(t.Context(), event); err != nil {
			t.Fatalf("EmitToolEvent() error = %v", err)
		}
		if len(writer.summaries) != 1 {
			t.Fatalf("summaries = %#v, want one summary", writer.summaries)
		}
		summary := writer.summaries[0]
		if summary.Type != eventspkg.ToolCallFailed ||
			summary.Outcome != string(eventspkg.OutcomeFailure) ||
			summary.Provider != "" ||
			!summary.Timestamp.Equal(now) {
			t.Fatalf("summary = %#v, want registered failed tool event summary without synthetic provider", summary)
		}
		var stored toolspkg.ToolCallEvent
		if err := json.Unmarshal(summary.Content, &stored); err != nil {
			t.Fatalf("Unmarshal(summary.Content) error = %v", err)
		}
		if stored.Kind != event.Kind || stored.ToolID != event.ToolID || stored.SourceKind != event.SourceKind {
			t.Fatalf("stored event = %#v, want %#v", stored, event)
		}
	})
}

type recordingToolEventSummaryStore struct {
	summaries []store.EventSummary
}

func (s *recordingToolEventSummaryStore) WriteEventSummary(
	_ context.Context,
	summary store.EventSummary,
) error {
	s.summaries = append(s.summaries, summary)
	return nil
}

func (s *recordingToolEventSummaryStore) ListEventSummaries(
	_ context.Context,
	_ store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return append([]store.EventSummary(nil), s.summaries...), nil
}

package session

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func TestObservedCompactionLifecycle(t *testing.T) {
	t.Parallel()
	for index, terminalFirst := range []bool{false, true, true} {
		name := "Should persist snapshots and dispatch each lifecycle hook once"
		if terminalFirst {
			name = "Should dispatch pre before post for a terminal first snapshot"
		}
		if index == 2 {
			name = "Should repair missing lifecycle attribution without duplicating the snapshot"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var calls []hookspkg.ContextCompactionPayload
			spy := &spyHookDispatcher{
				dispatchContextPreCompactFn: func(_ context.Context, p hookspkg.ContextPreCompactPayload) (hookspkg.ContextPreCompactPayload, error) {
					calls = append(calls, p)
					return p, nil
				},
				dispatchContextPostCompactFn: func(_ context.Context, p hookspkg.ContextPostCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
					calls = append(calls, p)
					return p, nil
				},
			}
			h := newHarness(t, WithHookSet(HookSet{Compaction: spy}))
			target := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
			ctx := t.Context()
			at := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
			if err := h.manager.recordEvent(
				ctx,
				target,
				acp.AgentEvent{
					Type:      acp.EventTypeUsage,
					TurnID:    "turn",
					Timestamp: at,
					Usage:     &acp.TokenUsage{ContextUsed: new(int64(90)), ContextSize: new(int64(100))},
				},
			); err != nil {
				t.Fatal(err)
			}
			statuses := []acp.CompactionObservation{
				{CompactionID: "c1", Status: "in_progress"},
				{CompactionID: "c1", Status: "completed", Summary: "summary", Terminal: true},
			}
			if terminalFirst {
				statuses = statuses[1:]
			}
			if index == 2 {
				content, err := transcript.MarshalAgentEvent(
					acp.AgentEvent{
						Type:       acp.EventTypeCompaction,
						TurnID:     "turn",
						Timestamp:  at,
						Compaction: &statuses[0],
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				if err := target.recorderHandle().
					Record(ctx, store.SessionEvent{AgentName: target.Info().AgentName, Type: acp.EventTypeCompaction, TurnID: "turn", Timestamp: at, Content: content}); err != nil {
					t.Fatal(err)
				}
			}
			statuses = append(
				statuses,
				acp.CompactionObservation{CompactionID: "c1", Status: "completed", Summary: "summary", Terminal: true},
				acp.CompactionObservation{
					CompactionID: "c1",
					Status:       "failed",
					Summary:      "corrected",
					Error:        "failure",
					Terminal:     true,
				},
			)
			for _, observation := range statuses {
				if err := h.manager.recordEvent(
					ctx,
					target,
					acp.AgentEvent{
						Type:       acp.EventTypeCompaction,
						TurnID:     "turn",
						Timestamp:  at,
						Compaction: &observation,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			if len(calls) != 2 || calls[0].Event != hookspkg.HookContextPreCompact ||
				calls[1].Event != hookspkg.HookContextPostCompact {
				t.Fatalf("hook calls = %+v", calls)
			}
			if calls[0].Trigger != "agent" || calls[0].CompactionID != "c1" || calls[0].Status != "" ||
				calls[1].Status != "completed" ||
				calls[1].Summary != "summary" {
				t.Fatalf("hook payloads = %+v", calls)
			}
			rows, err := h.manager.Events(ctx, target.ID, store.EventQuery{Type: events.SessionCompactionFired})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("fired rows = %d", len(rows))
			}
			fired, err := transcript.UnmarshalAgentEvent(rows[0].Content)
			if err != nil {
				t.Fatal(err)
			}
			var payload CompactionFiredPayload
			if err := json.Unmarshal(fired.Raw, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.ContextUsed == nil || *payload.ContextUsed != 90 || payload.ContextSize == nil ||
				*payload.ContextSize != 100 {
				t.Fatalf("fired payload = %+v", payload)
			}
			snapshots, err := h.manager.Events(ctx, target.ID, store.EventQuery{Type: acp.EventTypeCompaction})
			if err != nil {
				t.Fatal(err)
			}
			wantSnapshots := 3
			if terminalFirst {
				wantSnapshots = 2
			}
			if len(snapshots) != wantSnapshots {
				t.Fatalf("snapshots = %d, want %d", len(snapshots), wantSnapshots)
			}
			terminals := []int64{}
			for _, row := range snapshots {
				snapshot, err := transcript.UnmarshalAgentEvent(row.Content)
				if err != nil {
					t.Fatal(err)
				}
				if snapshot.Compaction.Terminal {
					terminals = append(terminals, row.Sequence)
				}
			}
			boundary, err := h.manager.CompactionBoundary(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(terminals) != 1 || boundary == nil || !slices.Contains(terminals, *boundary) {
				t.Fatalf("terminals = %v, boundary = %v", terminals, boundary)
			}
			markers, err := h.manager.Compactions(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(markers) != 1 || markers[0].Status != "failed" {
				t.Fatalf("markers = %+v", markers)
			}
		})
	}
}

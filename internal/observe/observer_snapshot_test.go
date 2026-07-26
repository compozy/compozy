package observe

import (
	"encoding/json"
	"testing"

	"github.com/compozy/agh/internal/acp"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestOnAgentEventPersistsTranscriptSnapshotServedContent(t *testing.T) {
	t.Parallel()

	t.Run("Should persist snapshot served diagnostic content", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := newSession("sess-snapshot-served", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)

		raw := json.RawMessage(
			`{"workspace_id":"ws-observe-workspace","session_id":"sess-snapshot-served","epoch":2,` +
				`"generation":3,"reset":true,"reason":"epoch_mismatch","max_sequence":9,` +
				`"has_older":true,"next_before_sequence":4}`,
		)
		h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
			Type: eventspkg.SessionStreamSnapshotServed,
			Raw:  raw,
		})

		events, err := h.observer.QueryEvents(testutil.Context(t), store.EventSummaryQuery{
			SessionID: sess.ID,
			Type:      eventspkg.SessionStreamSnapshotServed,
		})
		if err != nil {
			t.Fatalf("QueryEvents() error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}
		if string(events[0].Content) != string(raw) {
			t.Fatalf("events[0].Content = %s, want %s", string(events[0].Content), string(raw))
		}
		if events[0].WorkspaceID != h.workspaceID {
			t.Fatalf("events[0].WorkspaceID = %q, want %q", events[0].WorkspaceID, h.workspaceID)
		}
		var content struct {
			Epoch              int64  `json:"epoch"`
			Generation         int64  `json:"generation"`
			Reset              bool   `json:"reset"`
			Reason             string `json:"reason"`
			MaxSequence        int64  `json:"max_sequence"`
			HasOlder           bool   `json:"has_older"`
			NextBeforeSequence int64  `json:"next_before_sequence"`
		}
		if err := json.Unmarshal(events[0].Content, &content); err != nil {
			t.Fatalf("json.Unmarshal(events[0].Content) error = %v", err)
		}
		if content.Epoch != 2 ||
			content.Generation != 3 ||
			!content.Reset ||
			content.Reason != "epoch_mismatch" ||
			content.MaxSequence != 9 ||
			!content.HasOlder ||
			content.NextBeforeSequence != 4 {
			t.Fatalf("snapshot served content = %#v", content)
		}
	})
}

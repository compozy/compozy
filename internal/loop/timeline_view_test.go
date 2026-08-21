package loop

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestTimelineViewContract(t *testing.T) {
	t.Parallel()
	t.Run("Should satisfy UT-011 with an exhaustive event tier map and ordered all view", func(t *testing.T) {
		t.Parallel()
		for _, value := range RunEventKindValues() {
			tier, ok := TimelineTierFor(RunEventKind(value))
			if !ok {
				t.Fatalf("event kind %q is not classified", value)
			}
			if tier != TimelineNotable && tier != TimelineActivity && tier != TimelineChatter {
				t.Fatalf("event kind %q tier = %q", value, tier)
			}
			entry, err := ProjectTimelineEvent(
				RunEvent{LoopRunID: "run-a", Seq: 1, Kind: value},
				TimelineViewAll,
			)
			if err != nil || entry == nil || entry.Title == "" || strings.Contains(entry.Title, "_") ||
				strings.EqualFold(entry.Title, strings.ReplaceAll(value, "_", " ")) {
				t.Fatalf("event kind %q entry = %#v, error = %v", value, entry, err)
			}
		}
		if len(timelineTiers) != len(RunEventKindValues()) {
			t.Fatalf("classified kinds = %d, want %d", len(timelineTiers), len(RunEventKindValues()))
		}
		events := []RunEvent{
			{LoopRunID: "run-a", Seq: 1, Kind: string(RunEventTokenTick)},
			{LoopRunID: "run-a", Seq: 2, Kind: string(RunEventChannelMsg)},
			{LoopRunID: "run-a", Seq: 3, Kind: string(RunEventNodeSucceeded)},
		}
		page, err := ProjectTimeline("run-a", events, TimelineQuery{View: TimelineViewAll})
		if err != nil {
			t.Fatalf("ProjectTimeline() error = %v", err)
		}
		if len(page.Entries) != 3 || page.Entries[0].Seq != 3 || page.Entries[1].Seq != 2 ||
			page.Entries[2].Seq != 1 {
			t.Fatalf("entries = %#v", page.Entries)
		}
	})
	t.Run("Should satisfy UT-012 with a linked fork beat and heartbeat seq equal to last", func(t *testing.T) {
		t.Parallel()
		payload, err := json.Marshal(map[string]string{"fork_run_id": "run-b"})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		events := []RunEvent{
			{
				LoopRunID: "run-a",
				Seq:       1,
				Kind:      string(RunEventRunForked),
				Payload:   payload,
			},
			{LoopRunID: "run-a", Seq: 2, Kind: string(RunEventTokenTick)},
			{LoopRunID: "run-a", Seq: 3, Kind: string(RunEventTokenTick)},
		}
		page, err := ProjectTimeline("run-a", events, TimelineQuery{View: TimelineViewAll})
		if err != nil {
			t.Fatalf("ProjectTimeline() error = %v", err)
		}
		if len(page.Entries) != 2 || page.Entries[0].Seq != 3 || page.Entries[0].FirstSeq != 2 {
			t.Fatalf("coalesced entries = %#v", page.Entries)
		}
		if page.Entries[1].Title != "Run forked to run-b" {
			t.Fatalf("fork title = %q", page.Entries[1].Title)
		}
	})
	t.Run("Should satisfy UT-013 with typed cursor and beyond-head errors", func(t *testing.T) {
		t.Parallel()
		events := []RunEvent{
			{LoopRunID: "run-a", Seq: 1, Kind: string(RunEventNodeRunning)},
		}
		cursor, err := encodeTimelineCursor(timelineCursor{
			RunID:        "run-b",
			View:         TimelineViewAll,
			FixedHeadSeq: 1,
			BeforeSeq:    1,
		})
		if err != nil {
			t.Fatalf("encodeTimelineCursor() error = %v", err)
		}
		_, err = ProjectTimeline("run-a", events, TimelineQuery{View: TimelineViewAll, Cursor: cursor})
		if !errors.Is(err, ErrTimelineBranchChanged) {
			t.Fatalf("foreign cursor error = %v", err)
		}
		_, err = ProjectTimeline("run-a", events, TimelineQuery{View: TimelineViewAll, Cursor: "%%%"})
		if !errors.Is(err, ErrInvalidTimelineCursor) {
			t.Fatalf("malformed cursor error = %v", err)
		}
		_, err = ProjectTimeline("run-a", events, TimelineQuery{View: TimelineViewAll, AfterSeq: 2})
		if !errors.Is(err, ErrTimelinePositionBeyondHead) {
			t.Fatalf("beyond-head error = %v", err)
		}
	})
	t.Run("Should pass a named snapshot fence to backward timeline reads", func(t *testing.T) {
		t.Parallel()
		store := &pagedRouteEvidenceStore{events: []RunEvent{{
			LoopRunID: "run-a", WorkspaceID: "ws-a", Seq: 1, Kind: string(RunEventNodeSucceeded),
		}}}
		service := &computedRunReadService{store: store}
		page, err := service.Timeline(t.Context(), "ws-a", "run-a", TimelineQuery{View: TimelineViewAll})
		if err != nil {
			t.Fatalf("Timeline() error = %v", err)
		}
		want := RunEventBackwardQuery{
			WorkspaceID: "ws-a", RunID: "run-a", FixedHeadSeq: 1, BeforeSeq: 2, Limit: 500,
		}
		if len(page.Entries) != 1 || len(store.queries) != 1 || store.queries[0] != want {
			t.Fatalf("timeline page/queries = %#v/%#v, want one entry and %#v", page, store.queries, want)
		}
	})
}

package loop

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestTimelineViewContract(t *testing.T) {
	t.Parallel()
	t.Run("Should satisfy UT-011 with an exhaustive event tier map and ordered all view", func(t *testing.T) {
		t.Parallel()
		for _, value := range RunEventKindValues() {
			if _, ok := TimelineTierFor(RunEventKind(value)); !ok {
				t.Fatalf("event kind %q is not classified", value)
			}
		}
		if len(timelineTiers) != len(RunEventKindValues()) {
			t.Fatalf("classified kinds = %d, want %d", len(timelineTiers), len(RunEventKindValues()))
		}
		events := []RunEvent{
			{LoopRunID: "run-a", Seq: 1, Kind: string(RunEventTokenTick)},
			{LoopRunID: "run-a", Seq: 2, Kind: string(RunEventNodeSucceeded)},
		}
		page, err := ProjectTimeline("run-a", events, TimelineQuery{View: TimelineViewAll})
		if err != nil {
			t.Fatalf("ProjectTimeline() error = %v", err)
		}
		if len(page.Entries) != 2 || page.Entries[0].Seq != 2 || page.Entries[1].Seq != 1 {
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
			{LoopRunID: "run-a", Seq: 1, Kind: string(RunEventNodeRunning), At: time.Now()},
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
}

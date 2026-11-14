package sessioncleanup

import (
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

var planNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func daysAgo(days int) *time.Time {
	at := planNow.Add(-time.Duration(days) * 24 * time.Hour)
	return &at
}

func archivedRow(id, parent string, archivedDaysAgo int) store.SessionHistoryCleanupRow {
	return store.SessionHistoryCleanupRow{
		ID:              id,
		WorkspaceID:     "ws-1",
		Name:            "Thread " + id,
		ParentSessionID: parent,
		ArchivedAt:      daysAgo(archivedDaysAgo),
	}
}

func liveRow(id, parent string) store.SessionHistoryCleanupRow {
	return store.SessionHistoryCleanupRow{ID: id, WorkspaceID: "ws-1", ParentSessionID: parent}
}

func subagentRow(id, parent string, archivedDaysAgo int) store.SessionHistoryCleanupRow {
	row := archivedRow(id, parent, archivedDaysAgo)
	row.SpawnRole = store.SubagentSpawnRole
	return row
}

func everythingRequest() Request {
	return Request{
		Scope:     Scope{ProfileID: "profile-1", WorkspaceID: "ws-1"},
		Selection: Selection{Everything: true, Children: true, Subagents: true},
	}
}

func threadByRoot(t *testing.T, plan Plan, rootID string) Thread {
	t.Helper()
	for _, thread := range plan.Threads {
		if thread.RootID == rootID {
			return thread
		}
	}
	t.Fatalf("plan has no thread rooted at %q: %+v", rootID, plan.Threads)
	return Thread{}
}

func TestBuildPlan(t *testing.T) {
	t.Parallel()

	t.Run("Should delete a whole thread deepest first with Everything", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("root", "", 10),
			archivedRow("child", "root", 10),
			subagentRow("grand-sub", "child", 10),
			subagentRow("sub", "root", 10),
		}
		plan := BuildPlan(rows, everythingRequest())
		thread := threadByRoot(t, plan, "root")
		want := []string{"grand-sub", "child", "sub", "root"}
		if !slices.Equal(thread.Delete, want) {
			t.Fatalf("Delete = %v, want %v", thread.Delete, want)
		}
		if got := (Counts{Sessions: 1, Children: 1, Subagents: 2}); plan.Counts != got {
			t.Fatalf("Counts = %+v, want %+v", plan.Counts, got)
		}
		if plan.Counts.Total() != len(thread.Delete) {
			t.Fatalf("Total = %d, want the %d planned deletes", plan.Counts.Total(), len(thread.Delete))
		}
		if len(plan.Workspaces) != 1 || plan.Workspaces[0] != (WorkspaceTotal{WorkspaceID: "ws-1", Total: 4}) {
			t.Fatalf("Workspaces = %+v", plan.Workspaces)
		}
	})

	t.Run("Should keep the main session when only children or subagents go", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("root", "", 10),
			archivedRow("child", "root", 10),
			subagentRow("sub", "root", 10),
		}
		children := everythingRequest()
		children.Selection = Selection{Children: true}
		plan := BuildPlan(rows, children)
		if got := threadByRoot(t, plan, "root").Delete; !slices.Equal(got, []string{"child"}) {
			t.Fatalf("children-only Delete = %v, want [child]", got)
		}
		subagents := everythingRequest()
		subagents.Selection = Selection{Subagents: true}
		plan = BuildPlan(rows, subagents)
		if got := threadByRoot(t, plan, "root").Delete; !slices.Equal(got, []string{"sub"}) {
			t.Fatalf("subagents-only Delete = %v, want [sub]", got)
		}
		if plan.Counts.Sessions != 0 {
			t.Fatalf("Sessions = %d, want 0 without Everything", plan.Counts.Sessions)
		}
	})

	t.Run("Should keep a live child out of its parent's thread and count it as kept", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("root", "", 10),
			liveRow("live", "root"),
			archivedRow("under-live", "live", 10),
			archivedRow("gone", "root", 10),
		}
		plan := BuildPlan(rows, everythingRequest())
		thread := threadByRoot(t, plan, "root")
		if !slices.Equal(thread.Delete, []string{"gone", "root"}) {
			t.Fatalf("Delete = %v, want [gone root]", thread.Delete)
		}
		if plan.KeptLive != 1 {
			t.Fatalf("KeptLive = %d, want 1", plan.KeptLive)
		}
		// An archived session under a live one heads its own thread in the Archived list.
		if got := threadByRoot(t, plan, "under-live").Delete; !slices.Equal(got, []string{"under-live"}) {
			t.Fatalf("under-live Delete = %v, want its own thread", got)
		}
	})

	t.Run("Should never pick an always-on agent's main session", func(t *testing.T) {
		t.Parallel()
		main := archivedRow("main", "", 10)
		main.InstanceMain = true
		rows := []store.SessionHistoryCleanupRow{main, archivedRow("child", "main", 10)}
		plan := BuildPlan(rows, everythingRequest())
		if len(plan.Threads) != 0 || plan.Counts.Total() != 0 {
			t.Fatalf("plan = %+v, want nothing picked", plan)
		}
	})

	t.Run("Should pick only threads archived strictly before the cutoff", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("old", "", 30),
			archivedRow("edge", "", 7),
			archivedRow("new", "", 2),
		}
		request := everythingRequest()
		request.ArchivedBefore = daysAgo(7)
		plan := BuildPlan(rows, request)
		roots := make([]string, 0, len(plan.Threads))
		for _, thread := range plan.Threads {
			roots = append(roots, thread.RootID)
		}
		if !slices.Equal(roots, []string{"old"}) {
			t.Fatalf("picked roots = %v, want [old]", roots)
		}
	})

	t.Run("Should narrow the picked threads to RootIDs", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{archivedRow("a", "", 10), archivedRow("b", "", 10)}
		request := everythingRequest()
		request.RootIDs = []string{" b "}
		plan := BuildPlan(rows, request)
		if len(plan.Threads) != 1 || plan.Threads[0].RootID != "b" {
			t.Fatalf("Threads = %+v, want only b", plan.Threads)
		}
	})

	t.Run("Should treat continued and forked sessions as threads of their own", func(t *testing.T) {
		t.Parallel()
		continued := archivedRow("continued", "root", 10)
		continued.LineageKind = store.LineageKindContinue
		forked := archivedRow("forked", "root", 10)
		forked.LineageKind = store.LineageKindFork
		rows := []store.SessionHistoryCleanupRow{archivedRow("root", "", 10), continued, forked}
		request := everythingRequest()
		request.Selection = Selection{Children: true}
		plan := BuildPlan(rows, request)
		if plan.Counts.Total() != 0 {
			t.Fatalf("children-only Counts = %+v, want none: continue and fork are not children", plan.Counts)
		}
		plan = BuildPlan(rows, everythingRequest())
		if len(plan.Threads) != 3 {
			t.Fatalf("Threads = %d, want 3 separate threads", len(plan.Threads))
		}
	})

	t.Run("Should report each check's total beside the selection", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionHistoryCleanupRow{
			archivedRow("root", "", 10),
			archivedRow("child", "root", 10),
			subagentRow("sub-1", "root", 10),
			subagentRow("sub-2", "child", 10),
		}
		request := everythingRequest()
		request.Selection = Selection{Subagents: true}
		plan := BuildPlan(rows, request)
		want := ChoiceTotals{Everything: 4, Children: 2, Subagents: 2}
		if plan.Choices != want {
			t.Fatalf("Choices = %+v, want %+v", plan.Choices, want)
		}
		if plan.Counts.Total() != plan.Choices.Subagents {
			t.Fatalf("Total = %d, want the subagents choice %d", plan.Counts.Total(), plan.Choices.Subagents)
		}
	})
}

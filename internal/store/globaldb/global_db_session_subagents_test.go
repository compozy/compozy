package globaldb

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/store"
)

func subagentFixture(t *testing.T) (*GlobalDB, string, string, time.Time) {
	t.Helper()
	db := openTestGlobalDB(t)
	workspace := registerWorkspaceForGlobalTests(t, db, "subagents", filepath.Join(t.TempDir(), "workspace"))
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	registerSubagentSession(t, db, workspace, "parent", "", "", now)
	return db, workspace, "parent", now
}
func registerSubagentSession(t *testing.T, db *GlobalDB, workspace, id, parent, role string, now time.Time) {
	t.Helper()
	info := store.SessionInfo{
		ID:            id,
		ProfileID:     store.DefaultProfileID,
		AgentName:     "coder",
		Provider:      "claude",
		WorkspaceID:   workspace,
		RuntimeStatus: store.SessionRuntimeUnbound,
		SessionType:   defaultSessionType,
		State:         "stopped",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if parent != "" {
		info.Lineage = &store.SessionLineage{
			Kind:            store.LineageKindSpawn,
			ParentSessionID: parent,
			RootSessionID:   "parent",
			SpawnDepth:      1,
			SpawnRole:       role,
		}
	}
	if err := db.RegisterSession(t.Context(), info); err != nil {
		t.Fatal(err)
	}
}

func reserveSubagent(
	t *testing.T,
	s store.SubagentStore,
	workspace, parent, id string,
	now time.Time,
) store.SessionSubagent {
	t.Helper()
	row, created, err := s.ReserveSubagent(
		t.Context(),
		store.SessionSubagent{
			ID:                 id,
			WorkspaceID:        workspace,
			ParentSessionID:    parent,
			ParentTurnID:       "turn",
			Origin:             store.SubagentOriginDelegated,
			IdempotencyKey:     id,
			RequestFingerprint: "fingerprint",
			Title:              id,
			Depth:              1,
			WakePolicy:         store.SubagentWakePolicySettledOnly,
			PendingTask:        new("perform task"),
			CreatedAt:          now,
			UpdatedAt:          now,
		},
	)
	if err != nil || !created {
		t.Fatalf("reserve %s = %#v, %v, %v", id, row, created, err)
	}
	return row
}
func finalizeSubagent(t *testing.T, s store.SubagentStore, id string, now time.Time) {
	t.Helper()
	_, changed, err := s.FinalizeSubagent(
		t.Context(),
		store.SubagentFinalize{ID: id, Status: store.SubagentStatusCompleted, Result: new("done"), SettledAt: now},
	)
	if err != nil || !changed {
		t.Fatalf("finalize %s = %v, %v", id, changed, err)
	}
}
func TestGlobalDBSubagents(t *testing.T) {
	t.Run("Should reserve idempotently and preserve first prompt recovery until admission", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		var s store.SubagentStore = db.SessionRepo
		row := reserveSubagent(t, s, workspace, parent, "one", now)
		replay, created, err := s.ReserveSubagent(ctx, row)
		if err != nil || created || replay.PendingTask == nil || *replay.PendingTask != "perform task" {
			t.Fatalf("replay=%#v created=%v err=%v", replay, created, err)
		}
		row.RequestFingerprint = "different"
		if _, _, err = s.ReserveSubagent(ctx, row); !errors.Is(err, store.ErrSubagentIdempotencyConflict) {
			t.Fatalf("conflict=%v", err)
		}
		if _, err = s.GetSubagent(ctx, "foreign", row.ID); !errors.Is(err, store.ErrSubagentNotFound) {
			t.Fatalf("scope=%v", err)
		}
		stale, err := s.ListStaleReserved(ctx, now.Add(time.Second))
		if err != nil || len(stale) != 1 {
			t.Fatalf("stale=%#v %v", stale, err)
		}
		stale, err = s.ListStaleReserved(ctx, now)
		if err != nil || len(stale) != 0 {
			t.Fatalf("stale boundary=%#v %v", stale, err)
		}
		registerSubagentSession(t, db, workspace, "child", parent, store.SubagentSpawnRole, now)
		linked, err := s.LinkChild(ctx, row.ID, "child", now)
		if err != nil || linked.Status != store.SubagentStatusRunning {
			t.Fatalf("linked=%#v %v", linked, err)
		}
		byChild, err := s.GetSubagentByChild(ctx, "child")
		if err != nil || byChild.ID != row.ID {
			t.Fatalf("by child=%#v %v", byChild, err)
		}
		if err = s.DeleteReserved(ctx, row.ID); !errors.Is(err, store.ErrSubagentNotFound) {
			t.Fatalf("delete linked=%v", err)
		}
		if err = s.MarkFirstPromptAdmitted(ctx, row.ID); err != nil {
			t.Fatal(err)
		}
		if err = s.MarkFirstPromptAdmitted(ctx, row.ID); err != nil {
			t.Fatal(err)
		}
		linked, err = s.GetSubagent(ctx, workspace, row.ID)
		if err != nil || linked.PendingTask != nil {
			t.Fatalf("admitted=%#v %v", linked, err)
		}
		if err = s.UpdateProgress(ctx, row.ID, "tool running", now); err != nil {
			t.Fatal(err)
		}
		live, err := s.ListUnfinalizedDelegated(ctx)
		if err != nil || len(live) != 1 || live[0].Progress != "tool running" {
			t.Fatalf("live=%#v %v", live, err)
		}
		finalizeSubagent(t, s, row.ID, now)
		final, changed, err := s.FinalizeSubagent(
			ctx,
			store.SubagentFinalize{ID: row.ID, Status: store.SubagentStatusFailed, Error: new("later")},
		)
		if err != nil || changed || final.Status != store.SubagentStatusCompleted || final.Error != nil {
			t.Fatalf("final=%#v %v %v", final, changed, err)
		}
		if _, _, err = s.FinalizeSubagent(
			ctx,
			store.SubagentFinalize{ID: row.ID, Status: store.SubagentStatusRunning},
		); err == nil {
			t.Fatal("accepted nonterminal finalization")
		}
		reserveSubagent(t, s, workspace, parent, "denied", now)
		if err = s.DeleteReserved(ctx, "denied"); err != nil {
			t.Fatal(err)
		}
		if _, err = s.GetSubagent(ctx, workspace, "denied"); !errors.Is(err, store.ErrSubagentNotFound) {
			t.Fatalf("deleted=%v", err)
		}
		if err = s.DeleteReserved(ctx, "denied"); !errors.Is(err, store.ErrSubagentNotFound) {
			t.Fatalf("delete absent=%v", err)
		}
		if err = s.MarkFirstPromptAdmitted(ctx, "missing"); !errors.Is(err, store.ErrSubagentNotFound) {
			t.Fatalf("admit absent=%v", err)
		}
		registerSubagentSession(t, db, workspace, "orphan", parent, store.SubagentSpawnRole, now)
		orphans, err := s.ListOrphanSubagentSessions(ctx)
		if err != nil || !slices.Equal(orphans, []string{"orphan"}) {
			t.Fatalf("orphans=%v %v", orphans, err)
		}
	})
	// UT-056: one open batch; cancellation returns rows to a successor batch.
	t.Run("Should join one open wake and settle or cancel claimed rows atomically", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		var s store.SubagentStore = db.SessionRepo
		for _, id := range []string{"a", "b", "c"} {
			reserveSubagent(t, s, workspace, parent, id, now)
			finalizeSubagent(t, s, id, now)
		}
		wake, err := s.OpenOrJoinWake(ctx, parent, []string{"a"}, "wake-1")
		if err != nil {
			t.Fatal(err)
		}
		joined, err := s.OpenOrJoinWake(ctx, parent, []string{"b"}, "unused")
		if err != nil || joined.WakeMessageID != wake.WakeMessageID {
			t.Fatalf("join=%#v %v", joined, err)
		}
		if _, err = db.db.ExecContext(
			ctx,
			`INSERT INTO session_subagent_wakes SELECT 'duplicate',workspace_id,parent_session_id,state,route,input_entry_id,steer_requeued,created_at,updated_at FROM session_subagent_wakes WHERE wake_message_id='wake-1'`,
		); err == nil {
			t.Fatal("accepted second open wake")
		}
		if err = s.SetWakeInput(ctx, "wake-1", "steer", ""); err != nil {
			t.Fatal(err)
		}
		if err = s.MarkWakeSteerRequeued(ctx, "wake-1"); err != nil {
			t.Fatal(err)
		}
		if err = s.SetWakeInput(ctx, "wake-1", "queue", "input-1"); err != nil {
			t.Fatal(err)
		}
		w, items, err := s.GetWake(ctx, "wake-1")
		if err != nil || len(items) != 2 || !w.SteerRequeued || w.InputEntryID != "input-1" {
			t.Fatalf("wake=%#v %v %v", w, items, err)
		}
		if err = s.MarkWakeDispatched(ctx, "wake-1"); err != nil {
			t.Fatal(err)
		}
		if err = s.SetPending(ctx, []string{"c"}); err != nil {
			t.Fatal(err)
		}
		canceled, err := s.SettleWake(ctx, "wake-1", true)
		if err != nil || len(canceled) != 2 {
			t.Fatalf("cancel=%v %v", canceled, err)
		}
		for _, r := range canceled {
			if r.Delivery != store.SubagentDeliveryPending || r.WakeMessageID != nil {
				t.Fatalf("canceled row=%#v", r)
			}
		}
		pending, err := s.ListPending(ctx)
		if err != nil || len(pending) != 3 {
			t.Fatalf("pending=%v %v", pending, err)
		}
		successor, err := s.OpenOrJoinWake(ctx, parent, []string{"a", "b", "c"}, "wake-2")
		if err != nil || successor.WakeMessageID != "wake-2" {
			t.Fatalf("successor=%#v %v", successor, err)
		}
		settled, err := s.SettleWake(ctx, "wake-2", false)
		if err != nil || len(settled) != 3 {
			t.Fatalf("settled=%v %v", settled, err)
		}
		for _, r := range settled {
			if r.Delivery != store.SubagentDeliveryDelivered {
				t.Fatalf("settled row=%#v", r)
			}
		}
		repeated, err := s.SettleWake(ctx, "wake-2", true)
		if err != nil || len(repeated) != 0 {
			t.Fatalf("repeat=%v %v", repeated, err)
		}
		if err = s.SetPending(ctx, []string{"a"}); err != nil {
			t.Fatal(err)
		}
		pending, err = s.ListPending(ctx)
		if err != nil || len(pending) != 0 {
			t.Fatalf("terminal delivery reopened=%v %v", pending, err)
		}
	})
	t.Run("Should acknowledge terminal rows and dispose only selected nonfinal deliveries", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		var s store.SubagentStore = db.SessionRepo
		for _, id := range []string{"a", "b", "live"} {
			reserveSubagent(t, s, workspace, parent, id, now)
		}
		unchanged, w, err := s.Acknowledge(ctx, "live", "read")
		if err != nil || w != nil || unchanged.Delivery != store.SubagentDeliveryNone {
			t.Fatalf("ack live=%#v %v %v", unchanged, w, err)
		}
		for _, id := range []string{"a", "b"} {
			finalizeSubagent(t, s, id, now)
		}
		if _, err = s.OpenOrJoinWake(ctx, parent, []string{"a", "b"}, "wake"); err != nil {
			t.Fatal(err)
		}
		ack, w, err := s.Acknowledge(ctx, "a", "read")
		if err != nil || w == nil || w.State != "open" || ack.WakeMessageID != nil || ack.AcknowledgedTurnID != "read" {
			t.Fatalf("ack=%#v %#v %v", ack, w, err)
		}
		_, members, err := s.GetWake(ctx, "wake")
		if err != nil || len(members) != 1 || members[0].ID != "b" {
			t.Fatalf("members=%v %v", members, err)
		}
		_, w, err = s.Acknowledge(ctx, "b", "read")
		if err != nil || w == nil || w.State != "canceled" {
			t.Fatalf("empty wake=%#v %v", w, err)
		}
		upgraded, err := s.UpgradeWakePolicy(ctx, "live")
		if err != nil || upgraded.WakePolicy != store.SubagentWakePolicyAlways {
			t.Fatalf("upgrade=%#v %v", upgraded, err)
		}
		if got, err := s.Dispose(
			ctx,
			store.SubagentDisposeFilter{ParentSessionID: parent, ParentTurnID: "other"},
		); err != nil ||
			len(got) != 0 {
			t.Fatalf("foreign turn disposal=%v %v", got, err)
		}
		disposed, err := s.Dispose(
			ctx,
			store.SubagentDisposeFilter{ParentSessionID: parent, IDs: []string{"live", "a"}},
		)
		if err != nil || len(disposed) != 1 || disposed[0].ID != "live" ||
			disposed[0].Delivery != store.SubagentDeliveryDisposed {
			t.Fatalf("dispose=%v %v", disposed, err)
		}
		open, err := s.ListOpenWakes(ctx)
		if err != nil || len(open) != 0 {
			t.Fatalf("open=%v %v", open, err)
		}
	})
	t.Run("Should paginate newest first with query bound opaque cursors", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		var s store.SubagentStore = db.SessionRepo
		for i := range 3 {
			reserveSubagent(t, s, workspace, parent, fmt.Sprintf("row-%d", i), now.Add(time.Duration(i)*time.Second))
		}
		query := store.SubagentListQuery{
			WorkspaceID:     workspace,
			ParentSessionID: parent,
			Limit:           2,
			Origins:         []string{"delegated"},
			Statuses:        []string{"queued"},
		}
		first, err := s.ListSubagents(ctx, query)
		if err != nil || len(first.Items) != 2 || first.Items[0].ID != "row-2" || first.NextCursor == "" {
			t.Fatalf("first=%#v %v", first, err)
		}
		query.Cursor = first.NextCursor
		second, err := s.ListSubagents(ctx, query)
		if err != nil || len(second.Items) != 1 || second.Items[0].ID != "row-0" || second.NextCursor != "" {
			t.Fatalf("second=%#v %v", second, err)
		}
		query.WorkspaceID = "foreign"
		if _, err = s.ListSubagents(ctx, query); !errors.Is(err, listcursor.ErrInvalid) {
			t.Fatalf("cursor scope=%v", err)
		}
	})
}

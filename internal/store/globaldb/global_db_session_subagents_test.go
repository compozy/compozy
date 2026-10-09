package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
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
	// M6, m16: repository reads stay scoped and bounded.
	t.Run("Should reject unscoped lists and cap pages while looking up exact ids", func(t *testing.T) {
		t.Parallel()
		db, workspace, parent, now := subagentFixture(t)
		var port store.SubagentStore = db.SessionRepo
		ctx := t.Context()
		if _, err := port.ListSubagents(ctx, store.SubagentListQuery{}); err == nil {
			t.Fatal("accepted unscoped list")
		}
		for i := range 201 {
			reserveSubagent(
				t,
				port,
				workspace,
				parent,
				fmt.Sprintf("bounded-%03d", i),
				now.Add(time.Duration(i)*time.Second),
			)
		}
		query := store.SubagentListQuery{ParentSessionID: parent, Limit: 1000}
		page, err := port.ListSubagents(ctx, query)
		if err != nil || len(page.Items) != 200 || page.NextCursor == "" {
			t.Fatalf("page=%#v err=%v", page, err)
		}
		query.Cursor = page.NextCursor
		tail, err := port.ListSubagents(ctx, query)
		if err != nil || len(tail.Items) != 1 || tail.Items[0].ID != "bounded-000" {
			t.Fatalf("tail=%#v err=%v", tail, err)
		}
		row, err := port.GetSubagentByID(ctx, "bounded-000")
		if err != nil || row.ParentSessionID != parent {
			t.Fatalf("row=%#v err=%v", row, err)
		}
		if _, err = port.GetSubagentByID(ctx, "missing"); !errors.Is(err, store.ErrSubagentNotFound) {
			t.Fatal(err)
		}
	})
	t.Run("Should exclude native reservations and report a missing wake parent", func(t *testing.T) {
		t.Parallel()
		db, workspace, parent, now := subagentFixture(t)
		var port store.SubagentStore = db.SessionRepo
		native := reserveSubagent(t, port, workspace, parent, "delegated", now)
		native.ID, native.IdempotencyKey, native.Origin = "native", "native", store.SubagentOriginProviderNative
		if _, _, err := port.ReserveSubagent(t.Context(), native); err != nil {
			t.Fatal(err)
		}
		rows, err := port.ListStaleReserved(t.Context(), now.Add(time.Second))
		if err != nil || len(rows) != 1 || rows[0].ID != "delegated" {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
		if _, err = port.OpenOrJoinWake(
			t.Context(),
			"missing",
			nil,
			"absent-wake",
		); !errors.Is(
			err,
			store.ErrSessionNotFound,
		) {
			t.Fatal(err)
		}
	})
	// M7, UT-056: wake reads never wait for a writer.
	t.Run("Should read filtered wakes without the writer", func(t *testing.T) {
		t.Parallel()
		db, workspace, parent, now := subagentFixture(t)
		var port store.SubagentStore = db.SessionRepo
		ctx := t.Context()
		registerSubagentSession(t, db, workspace, "other-parent", "", "", now)
		for _, id := range []string{"first", "second"} {
			reserveSubagent(t, port, workspace, parent, id, now)
			finalizeSubagent(t, port, id, now)
		}
		if _, err := port.OpenOrJoinWake(ctx, parent, []string{"first"}, "wake-first"); err != nil {
			t.Fatal(err)
		}
		if err := port.MarkWakeDispatched(ctx, "wake-first"); err != nil {
			t.Fatal(err)
		}
		if _, err := port.OpenOrJoinWake(ctx, parent, []string{"second"}, "wake-second"); err != nil {
			t.Fatal(err)
		}
		if _, err := port.OpenOrJoinWake(ctx, "other-parent", nil, "wake-other"); err != nil {
			t.Fatal(err)
		}
		writer, err := db.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := writer.Close(); err != nil {
				t.Error(err)
			}
		}()
		if _, err = writer.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := writer.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}()
		readCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		wake, rows, err := port.GetWake(readCtx, "wake-first")
		if err != nil || wake.State != store.SubagentWakeStateDispatched || len(rows) != 1 || rows[0].ID != "first" {
			t.Fatalf("wake=%#v rows=%v err=%v", wake, rows, err)
		}
		wakes, err := port.ListWakesByParent(readCtx, parent, []string{store.SubagentWakeStateOpen})
		if err != nil || len(wakes) != 1 || wakes[0].WakeMessageID != "wake-second" {
			t.Fatalf("wakes=%v err=%v", wakes, err)
		}
	})

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
		registerSubagentSession(t, db, workspace, "stopped-orphan", parent, store.SubagentSpawnRole, now)
		registerSubagentSession(t, db, workspace, "orphan", parent, store.SubagentSpawnRole, now)
		if _, err := db.db.ExecContext(ctx, "UPDATE sessions SET state = 'active' WHERE id = 'orphan'"); err != nil {
			t.Fatal(err)
		}
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

func TestGlobalDBSubagentStateUpdates(t *testing.T) {
	t.Run("Should update only nonterminal state and preserve unchanged timestamps", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		var port store.SubagentStore = db.SessionRepo
		original := reserveSubagent(t, port, workspace, parent, "state", now)
		row, changed, err := port.UpdateSubagentState(
			ctx,
			original.ID,
			original.Status,
			original.WorkState,
			now.Add(time.Second),
		)
		if err != nil || changed || !row.UpdatedAt.Equal(original.UpdatedAt) {
			t.Fatalf("unchanged = %#v, %v, %v", row, changed, err)
		}
		transitions := []struct{ status, work string }{
			{store.SubagentStatusRunning, store.SubagentWorkStateWorking},
			{store.SubagentStatusWaiting, store.SubagentWorkStateWaitingForChildren},
			{store.SubagentStatusWaiting, store.SubagentWorkStateResultAvailable},
			{store.SubagentStatusQueued, store.SubagentWorkStateWorking},
		}
		for i, transition := range transitions {
			at := now.Add(time.Duration(i+2) * time.Second)
			row, changed, err = port.UpdateSubagentState(ctx, original.ID, transition.status, transition.work, at)
			if err != nil || !changed || row.Status != transition.status || row.WorkState != transition.work ||
				!row.UpdatedAt.Equal(at) {
				t.Fatalf("transition %d = %#v, %v, %v", i, row, changed, err)
			}
			if row.PendingTask == nil || *row.PendingTask != *original.PendingTask ||
				row.ParentSessionID != original.ParentSessionID {
				t.Fatalf("transition changed reservation = %#v", row)
			}
		}
		for _, invalid := range []struct{ status, work string }{
			{store.SubagentStatusCompleted, store.SubagentWorkStateResultAvailable},
			{"invalid", store.SubagentWorkStateWorking},
			{store.SubagentStatusRunning, "invalid"},
		} {
			if _, _, err := port.UpdateSubagentState(ctx, original.ID, invalid.status, invalid.work, now); err == nil {
				t.Fatalf("accepted invalid transition = %#v", invalid)
			}
		}
		preserved, err := port.GetSubagent(ctx, workspace, original.ID)
		if err != nil || preserved.Status != row.Status || preserved.WorkState != row.WorkState ||
			!preserved.UpdatedAt.Equal(row.UpdatedAt) {
			t.Fatalf("invalid transition mutated row = %#v, %v", preserved, err)
		}
		if _, _, err := port.UpdateSubagentState(
			ctx,
			"missing",
			store.SubagentStatusRunning,
			store.SubagentWorkStateWorking,
			now,
		); !errors.Is(
			err,
			store.ErrSubagentNotFound,
		) {
			t.Fatalf("missing = %v", err)
		}
	})
	for _, status := range []string{store.SubagentStatusCompleted, store.SubagentStatusFailed, store.SubagentStatusCanceled, store.SubagentStatusInterrupted} {
		t.Run("Should preserve terminal "+status+" state", func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db, workspace, parent, now := subagentFixture(t)
			var port store.SubagentStore = db.SessionRepo
			reserveSubagent(t, port, workspace, parent, "terminal", now)
			final, changed, err := port.FinalizeSubagent(
				ctx,
				store.SubagentFinalize{
					ID:        "terminal",
					Status:    status,
					WorkState: store.SubagentWorkStateResultAvailable,
					Result:    new("result"),
					SettledAt: now,
				},
			)
			if err != nil || !changed {
				t.Fatalf("final=%#v %v %v", final, changed, err)
			}
			row, changed, err := port.UpdateSubagentState(
				ctx,
				"terminal",
				store.SubagentStatusRunning,
				store.SubagentWorkStateWorking,
				now.Add(time.Hour),
			)
			if err != nil || changed || !reflect.DeepEqual(row, final) {
				t.Fatalf("terminal moved=%#v %v %v", row, changed, err)
			}
		})
	}
}

func TestGlobalDBSubagentWakeInputRewrite(t *testing.T) {
	cases := []struct {
		name, wakeState, inputState string
		wantError                   bool
	}{
		{"Should rewrite an open queued wake in place", "open", "queued", false},
		{"Should reject a dispatched wake", "dispatched", "queued", true},
		{"Should reject a settled wake", "settled", "queued", true},
		{"Should reject a canceled wake", "canceled", "queued", true},
		{"Should reject an input already claimed by the pump", "open", "dispatching", true},
		{"Should reject a sent input", "open", "sent", true},
		{"Should reject a failed input", "open", "failed", true},
		{"Should reject a canceled input", "open", "canceled", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db, workspace, parent, now := subagentFixture(t)
			var port store.SubagentStore = db.SessionRepo
			reserveSubagent(t, port, workspace, parent, "rewrite", now)
			finalizeSubagent(t, port, "rewrite", now)
			if _, err := port.OpenOrJoinWake(ctx, parent, []string{"rewrite"}, "wake"); err != nil {
				t.Fatal(err)
			}
			before, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID:        "wake-input",
				SessionID: parent,
				MessageID: "wake",
				TurnID:    "wake-turn",
				Priority:  1,
				Text:      "original",
				QueueCap:  10,
				Now:       now,
				OwnerKind: store.SessionInputOwnerSynthetic,
				SyntheticPrompt: &store.SessionInputSyntheticPrompt{
					RunID:    "run",
					Delivery: "followup",
					Metadata: json.RawMessage(`{"subagent_ids":["a","b"]}`),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err = port.SetWakeInput(ctx, "wake", "queue", before.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = db.db.ExecContext(
				ctx,
				`UPDATE session_subagent_wakes SET state=? WHERE wake_message_id='wake'`,
				tc.wakeState,
			); err != nil {
				t.Fatal(err)
			}
			if _, err = db.db.ExecContext(
				ctx,
				`UPDATE session_input_queue SET status=? WHERE id=?`,
				tc.inputState,
				before.ID,
			); err != nil {
				t.Fatal(err)
			}
			before.Status = tc.inputState
			metadata := json.RawMessage(`{"subagent_ids":["b"],"kind":"subagent_wake"}`)
			err = port.RewriteSubagentWakeInput(ctx, "wake", "remaining result", metadata)
			if (err != nil) != tc.wantError {
				t.Fatalf("rewrite error=%v wantError=%v", err, tc.wantError)
			}
			after, err := db.GetSessionInputQueueEntry(ctx, parent, before.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.wantError {
				before.Text = "remaining result"
				before.SyntheticPrompt.Metadata = metadata
				before.UpdatedAt = after.UpdatedAt
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("rewritten input=%#v want=%#v", after, before)
			}
			if !tc.wantError {
				if err := port.RewriteSubagentWakeInput(ctx, "wake", "invalid", json.RawMessage(`{`)); err == nil {
					t.Fatal("accepted invalid metadata")
				}
				preserved, err := db.GetSessionInputQueueEntry(ctx, parent, before.ID)
				if err != nil || !reflect.DeepEqual(preserved, after) {
					t.Fatalf("invalid rewrite mutated input=%#v %v", preserved, err)
				}
				if err := port.RewriteSubagentWakeInput(
					ctx,
					"missing",
					"missing",
					metadata,
				); !errors.Is(
					err,
					store.ErrSubagentWakeNotFound,
				) {
					t.Fatalf("missing wake=%v", err)
				}
			}
		})
	}
}

// Invariant: wake failure attempts survive successor batches and abandon only claimed results at three failures.
// Owner: globaldb transactions; canonical subagent persistence suite.
func TestGlobalDBSubagentWakeFailures(t *testing.T) {
	t.Run("Should carry attempts across successors and dispose after three failures", func(t *testing.T) {
		t.Parallel()
		db, workspace, parent, now := subagentFixture(t)
		s := db.SessionRepo
		row := reserveSubagent(t, s, workspace, parent, "retry", now)
		finalizeSubagent(t, s, row.ID, now)
		for attempt := 1; attempt <= 3; attempt++ {
			id := fmt.Sprintf("wake-%d", attempt)
			wake, err := s.OpenOrJoinWake(t.Context(), parent, []string{row.ID}, id)
			if err != nil || wake.Attempts != attempt-1 {
				t.Fatal(wake, err)
			}
			if err := s.MarkWakeDispatched(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			wake, rows, err := s.FailWake(t.Context(), id)
			want := store.SubagentDeliveryPending
			if attempt == 3 {
				want = store.SubagentDeliveryDisposed
			}
			if err != nil || wake.Attempts != attempt || len(rows) != 1 || rows[0].Delivery != want {
				t.Fatal(wake, rows, err)
			}
			// Reopen real SQLite between attempts: retry history must survive daemon replacement.
			path := db.Path()
			if err := db.Close(t.Context()); err != nil {
				t.Fatal(err)
			}
			db = openGlobalDBForTest(t, path)
			s = db.SessionRepo
			repeated, again, err := s.FailWake(t.Context(), id)
			if err != nil || repeated.Attempts != attempt || len(again) != 0 {
				t.Fatal(repeated, again, err)
			}
		}
		pending, err := s.ListPending(t.Context())
		if err != nil || len(pending) != 0 {
			t.Fatal(pending, err)
		}
	})
	t.Run("Should retain failed attempt history through cancellation", func(t *testing.T) {
		t.Parallel()
		db, ws, parent, now := subagentFixture(t)
		s := db.SessionRepo
		row := reserveSubagent(t, s, ws, parent, "retry", now)
		finalizeSubagent(t, s, row.ID, now)
		if _, err := s.OpenOrJoinWake(t.Context(), parent, []string{row.ID}, "failed"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.FailWake(t.Context(), "failed"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.OpenOrJoinWake(t.Context(), parent, []string{row.ID}, "cancel"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SettleWake(t.Context(), "cancel", true); err != nil {
			t.Fatal(err)
		}
		wake, err := s.OpenOrJoinWake(t.Context(), parent, []string{row.ID}, "successor")
		if err != nil || wake.Attempts != 1 {
			t.Fatal(wake, err)
		}
	})
}

// Invariant: native recovery selects only unfinished provider-owned work without crossing delivery ownership.
// Owner: globaldb read and transition boundary.
func TestGlobalDBSubagentNativeRecovery(t *testing.T) {
	t.Run("Should return only unfinalized native rows and preserve no delivery on dispose", func(t *testing.T) {
		t.Parallel()
		db, ws, parent, now := subagentFixture(t)
		s := db.SessionRepo
		reserveSubagent(t, s, ws, parent, "delegated", now)
		for _, id := range []string{"native-live", "native-done"} {
			row := store.SessionSubagent{
				ID:                 id,
				WorkspaceID:        ws,
				ParentSessionID:    parent,
				ParentTurnID:       "turn",
				Origin:             store.SubagentOriginProviderNative,
				ProviderToolCallID: id,
				IdempotencyKey:     id,
				RequestFingerprint: id,
				Title:              id,
				Depth:              1,
				Status:             store.SubagentStatusRunning,
				WakePolicy:         store.SubagentWakePolicySettledOnly,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			if _, _, err := s.ReserveSubagent(t.Context(), row); err != nil {
				t.Fatal(err)
			}
		}
		finalizeSubagent(t, s, "native-done", now)
		rows, err := s.ListUnfinalizedNative(t.Context())
		if err != nil || len(rows) != 1 || rows[0].ID != "native-live" {
			t.Fatal(rows, err)
		}
		if _, err := s.Dispose(t.Context(), store.SubagentDisposeFilter{ParentSessionID: parent}); err != nil {
			t.Fatal(err)
		}
		row, err := s.GetSubagent(t.Context(), ws, "native-live")
		if err != nil || row.Delivery != store.SubagentDeliveryNone {
			t.Fatal(row, err)
		}
	})
}

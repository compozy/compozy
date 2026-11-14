package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/compozy/compozy/internal/store"
)

// staleChildSubagents answers the first row read from a snapshot taken before a
// delegation linked its child, then falls through to the live store.
type staleChildSubagents struct {
	*memorySubagents
	stale store.SessionSubagent
}

func (d *staleChildSubagents) GetSubagent(
	ctx context.Context,
	workspaceID, id string,
) (store.SessionSubagent, error) {
	if d.stale.ID != "" {
		stale := d.stale
		d.stale = store.SessionSubagent{}
		return stale, nil
	}
	return d.memorySubagents.GetSubagent(ctx, workspaceID, id)
}

// Invariant: deleting a delegated subagent removes its child session and then its roster row, and
// the settlement the child deletion triggers never finalizes or hooks the row on its way out.
// Owner: session subagent delete suite (feature: subagent context menu).
func TestSubagentDelete(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should delete the child session then the row and publish its removal",
		func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				s, db, runtime := newSubagentTestService(t)
				hook := &subagentTestHook{}
				s.settled = hook
				updates, cancel, err := s.SubscribeSubagentUpdates(t.Context(), "parent")
				if err != nil {
					t.Fatal(err)
				}
				defer cancel()
				row := requireSubagent(t, s, subagentTestRequest())
				child := *row.ChildSessionID
				finalizes, publishes := db.finalizeCalls, len(runtime.publishes)
				runtime.deleteSettles = func(ctx context.Context, id string) error {
					if id != child {
						t.Errorf("deleted child = %s, want %s", id, child)
					}
					if _, ok := db.rows[row.ID]; !ok {
						t.Error("roster row was removed before its child session")
					}
					return s.OnChildSettled(ctx, id)
				}
				if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err != nil {
					t.Fatal(err)
				}
				collected := make([]SubagentUpdate, 0, 2)
				for {
					synctest.Wait()
					select {
					case update := <-updates:
						collected = append(collected, update)
						continue
					default:
					}
					break
				}
				if _, ok := db.rows[row.ID]; ok {
					t.Fatal("deleted row is still readable")
				}
				if len(runtime.deleted) != 1 || runtime.deleted[0] != child {
					t.Fatalf("deleted sessions = %v, want [%s]", runtime.deleted, child)
				}
				if db.finalizeCalls != finalizes {
					t.Fatalf(
						"child settlement finalized %d row(s) during deletion",
						db.finalizeCalls-finalizes,
					)
				}
				if hook.calls != 0 {
					t.Fatalf("settled hook dispatched %d time(s) for a deleted row", hook.calls)
				}
				if len(runtime.publishes) != publishes+1 ||
					runtime.publishes[publishes] != "parent" {
					t.Fatalf("parent catalog publishes = %v", runtime.publishes)
				}
				removals := make([]SubagentUpdate, 0, len(collected))
				for _, update := range collected {
					if update.Removed {
						removals = append(removals, update)
					}
				}
				if len(removals) != 1 || removals[0].Subagent.ID != row.ID ||
					removals[0].ParentSessionID != "parent" {
					t.Fatalf("removal updates = %+v (all=%+v)", removals, collected)
				}
				if s.childSettlementSuppressed(child) {
					t.Fatal("settlement suppression outlived the deletion")
				}
			})
		},
	)
	t.Run("Should refuse a provider-native row and keep it", func(t *testing.T) {
		t.Parallel()
		s, db, _ := newSubagentTestService(t)
		db.rows["native"] = store.SessionSubagent{
			ID: "native", WorkspaceID: "ws", ParentSessionID: "parent",
			Origin: store.SubagentOriginProviderNative, Status: store.SubagentStatusRunning,
		}
		err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", "native")
		if !errors.Is(err, ErrSubagentNotDeletable) {
			t.Fatalf("native delete = %v", err)
		}
		if _, ok := db.rows["native"]; !ok {
			t.Fatal("refused delete removed the provider-native row")
		}
	})
	t.Run("Should keep the row when its child session cannot be deleted", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		child := *row.ChildSessionID
		failure := errors.New("session: stop could not be verified")
		runtime.errorDelete = failure
		err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID)
		if !errors.Is(err, failure) {
			t.Fatalf("failed child delete = %v", err)
		}
		if _, ok := db.rows[row.ID]; !ok {
			t.Fatal("child delete failure removed the roster row")
		}
		if s.childSettlementSuppressed(child) {
			t.Fatal("failed child delete left settlement suppressed")
		}
		runtime.errorDelete = nil
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err != nil {
			t.Fatalf("retry = %v", err)
		}
		if _, ok := db.rows[row.ID]; ok {
			t.Fatal("retry kept the row")
		}
	})
	t.Run("Should delete a delegation that has no child session yet", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		db.rows["reserved"] = store.SessionSubagent{
			ID: "reserved", WorkspaceID: "ws", ParentSessionID: "parent",
			Origin: store.SubagentOriginDelegated, Status: store.SubagentStatusQueued,
		}
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", "reserved"); err != nil {
			t.Fatal(err)
		}
		if _, ok := db.rows["reserved"]; ok {
			t.Fatal("reserved row survived deletion")
		}
		if len(runtime.deleted) != 0 {
			t.Fatalf("deleted sessions = %v, want none", runtime.deleted)
		}
	})
	t.Run("Should delete a child session linked after the pre-lock read", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		child := *row.ChildSessionID
		stale := db.rows[row.ID]
		stale.ChildSessionID = nil
		s.store = &staleChildSubagents{memorySubagents: db, stale: stale}
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err != nil {
			t.Fatal(err)
		}
		if _, ok := db.rows[row.ID]; ok {
			t.Fatal("row survived deletion")
		}
		if len(runtime.deleted) != 1 || runtime.deleted[0] != child {
			t.Fatalf("deleted sessions = %v, want [%s]", runtime.deleted, child)
		}
	})
	t.Run("Should rewrite a queued completion message without the deleted row", func(t *testing.T) {
		t.Parallel()
		s, db, _ := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		wakeID := "wake-1"
		db.rows["other"] = store.SessionSubagent{
			ID: "other", WorkspaceID: "ws", ParentSessionID: "parent",
			Origin: store.SubagentOriginDelegated, Status: store.SubagentStatusCompleted,
			Title: "Other work", WakeMessageID: &wakeID, Delivery: store.SubagentDeliveryClaimed,
		}
		db.wakes[wakeID] = store.SessionSubagentWake{
			WakeMessageID: wakeID, ParentSessionID: "parent",
			State: store.SubagentWakeStateOpen, InputEntryID: "entry-1",
		}
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err != nil {
			t.Fatal(err)
		}
		text, ok := db.rewrites[wakeID]
		if !ok {
			t.Fatal("queued completion message was not rewritten")
		}
		if strings.Contains(text, row.ID) || !strings.Contains(text, "Other work") {
			t.Fatalf("rewritten wake = %q", text)
		}
	})
	t.Run("Should let an agent delete only its own delegation", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		runtime.snapshots["other"] = subagentSnapshot{
			Info:   &Info{ID: "other", WorkspaceID: "ws", State: StateActive},
			Active: true,
		}
		foreign := SubagentActor{
			Kind:   SubagentActorAgent,
			Caller: &SubagentCaller{WorkspaceID: "ws", SessionID: "other"},
		}
		if err := s.Delete(t.Context(), foreign, "ws", row.ID); !errors.Is(
			err,
			ErrSubagentNotFound,
		) {
			t.Fatalf("foreign agent delete = %v", err)
		}
		if _, ok := db.rows[row.ID]; !ok {
			t.Fatal("foreign agent delete removed the row")
		}
		owner := SubagentActor{
			Kind:   SubagentActorAgent,
			Caller: &SubagentCaller{WorkspaceID: "ws", SessionID: "parent"},
		}
		if err := s.Delete(t.Context(), owner, "ws", row.ID); err != nil {
			t.Fatalf("owner agent delete = %v", err)
		}
		if _, ok := db.rows[row.ID]; ok {
			t.Fatal("owner agent delete kept the row")
		}
	})
	t.Run("Should refuse an unknown actor", func(t *testing.T) {
		t.Parallel()
		s, db, _ := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		err := s.Delete(t.Context(), SubagentActor{Kind: "stranger"}, "ws", row.ID)
		if !errors.Is(err, ErrSubagentNotFound) {
			t.Fatalf("unknown actor = %v", err)
		}
		if _, ok := db.rows[row.ID]; !ok {
			t.Fatal("unknown actor removed the row")
		}
	})
	t.Run("Should report unknown and out-of-workspace rows as not found", func(t *testing.T) {
		t.Parallel()
		s, db, _ := newSubagentTestService(t)
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", "missing"); !errors.Is(
			err,
			ErrSubagentNotFound,
		) {
			t.Fatalf("missing row = %v", err)
		}
		db.rows["scoped"] = store.SessionSubagent{
			ID: "scoped", WorkspaceID: "ws", ParentSessionID: "parent",
			Origin: store.SubagentOriginDelegated, Status: store.SubagentStatusCompleted,
		}
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "other", "scoped"); !errors.Is(
			err,
			ErrSubagentNotFound,
		) {
			t.Fatalf("out-of-workspace row = %v", err)
		}
		if _, ok := db.rows["scoped"]; !ok {
			t.Fatal("out-of-workspace delete removed the row")
		}
	})
}

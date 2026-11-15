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

type contextCheckingDeleteSubagents struct {
	*memorySubagents
}

func (d contextCheckingDeleteSubagents) GetSubagent(
	ctx context.Context,
	workspaceID, id string,
) (store.SessionSubagent, error) {
	if err := ctx.Err(); err != nil {
		return store.SessionSubagent{}, err
	}
	return d.memorySubagents.GetSubagent(ctx, workspaceID, id)
}

func (d contextCheckingDeleteSubagents) DeleteSubagent(ctx context.Context, workspaceID, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.memorySubagents.DeleteSubagent(ctx, workspaceID, id)
}

type failingRosterDelete struct {
	*memorySubagents
	failure         error
	waitForDeadline bool
}

func (d *failingRosterDelete) DeleteSubagent(ctx context.Context, workspaceID, id string) error {
	if d.waitForDeadline {
		<-ctx.Done()
		return errors.Join(d.failure, ctx.Err())
	}
	if d.failure != nil {
		return d.failure
	}
	return d.memorySubagents.DeleteSubagent(ctx, workspaceID, id)
}

func (d *failingRosterDelete) Dispose(
	ctx context.Context, filter store.SubagentDisposeFilter,
) ([]store.SessionSubagent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return d.memorySubagents.Dispose(ctx, filter)
}

type failingRosterDeleteSubagents struct{ *memorySubagents }

func (d failingRosterDeleteSubagents) DeleteSubagent(context.Context, string, string) error {
	return errors.New("roster delete failed")
}

// Invariant: deleting a delegated subagent removes its child session and then its roster row, and
// the settlement the child deletion triggers never finalizes or hooks the row on its way out.
// Owner: session subagent delete suite (feature: subagent context menu).
func TestSubagentDelete(t *testing.T) {
	t.Parallel()
	// Invariant: a failed roster removal after child deletion leaves a terminal, retryable row.
	t.Run("Should settle a retained row after its child was deleted", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			failure := errors.New("roster delete failed")
			fault := &failingRosterDelete{memorySubagents: db, failure: failure, waitForDeadline: true}
			s.store = fault
			err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID)
			if !errors.Is(err, failure) {
				t.Fatalf("delete err=%v", err)
			}
			retained, err := db.GetSubagent(t.Context(), "ws", row.ID)
			if err != nil || retained.Status != store.SubagentStatusCanceled ||
				retained.Delivery != store.SubagentDeliveryDisposed {
				t.Fatalf("retained=%+v err=%v", retained, err)
			}
			if len(runtime.deleted) != 1 {
				t.Fatal("child was not deleted")
			}
			fault.failure = nil
			fault.waitForDeadline = false
			if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.GetSubagent(t.Context(), "ws", row.ID); !errors.Is(err, store.ErrSubagentNotFound) {
				t.Fatalf("retry did not remove row: %v", err)
			}
		})
	})

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
	// Invariant: catalog terminalization cannot leak a wake while the roster delete is in progress.
	t.Run("Should suppress a concurrent delivery after child history is removed", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		runtime.deleteSettles = func(ctx context.Context, _ string) error {
			current := db.rows[row.ID]
			current.Status = store.SubagentStatusInterrupted
			current.ChildSessionID = nil
			current.Delivery = store.SubagentDeliveryPending
			db.rows[row.ID] = current
			return s.OnParentTurnSettled(ctx, row.ParentSessionID, row.ParentTurnID)
		}
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err != nil {
			t.Fatal(err)
		}
		if len(db.wakes) != 0 || len(runtime.queues) != 0 {
			t.Fatalf("deleted delegation leaked a wake: %+v, %+v", db.wakes, runtime.queues)
		}
		if s.deletingDelivery(row.ID) {
			t.Fatal("delivery suppression outlived roster deletion")
		}
	})
	// Invariant: a failed removal preserves a completed inline result without an extra wake.
	t.Run("Should preserve completed wait delivery when deleting its row fails", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		current := db.rows[row.ID]
		current.WakePolicy = store.SubagentWakePolicySettledOnly
		db.rows[row.ID] = current
		settleTestChild(t, s, runtime, &row)
		parent := runtime.snapshots[row.ParentSessionID]
		parent.Active = false
		runtime.snapshots[row.ParentSessionID] = parent
		runtime.errorDelete = errors.New("child delete refused")
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err == nil {
			t.Fatal("expected child deletion failure")
		}
		if got := db.rows[row.ID]; got.Delivery != store.SubagentDeliveryNone || len(db.wakes) != 0 {
			t.Fatalf("failed deletion woke completed wait result: %+v, %+v", got, db.wakes)
		}
	})
	// Invariant: failed roster removal publishes catalog terminalization to waiters and settles the parent.
	t.Run("Should reconcile a retained interruption when roster removal fails", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			s.store = failingRosterDeleteSubagents{db}
			hook := &subagentTestHook{}
			s.settled = hook
			parent := runtime.snapshots[row.ParentSessionID]
			parent.Active = false
			runtime.snapshots[row.ParentSessionID] = parent
			runtime.deleteSettles = func(context.Context, string) error {
				current := db.rows[row.ID]
				current.Status = store.SubagentStatusInterrupted
				current.WorkState = store.SubagentWorkStateResultAvailable
				current.ChildSessionID = nil
				current.WakePolicy = store.SubagentWakePolicySettledOnly
				db.rows[row.ID] = current
				return nil
			}
			if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err == nil {
				t.Fatal("expected roster deletion failure")
			}
			synctest.Wait()
			if hook.calls != 1 || runtime.settles != 1 || s.deletingDelivery(row.ID) {
				t.Fatalf("retained interruption not reconciled: hooks=%d, settles=%d", hook.calls, runtime.settles)
			}
			if got := db.rows[row.ID]; got.Status != store.SubagentStatusInterrupted || len(db.wakes) != 1 {
				t.Fatalf("retained interruption lost its delivery: %+v, %+v", got, db.wakes)
			}
		})
	})
	t.Run("Should finish roster cleanup after the caller disconnects during child deletion", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		s.store = contextCheckingDeleteSubagents{memorySubagents: db}
		parent := runtime.snapshots["parent"]
		parent.Active = false
		runtime.snapshots["parent"] = parent
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		runtime.deleteSettles = func(ctx context.Context, id string) error {
			if err := s.OnChildSettled(ctx, id); err != nil {
				return err
			}
			cancel()
			return nil
		}

		if err := s.Delete(ctx, SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err != nil {
			t.Fatal(err)
		}
		if _, ok := db.rows[row.ID]; ok {
			t.Fatal("deleted child's roster row survived request cancellation")
		}
		if len(runtime.deleted) != 1 || runtime.deleted[0] != *row.ChildSessionID {
			t.Fatalf("deleted children = %v", runtime.deleted)
		}
		if runtime.settles != 1 {
			t.Fatalf("parent settlements = %d, want one", runtime.settles)
		}
	})
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
	t.Run("Should reconcile a stopped child when its delete fails", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		child := *row.ChildSessionID
		runtime.snapshots[child] = subagentSnapshot{
			Info: &Info{ID: child, WorkspaceID: "ws", State: StateStopped},
		}
		runtime.errorDelete = errors.New("catalog delete failed")
		finalizes := db.finalizeCalls
		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", row.ID); err == nil {
			t.Fatal("expected the child delete to fail")
		}
		if _, ok := db.rows[row.ID]; !ok {
			t.Fatal("failed delete removed the roster row")
		}
		if db.finalizeCalls == finalizes {
			t.Fatal("failed delete did not reconcile the stopped child")
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
		deleted := db.rows[row.ID]
		deleted.WakeMessageID = &wakeID
		deleted.Delivery = store.SubagentDeliveryClaimed
		db.rows[row.ID] = deleted
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
	t.Run("Should queue later completions after deleting the only result in a wake", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		first := requireSubagent(t, s, subagentTestRequest())
		req := subagentTestRequest()
		req.IdempotencyKey = "later"
		second := requireSubagent(t, s, req)
		settleTestChild(t, s, runtime, &first)
		wakeID := *db.rows[first.ID].WakeMessageID
		if _, ok := runtime.queues[wakeID]; !ok {
			t.Fatal("first completion was not queued")
		}

		if err := s.Delete(t.Context(), SubagentActor{Kind: SubagentActorOperator}, "ws", first.ID); err != nil {
			t.Fatal(err)
		}
		if _, ok := runtime.queues[wakeID]; ok {
			t.Fatal("deleted result still has a queued wake")
		}
		if db.wakes[wakeID].State != store.SubagentWakeStateCanceled {
			t.Fatalf("empty wake state = %s, want canceled", db.wakes[wakeID].State)
		}

		settleTestChild(t, s, runtime, &second)
		later := db.rows[second.ID]
		if later.WakeMessageID == nil || *later.WakeMessageID == wakeID {
			t.Fatalf("later completion reused the empty wake: %+v", later)
		}
		if text := runtime.queues[*later.WakeMessageID]; !strings.Contains(text, second.ID) {
			t.Fatalf("later completion was not queued: %q", text)
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

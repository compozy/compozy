package session

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// An orphaned delegation is a row that reached running without ever linking a
// child session: the start failed after the reservation and the failure
// settlement was lost to storage contention. Nothing can resume it.
func orphanedDelegationRow() store.SessionSubagent {
	return store.SessionSubagent{
		ID:              "orphan",
		WorkspaceID:     "ws",
		ParentSessionID: "parent",
		ParentTurnID:    "turn",
		Origin:          store.SubagentOriginDelegated,
		Status:          store.SubagentStatusRunning,
		WakePolicy:      store.SubagentWakePolicyAlways,
		Delivery:        store.SubagentDeliveryPending,
		StartedAt:       new(time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)),
	}
}

func TestRecoverFinalizesOrphanedDelegation(t *testing.T) {
	t.Parallel()

	s, db, _ := newSubagentTestService(t)
	db.rows["orphan"] = orphanedDelegationRow()

	if err := s.Recover(t.Context()); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}

	row := db.rows["orphan"]
	if row.Status != store.SubagentStatusFailed {
		t.Fatalf("status = %q, want %q", row.Status, store.SubagentStatusFailed)
	}
	if row.SettledAt == nil {
		t.Fatal("settled_at = nil, want the recovery instant")
	}
	if row.Error == nil || !strings.Contains(*row.Error, "delegation interrupted") {
		t.Fatalf("error = %v, want the delegation-interrupted summary", row.Error)
	}
}

func TestRecoverFinalizesOrphanedIsolatedDelegation(t *testing.T) {
	t.Parallel()

	s, db, _ := newSubagentTestService(t)
	// The isolated recovery path compensates through the worktree service, which the
	// daemon always wires; an empty stub means the reservation never got a worktree.
	s.worktrees = &subagentWorktreeStub{}
	row := orphanedDelegationRow()
	row.Isolation = SubagentIsolationWorktree
	db.rows["orphan"] = row

	if err := s.Recover(t.Context()); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}

	settled := db.rows["orphan"]
	if settled.Status != store.SubagentStatusFailed {
		t.Fatalf("status = %q, want %q", settled.Status, store.SubagentStatusFailed)
	}
	if settled.WakePolicy != store.SubagentWakePolicyAlways {
		t.Fatalf("wake policy = %q, want the parent woken for an orphaned isolated lane", settled.WakePolicy)
	}
	if settled.SettledAt == nil {
		t.Fatal("settled_at = nil, want the recovery instant")
	}
}

// A rollback that fails cannot keep the row running until the next boot: the
// row still settles and the parent still wakes, while the checkout keeps its
// pending cleanup so recoverIsolationCleanup retries it.
func TestRecoverSettlesOrphanedIsolationWhenCompensationFails(t *testing.T) {
	t.Parallel()

	s, db, r := newSubagentTestService(t)
	s.worktrees = &subagentWorktreeStub{lookupErr: errors.New("worktree lookup unavailable")}
	var logs strings.Builder
	s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	row := orphanedDelegationRow()
	row.Isolation = SubagentIsolationWorktree
	row.WakePolicy = store.SubagentWakePolicySettledOnly
	db.rows["orphan"] = row

	if err := s.Recover(t.Context()); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}

	settled := db.rows["orphan"]
	if settled.Status != store.SubagentStatusFailed {
		t.Fatalf("status = %q, want %q even when the rollback failed", settled.Status, store.SubagentStatusFailed)
	}
	if settled.WakePolicy != store.SubagentWakePolicyAlways {
		t.Fatalf("wake policy = %q, want %q for a failed orphan", settled.WakePolicy, store.SubagentWakePolicyAlways)
	}
	if got := settled.WorktreeState().Cleanup; got != "pending" {
		t.Fatalf("worktree cleanup = %q, want %q for recoverIsolationCleanup to retry", got, "pending")
	}
	if len(db.wakes) != 1 || len(r.queues) != 1 {
		t.Fatalf("wakes = %+v queues = %+v, want one planned wake", db.wakes, r.queues)
	}
	// Both failures surface: the settle result is not allowed to swallow the
	// compensation error that made the fallback necessary.
	if !strings.Contains(logs.String(), "orphaned_isolation_compensation") ||
		!strings.Contains(logs.String(), "worktree lookup unavailable") {
		t.Fatalf("logs = %q, want the compensation error reported", logs.String())
	}
}

func TestRecoverSkipsInFlightOrphan(t *testing.T) {
	t.Parallel()

	s, db, _ := newSubagentTestService(t)
	db.rows["orphan"] = orphanedDelegationRow()
	s.mu.Lock()
	s.flights["orphan"] = &subagentFlight{}
	s.mu.Unlock()

	if err := s.Recover(t.Context()); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}

	if row := db.rows["orphan"]; row.Status != store.SubagentStatusRunning {
		t.Fatalf("status = %q, want the live delegation untouched", row.Status)
	}
}

// A terminal write that loses writer admission must be retried: surfacing it once
// leaves the delegation running forever with no settlement. The retry has to
// outlive the caller budget that the first attempt already spent.
func TestFinalizeSubagentRetryOutlivesExpiredCallerContext(t *testing.T) {
	t.Parallel()

	s, db, _ := newSubagentTestService(t)
	db.rows["orphan"] = orphanedDelegationRow()
	db.finalizeFailures = 1

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	<-ctx.Done()

	settled, changed, err := s.finalizeSubagentWithRetry(ctx, store.SubagentFinalize{
		ID:        "orphan",
		Status:    store.SubagentStatusFailed,
		WorkState: store.SubagentWorkStateResultAvailable,
		SettledAt: time.Date(2026, 10, 8, 0, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("finalizeSubagentWithRetry() error = %v", err)
	}
	if !changed || settled.Status != store.SubagentStatusFailed {
		t.Fatalf("settled = %+v changed = %v, want a failed terminal write", settled, changed)
	}
}

func TestFinalizeSubagentRetriesWriterContention(t *testing.T) {
	t.Parallel()

	s, db, _ := newSubagentTestService(t)
	db.rows["orphan"] = orphanedDelegationRow()
	db.finalizeFailures = 2

	settled, changed, err := s.finalizeSubagentWithRetry(t.Context(), store.SubagentFinalize{
		ID:        "orphan",
		Status:    store.SubagentStatusFailed,
		WorkState: store.SubagentWorkStateResultAvailable,
		Error:     new("delegation interrupted"),
		SettledAt: time.Date(2026, 10, 8, 0, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("finalizeSubagentWithRetry() error = %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want the terminal write to apply")
	}
	if settled.Status != store.SubagentStatusFailed {
		t.Fatalf("status = %q, want %q", settled.Status, store.SubagentStatusFailed)
	}
	if want := 3; db.finalizeCalls != want {
		t.Fatalf("finalize calls = %d, want %d after two contention failures", db.finalizeCalls, want)
	}
}

// A permanent store error is not contention and must surface immediately.
func TestFinalizeSubagentDoesNotRetryPermanentErrors(t *testing.T) {
	t.Parallel()

	s, db, _ := newSubagentTestService(t)

	if _, _, err := s.finalizeSubagentWithRetry(t.Context(), store.SubagentFinalize{
		ID:        "missing",
		Status:    store.SubagentStatusFailed,
		SettledAt: time.Date(2026, 10, 8, 0, 1, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("error = nil, want the missing-row error")
	}
	if want := 1; db.finalizeCalls != want {
		t.Fatalf("finalize calls = %d, want %d", db.finalizeCalls, want)
	}
}

// slowFinalizeSubagents makes each terminal write spend fake time, so a retry
// can outlive the caller's cleanup context without slowing the suite down.
type slowFinalizeSubagents struct {
	store.SubagentStore
	delay time.Duration
}

func (d slowFinalizeSubagents) FinalizeSubagent(
	ctx context.Context,
	in store.SubagentFinalize,
) (store.SessionSubagent, bool, error) {
	time.Sleep(d.delay)
	return d.SubagentStore.FinalizeSubagent(ctx, in)
}

func publishContextProbe(r *subagentTestRuntime) (published *int, publishErr *error) {
	published, publishErr = new(int), new(error)
	r.publishParent = func(ctx context.Context, _ string) {
		*published++
		*publishErr = ctx.Err()
	}
	return published, publishErr
}

// The retry owns a 30 s budget while the caller's cleanup context lasts 5 s.
// Publishing the settled row on that spent context silently drops the parent
// catalog refresh, so the publication needs a bounded context of its own.
func TestFailDelegationPublishesTerminalWithLiveContext(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s, db, r := newSubagentTestService(t)
		db.rows["orphan"] = orphanedDelegationRow()
		db.finalizeFailures = 1
		s.store = slowFinalizeSubagents{SubagentStore: db, delay: 2 * defaultLifecycleTimeout}
		published, publishErr := publishContextProbe(r)

		if _, err := s.failDelegation(t.Context(), db.rows["orphan"], "", testSubagentError()); err != nil {
			t.Fatalf("failDelegation() error = %v", err)
		}
		if *published != 1 {
			t.Fatalf("PublishParent calls = %d, want 1", *published)
		}
		if *publishErr != nil {
			t.Fatalf("PublishParent context error = %v, want a live publication context", *publishErr)
		}
	})
}

func TestFailIsolatedDelegationPublishesTerminalWithLiveContext(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s, db, r := newSubagentTestService(t)
		row := orphanedDelegationRow()
		row.Isolation = SubagentIsolationWorktree
		row.Worktree = &store.SubagentWorktreeState{ID: "wt-1", BaseSHA: "base"}
		db.rows["orphan"] = row
		db.finalizeFailures = 1
		s.worktrees = &subagentWorktreeStub{worktree: SubagentWorktree{ID: "wt-1"}}
		s.store = slowFinalizeSubagents{SubagentStore: db, delay: 2 * defaultLifecycleTimeout}
		published, publishErr := publishContextProbe(r)

		cause := testSubagentError()
		if _, err := s.failIsolatedDelegation(t.Context(), db.rows["orphan"], "", cause); !errors.Is(err, cause) {
			t.Fatalf("failIsolatedDelegation() error = %v, want %v", err, cause)
		}
		if *published != 1 {
			t.Fatalf("PublishParent calls = %d, want 1", *published)
		}
		if *publishErr != nil {
			t.Fatalf("PublishParent context error = %v, want a live publication context", *publishErr)
		}
	})
}

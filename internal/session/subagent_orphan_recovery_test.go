package session

import (
	"strings"
	"testing"
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
// leaves the delegation running forever with no settlement.
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

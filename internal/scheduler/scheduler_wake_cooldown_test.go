package scheduler

import (
	"slices"
	"testing"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/jonboulle/clockwork"
)

func TestWakeCooldownDisabledContract(t *testing.T) {
	t.Parallel()

	t.Run("Should wake repeatedly without retaining cooldown keys when cooldown is disabled", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 17, 17, 10, 0, 0, time.UTC)
		source := &fakeTaskSource{}
		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			sessionSnapshot("sess-1", "ws-1", "active", false, nil, base),
		}}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			source,
			sessions,
			waker,
			WithClock(clockwork.NewFakeClockAt(base)),
			WithWakeCooldown(0),
		)

		runIDs := []string{"run-1", "run-2", "run-3"}
		for _, runID := range runIDs {
			setPendingRunsForWakeCooldownContract(
				source,
				workSnapshot("task-1", runID, taskpkg.ScopeWorkspace, "ws-1", nil, base),
			)
			result, err := scheduler.RunOnce(testutil.Context(t))
			if err != nil {
				t.Fatalf("RunOnce(%s) error = %v", runID, err)
			}
			if result.WakeSucceeded != 1 || result.RecentlyNotified != 0 {
				t.Fatalf("RunOnce(%s) result = %#v, want one wake and no cooldown suppression", runID, result)
			}
		}

		targets := waker.targetsSnapshot()
		if got, want := len(targets), len(runIDs); got != want {
			t.Fatalf("wake targets = %d, want %d", got, want)
		}
		gotRunIDs := make([]string, 0, len(targets))
		for _, target := range targets {
			gotRunIDs = append(gotRunIDs, target.Work.Run.ID)
		}
		if !slices.Equal(gotRunIDs, runIDs) {
			t.Fatalf("woken run IDs = %v, want %v", gotRunIDs, runIDs)
		}

		rebuild, err := scheduler.Rebuild(testutil.Context(t))
		if err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}
		if rebuild.ClearedWakeKeys != 0 {
			t.Fatalf("ClearedWakeKeys = %d, want 0 when cooldown is disabled", rebuild.ClearedWakeKeys)
		}
	})
}

func setPendingRunsForWakeCooldownContract(source *fakeTaskSource, pending ...RunSnapshot) {
	source.mu.Lock()
	defer source.mu.Unlock()

	source.pending = append([]RunSnapshot(nil), pending...)
}

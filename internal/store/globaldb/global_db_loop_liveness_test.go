package globaldb

import (
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/testutil"
)

// Invariant: the coordinator liveness read follows the stored Loop run, so an evaluation that
// registers after a cancellation sees the run as no longer live.
func TestLoopRepoShouldReportCoordinatorLivenessFromTheStoredRun(t *testing.T) {
	t.Parallel()
	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	run, err := globalDB.CreateLoopRunForStart(
		ctx,
		testLoopRun("looprun-coordinator-liveness", now, looppkg.StatusRunning),
		dsl.ConcurrencyAllow,
	)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}

	t.Run("Should report a running Loop run as live", func(t *testing.T) {
		live, err := globalDB.CoordinatedLoopRunLive(ctx, string(run.ID))
		if err != nil {
			t.Fatalf("CoordinatedLoopRunLive(running) error = %v", err)
		}
		if !live {
			t.Fatal("CoordinatedLoopRunLive(running) = false, want a live Loop run")
		}
	})

	t.Run("Should report an unknown Loop run as not live", func(t *testing.T) {
		live, err := globalDB.CoordinatedLoopRunLive(ctx, "looprun-missing")
		if err != nil {
			t.Fatalf("CoordinatedLoopRunLive(missing) error = %v", err)
		}
		if live {
			t.Fatal("CoordinatedLoopRunLive(missing) = true, want an unknown run to be not live")
		}
	})

	t.Run("Should report a blank Loop run id as not live", func(t *testing.T) {
		live, err := globalDB.CoordinatedLoopRunLive(ctx, "  ")
		if err != nil || live {
			t.Fatalf("CoordinatedLoopRunLive(blank) = %t, %v; want false, nil", live, err)
		}
	})

	// Invariant: coordinator progress can advance a live Loop, but cannot revive
	// or rewrite a terminal Loop. Owner: SQLite Loop repository, this liveness suite.
	t.Run("Should preserve terminal progress after a cancellation commits", func(t *testing.T) {
		if err := globalDB.AdvanceLoopRunProgress(ctx, string(run.ID), now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		active, err := globalDB.GetLoopRunByID(ctx, run.ID)
		if err != nil || !active.LastProgressAt.Equal(now.Add(time.Minute)) {
			t.Fatalf("active progress = %#v, %v", active, err)
		}
		result, err := globalDB.RequestRunCancellation(ctx, looppkg.CancellationMutation{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, RequestedAt: now.Add(2 * time.Minute),
			Actor: operatorActorContextForTest("operator:cancel"), Reason: "progress fence",
		})
		if err != nil || result.Run.Status != looppkg.StatusCanceled {
			t.Fatalf("cancellation = %#v, %v", result, err)
		}
		if err := globalDB.AdvanceLoopRunProgress(ctx, string(run.ID), now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		terminal, err := globalDB.GetLoopRunByID(ctx, run.ID)
		if err != nil || terminal.Status != looppkg.StatusCanceled ||
			!terminal.LastProgressAt.Equal(result.Run.LastProgressAt) {
			t.Fatalf("terminal progress changed = %#v, %v", terminal, err)
		}
		live, err := globalDB.CoordinatedLoopRunLive(ctx, string(run.ID))
		if err != nil || live {
			t.Fatalf("canceled liveness = %t, %v; want false", live, err)
		}
	})
}

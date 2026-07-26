package globaldb

import (
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBGetSchedulerPauseToleratesNonCanonicalTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("Should read the authoritative paused flag when updated_at is non-canonical", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		if _, err := globalDB.SetSchedulerPaused(ctx, "operator:ops", "maintenance"); err != nil {
			t.Fatalf("SetSchedulerPaused() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE scheduler_pause SET updated_at = '2026-05-28 15:50:25' WHERE id = 1`,
		); err != nil {
			t.Fatalf("corrupt updated_at error = %v", err)
		}

		state, err := globalDB.GetSchedulerPause(ctx)
		if err != nil {
			t.Fatalf("GetSchedulerPause() error = %v, want nil", err)
		}
		if !state.Paused {
			t.Fatalf("state.Paused = false, want true (authoritative flag must survive a bad timestamp)")
		}
		if !state.UpdatedAt.IsZero() {
			t.Fatalf("state.UpdatedAt = %s, want zero (non-canonical value parsed best-effort)", state.UpdatedAt)
		}
	})

	t.Run("Should read the authoritative paused flag when paused_at is non-canonical", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		if _, err := globalDB.SetSchedulerPaused(ctx, "operator:ops", "maintenance"); err != nil {
			t.Fatalf("SetSchedulerPaused() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE scheduler_pause SET paused_at = '2026-05-28 15:50:25' WHERE id = 1`,
		); err != nil {
			t.Fatalf("corrupt paused_at error = %v", err)
		}

		state, err := globalDB.GetSchedulerPause(ctx)
		if err != nil {
			t.Fatalf("GetSchedulerPause() error = %v, want nil", err)
		}
		if !state.Paused {
			t.Fatalf("state.Paused = false, want true")
		}
		if !state.PausedAt.IsZero() {
			t.Fatalf("state.PausedAt = %s, want zero (non-canonical value parsed best-effort)", state.PausedAt)
		}
	})
}

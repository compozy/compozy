package globaldb

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestDailyBudgetDirectDispatchUsesAdmissionDay(t *testing.T) {
	t.Run("Should check the new day at dispatch after midnight", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		clock := time.Date(2026, 11, 19, 23, 59, 0, 0, time.Local)
		db.now = func() time.Time { return clock }
		main := budgetBoundSession(t, db, clock)
		req := promptAdmissionRequest("ws-input-queue-workspace", main, "budget-after-midnight", clock)
		req.HumanIngress, req.AppIngress = true, true
		if _, _, err := db.ClaimSessionPromptAdmission(ctx, req); err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(2 * time.Minute)
		if err := db.CommitSessionPromptDispatch(ctx, req.WorkspaceID, main, req.IdempotencyKey, req.Now); err != nil {
			t.Fatalf("new-day dispatch used yesterday's receipts: %v", err)
		}
		if instance, err := db.Instances().Get(ctx, "daily-dispatch-instance"); err != nil ||
			instance.State != "active" {
			t.Fatalf("instance after a new-day dispatch=%+v err=%v; want active", instance, err)
		}
		replay, found, err := db.LookupSessionPromptAdmission(ctx, req.WorkspaceID, main, req.IdempotencyKey)
		if err != nil || !found || replay.State != store.SessionPromptAdmissionDispatchCommitted {
			t.Fatalf("persisted dispatch=%+v found=%t err=%v", replay, found, err)
		}
	})
}

package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestDailyBudgetQuarantinePreservesExistingManualPause(t *testing.T) {
	t.Run("Should prevent unsafe resume despite an existing manual pause", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		now := time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)
		db, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "quarantine.db"),
			WithClock(func() time.Time { return now }))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close global DB: %v", err)
			}
		})
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "writer", CreatedByKind: "human",
			PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().
			ExecContext(ctx, `UPDATE agent_instances SET state='active' WHERE id=?`, instance.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Pause(ctx, instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
			t.Fatal(err)
		}
		if quarantined, err := db.QuarantineDailyBudgetInstance(ctx, instance.ID); err != nil || !quarantined {
			t.Fatalf("quarantine = %t, error = %v", quarantined, err)
		}
		paused, err := db.Instances().Get(ctx, instance.ID)
		if err != nil || paused.PauseReason != "manual" {
			t.Fatalf("quarantine must preserve the manual pause, got %q (err=%v)", paused.PauseReason, err)
		}
		if _, err := db.Instances().Resume(ctx, instance.ID, 10, true); !errors.Is(err, agentinstance.ErrPaused) ||
			!strings.Contains(err.Error(), "instance daily budget ledger recovery is incomplete") {
			t.Fatalf("operator resume error = %v, want incomplete ledger recovery", err)
		}
		generation, err := db.DailyBudgetRecoveryGeneration(ctx, instance.ID)
		if err != nil || generation == 0 {
			t.Fatalf("budget recovery generation=%d err=%v", generation, err)
		}
		if err := db.ResolveDailyBudgetRecovery(ctx, instance.ID, generation); err != nil {
			t.Fatal(err)
		}
		paused, err = db.Instances().Get(ctx, instance.ID)
		if err != nil || paused.PauseReason != "manual" {
			t.Fatalf("resolved quarantine must preserve the manual pause, got %q (err=%v)", paused.PauseReason, err)
		}
	})
}

func TestDailyBudgetQuarantineChecksCurrentCaps(t *testing.T) {
	t.Run("Should not quarantine when the caps were cleared before recovery failed", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		budgetBoundSession(t, db, at)
		const instanceID = "daily-dispatch-instance"
		if err := db.Instances().SetDailyBudgetLimits(ctx, instanceID, agentinstance.DailyBudgetLimits{}); err != nil {
			t.Fatal(err)
		}
		if quarantined, err := db.QuarantineDailyBudgetInstance(ctx, instanceID); err != nil || quarantined {
			t.Fatalf("quarantine after removing caps = %t, error = %v", quarantined, err)
		}
		instance, err := db.Instances().Get(ctx, instanceID)
		if err != nil || instance.State != "active" || instance.PauseReason != "" {
			t.Fatalf("instance = %+v, error = %v; want unpaused", instance, err)
		}
		generation, err := db.DailyBudgetRecoveryGeneration(ctx, instanceID)
		if err != nil || generation != 0 {
			t.Fatalf("generation = %d, error = %v; want no recovery fence", generation, err)
		}
	})
}

func TestDailyBudgetResumeCannotBypassCapThroughOtherPauseReason(t *testing.T) {
	t.Run("Should preserve an exhausted budget under an existing manual pause", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "resume-manual-budget.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close resumed budget DB: %v", err)
			}
		})
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "writer", CreatedByKind: "human",
			PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().
			ExecContext(ctx, `UPDATE agent_instances SET state='active' WHERE id=?`, instance.ID); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instance.ID,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(0))}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Pause(ctx, instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Resume(ctx, instance.ID, 10, false); !errors.Is(err, agentinstance.ErrForbidden) {
			t.Fatalf("agent resume under manual pause=%v, want forbidden", err)
		}
		if _, err := db.Instances().Resume(ctx, instance.ID, 10, true); !errors.Is(err, agentinstance.ErrPaused) {
			t.Fatalf("operator resume despite exhausted cap=%v, want paused", err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instance.ID, agentinstance.DailyBudgetLimits{}); err != nil {
			t.Fatal(err)
		}
		resumed, err := db.Instances().Resume(ctx, instance.ID, 10, true)
		if err != nil || resumed.State != "active" {
			t.Fatalf("operator resume after cap removal=%+v err=%v", resumed, err)
		}
	})
}

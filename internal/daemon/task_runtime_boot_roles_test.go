package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestTaskManagerOptionsShouldWireLoopCoordinatorTerminalStatusValidator(t *testing.T) {
	t.Parallel()

	validStatuses := []looppkg.Status{
		looppkg.StatusQueued,
		looppkg.StatusRunning,
		looppkg.StatusWatching,
		looppkg.StatusNeedsApproval,
		looppkg.StatusPaused,
		looppkg.StatusDone,
		looppkg.StatusNoOp,
		looppkg.StatusBlocked,
		looppkg.StatusFailed,
		looppkg.StatusExhausted,
		looppkg.StatusStalled,
	}
	for _, status := range validStatuses {
		t.Run("Should accept "+string(status), func(t *testing.T) {
			t.Parallel()

			if err := runBootWiredCoordinatorTerminalStatus(t, string(status)); err != nil {
				t.Fatalf("StartRun(%q) error = %v", status, err)
			}
		})
	}

	t.Run("Should reject statuses outside the loop vocabulary", func(t *testing.T) {
		t.Parallel()

		err := runBootWiredCoordinatorTerminalStatus(t, "not-a-loop-status")
		if !errors.Is(err, taskpkg.ErrValidation) || !coordinatorOwnerRejectedStatus(err) {
			t.Fatalf("StartRun(invalid status) error = %v, want owner ErrValidation", err)
		}
	})
}

func runBootWiredCoordinatorTerminalStatus(t *testing.T, status string) error {
	t.Helper()

	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	db := openDaemonTestGlobalDB(t)
	runner := &bootWiredCoordinatorRunner{statusByRun: make(map[taskpkg.RunID]string)}
	options := taskManagerOptions(
		db,
		nil,
		nil,
		nil,
		nil,
		nil,
		runner,
		schedulerBackstopGenerationFinalizer{},
		aghconfig.TaskRecoveryConfig{},
		aghconfig.SchedulerConfig{},
		0,
		0,
	)
	options = append(options, taskpkg.WithManagerNow(func() time.Time { return now }))
	manager, err := taskpkg.NewManager(options...)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("loop", "daemon.loop")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	return startBootWiredCoordinatorWithTerminalStatus(ctx, t, db, manager, runner, actor, now, status)
}

func startBootWiredCoordinatorWithTerminalStatus(
	ctx context.Context,
	t *testing.T,
	db *globaldb.GlobalDB,
	manager *taskpkg.Service,
	runner *bootWiredCoordinatorRunner,
	actor taskpkg.ActorContext,
	now time.Time,
	status string,
) error {
	t.Helper()

	workspaceID := "ws-boot-" + terminalStatusIDPart(status)
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID:        workspaceID,
		RootDir:   t.TempDir(),
		Name:      workspaceID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace(%q) error = %v", workspaceID, err)
	}
	loopRun := looppkg.Run{
		ID:                looppkg.RunID("looprun-boot-" + terminalStatusIDPart(status)),
		WorkspaceID:       looppkg.WorkspaceID(workspaceID),
		LoopName:          "boot-status-" + terminalStatusIDPart(status),
		Status:            looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly,
		IterationCap:      1,
		BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
		CreatedAt:         now,
		LastProgressAt:    now,
	}
	applyLoopRunPinningForTest(t, &loopRun, now)
	if _, err := db.CreateLoopRunForStart(ctx, loopRun, loopdsl.ConcurrencyAllow); err != nil {
		t.Fatalf("CreateLoopRunForStart(%q) error = %v", loopRun.ID, err)
	}
	claim, err := db.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		RunKind:          taskpkg.RunKindCoordinator,
		ClaimerSessionID: "daemon-loop-boot-test-" + terminalStatusIDPart(status),
		ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
		LeaseDuration:    time.Minute,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(%q) error = %v", status, err)
	}
	runner.statusByRun[taskpkg.RunID(claim.Run.ID)] = strings.TrimSpace(status)
	_, err = manager.StartRun(ctx, claim.Run.ID, taskpkg.StartRun{
		ClaimToken:     claim.ClaimToken,
		IdempotencyKey: claim.Run.IdempotencyKey,
	}, actor)
	return err
}

func coordinatorOwnerRejectedStatus(err error) bool {
	return err != nil && strings.Contains(err.Error(), "must be accepted by the coordinator owner")
}

func terminalStatusIDPart(status string) string {
	replacer := strings.NewReplacer(" ", "-", "\t", "-", "\n", "-", "_", "-", "/", "-")
	return replacer.Replace(strings.TrimSpace(status))
}

type bootWiredCoordinatorRunner struct {
	statusByRun map[taskpkg.RunID]string
}

func (r *bootWiredCoordinatorRunner) Run(
	_ context.Context,
	runID taskpkg.RunID,
) (taskpkg.CoordinatorCompletionPlan, error) {
	status, ok := r.statusByRun[runID]
	if !ok {
		return taskpkg.CoordinatorCompletionPlan{}, fmt.Errorf("missing terminal status for coordinator run %q", runID)
	}
	return taskpkg.CoordinatorCompletionPlan{
		Snapshot: taskpkg.GenerationSnapshot{Generation: 1},
		Terminal: &taskpkg.CoordinatorTerminal{
			Status: status,
			Cause:  "boot-wiring-test",
			GateID: "gate-boot-wiring",
		},
	}, nil
}

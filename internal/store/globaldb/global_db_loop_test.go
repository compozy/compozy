package globaldb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	storepkg "github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestLoopRunEventKindValidShouldMatchPublicContract(t *testing.T) {
	t.Parallel()

	localKinds := map[string]struct{}{
		loopRunEventNodeRunning:       {},
		loopRunEventNodeSucceeded:     {},
		loopRunEventNodeFailed:        {},
		loopRunEventGateVerdict:       {},
		loopRunEventGenerationStarted: {},
		loopRunEventChannelMsg:        {},
		loopRunEventTokenTick:         {},
		loopRunEventNeedsApproval:     {},
		loopRunEventStatusChanged:     {},
		loopRunEventGoalTurnStarted:   {},
		loopRunEventGoalTurnCompleted: {},
		loopRunEventGoalStatusChanged: {},
	}
	for _, kind := range contract.LoopRunEventKindValues() {
		t.Run("Should accept public kind "+kind, func(t *testing.T) {
			t.Parallel()
			if !loopRunEventKindValid(kind) {
				t.Fatalf("loopRunEventKindValid(%q) = false, want true", kind)
			}
			if _, ok := localKinds[kind]; !ok {
				t.Fatalf("contract kind %q is missing from local loop event constants", kind)
			}
		})
	}
	for kind := range localKinds {
		t.Run("Should publish local kind "+kind, func(t *testing.T) {
			t.Parallel()
			if !slices.Contains(contract.LoopRunEventKindValues(), kind) {
				t.Fatalf("local loop event kind %q is missing from public contract", kind)
			}
		})
	}
}

func TestGlobalDBLoopConfigShouldPersistOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip loop config by workspace and loop name", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		humanGate := true
		reattempt := looppkg.ReattemptFullBody
		onExceeded := dsl.BudgetExceededEscalate
		workerModel := "stored-worker"
		judgeModel := "stored-judge"

		err := globalDB.UpsertLoopConfig(ctx, "ws-1", "delivery", looppkg.LoopConfig{
			HumanGateEnabled:  &humanGate,
			ReattemptStrategy: &reattempt,
			EnabledChecks:     []byte(`{"command":true}`),
			IterationCap:      new(11),
			BudgetTokens:      new(2000),
			BudgetWallSec:     new(300),
			BudgetOnExceeded:  &onExceeded,
			NoProgressWindow:  new(4),
			FanOutWidth:       new(5),
			GateMaxRevisions:  new(6),
			ModelDefaults: &looppkg.ModelDefaults{
				Worker: &workerModel,
				Judge:  &judgeModel,
			},
		})
		if err != nil {
			t.Fatalf("UpsertLoopConfig() error = %v", err)
		}

		got, err := globalDB.GetLoopConfig(ctx, "ws-1", "delivery")
		if err != nil {
			t.Fatalf("GetLoopConfig() error = %v", err)
		}
		if got.HumanGateEnabled == nil || !*got.HumanGateEnabled {
			t.Fatalf("HumanGateEnabled = %#v, want true", got.HumanGateEnabled)
		}
		if got.ReattemptStrategy == nil || *got.ReattemptStrategy != looppkg.ReattemptFullBody {
			t.Fatalf("ReattemptStrategy = %#v, want full_body", got.ReattemptStrategy)
		}
		if string(got.EnabledChecks) != `{"command":true}` {
			t.Fatalf("EnabledChecks = %s, want command check", got.EnabledChecks)
		}
		if got.FanOutWidth == nil || *got.FanOutWidth != 5 {
			t.Fatalf("FanOutWidth = %#v, want 5", got.FanOutWidth)
		}
		if got.ModelDefaults == nil {
			t.Fatal("ModelDefaults = nil, want stored defaults")
		}
		if got.ModelDefaults.Worker == nil || *got.ModelDefaults.Worker != "stored-worker" {
			t.Fatalf("ModelDefaults.Worker = %#v, want stored-worker", got.ModelDefaults.Worker)
		}
		if got.ModelDefaults.Judge == nil || *got.ModelDefaults.Judge != "stored-judge" {
			t.Fatalf("ModelDefaults.Judge = %#v, want stored-judge", got.ModelDefaults.Judge)
		}
		_, err = globalDB.GetLoopConfig(ctx, "ws-2", "delivery")
		if !errors.Is(err, looppkg.ErrConfigNotFound) {
			t.Fatalf("GetLoopConfig(other workspace) error = %v, want ErrConfigNotFound", err)
		}
	})

	t.Run("Should reject empty loop config keys", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		cases := []struct {
			name     string
			ws       looppkg.WorkspaceID
			loopName string
			want     string
		}{
			{name: "Should reject empty workspace", ws: " ", loopName: "delivery", want: "workspace_id is required"},
			{name: "Should reject empty loop name", ws: "ws-1", loopName: " ", want: "loop_name is required"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				err := globalDB.UpsertLoopConfig(ctx, tc.ws, tc.loopName, looppkg.LoopConfig{})
				if !errors.Is(err, looppkg.ErrValidation) {
					t.Fatalf("UpsertLoopConfig() error = %v, want ErrValidation", err)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("UpsertLoopConfig() error = %v, want %q", err, tc.want)
				}
			})
		}
	})

	t.Run("Should preserve omitted overrides on partial update", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		humanGate := true
		reattempt := looppkg.ReattemptFullBody
		workerModel := "stored-worker"
		if err := globalDB.UpsertLoopConfig(ctx, "ws-1", "delivery", looppkg.LoopConfig{
			HumanGateEnabled:  &humanGate,
			ReattemptStrategy: &reattempt,
			EnabledChecks:     []byte(`{"command":true}`),
			BudgetTokens:      new(2000),
			FanOutWidth:       new(5),
			ModelDefaults:     &looppkg.ModelDefaults{Worker: &workerModel},
		}); err != nil {
			t.Fatalf("UpsertLoopConfig(initial) error = %v", err)
		}
		if err := globalDB.UpsertLoopConfig(ctx, "ws-1", "delivery", looppkg.LoopConfig{
			BudgetTokens: new(5000),
		}); err != nil {
			t.Fatalf("UpsertLoopConfig(partial) error = %v", err)
		}

		got, err := globalDB.GetLoopConfig(ctx, "ws-1", "delivery")
		if err != nil {
			t.Fatalf("GetLoopConfig() error = %v", err)
		}
		if got.HumanGateEnabled == nil || !*got.HumanGateEnabled {
			t.Fatalf("HumanGateEnabled = %#v, want preserved true", got.HumanGateEnabled)
		}
		if got.ReattemptStrategy == nil || *got.ReattemptStrategy != looppkg.ReattemptFullBody {
			t.Fatalf("ReattemptStrategy = %#v, want preserved full_body", got.ReattemptStrategy)
		}
		if string(got.EnabledChecks) != `{"command":true}` {
			t.Fatalf("EnabledChecks = %s, want preserved command check", got.EnabledChecks)
		}
		if got.FanOutWidth == nil || *got.FanOutWidth != 5 {
			t.Fatalf("FanOutWidth = %#v, want preserved 5", got.FanOutWidth)
		}
		if got.BudgetTokens == nil || *got.BudgetTokens != 5000 {
			t.Fatalf("BudgetTokens = %#v, want updated 5000", got.BudgetTokens)
		}
		if got.ModelDefaults == nil || got.ModelDefaults.Worker == nil || *got.ModelDefaults.Worker != "stored-worker" {
			t.Fatalf("ModelDefaults.Worker = %#v, want preserved stored-worker", got.ModelDefaults)
		}
	})
}

func TestGlobalDBLoopRunCreateShouldSeedInitialCoordinator(t *testing.T) {
	t.Parallel()

	t.Run("Should create a workspace-scoped coordinator for a running loop", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-seed", now, looppkg.StatusRunning)
		run.GoalContextNudgeRatio = 0.37
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if created.Status != looppkg.StatusRunning {
			t.Fatalf("created status = %q, want running", created.Status)
		}
		persisted, err := globalDB.GetLoopRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if persisted.GoalContextNudgeRatio != 0.37 {
			t.Fatalf("GoalContextNudgeRatio = %v, want pinned 0.37", persisted.GoalContextNudgeRatio)
		}

		queued, err := globalDB.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
		if err != nil {
			t.Fatalf("ListTaskRunsByStatus() error = %v", err)
		}
		if got, want := len(queued), 1; got != want {
			t.Fatalf("queued task runs = %d, want %d", got, want)
		}
		coordinator := queued[0]
		if coordinator.RunKind.Normalize() != taskpkg.RunKindCoordinator {
			t.Fatalf("RunKind = %q, want coordinator", coordinator.RunKind)
		}
		if got, want := coordinator.LoopRunID, string(created.ID); got != want {
			t.Fatalf("LoopRunID = %q, want %q", got, want)
		}
		if got, want := coordinator.ID, loopCoordinatorRunID(created.ID, created.Generation+1); got != want {
			t.Fatalf("coordinator run id = %q, want %q", got, want)
		}
		wantIdempotencyKey := loopCoordinatorIdempotencyKey(created.ID, created.Generation+1)
		if got, want := coordinator.IdempotencyKey, wantIdempotencyKey; got != want {
			t.Fatalf("IdempotencyKey = %q, want %q", got, want)
		}

		taskRecord, err := globalDB.GetTask(ctx, coordinator.TaskID)
		if err != nil {
			t.Fatalf("GetTask(coordinator) error = %v", err)
		}
		if got, want := taskRecord.ID, loopCoordinatorTaskID(created.ID); got != want {
			t.Fatalf("coordinator task id = %q, want %q", got, want)
		}
		if taskRecord.Scope.Normalize() != taskpkg.ScopeWorkspace {
			t.Fatalf("coordinator task scope = %q, want workspace", taskRecord.Scope)
		}
		if got, want := taskRecord.WorkspaceID, string(created.WorkspaceID); got != want {
			t.Fatalf("coordinator task workspace_id = %q, want %q", got, want)
		}
		if taskRecord.AutoEnqueueOnReady {
			t.Fatal("coordinator task AutoEnqueueOnReady = true, want false")
		}
	})

	t.Run("Should not create a coordinator for queued loop starts", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 7, 9, 5, 0, 0, time.UTC)
		first := testLoopRun("looprun-seed-running", now, looppkg.StatusRunning)
		if _, err := globalDB.CreateLoopRunForStart(ctx, first, dsl.ConcurrencyQueue); err != nil {
			t.Fatalf("CreateLoopRunForStart(first) error = %v", err)
		}
		second := testLoopRun("looprun-seed-queued", now.Add(time.Second), looppkg.StatusRunning)
		queuedRun, err := globalDB.CreateLoopRunForStart(ctx, second, dsl.ConcurrencyQueue)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart(second) error = %v", err)
		}
		if queuedRun.Status != looppkg.StatusQueued {
			t.Fatalf("second status = %q, want queued", queuedRun.Status)
		}
		if got := countCoordinatorTaskRunsForLoop(ctx, t, globalDB, queuedRun.ID); got != 0 {
			t.Fatalf("queued loop coordinator task runs = %d, want 0", got)
		}
	})

	t.Run("Should reserve coordinator wakes beyond the task retry budget", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 7, 9, 10, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-coordinator-budget", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		actor := coordinatorActorContextForTest()
		claimCoordinator := func(runID string, at time.Time) taskpkg.ClaimResult {
			t.Helper()
			claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				RunID:            runID,
				Scope:            taskpkg.ScopeWorkspace,
				WorkspaceID:      string(created.WorkspaceID),
				RunKind:          taskpkg.RunKindCoordinator,
				ClaimerSessionID: "daemon-loop-coordinator-budget",
				ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
				LeaseDuration:    time.Minute,
				Now:              at,
			})
			if err != nil {
				t.Fatalf("ClaimNextRun(%s) error = %v", runID, err)
			}
			return claim
		}
		completeCoordinator := func(claim taskpkg.ClaimResult, at time.Time) {
			t.Helper()
			_, err := globalDB.CompleteCoordinatorAndEnqueueNext(ctx, taskpkg.CoordinatorCompletion{
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
				Actor:      actor,
				Plan: taskpkg.CoordinatorCompletionPlan{
					Snapshot: taskpkg.GenerationSnapshot{
						LoopRunID:  string(created.ID),
						Generation: created.Generation + 1,
					},
					Yield: true,
				},
				Now: at,
			}, looppkg.NewStoreFinalizer())
			if err != nil {
				t.Fatalf("CompleteCoordinatorAndEnqueueNext(%s) error = %v", claim.Run.ID, err)
			}
		}

		initialRunID := loopCoordinatorRunID(created.ID, created.Generation+1)
		completeCoordinator(claimCoordinator(initialRunID, now.Add(time.Second)), now.Add(2*time.Second))
		for attempt := 2; attempt <= taskpkg.DefaultTaskMaxAttempts; attempt++ {
			wakeRun, added, err := globalDB.EnqueueLoopCoordinatorWake(
				ctx,
				string(created.ID),
				fmt.Sprintf("coordinator-budget-wake-%d", attempt),
				actor.Origin,
				now.Add(time.Duration(attempt*2+1)*time.Second),
			)
			if err != nil {
				t.Fatalf("EnqueueLoopCoordinatorWake(%d) error = %v", attempt, err)
			}
			if !added {
				t.Fatalf("EnqueueLoopCoordinatorWake(%d) added = false, want true", attempt)
			}
			completeCoordinator(
				claimCoordinator(wakeRun.ID, now.Add(time.Duration(attempt*2+2)*time.Second)),
				now.Add(time.Duration(attempt*2+3)*time.Second),
			)
		}

		afterBudget, added, err := globalDB.EnqueueLoopCoordinatorWake(
			ctx,
			string(created.ID),
			"coordinator-budget-after-default",
			actor.Origin,
			now.Add(20*time.Second),
		)
		if err != nil {
			t.Fatalf("EnqueueLoopCoordinatorWake(after budget) error = %v", err)
		}
		if !added {
			t.Fatal("EnqueueLoopCoordinatorWake(after budget) added = false, want true")
		}
		if got, want := afterBudget.Attempt, int32(taskpkg.DefaultTaskMaxAttempts+1); got != want {
			t.Fatalf("after-budget attempt = %d, want %d", got, want)
		}
	})
}

func TestGlobalDBLoopRunShouldPreserveGoalPolicyAcrossReopen(t *testing.T) {
	t.Parallel()

	t.Run("Should load the original context nudge ratio after a database restart", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)
		globalDB, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		activeDB := globalDB
		t.Cleanup(func() {
			if activeDB == nil {
				return
			}
			if closeErr := activeDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close() error = %v", closeErr)
			}
		})
		seedLoopTestWorkspaces(t, globalDB, "ws-1")

		run := testLoopRun(
			"looprun-goal-policy-reopen",
			time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC),
			looppkg.StatusRunning,
		)
		run.GoalContextNudgeRatio = 0.23
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before reopen) error = %v", err)
		}
		activeDB = nil

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		activeDB = reopened
		persisted, err := reopened.GetLoopRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(after reopen) error = %v", err)
		}
		if persisted.GoalContextNudgeRatio != 0.23 {
			t.Fatalf("GoalContextNudgeRatio = %v, want pinned 0.23", persisted.GoalContextNudgeRatio)
		}
	})
}

func TestGlobalDBLoopRunStatusShouldUseCompareAndSwap(t *testing.T) {
	t.Parallel()

	t.Run("Should allow only one concurrent transition from the same status", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-cas", now, looppkg.StatusRunning)
		created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if created.Status != looppkg.StatusRunning {
			t.Fatalf("CreateLoopRunForStart() status = %s, want running", created.Status)
		}
		snapshot, err := globalDB.GetLoopDefinitionSnapshot(ctx, created.WorkspaceID, created.DefinitionDigest)
		if err != nil {
			t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
		}
		if snapshot.Digest != created.DefinitionDigest || snapshot.ByteSize != len(created.DefinitionSnapshot) {
			t.Fatalf("snapshot = %#v, want digest %q and byte size %d",
				snapshot,
				created.DefinitionDigest,
				len(created.DefinitionSnapshot),
			)
		}

		attempts := make([]error, 8)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				attempts[idx] = globalDB.CompareAndSwapLoopRunStatus(
					context.Background(),
					run.ID,
					looppkg.StatusRunning,
					looppkg.StatusPaused,
					looppkg.TransitionCausePauseBoundary,
					now.Add(time.Duration(idx)*time.Millisecond),
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		wins := 0
		conflicts := 0
		for idx, err := range attempts {
			if err == nil {
				wins++
				continue
			}
			if errors.Is(err, looppkg.ErrTransitionConflict) {
				conflicts++
				continue
			}
			t.Fatalf("attempt %d error = %v, want nil or ErrTransitionConflict", idx, err)
		}
		if wins != 1 {
			t.Fatalf("wins = %d, want 1", wins)
		}
		if conflicts != len(attempts)-1 {
			t.Fatalf("conflicts = %d, want %d", conflicts, len(attempts)-1)
		}
		stored, err := globalDB.GetLoopRun(ctx, "ws-1", run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if stored.Status != looppkg.StatusPaused {
			t.Fatalf("stored status = %q, want paused", stored.Status)
		}
		if got, want := stored.IterationCap, run.IterationCap; got != want {
			t.Fatalf("stored iteration cap = %d, want %d", got, want)
		}
		eventCount := countLoopRunEvents(ctx, t, globalDB, run.ID)
		if eventCount != 2 {
			t.Fatalf("status event count = %d, want create + transition events", eventCount)
		}
	})

	t.Run("Should ignore same-status compare-and-swap without appending an event", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 5, 0, 0, time.UTC)
		run := testLoopRun("looprun-cas-noop", now, looppkg.StatusRunning)
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			run.ID,
			looppkg.StatusRunning,
			looppkg.StatusRunning,
			looppkg.TransitionCauseApproval,
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(no-op) error = %v", err)
		}
		if eventCount := countLoopRunEvents(ctx, t, globalDB, run.ID); eventCount != 1 {
			t.Fatalf("status event count = %d, want only create event", eventCount)
		}
	})
}

func TestGlobalDBLoopDefinitionSnapshotShouldRejectDigestCollisions(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back a new Run when the workspace digest already owns different content", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		first := testLoopRun("looprun-snapshot-first", now, looppkg.StatusRunning)
		if _, err := globalDB.CreateLoopRunForStart(ctx, first, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(first) error = %v", err)
		}
		collidingJSON := `{"format_version":1,"different":true}`
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_definition_snapshots
			 SET definition_json = ?, byte_size = ?
			 WHERE workspace_id = ? AND definition_digest = ?`,
			collidingJSON,
			len(collidingJSON),
			string(first.WorkspaceID),
			first.DefinitionDigest,
		); err != nil {
			t.Fatalf("corrupt existing snapshot fixture error = %v", err)
		}

		second := first
		second.ID = "looprun-snapshot-second"
		second.CreatedAt = now.Add(time.Minute)
		second.StartedAt = second.CreatedAt
		second.LastProgressAt = second.CreatedAt
		_, err := globalDB.CreateLoopRunForStart(ctx, second, dsl.ConcurrencyAllow)
		if !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("CreateLoopRunForStart(collision) error = %v, want ErrValidation", err)
		}
		if _, err := globalDB.GetLoopRunByID(ctx, second.ID); !errors.Is(err, looppkg.ErrRunNotFound) {
			t.Fatalf("GetLoopRunByID(rolled back) error = %v, want ErrRunNotFound", err)
		}
	})
}

func TestGlobalDBLoopRunCreateShouldApplyConcurrencyPolicyAtomically(t *testing.T) {
	t.Parallel()

	t.Run("Should allow only one concurrent forbid start", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 15, 0, 0, time.UTC)
		attempts := make([]error, 8)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				run := testLoopRun(
					"looprun-forbid-"+time.Duration(idx).String(),
					now.Add(time.Duration(idx)*time.Millisecond),
					looppkg.StatusRunning,
				)
				_, attempts[idx] = globalDB.CreateLoopRunForStart(
					context.Background(),
					run,
					dsl.ConcurrencyForbid,
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		wins := 0
		conflicts := 0
		for idx, err := range attempts {
			if err == nil {
				wins++
				continue
			}
			if errors.Is(err, looppkg.ErrConcurrencyConflict) {
				conflicts++
				continue
			}
			t.Fatalf("attempt %d error = %v, want nil or ErrConcurrencyConflict", idx, err)
		}
		if wins != 1 {
			t.Fatalf("wins = %d, want 1", wins)
		}
		if conflicts != len(attempts)-1 {
			t.Fatalf("conflicts = %d, want %d", conflicts, len(attempts)-1)
		}
		running := countLoopRunsByStatus(ctx, t, globalDB, "ws-1", "delivery", looppkg.StatusRunning)
		if running != 1 {
			t.Fatalf("running loop_runs = %d, want 1", running)
		}
	})

	t.Run("Should queue concurrent queue starts after the first running run", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 4, 14, 20, 0, 0, time.UTC)
		attempts := make([]error, 8)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(attempts))
		for idx := range attempts {
			go func(idx int) {
				defer wg.Done()
				<-start
				run := testLoopRun(
					"looprun-queue-"+time.Duration(idx).String(),
					now.Add(time.Duration(idx)*time.Millisecond),
					looppkg.StatusRunning,
				)
				_, attempts[idx] = globalDB.CreateLoopRunForStart(
					context.Background(),
					run,
					dsl.ConcurrencyQueue,
				)
			}(idx)
		}
		close(start)
		wg.Wait()

		for idx, err := range attempts {
			if err != nil {
				t.Fatalf("attempt %d error = %v, want nil", idx, err)
			}
		}
		running := countLoopRunsByStatus(ctx, t, globalDB, "ws-1", "delivery", looppkg.StatusRunning)
		if running != 1 {
			t.Fatalf("running loop_runs = %d, want 1", running)
		}
		queued := countLoopRunsByStatus(ctx, t, globalDB, "ws-1", "delivery", looppkg.StatusQueued)
		if queued != len(attempts)-1 {
			t.Fatalf("queued loop_runs = %d, want %d", queued, len(attempts)-1)
		}
	})
}

func testLoopRun(id string, at time.Time, status looppkg.Status) looppkg.Run {
	definition, err := dsl.Parse([]byte(
		`{"apiVersion":"agh.loop/v1","kind":"Loop","meta":{"name":"delivery","version":1},"contract":{"goal":"test","definition_of_done":"done","iteration_cap":7,"no_progress":{"window":1},"budget":{"tokens":1,"wall_clock_sec":1}},"graph":{"nodes":[{"id":"finish","class":"action","kind":"transform","params":{"map":{"ok":{"value":true}}}}],"edges":[]}}`,
	))
	if err != nil {
		panic(fmt.Sprintf("parse test Loop definition: %v", err))
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		panic(fmt.Sprintf("compile test Loop definition: %v", err))
	}
	effective, err := looppkg.ResolveEffectiveConfig(
		resolved,
		looppkg.DefaultLoopDefaults(),
		nil,
		looppkg.LoopConfig{},
	)
	if err != nil {
		panic(fmt.Sprintf("resolve test Loop config: %v", err))
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		panic(fmt.Sprintf("build test executed definition snapshot: %v", err))
	}
	return looppkg.Run{
		ID:                  looppkg.RunID(id),
		WorkspaceID:         "ws-1",
		LoopName:            "delivery",
		Status:              status,
		ReattemptStrategy:   looppkg.ReattemptFailedOnly,
		CreatedAt:           at,
		StartedAt:           at,
		LastProgressAt:      at,
		DefinitionVersion:   resolved.DefinitionVersion,
		DefinitionDigest:    digest,
		DefinitionSnapshot:  snapshot,
		ActiveHumanCriteria: []byte(`[]`),
		StartMetadata:       map[string]any{},
		IterationCap:        7,
		BudgetOnExceeded:    dsl.BudgetExceededHalt,
		Origin:              &looppkg.RunOrigin{Kind: looppkg.RunOriginCatalog},
		Inputs:              map[string]any{"tasks": "task-ref"},
	}
}

func countLoopRunsByStatus(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	workspaceID looppkg.WorkspaceID,
	loopName string,
	status looppkg.Status,
) int {
	t.Helper()
	var count int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM loop_runs WHERE workspace_id = ? AND loop_name = ? AND status = ?`,
		string(workspaceID),
		loopName,
		string(status),
	).Scan(&count); err != nil {
		t.Fatalf("count loop_runs by status error = %v", err)
	}
	return count
}

func countLoopRunEvents(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	runID looppkg.RunID,
) int {
	t.Helper()
	var count int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM loop_run_events WHERE loop_run_id = ?`,
		string(runID),
	).Scan(&count); err != nil {
		t.Fatalf("count loop_run_events error = %v", err)
	}
	return count
}

func countCoordinatorTaskRunsForLoop(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	runID looppkg.RunID,
) int {
	t.Helper()
	var count int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM task_runs WHERE loop_run_id = ? AND run_kind = 'coordinator'`,
		string(runID),
	).Scan(&count); err != nil {
		t.Fatalf("count coordinator task runs error = %v", err)
	}
	return count
}

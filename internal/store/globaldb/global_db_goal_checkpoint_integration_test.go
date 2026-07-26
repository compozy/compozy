//go:build integration

package globaldb

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGoalCheckpointControlIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should fence control mutation by the exact checkpoint owner tuple", func(t *testing.T) {
		t.Parallel()

		globalDB, key, _, _ := seedGoalTurnRuntime(t, "run-goal-control-owner")
		ctx := testutil.Context(t)
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		request := goal.ControlCheckpointRequest{
			Key:                  key,
			ExpectedControlEpoch: checkpoint.ControlEpoch,
			ExpectedBindingEpoch: checkpoint.BindingEpoch,
			ExpectedPhase:        checkpoint.Phase,
			TaskRunID:            checkpoint.TaskRunID,
			QueueEntryID:         checkpoint.QueueEntryID,
			PromptID:             checkpoint.PromptID,
			Disposition:          looppkg.ActionDispositionPaused,
			Status:               "paused",
			Cause:                looppkg.ReasonCode(looppkg.TransitionCausePauseBoundary),
			ActorKind:            "user",
			ActorID:              "operator-control-owner",
		}
		stale := request
		stale.PromptID = "prompt-stale-owner"
		if err := globalDB.CheckpointControl(ctx, stale); err == nil {
			t.Fatal("CheckpointControl(stale prompt owner) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
		}
		invalid := request
		invalid.Status = "complete"
		if err := globalDB.CheckpointControl(ctx, invalid); !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("CheckpointControl(invalid mapping) error = %v, want ErrValidation", err)
		}
		if err := globalDB.CheckpointControl(ctx, request); err != nil {
			t.Fatalf("CheckpointControl() error = %v", err)
		}
		if err := globalDB.CheckpointControl(ctx, request); err == nil {
			t.Fatal("CheckpointControl(post-commit replay) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
		}
		staleDisposition := request
		staleDisposition.Disposition = looppkg.ActionDispositionNeedsApproval
		if err := globalDB.CheckpointControl(ctx, staleDisposition); err == nil {
			t.Fatal("CheckpointControl(post-commit disposition change) error = nil")
		}
		persisted, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(after control) error = %v", err)
		}
		if persisted.Phase != "awaiting_control" || persisted.Status != "paused" ||
			persisted.ControlActorID != request.ActorID || persisted.ControlRequestedAt == nil {
			t.Fatalf("persisted control = %#v", persisted)
		}
	})

	t.Run("Should preserve the first pause writer and reject a pause after terminal status", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		firstAt := time.Date(2026, 7, 11, 7, 15, 0, 0, time.UTC)
		run, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("run-goal-pause-cas", firstAt.Add(-time.Minute), looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		firstActor := operatorActorContextForTest("operator-first-pause")
		globalDB.now = func() time.Time { return firstAt }
		if err := globalDB.SetLoopRunPauseRequested(ctx, run.WorkspaceID, run.ID, true, firstActor); err != nil {
			t.Fatalf("SetLoopRunPauseRequested(first) error = %v", err)
		}
		globalDB.now = func() time.Time { return firstAt.Add(time.Minute) }
		if err := globalDB.SetLoopRunPauseRequested(
			ctx,
			run.WorkspaceID,
			run.ID,
			true,
			operatorActorContextForTest("operator-second-pause"),
		); !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("SetLoopRunPauseRequested(competing writer) error = %v, want transition conflict", err)
		}
		if err := globalDB.SetLoopRunPauseRequested(ctx, run.WorkspaceID, run.ID, true, firstActor); err != nil {
			t.Fatalf("SetLoopRunPauseRequested(idempotent first writer) error = %v", err)
		}
		persisted, err := globalDB.GetLoopRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if !persisted.ControlRequestedAt.Equal(firstAt) ||
			persisted.ControlActor.Ref != firstActor.Actor.Ref {
			t.Fatalf("persisted pause writer = %#v, want first writer at %v", persisted, firstAt)
		}

		terminalRun, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("run-goal-late-pause", firstAt, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart(terminal race) error = %v", err)
		}
		if err := globalDB.SetLoopRunPauseRequested(
			ctx,
			terminalRun.WorkspaceID,
			terminalRun.ID,
			true,
			firstActor,
		); err != nil {
			t.Fatalf("SetLoopRunPauseRequested(before terminal) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			terminalRun.ID,
			looppkg.StatusRunning,
			looppkg.StatusFailed,
			looppkg.TransitionCauseOperatorStop,
			firstAt.Add(time.Minute),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus() error = %v", err)
		}
		terminalPersisted, err := globalDB.GetLoopRun(ctx, terminalRun.WorkspaceID, terminalRun.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(terminal) error = %v", err)
		}
		if terminalPersisted.PauseRequested || terminalPersisted.ControlActor.Ref != "" ||
			!terminalPersisted.ControlRequestedAt.IsZero() {
			t.Fatalf("terminal Run retained pause actor fields: %#v", terminalPersisted)
		}
		if err := globalDB.SetLoopRunPauseRequested(
			ctx,
			terminalRun.WorkspaceID,
			terminalRun.ID,
			true,
			firstActor,
		); !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("SetLoopRunPauseRequested(after terminal) error = %v, want transition conflict", err)
		}
	})

	t.Run("Should reject incoherent grant kind and scope pairs", func(t *testing.T) {
		t.Parallel()

		base := goal.GrantRequest{
			Key: goal.TurnKey{
				WorkspaceID: "ws-goal-grant-validation",
				LoopRunID:   "run-goal-grant-validation",
				Generation:  1,
				NodeID:      "converge",
			},
			ExpectedControlEpoch: 1,
			Cause:                looppkg.ReasonCodeGoalBudgetFenced,
			Turn:                 1,
			ActorKind:            "user",
			ActorID:              "operator-1",
		}
		cases := []struct {
			name          string
			kind          goal.ControlGrantKind
			scope         goal.ControlGrantScope
			turnIncrement int
			wantValid     bool
		}{
			{
				name:          "Should accept turn extension turn limit",
				kind:          goal.ControlGrantTurnExtension,
				scope:         goal.ControlGrantScopeTurnLimit,
				turnIncrement: 1,
				wantValid:     true,
			},
			{
				name:      "Should accept budget settle current",
				kind:      goal.ControlGrantBudget,
				scope:     goal.ControlGrantScopeSettleCurrent,
				wantValid: true,
			},
			{
				name:      "Should accept budget work and settle",
				kind:      goal.ControlGrantBudget,
				scope:     goal.ControlGrantScopeWorkAndSettle,
				wantValid: true,
			},
			{
				name:      "Should accept reseed rotation",
				kind:      goal.ControlGrantReseed,
				scope:     goal.ControlGrantScopeRotateBinding,
				wantValid: true,
			},
			{
				name:      "Should accept plain resume reactivation",
				kind:      goal.ControlGrantPlainResume,
				scope:     goal.ControlGrantScopeReactivate,
				wantValid: true,
			},
			{
				name:          "Should reject turn extension settle current",
				kind:          goal.ControlGrantTurnExtension,
				scope:         goal.ControlGrantScopeSettleCurrent,
				turnIncrement: 1,
			},
			{
				name:  "Should reject budget turn limit",
				kind:  goal.ControlGrantBudget,
				scope: goal.ControlGrantScopeTurnLimit,
			},
			{
				name:  "Should reject reseed reactivation",
				kind:  goal.ControlGrantReseed,
				scope: goal.ControlGrantScopeReactivate,
			},
			{
				name:  "Should reject resume rotation",
				kind:  goal.ControlGrantPlainResume,
				scope: goal.ControlGrantScopeRotateBinding,
			},
			{
				name:  "Should reject unknown kind",
				kind:  goal.ControlGrantKind("unknown"),
				scope: goal.ControlGrantScopeReactivate,
			},
			{
				name:  "Should reject unknown scope",
				kind:  goal.ControlGrantBudget,
				scope: goal.ControlGrantScope("unknown"),
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				request := base
				request.Kind = tc.kind
				request.Scope = tc.scope
				request.TurnIncrement = tc.turnIncrement
				err := validateGoalGrantRequest(request)
				if tc.wantValid && err != nil {
					t.Fatalf("validateGoalGrantRequest() error = %v", err)
				}
				if !tc.wantValid && !errors.Is(err, looppkg.ErrValidation) {
					t.Fatalf("validateGoalGrantRequest() error = %v, want %v", err, looppkg.ErrValidation)
				}
			})
		}
	})

	t.Run("Should persist complete checkpoint shape and advance one typed grant by CAS", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-checkpoint", "ws-goal-checkpoint-foreign")
		insertGoalSchemaLoopRun(t, globalDB, "run-checkpoint", "ws-goal-checkpoint", "catalog", nil)
		now := time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC)
		usageSequence := int64(0)
		key := goal.TurnKey{
			WorkspaceID: "ws-goal-checkpoint",
			LoopRunID:   "run-checkpoint",
			Generation:  1,
			NodeID:      "converge",
			ItemIndex:   0,
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status,
				goal_status, goal_turns_used, goal_turn_limit
			) VALUES (?, 1, 'converge', 0, 'running', 'paused', 2, 10)`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("insert Goal grant output error = %v", err)
		}
		created, err := globalDB.CreateCheckpoint(
			testutil.Context(t),
			goal.CreateCheckpointRequest{Checkpoint: goal.Checkpoint{
				Key:               key,
				ControlEpoch:      1,
				Phase:             "awaiting_control",
				Status:            "paused",
				TurnsUsed:         2,
				TurnLimit:         10,
				BrokenStreak:      1,
				RecoveryStreak:    1,
				ContextState:      "known",
				UsageSequence:     &usageSequence,
				ContextNudgeRatio: 0.75,
				ControlActorKind:  "user",
				ControlActorID:    "operator-1",
				ControlRequestedAt: func() *time.Time {
					value := now
					return &value
				}(),
				UpdatedAt: now,
			}},
		)
		if err != nil {
			t.Fatalf("CreateCheckpoint() error = %v", err)
		}
		if created.ContextNudgeRatio != 0.75 || created.UsageSequence == nil || *created.UsageSequence != 0 {
			t.Fatalf("created checkpoint = %#v, want pinned known usage and ratio", created)
		}

		if err := globalDB.GrantAndReactivate(testutil.Context(t), goal.GrantRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TurnIncrement:        10,
			Kind:                 goal.ControlGrantTurnExtension,
			Cause:                "goal_turns_exhausted",
			Turn:                 2,
			Scope:                goal.ControlGrantScopeTurnLimit,
			ActorKind:            "user",
			ActorID:              "operator-1",
		}); err != nil {
			t.Fatalf("GrantAndReactivate() error = %v", err)
		}
		loaded, err := globalDB.LoadCheckpoint(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if loaded.ControlEpoch != 2 || loaded.Phase != "idle" || loaded.Status != "active" || loaded.TurnLimit != 20 {
			t.Fatalf("reactivated checkpoint = %#v, want epoch 2 active limit 20", loaded)
		}
		if loaded.ControlGrant == nil || loaded.ControlGrant.ID != 1 ||
			loaded.ControlGrant.Kind != goal.ControlGrantTurnExtension ||
			loaded.ControlGrant.Scope != goal.ControlGrantScopeTurnLimit || !loaded.ControlGrant.Consumed {
			t.Fatalf("control grant = %#v, want complete consumed turn extension", loaded.ControlGrant)
		}
		if err := globalDB.GrantAndReactivate(testutil.Context(t), goal.GrantRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			Kind:                 goal.ControlGrantPlainResume,
			Cause:                "operator_resume",
			Turn:                 2,
			Scope:                goal.ControlGrantScopeReactivate,
			ActorKind:            "user",
			ActorID:              "operator-1",
		}); err == nil {
			t.Fatal("GrantAndReactivate(stale epoch) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
		}

		foreign := key
		foreign.WorkspaceID = "ws-goal-checkpoint-foreign"
		if _, err := globalDB.LoadCheckpoint(testutil.Context(t), foreign); err == nil {
			t.Fatal("LoadCheckpoint(foreign workspace) error = nil")
		}
	})

	t.Run("Should inherit a pause that commits before the initial checkpoint", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-pause-before-checkpoint")
		originSessionID := "session-goal-pause-before-checkpoint"
		registeredAt := time.Date(2026, 7, 10, 20, 59, 0, 0, time.UTC)
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			goalSessionInfoForTest(originSessionID, "ws-goal-pause-before-checkpoint", registeredAt),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-pause-before-checkpoint",
				PolicySpecDigest:   "policy-pause-before-checkpoint",
				CreationDigest:     "creation-pause-before-checkpoint",
			},
		)
		insertGoalSchemaLoopRun(
			t,
			globalDB,
			"run-goal-pause-before-checkpoint",
			"ws-goal-pause-before-checkpoint",
			"session",
			&originSessionID,
		)
		ctx := testutil.Context(t)
		actor := operatorActorContextForTest("operator-pause-before-checkpoint")
		if err := globalDB.SetLoopRunPauseRequested(
			ctx,
			"ws-goal-pause-before-checkpoint",
			"run-goal-pause-before-checkpoint",
			true,
			actor,
		); err != nil {
			t.Fatalf("SetLoopRunPauseRequested() error = %v", err)
		}
		checkpoint, err := globalDB.CreateCheckpoint(ctx, goal.CreateCheckpointRequest{Checkpoint: goal.Checkpoint{
			Key: goal.TurnKey{
				WorkspaceID: "ws-goal-pause-before-checkpoint",
				LoopRunID:   "run-goal-pause-before-checkpoint",
				Generation:  1,
				NodeID:      "goal",
			},
			ControlEpoch:      1,
			Phase:             "idle",
			Status:            "active",
			TurnLimit:         20,
			ContextState:      "unknown",
			ContextNudgeRatio: 0.8,
			UpdatedAt:         time.Date(2026, 7, 10, 21, 0, 0, 0, time.UTC),
		}})
		if err != nil {
			t.Fatalf("CreateCheckpoint() error = %v", err)
		}
		if checkpoint.ControlActorKind != string(actor.Actor.Kind.Normalize()) ||
			checkpoint.ControlActorID != actor.Actor.Ref || checkpoint.ControlRequestedAt == nil {
			t.Fatalf("checkpoint pause projection = %#v", checkpoint)
		}
		if checkpoint.Phase != "awaiting_control" || checkpoint.Status != "paused" ||
			checkpoint.ControlCause != looppkg.ReasonCode(looppkg.TransitionCausePauseBoundary) {
			t.Fatalf("checkpoint pause state = %#v", checkpoint)
		}
		pending, err := globalDB.ClaimGoalSessionOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox() error = %v", err)
		}
		if len(pending) != 1 || pending[0].BoundSessionID == nil ||
			*pending[0].BoundSessionID != originSessionID {
			t.Fatalf("pre-prompt pause outbox = %#v", pending)
		}
	})

	t.Run("Should advance context freshness only for a newer exact-binding observation", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-context", "ws-goal-context-foreign")
		insertGoalSchemaLoopRun(t, globalDB, "run-context", "ws-goal-context", "catalog", nil)
		now := time.Date(2026, 7, 10, 15, 30, 0, 0, time.UTC)
		key := goal.TurnKey{
			WorkspaceID: "ws-goal-context",
			LoopRunID:   "run-context",
			Generation:  1,
			NodeID:      "converge",
		}
		seedActiveGoalBindingForTest(
			t,
			globalDB,
			string(key.LoopRunID),
			string(key.WorkspaceID),
			"goal:context",
			2,
			"session-context",
			now,
		)
		if _, err := globalDB.CreateCheckpoint(testutil.Context(t), goal.CreateCheckpointRequest{
			Checkpoint: goal.Checkpoint{
				Key: key, ControlEpoch: 3, Phase: "idle", Status: "active", TurnLimit: 10,
				SessionID: "session-context", BindingHandle: "goal:context", BindingEpoch: 2,
				ContextState: "unknown", ContextNudgeRatio: 0.8, UpdatedAt: now,
			},
		}); err != nil {
			t.Fatalf("CreateCheckpoint(context) error = %v", err)
		}
		request := goal.RecordContextUsageRequest{
			Key: key, ExpectedControlEpoch: 3, ExpectedBindingEpoch: 2, ExpectedPhase: "idle",
			SessionID: "session-context", BindingHandle: "goal:context",
			Usage: goal.ContextUsage{Known: true, Used: 8, Size: 10, Sequence: 10, ReportedAt: now},
		}
		known, err := globalDB.RecordContextUsage(testutil.Context(t), request)
		if err != nil {
			t.Fatalf("RecordContextUsage(unknown to known) error = %v", err)
		}
		if known.ContextState != "known" || known.UsageSequence == nil || *known.UsageSequence != 10 ||
			known.UsagePendingAfterSequence != nil {
			t.Fatalf("known context checkpoint = %#v", known)
		}

		stale := request
		stale.Usage.Sequence = 9
		unchanged, err := globalDB.RecordContextUsage(testutil.Context(t), stale)
		if err != nil {
			t.Fatalf("RecordContextUsage(stale known) error = %v", err)
		}
		if unchanged.UsageSequence == nil || *unchanged.UsageSequence != 10 {
			t.Fatalf("stale observation checkpoint = %#v", unchanged)
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_goal_checkpoints
			 SET context_state = 'pending', usage_pending_after_sequence = usage_sequence,
			     compaction_baseline_used = 8, compaction_recovery_required = 0, recovery_streak = 1
			 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
			string(key.LoopRunID), key.Generation, string(key.NodeID), key.ItemIndex,
		); err != nil {
			t.Fatalf("set pending context error = %v", err)
		}
		atFloor := request
		pending, err := globalDB.RecordContextUsage(testutil.Context(t), atFloor)
		if err != nil {
			t.Fatalf("RecordContextUsage(at floor) error = %v", err)
		}
		if pending.ContextState != "pending" || pending.UsagePendingAfterSequence == nil ||
			*pending.UsagePendingAfterSequence != 10 {
			t.Fatalf("pending context checkpoint = %#v", pending)
		}
		fresh := request
		fresh.Usage.Sequence = 11
		fresh.Usage.Used = 3
		fresh.Usage.ReportedAt = now.Add(time.Second)
		refreshed, err := globalDB.RecordContextUsage(testutil.Context(t), fresh)
		if err != nil {
			t.Fatalf("RecordContextUsage(newer than floor) error = %v", err)
		}
		if refreshed.ContextState != "known" || refreshed.UsageSequence == nil ||
			*refreshed.UsageSequence != 11 || refreshed.UsagePendingAfterSequence != nil ||
			refreshed.CompactionBaselineUsed != nil || refreshed.CompactionRecoveryRequired ||
			refreshed.RecoveryStreak != 0 {
			t.Fatalf("refreshed context checkpoint = %#v", refreshed)
		}

		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_goal_checkpoints
			 SET context_state = 'pending', usage_pending_after_sequence = usage_sequence,
			     compaction_baseline_used = 3, compaction_recovery_required = 0, recovery_streak = 1
			 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
			string(key.LoopRunID), key.Generation, string(key.NodeID), key.ItemIndex,
		); err != nil {
			t.Fatalf("set second pending context error = %v", err)
		}
		nonDecreasing := fresh
		nonDecreasing.Usage.Sequence = 12
		nonDecreasing.Usage.Used = 4
		nonDecreasing.Usage.ReportedAt = now.Add(2 * time.Second)
		ineffective, err := globalDB.RecordContextUsage(testutil.Context(t), nonDecreasing)
		if err != nil {
			t.Fatalf("RecordContextUsage(non-decreasing) error = %v", err)
		}
		if ineffective.ContextState != "known" || !ineffective.CompactionRecoveryRequired ||
			ineffective.CompactionBaselineUsed != nil || ineffective.RecoveryStreak != 1 {
			t.Fatalf("ineffective compaction recovery = %#v", ineffective)
		}

		wrongPhase := fresh
		wrongPhase.ExpectedPhase = "prompting"
		if _, err := globalDB.RecordContextUsage(testutil.Context(t), wrongPhase); err == nil {
			t.Fatal("RecordContextUsage(wrong phase) error = nil")
		}
		wrongSession := fresh
		wrongSession.SessionID = "session-other"
		if _, err := globalDB.RecordContextUsage(testutil.Context(t), wrongSession); err == nil {
			t.Fatal("RecordContextUsage(wrong session) error = nil")
		}
		foreign := fresh
		foreign.Key.WorkspaceID = "ws-goal-context-foreign"
		if _, err := globalDB.RecordContextUsage(testutil.Context(t), foreign); err == nil {
			t.Fatal("RecordContextUsage(foreign workspace) error = nil")
		}
	})

	t.Run("Should reject invalid context observations before persistence", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 10, 15, 45, 0, 0, time.UTC)
		base := goal.RecordContextUsageRequest{
			Key: goal.TurnKey{
				WorkspaceID: "ws-goal-context-validation", LoopRunID: "run-context-validation",
				Generation: 1, NodeID: "converge",
			},
			ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, ExpectedPhase: "idle",
			SessionID: "session-context", BindingHandle: "goal:context",
			Usage: goal.ContextUsage{Known: true, Used: 8, Size: 10, Sequence: 1, ReportedAt: now},
		}
		cases := []struct {
			name   string
			mutate func(*goal.RecordContextUsageRequest)
		}{
			{
				name:   "Should reject unknown usage",
				mutate: func(req *goal.RecordContextUsageRequest) { req.Usage.Known = false },
			},
			{
				name:   "Should reject negative used tokens",
				mutate: func(req *goal.RecordContextUsageRequest) { req.Usage.Used = -1 },
			},
			{
				name:   "Should reject zero context size",
				mutate: func(req *goal.RecordContextUsageRequest) { req.Usage.Size = 0 },
			},
			{
				name:   "Should reject negative sequence",
				mutate: func(req *goal.RecordContextUsageRequest) { req.Usage.Sequence = -1 },
			},
			{
				name:   "Should reject missing report time",
				mutate: func(req *goal.RecordContextUsageRequest) { req.Usage.ReportedAt = time.Time{} },
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				request := base
				tc.mutate(&request)
				if err := validateGoalContextUsageRequest(request); !errors.Is(err, looppkg.ErrValidation) {
					t.Fatalf("validateGoalContextUsageRequest() error = %v, want %v", err, looppkg.ErrValidation)
				}
			})
		}
	})
}

func TestGoalCheckpointIntentsIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should atomically materialize report evidence for the active bound prompt", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-tool-report")
		originSessionID := "session-origin"
		insertGoalSchemaLoopRun(
			t,
			globalDB,
			"run-tool-report",
			"ws-goal-tool-report",
			"session",
			&originSessionID,
		)
		now := time.Date(2026, 7, 10, 15, 30, 0, 0, time.UTC)
		key := goal.TurnKey{
			WorkspaceID: "ws-goal-tool-report",
			LoopRunID:   "run-tool-report",
			Generation:  2,
			NodeID:      "goal",
		}
		seedActiveGoalBindingForTest(
			t,
			globalDB,
			"run-tool-report",
			"ws-goal-tool-report",
			"goal:tool-report",
			3,
			"session-bound",
			now,
		)
		if _, err := globalDB.CreateCheckpoint(testutil.Context(t), goal.CreateCheckpointRequest{
			Checkpoint: goal.Checkpoint{
				Key: key, ControlEpoch: 5, Phase: "prompting", Status: "active",
				TurnsUsed: 1, TurnLimit: 6, TaskRunID: "task-tool-report",
				QueueEntryID: "queue-tool-report", PromptID: "prompt-tool-report",
				PromptKind: "work", SessionID: "session-bound", BindingHandle: "goal:tool-report",
				BindingEpoch: 3, ContextState: "unknown", ContextNudgeRatio: 0.8, UpdatedAt: now,
			},
		}); err != nil {
			t.Fatalf("CreateCheckpoint(tool report) error = %v", err)
		}

		target, found, err := globalDB.FindGoalReportTarget(
			testutil.Context(t),
			"ws-goal-tool-report",
			"session-bound",
		)
		if err != nil {
			t.Fatalf("FindGoalReportTarget() error = %v", err)
		}
		if !found || target.Key != key || target.ExpectedControlEpoch != 5 ||
			target.ExpectedBindingEpoch != 3 || target.PromptID != "prompt-tool-report" ||
			target.OriginSessionID != originSessionID || target.BoundSessionID != "session-bound" {
			t.Fatalf("FindGoalReportTarget() = %#v, found=%t", target, found)
		}
		resolvedOrigin, aliasFound, err := globalDB.ResolveActiveGoalOriginAlias(
			testutil.Context(t),
			"ws-goal-tool-report",
			"session-bound",
		)
		if err != nil {
			t.Fatalf("ResolveActiveGoalOriginAlias() error = %v", err)
		}
		if !aliasFound || resolvedOrigin != originSessionID {
			t.Fatalf("ResolveActiveGoalOriginAlias() = %q, found=%t", resolvedOrigin, aliasFound)
		}

		report := goal.RecordToolReportRequest{
			Target: target, Status: "blocked", Evidence: "release approval is pending",
			ActorKind: "agent_session", ActorID: "session-bound",
		}
		first, err := globalDB.RecordGoalReport(testutil.Context(t), report)
		if err != nil {
			t.Fatalf("RecordGoalReport(first) error = %v", err)
		}
		second, err := globalDB.RecordGoalReport(testutil.Context(t), report)
		if err != nil {
			t.Fatalf("RecordGoalReport(second) error = %v", err)
		}
		if first != second || first.EvidenceRef == "" {
			t.Fatalf("RecordGoalReport repeats = %#v/%#v", first, second)
		}
		payload, err := getLoopOutputByRefWithExecutor(
			testutil.Context(t),
			globalDB.db,
			first.EvidenceRef,
		)
		if err != nil {
			t.Fatalf("getLoopOutputByRefWithExecutor(report) error = %v", err)
		}
		var evidence string
		if err := json.Unmarshal(payload, &evidence); err != nil {
			t.Fatalf("decode report evidence error = %v", err)
		}
		if evidence != report.Evidence {
			t.Fatalf("report evidence = %q, want %q", evidence, report.Evidence)
		}

		conflict := report
		conflict.Evidence = "different evidence"
		conflictPayload, err := json.Marshal(conflict.Evidence)
		if err != nil {
			t.Fatalf("marshal conflicting evidence error = %v", err)
		}
		conflictRef := looppkg.OutputRefForPayload(conflictPayload)
		if _, err := globalDB.RecordGoalReport(testutil.Context(t), conflict); err == nil {
			t.Fatal("RecordGoalReport(conflict) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalReportConflict)
		}
		if _, err := getLoopOutputByRefWithExecutor(
			testutil.Context(t),
			globalDB.db,
			conflictRef,
		); !errors.Is(err, looppkg.ErrOutputRefNotFound) {
			t.Fatalf("conflicting evidence blob error = %v, want not found", err)
		}

		oversized := report
		oversized.Evidence = strings.Repeat("é", goal.MaxReportEvidenceBytes/2+1)
		if _, err := globalDB.RecordGoalReport(testutil.Context(t), oversized); err == nil {
			t.Fatal("RecordGoalReport(oversized) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalEvidenceTooLarge)
		}

		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = 'run-tool-report' AND handle = 'goal:tool-report' AND binding_epoch = 3`,
			now.Add(time.Second).Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("close tool report binding error = %v", err)
		}
		if _, found, err := globalDB.FindGoalReportTarget(
			testutil.Context(t),
			"ws-goal-tool-report",
			"session-bound",
		); err != nil || found {
			t.Fatalf("FindGoalReportTarget(closed) found=%t error=%v", found, err)
		}
		if _, err := globalDB.RecordGoalReport(testutil.Context(t), report); err == nil {
			t.Fatal("RecordGoalReport(closed) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalNotActive)
		}
	})

	t.Run("Should dedupe identical report intent and reject conflicting or stale reports", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-report")
		insertGoalSchemaLoopRun(t, globalDB, "run-report", "ws-goal-report", "catalog", nil)
		now := time.Date(2026, 7, 10, 16, 0, 0, 0, time.UTC)
		key := goal.TurnKey{
			WorkspaceID: "ws-goal-report",
			LoopRunID:   "run-report",
			Generation:  1,
			NodeID:      "converge",
			ItemIndex:   0,
		}
		seedActiveGoalBindingForTest(
			t,
			globalDB,
			"run-report",
			"ws-goal-report",
			"goal:report",
			2,
			"session-report",
			now,
		)
		if _, err := globalDB.CreateCheckpoint(testutil.Context(t), goal.CreateCheckpointRequest{
			Checkpoint: goal.Checkpoint{
				Key:               key,
				ControlEpoch:      4,
				Phase:             "prompting",
				Status:            "active",
				TurnsUsed:         1,
				TurnLimit:         10,
				TaskRunID:         "task-run-report",
				QueueEntryID:      "queue-report",
				PromptID:          "prompt-report",
				PromptKind:        "work",
				SessionID:         "session-report",
				BindingHandle:     "goal:report",
				BindingEpoch:      2,
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
				UpdatedAt:         now,
			},
		}); err != nil {
			t.Fatalf("CreateCheckpoint(report) error = %v", err)
		}
		req := goal.RecordReportIntentRequest{
			Key:                  key,
			ExpectedControlEpoch: 4,
			ExpectedBindingEpoch: 2,
			PromptID:             "prompt-report",
			Status:               "blocked",
			EvidenceRef:          "blob:evidence",
			ActorKind:            "agent",
			ActorID:              "session-report",
		}
		first, err := globalDB.RecordReportIntent(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("RecordReportIntent(first) error = %v", err)
		}
		second, err := globalDB.RecordReportIntent(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("RecordReportIntent(second) error = %v", err)
		}
		if first.PromptID != second.PromptID || first.Status != second.Status || first.RecordedAt != second.RecordedAt {
			t.Fatalf("report repeats = %#v/%#v, want identical intent", first, second)
		}
		conflict := req
		conflict.Status = "complete"
		conflict.EvidenceRef = ""
		if _, err := globalDB.RecordReportIntent(testutil.Context(t), conflict); err == nil {
			t.Fatal("RecordReportIntent(conflict) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalReportConflict)
		}
		prior := req
		prior.PromptID = "prompt-prior"
		if _, err := globalDB.RecordReportIntent(testutil.Context(t), prior); err == nil {
			t.Fatal("RecordReportIntent(prior prompt) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalNotActive)
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = 'run-report' AND handle = 'goal:report' AND binding_epoch = 2`,
			now.Add(time.Second).Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("close report binding error = %v", err)
		}
		if _, err := globalDB.RecordReportIntent(testutil.Context(t), req); err == nil {
			t.Fatal("RecordReportIntent(closed binding) error = nil")
		}
	})

	t.Run("Should persist one compaction timeout cancel intent idempotently", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-cancel")
		insertGoalSchemaLoopRun(t, globalDB, "run-cancel", "ws-goal-cancel", "catalog", nil)
		now := time.Date(2026, 7, 10, 17, 0, 0, 0, time.UTC)
		key := goal.TurnKey{
			WorkspaceID: "ws-goal-cancel",
			LoopRunID:   "run-cancel",
			Generation:  1,
			NodeID:      "converge",
		}
		seedActiveGoalBindingForTest(
			t,
			globalDB,
			"run-cancel",
			"ws-goal-cancel",
			"goal:cancel",
			5,
			"session-cancel",
			now,
		)
		if _, err := globalDB.CreateCheckpoint(testutil.Context(t), goal.CreateCheckpointRequest{
			Checkpoint: goal.Checkpoint{
				Key:               key,
				ControlEpoch:      3,
				Phase:             "compacting",
				Status:            "active",
				TurnLimit:         10,
				TaskRunID:         "task-run-cancel",
				QueueEntryID:      "queue-cancel",
				PromptID:          "prompt-cancel",
				PromptKind:        "compact",
				SessionID:         "session-cancel",
				BindingHandle:     "goal:cancel",
				BindingEpoch:      5,
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
				UpdatedAt:         now,
			},
		}); err != nil {
			t.Fatalf("CreateCheckpoint(cancel) error = %v", err)
		}
		req := goal.BeginCompactionCancelRequest{
			Key:                  key,
			ExpectedControlEpoch: 3,
			ExpectedBindingEpoch: 5,
			TaskRunID:            "task-run-cancel",
			QueueEntryID:         "queue-cancel",
			PromptID:             "prompt-cancel",
			Cause:                "timeout",
		}
		if err := globalDB.BeginCompactionCancel(testutil.Context(t), req); err != nil {
			t.Fatalf("BeginCompactionCancel(first) error = %v", err)
		}
		if err := globalDB.BeginCompactionCancel(testutil.Context(t), req); err != nil {
			t.Fatalf("BeginCompactionCancel(second) error = %v", err)
		}
		loaded, err := globalDB.LoadCheckpoint(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(cancel) error = %v", err)
		}
		if loaded.CompactionCancel == nil || loaded.CompactionCancel.PromptID != "prompt-cancel" ||
			loaded.CompactionCancel.Cause != "timeout" {
			t.Fatalf("compaction cancel = %#v, want persisted timeout", loaded.CompactionCancel)
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = 'run-cancel' AND handle = 'goal:cancel' AND binding_epoch = 5`,
			now.Add(time.Second).Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("close compaction-cancel binding error = %v", err)
		}
		if err := globalDB.BeginCompactionCancel(testutil.Context(t), req); err == nil {
			t.Fatal("BeginCompactionCancel(closed binding) error = nil")
		}
	})
}

package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

var _ looppkg.GoalControlStore = (*GoalRepo)(nil)

// LoadAwaitingGoalControl returns the one node checkpoint that currently owns Run re-entry.
func (g *GoalRepo) LoadAwaitingGoalControl(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (looppkg.GoalControlState, bool, error) {
	if err := g.checkReady(ctx, "load awaiting Goal control"); err != nil {
		return looppkg.GoalControlState{}, false, err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(runID)) == "" {
		return looppkg.GoalControlState{}, false, fmt.Errorf(
			"%w: workspace_id and run_id are required",
			looppkg.ErrValidation,
		)
	}
	row, found, err := loadAwaitingGoalControlWithExecutor(ctx, g.db, workspaceID, runID)
	if err != nil || !found {
		return looppkg.GoalControlState{}, found, err
	}
	return row.state, true, nil
}

// ReactivateGoalRun atomically grants control, clears the Run gate, and queues one successor segment.
func (g *GoalRepo) ReactivateGoalRun(
	ctx context.Context,
	req looppkg.GoalReactivationRequest,
) (looppkg.GoalReactivationResult, error) {
	if err := g.checkReady(ctx, "reactivate Goal run"); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	if err := validateGoalReactivationRequest(req); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	decisions, err := normalizeGoalReentryDecisions(req.Decisions, g.now)
	if err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	var result looppkg.GoalReactivationResult
	err = g.withTaskImmediateTransaction(ctx, "reactivate Goal run", func(exec taskSQLExecutor) error {
		applied, applyErr := g.reactivateGoalRunWithExecutor(ctx, exec, req, decisions)
		if applyErr != nil {
			return applyErr
		}
		result = applied
		return nil
	})
	if err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	return result, nil
}

func (g *GoalRepo) reactivateGoalRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req looppkg.GoalReactivationRequest,
	decisions []looppkg.GateDecisionRecord,
) (looppkg.GoalReactivationResult, error) {
	current, err := loadGoalReentryRowWithExecutor(ctx, exec, req.State)
	if err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	if !goalReentryStillAwaiting(current, req.State) {
		return g.replayedGoalReactivation(ctx, exec, current, req)
	}
	if err := validateGoalReentryControl(&current, req.State); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	oldRun, err := g.loadCompletedGoalSegment(ctx, exec, current)
	if err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	successor, nextEpoch, grantID, err := g.reserveGoalReentrySuccessor(ctx, exec, current, oldRun)
	if err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	return g.applyGoalReentry(ctx, exec, req, decisions, current, successor, nextEpoch, grantID)
}

func validateGoalReentryControl(
	current *goalReentryRow,
	expected looppkg.GoalControlState,
) error {
	current.state.Cause = goalControlCause(current.state)
	if current.state.Cause == "" {
		return goalReentryStaleError("Goal control cause is invalid")
	}
	if current.state.Cause != expected.Cause || current.state.GoalStatus != expected.GoalStatus {
		return goalReentryStaleError("Goal control cause changed")
	}
	return nil
}

func (g *GoalRepo) loadCompletedGoalSegment(
	ctx context.Context,
	exec taskSQLExecutor,
	current goalReentryRow,
) (taskpkg.Run, error) {
	oldRun, err := g.tasks.getTaskRunWithExecutor(ctx, exec, current.state.TaskRunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if oldRun.Status.Normalize() != taskpkg.TaskRunStatusCompleted || !oldRun.IsLoopWorker() {
		return taskpkg.Run{}, goalReentryStaleError("previous Goal segment is not completed")
	}
	metadata, ok, err := loopNodeMetadataFromTaskRun(oldRun.Metadata)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if !ok || metadata.Generation != current.state.Generation ||
		metadata.NodeID != string(current.state.NodeID) || metadata.ItemIndex != current.state.ItemIndex {
		return taskpkg.Run{}, goalReentryStaleError("Goal worker metadata changed")
	}
	return oldRun, nil
}

func (g *GoalRepo) reserveGoalReentrySuccessor(
	ctx context.Context,
	exec taskSQLExecutor,
	current goalReentryRow,
	oldRun taskpkg.Run,
) (taskpkg.Run, int64, int64, error) {
	nextEpoch := current.state.ControlEpoch + 1
	grantID := int64(1)
	if current.grantID.Valid {
		grantID = current.grantID.Int64 + 1
	}
	successorRunID := looppkg.GoalSegmentRunID(
		current.state.LoopRunID,
		current.state.Generation,
		current.state.NodeID,
		current.state.ItemIndex,
		nextEpoch,
	)
	successorMetadata, err := goalSuccessorMetadata(oldRun.Metadata, nextEpoch)
	if err != nil {
		return taskpkg.Run{}, 0, 0, err
	}
	idempotencyKey := looppkg.GoalSegmentIdempotencyKey(
		current.state.LoopRunID,
		current.state.Generation,
		current.state.NodeID,
		current.state.ItemIndex,
		nextEpoch,
	)
	_, successor, existing, err := g.tasks.reserveQueuedRunWithExecutor(ctx, exec, queuedRunReservationInput{
		taskID:             oldRun.TaskID,
		runID:              successorRunID,
		runKind:            taskpkg.RunKindWorker,
		loopRunID:          string(current.state.LoopRunID),
		idempotencyKey:     idempotencyKey,
		origin:             oldRun.Origin,
		networkSpec:        oldRun.NetworkSpecSnapshot(),
		designationGroupID: oldRun.DesignationGroupID,
		metadata:           successorMetadata,
		queuedAt:           g.now(),
	})
	if err != nil {
		return taskpkg.Run{}, 0, 0, err
	}
	if existing {
		return taskpkg.Run{}, 0, 0, goalReentryStaleError(
			"successor segment already existed before grant",
		)
	}
	return successor, nextEpoch, grantID, nil
}

func (g *GoalRepo) applyGoalReentry(
	ctx context.Context,
	exec taskSQLExecutor,
	req looppkg.GoalReactivationRequest,
	decisions []looppkg.GateDecisionRecord,
	current goalReentryRow,
	successor taskpkg.Run,
	nextEpoch int64,
	grantID int64,
) (looppkg.GoalReactivationResult, error) {
	turnLimit := current.state.TurnLimit + req.TurnIncrement
	if err := updateGoalPromptOwnerForReentry(ctx, exec, req, current, successor.ID, nextEpoch, g.now()); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	if err := updateGoalCheckpointForReentry(
		ctx,
		exec,
		req,
		current,
		successor.ID,
		nextEpoch,
		grantID,
		turnLimit,
		g.now(),
	); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	if err := updateGoalOutputForReentry(ctx, exec, current, successor.ID, turnLimit); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	for _, decision := range decisions {
		if err := insertLoopGateDecision(ctx, exec, decision); err != nil {
			return looppkg.GoalReactivationResult{}, err
		}
	}
	if err := g.transitionGoalRunForReentry(ctx, exec, req, current); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	if err := incrementGoalBudgetVersion(ctx, exec, current.state.LoopRunID); err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	return looppkg.GoalReactivationResult{
		Run:          successor,
		ControlEpoch: nextEpoch,
		GrantID:      grantID,
	}, nil
}

func (g *GoalRepo) replayedGoalReactivation(
	ctx context.Context,
	exec taskSQLExecutor,
	current goalReentryRow,
	req looppkg.GoalReactivationRequest,
) (looppkg.GoalReactivationResult, error) {
	nextEpoch := req.State.ControlEpoch + 1
	expectedRunID := looppkg.GoalSegmentRunID(
		req.State.LoopRunID,
		req.State.Generation,
		req.State.NodeID,
		req.State.ItemIndex,
		nextEpoch,
	)
	if current.state.ControlEpoch != nextEpoch || current.outputStatus != goalGenerationOutputEnqueued ||
		current.outputTaskRun != expectedRunID || !current.grantID.Valid ||
		current.grantKind.String != string(req.Kind) || current.grantCause.String != string(req.State.Cause) ||
		!current.grantTurn.Valid || current.grantTurn.Int64 != int64(goalReentryGrantTurn(req)) ||
		current.grantScope.String != string(req.Scope) || !current.grantConsumed.Valid ||
		(current.grantConsumed.Int64 != 0) != goalReentryGrantConsumed(req.Kind) {
		return looppkg.GoalReactivationResult{}, goalReentryStaleError("Goal control epoch changed")
	}
	run, err := g.tasks.getTaskRunWithExecutor(ctx, exec, expectedRunID)
	if err != nil {
		return looppkg.GoalReactivationResult{}, err
	}
	return looppkg.GoalReactivationResult{
		Run:          run,
		ControlEpoch: nextEpoch,
		GrantID:      current.grantID.Int64,
		Existing:     true,
	}, nil
}

func updateGoalCheckpointForReentry(
	ctx context.Context,
	exec taskSQLExecutor,
	req looppkg.GoalReactivationRequest,
	current goalReentryRow,
	successorRunID string,
	nextEpoch int64,
	grantID int64,
	turnLimit int,
	now time.Time,
) error {
	clearOperation := req.State.Cause == looppkg.ReasonCodeGoalRecoveryAmbiguous
	phase, err := goalCheckpointPhaseForReentry(ctx, exec, current, clearOperation)
	if err != nil {
		return err
	}
	grantTurn, err := goalSQLNullInt64(goalReentryGrantTurn(req))
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).UpdateGoalCheckpointForReentry(ctx, sqlcgen.UpdateGoalCheckpointForReentryParams{
		NextControlEpoch: nextEpoch, Phase: phase, TurnLimit: int64(turnLimit),
		SuccessorTaskRunID: goalNullableString(successorRunID), ClearOperation: boolToInt(clearOperation),
		ControlGrantID: grantID, ControlGrantKind: goalNullableString(string(req.Kind)),
		ControlGrantCause: goalNullableString(string(req.State.Cause)), ControlGrantTurn: grantTurn,
		ControlGrantScope:    goalNullableString(string(req.Scope)),
		ControlGrantConsumed: int64(boolToInt(goalReentryGrantConsumed(req.Kind))),
		UpdatedAt:            store.FormatTimestamp(now), LoopRunID: string(current.state.LoopRunID),
		Generation: int64(current.state.Generation), NodeID: string(current.state.NodeID),
		ItemIndex: int64(current.state.ItemIndex), ExpectedControlEpoch: current.state.ControlEpoch,
		ExpectedTaskRunID: goalNullableString(current.state.TaskRunID),
	})
	if err != nil {
		return fmt.Errorf("store: reactivate Goal checkpoint: %w", err)
	}
	return requireGoalAffectedCount(affected, "reactivate Goal checkpoint")
}

func goalCheckpointPhaseForReentry(
	ctx context.Context,
	exec taskSQLExecutor,
	current goalReentryRow,
	clearOperation bool,
) (string, error) {
	if clearOperation || strings.TrimSpace(current.state.QueueEntryID) == "" ||
		strings.TrimSpace(current.state.PromptID) == "" {
		return goalCheckpointPhaseIdle, nil
	}
	if !current.state.OpenTurn {
		return goalCheckpointPhaseQueued, nil
	}
	row, err := loadGoalPromptRow(ctx, exec, current.state.LoopRunID, current.state.PromptID)
	if err != nil {
		return "", err
	}
	if row.terminalAt == nil {
		return "", goalReentryStaleError("open Goal turn has no terminal result to settle")
	}
	if row.terminalKind.Valid && row.terminalKind.String == string(looppkg.ActionPromptOutcomeCompleted) &&
		row.terminalStopReason.Valid &&
		(row.terminalStopReason.String == string(looppkg.ActionStopEndTurn) ||
			row.terminalStopReason.String == string(looppkg.ActionStopMaxTurnRequests)) {
		return goalCheckpointPhaseJudging, nil
	}
	return goalCheckpointPhasePersisting, nil
}

func updateGoalPromptOwnerForReentry(
	ctx context.Context,
	exec taskSQLExecutor,
	req looppkg.GoalReactivationRequest,
	current goalReentryRow,
	successorRunID string,
	nextEpoch int64,
	now time.Time,
) error {
	if strings.TrimSpace(current.state.QueueEntryID) == "" ||
		strings.TrimSpace(current.state.PromptID) == "" ||
		req.State.Cause == looppkg.ReasonCodeGoalRecoveryAmbiguous {
		return nil
	}
	clearControlFence := req.Kind == looppkg.GoalGrantPlainResume
	affected, err := sqlcgen.New(exec).
		UpdateGoalPromptOwnerForReentry(ctx, sqlcgen.UpdateGoalPromptOwnerForReentryParams{
			SuccessorTaskRunID: successorRunID,
			NextOwnerEpoch:     goalNullableInt64(&nextEpoch),
			ClearControlFence:  boolToInt(clearControlFence),
			UpdatedAt:          store.FormatTimestamp(now),
			ID:                 current.state.QueueEntryID,
			LoopRunID:          goalNullableString(string(current.state.LoopRunID)),
			PromptID:           goalNullableString(current.state.PromptID),
			ExpectedOwnerEpoch: goalNullableInt64(
				&current.state.ControlEpoch,
			),
			ExpectedTaskRunID: current.state.TaskRunID,
		})
	if err != nil {
		return fmt.Errorf("store: reassign Goal prompt to successor segment: %w", err)
	}
	return requireGoalAffectedCount(affected, "reassign Goal prompt to successor segment")
}

func updateGoalOutputForReentry(
	ctx context.Context,
	exec taskSQLExecutor,
	current goalReentryRow,
	successorRunID string,
	turnLimit int,
) error {
	turnsUsed64, turnLimit64 := int64(current.state.TurnsUsed), int64(turnLimit)
	affected, err := sqlcgen.New(exec).UpdateGoalOutputForReentry(ctx, sqlcgen.UpdateGoalOutputForReentryParams{
		SuccessorTaskRunID: goalNullableString(successorRunID), GoalTurnsUsed: goalNullableInt64(&turnsUsed64),
		GoalTurnLimit: goalNullableInt64(&turnLimit64), LoopRunID: string(current.state.LoopRunID),
		Generation: int64(current.state.Generation), NodeID: string(current.state.NodeID),
		ItemIndex: int64(current.state.ItemIndex), ExpectedStatus: looppkg.GenerationOutputStatusAwaitingGoal,
		ExpectedTaskRunID: goalNullableString(current.state.TaskRunID),
	})
	if err != nil {
		return fmt.Errorf("store: reactivate Goal generation output: %w", err)
	}
	return requireGoalAffectedCount(affected, "reactivate Goal generation output")
}

func (g *GoalRepo) transitionGoalRunForReentry(
	ctx context.Context,
	exec taskSQLExecutor,
	req looppkg.GoalReactivationRequest,
	current goalReentryRow,
) error {
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, current.state.LoopRunID)
	if err != nil {
		return err
	}
	if loopRun.WorkspaceID != current.state.WorkspaceID || loopRun.Status != current.state.RunStatus ||
		loopRun.ActiveGateID != current.state.GateID {
		return goalReentryStaleError("Loop Run control changed")
	}
	cause := looppkg.TransitionCauseApproval
	if current.state.RunStatus == looppkg.StatusPaused {
		cause = looppkg.TransitionCauseOperatorResume
	}
	if err := updateLoopBoundaryStatusWithExecutor(
		ctx,
		exec,
		loopRun,
		looppkg.StatusRunning,
		cause,
		g.now(),
		current.state.Generation,
	); err != nil {
		return err
	}
	if err := sqlcgen.New(exec).ClearGoalRunControlActor(ctx, string(current.state.LoopRunID)); err != nil {
		return fmt.Errorf("store: clear Goal control actor: %w", err)
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		current.state.LoopRunID,
		current.state.WorkspaceID,
		loopRunEventGoalStatusChanged,
		map[string]any{
			loopRunEventPayloadKeyNodeID:     string(current.state.NodeID),
			loopRunEventPayloadKeyItemIndex:  current.state.ItemIndex,
			loopRunEventPayloadKeyGeneration: current.state.Generation,
			loopRunEventPayloadKeyFrom:       current.state.GoalStatus,
			loopRunEventPayloadKeyTo:         goalStatusActive,
			loopRunEventPayloadKeyCause:      string(current.state.Cause),
			loopRunEventPayloadKeyActorKind:  string(req.Actor.Actor.Kind.Normalize()),
			loopRunEventPayloadKeyActorID:    strings.TrimSpace(req.Actor.Actor.Ref),
		},
		g.now(),
	)
}

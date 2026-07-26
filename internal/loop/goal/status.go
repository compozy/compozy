package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
)

const (
	goalStatusActive        = "active"
	goalStatusPaused        = "paused"
	goalStatusBlocked       = "blocked"
	goalStatusUsageLimited  = "usage-limited"
	goalStatusBudgetLimited = "budget-limited"
	goalStatusComplete      = "complete"

	checkpointPhaseIdle            = "idle"
	checkpointPhasePreparing       = "preparing"
	checkpointPhaseQueued          = "queued"
	checkpointPhasePrompting       = "prompting"
	checkpointPhaseCompacting      = "compacting"
	checkpointPhaseJudging         = "judging"
	checkpointPhasePersisting      = "persisting"
	checkpointPhaseAwaitingControl = "awaiting_control"
	checkpointPhaseTerminal        = "terminal"

	contextStateKnown   = "known"
	contextStateUnknown = "unknown"
	contextStatePending = "pending"

	judgeAttemptStatusRunning   = "running"
	judgeAttemptStatusCompleted = "completed"
	judgeAttemptStatusAmbiguous = "ambiguous"

	reportStatusComplete = "complete"
	reportStatusBlocked  = "blocked"
)

type causalActor struct {
	kind string
	id   string
}

func automaticActor() causalActor {
	return causalActor{kind: "system", id: "goal-executor"}
}

func checkpointActor(checkpoint Checkpoint) causalActor {
	if strings.TrimSpace(checkpoint.ControlActorKind) != "" && strings.TrimSpace(checkpoint.ControlActorID) != "" {
		return causalActor{kind: checkpoint.ControlActorKind, id: checkpoint.ControlActorID}
	}
	return automaticActor()
}

func reportActor(checkpoint Checkpoint) causalActor {
	if checkpoint.ReportIntent != nil && strings.TrimSpace(checkpoint.ReportIntent.ActorKind) != "" &&
		strings.TrimSpace(checkpoint.ReportIntent.ActorID) != "" {
		return causalActor{kind: checkpoint.ReportIntent.ActorKind, id: checkpoint.ReportIntent.ActorID}
	}
	return automaticActor()
}

func checkpointIdentity(checkpoint Checkpoint) string {
	return fmt.Sprintf(
		"%s:%d:%s:%d:%d",
		checkpoint.Key.LoopRunID,
		checkpoint.Key.Generation,
		checkpoint.Key.NodeID,
		checkpoint.Key.ItemIndex,
		checkpoint.ControlEpoch,
	)
}

func (e *Executor) checkpointBoundary(
	ctx context.Context,
	segment *segmentState,
	disposition loop.ActionDisposition,
	status string,
	cause loop.ReasonCode,
	actor causalActor,
) (*loop.ActionControl, error) {
	checkpoint := segment.checkpoint
	if !boundaryAlreadyPersisted(checkpoint, disposition, status, cause) {
		if err := e.store.CheckpointControl(ctx, ControlCheckpointRequest{
			Key:                  segment.key,
			ExpectedControlEpoch: checkpoint.ControlEpoch,
			ExpectedBindingEpoch: checkpoint.BindingEpoch,
			ExpectedPhase:        checkpoint.Phase,
			TaskRunID:            checkpoint.TaskRunID,
			QueueEntryID:         checkpoint.QueueEntryID,
			PromptID:             checkpoint.PromptID,
			Disposition:          disposition,
			Status:               status,
			Cause:                cause,
			ActorKind:            actor.kind,
			ActorID:              actor.id,
		}); err != nil {
			return nil, fmt.Errorf("goal: persist %s checkpoint boundary: %w", disposition, err)
		}
		updated, err := e.store.LoadCheckpoint(ctx, segment.key)
		if err != nil {
			return nil, fmt.Errorf("goal: reload checkpoint after %s boundary: %w", disposition, err)
		}
		segment.checkpoint = updated
		checkpoint = updated
	}
	control := &loop.ActionControl{
		Disposition:  disposition,
		GoalStatus:   status,
		Cause:        cause,
		CheckpointID: checkpointIdentity(checkpoint),
	}
	if disposition == loop.ActionDispositionNeedsApproval {
		control.GateID = loop.SyntheticGoalGateID(
			segment.key.NodeID,
			segment.key.Generation,
			segment.key.ItemIndex,
			cause,
		)
	}
	if err := control.Validate(); err != nil {
		return nil, err
	}
	return control, nil
}

func boundaryAlreadyPersisted(
	checkpoint Checkpoint,
	disposition loop.ActionDisposition,
	status string,
	cause loop.ReasonCode,
) bool {
	if checkpoint.Status != status || checkpoint.ControlCause != cause {
		return false
	}
	if disposition == loop.ActionDispositionSucceeded || disposition == loop.ActionDispositionBlocked ||
		disposition == loop.ActionDispositionExhausted {
		return checkpoint.Phase == checkpointPhaseTerminal
	}
	return checkpoint.Phase == checkpointPhaseAwaitingControl
}

func (e *Executor) recoveredControl(ctx context.Context, segment *segmentState) (*loop.ActionControl, error) {
	checkpoint := segment.checkpoint
	switch checkpoint.Status {
	case goalStatusComplete:
		return e.checkpointBoundary(
			ctx, segment, loop.ActionDispositionSucceeded, goalStatusComplete, "", automaticActor(),
		)
	case goalStatusBlocked:
		cause := checkpoint.ControlCause
		if cause == "" {
			cause = loop.ReasonCodeGoalReportedBlocked
		}
		return e.checkpointBoundary(
			ctx, segment, loop.ActionDispositionBlocked, goalStatusBlocked, cause, reportActor(checkpoint),
		)
	case goalStatusBudgetLimited:
		cause := checkpoint.ControlCause
		if cause == "" {
			cause = loop.ReasonCodeGoalBudgetFenced
		}
		if checkpoint.Phase == checkpointPhaseTerminal {
			return e.checkpointBoundary(
				ctx, segment, loop.ActionDispositionExhausted, goalStatusBudgetLimited, cause, automaticActor(),
			)
		}
		return e.checkpointBoundary(
			ctx,
			segment,
			loop.ActionDispositionNeedsApproval,
			goalStatusBudgetLimited,
			cause,
			checkpointActor(checkpoint),
		)
	case goalStatusUsageLimited:
		cause := checkpoint.ControlCause
		if cause == "" {
			cause = loop.ReasonCodeGoalReseedConfirmationRequired
		}
		return e.checkpointBoundary(
			ctx,
			segment,
			loop.ActionDispositionNeedsApproval,
			goalStatusUsageLimited,
			cause,
			checkpointActor(checkpoint),
		)
	case goalStatusPaused:
		cause := checkpoint.ControlCause
		if cause == "" {
			cause = loop.ReasonCodeGoalPromptFenced
		}
		if cause == loop.ReasonCode(loop.TransitionCausePauseBoundary) {
			return e.checkpointBoundary(
				ctx, segment, loop.ActionDispositionPaused, goalStatusPaused, cause, checkpointActor(checkpoint),
			)
		}
		return e.checkpointBoundary(
			ctx, segment, loop.ActionDispositionNeedsApproval, goalStatusPaused, cause, checkpointActor(checkpoint),
		)
	default:
		return nil, fmt.Errorf("%w: cannot recover Goal control from status %q", loop.ErrValidation, checkpoint.Status)
	}
}

func (e *Executor) budgetBoundary(
	ctx context.Context,
	segment *segmentState,
	decision BudgetDecision,
) (*loop.ActionControl, error) {
	if decision.Allowed ||
		(decision.Disposition != loop.ActionDispositionNeedsApproval &&
			decision.Disposition != loop.ActionDispositionExhausted) {
		return nil, fmt.Errorf("%w: denied Goal budget decision is invalid", loop.ErrValidation)
	}
	cause := decision.Cause
	if cause == "" {
		cause = loop.ReasonCodeGoalBudgetFenced
	}
	return e.checkpointBoundary(
		ctx,
		segment,
		decision.Disposition,
		goalStatusBudgetLimited,
		cause,
		automaticActor(),
	)
}

func (e *Executor) fencedPromptBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*loop.ActionControl, error) {
	status := goalStatusPaused
	if result.Outcome == loop.ActionPromptOutcomeBudgetFenced {
		status = goalStatusBudgetLimited
	}
	if result.ReasonCode == loop.ReasonCodeGoalReseedConfirmationRequired {
		status = goalStatusUsageLimited
	}
	actor := automaticActor()
	if result.FenceDisposition == loop.ActionDispositionPaused {
		actor = checkpointActor(segment.checkpoint)
	}
	return e.checkpointBoundary(ctx, segment, result.FenceDisposition, status, result.ReasonCode, actor)
}

func (e *Executor) exhaustedBoundary(ctx context.Context, segment *segmentState) (*turnBoundary, error) {
	disposition := loop.ActionDispositionExhausted
	if segment.params.OnExhausted == "" || segment.params.OnExhausted == "halt" {
		control, err := e.checkpointBoundary(
			ctx,
			segment,
			disposition,
			goalStatusBudgetLimited,
			loop.ReasonCodeGoalTurnsExhausted,
			automaticActor(),
		)
		return &turnBoundary{checkpoint: segment.checkpoint, control: control}, err
	}
	if segment.params.OnExhausted != "escalate" {
		return nil, fmt.Errorf("%w: Goal on_exhausted %q is invalid", loop.ErrValidation, segment.params.OnExhausted)
	}
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionNeedsApproval,
		goalStatusBudgetLimited,
		loop.ReasonCodeGoalTurnsExhausted,
		automaticActor(),
	)
	return &turnBoundary{checkpoint: segment.checkpoint, control: control}, err
}

func (e *Executor) settleFailureBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
	cause loop.ReasonCode,
) (*turnBoundary, error) {
	settled, err := e.completeTurn(ctx, segment, result, nil)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = settled
	if control, ok, pauseErr := e.pendingPauseBoundary(ctx, segment); ok || pauseErr != nil {
		return &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}, pauseErr
	}
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionNeedsApproval,
		goalStatusPaused,
		cause,
		automaticActor(),
	)
	return &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}, err
}

func (e *Executor) settleCancelledBoundary(
	ctx context.Context,
	segment *segmentState,
	result loop.ActionPromptResult,
) (*turnBoundary, error) {
	settled, err := e.completeTurn(ctx, segment, result, nil)
	if err != nil {
		return nil, err
	}
	segment.checkpoint = settled
	if control, ok, pauseErr := e.pendingPauseBoundary(ctx, segment); ok || pauseErr != nil {
		return &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}, pauseErr
	}
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionNeedsApproval,
		goalStatusPaused,
		loop.ReasonCodeGoalPromptFenced,
		automaticActor(),
	)
	return &turnBoundary{checkpoint: settled, result: result, control: control, completed: true}, err
}

func (e *Executor) pendingPauseBoundary(
	ctx context.Context,
	segment *segmentState,
) (*loop.ActionControl, bool, error) {
	checkpoint := segment.checkpoint
	if checkpoint.Status == goalStatusPaused && checkpoint.Phase == checkpointPhaseAwaitingControl {
		control, err := e.recoveredControl(ctx, segment)
		return control, true, err
	}
	if !checkpointHasPendingPause(checkpoint) {
		return nil, false, nil
	}
	control, err := e.checkpointBoundary(
		ctx,
		segment,
		loop.ActionDispositionPaused,
		goalStatusPaused,
		loop.ReasonCode(loop.TransitionCausePauseBoundary),
		checkpointActor(checkpoint),
	)
	return control, true, err
}

func checkpointHasPendingPause(checkpoint Checkpoint) bool {
	return strings.TrimSpace(checkpoint.ControlActorKind) != "" &&
		strings.TrimSpace(checkpoint.ControlActorID) != ""
}

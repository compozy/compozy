package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/task"
)

var _ loop.GoalPauseStore = (*LoopRepo)(nil)

// RequestGoalPause preserves terminal ownership while fencing every subsequent prompt claim.
func (g *LoopRepo) RequestGoalPause(ctx context.Context, ws loop.WorkspaceID, runID loop.RunID,
	actor task.ActorContext) ([]loop.GoalPromptLease, error) {
	if err := g.checkReady(ctx, "pause session Goal"); err != nil {
		return nil, err
	}
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	var leases []loop.GoalPromptLease
	err := g.withTaskImmediateTransaction(ctx, "pause session Goal", func(exec taskSQLExecutor) error {
		now := g.now()
		state, err := sqlcgen.New(exec).GetLoopRunPauseState(ctx, sqlcgen.GetLoopRunPauseStateParams{
			WorkspaceID: string(ws), ID: string(runID),
		})
		if err != nil {
			return err
		}
		if state.Status == string(loop.StatusPaused) {
			return nil
		}
		if state.PauseRequested == 0 {
			if _, err := setLoopRunPauseState(ctx, exec, string(ws), string(runID), true, actor, now); err != nil {
				return err
			}
			if err := projectLoopPauseToGoals(ctx, exec, string(runID), true, actor, now); err != nil {
				return err
			}
		}
		keys, err := loadStoppableGoalCheckpointKeys(ctx, exec, goalCancellationRequest{WorkspaceID: ws, RunID: runID})
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			// There is no Goal prompt owner to drain yet. The run status fences
			// initial binding/dispatch, including a worker racing with this transaction.
			return compareAndSwapLoopRunStatusWithExecutor(ctx, exec, runID,
				loop.StatusRunning, loop.StatusPaused, loop.TransitionCausePauseBoundary, now)
		}
		for _, key := range keys {
			checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, goal.TurnKey{
				WorkspaceID: ws,
				LoopRunID:   runID,
				Generation:  key.generation,
				NodeID:      key.nodeID,
				ItemIndex:   key.itemIndex,
			})
			if err != nil {
				return err
			}
			if checkpoint.Phase == goalCheckpointPhaseQueued {
				// A stopped session cannot claim queued input. Publish the control
				// receipt here so its waiting executor can settle the pause boundary.
				if err := fencePreparedGoalPromptWithExecutor(ctx, exec, goal.FencePreparedPromptRequest{
					Key: checkpoint.Key, ExpectedControlEpoch: checkpoint.ControlEpoch,
					ExpectedBindingEpoch: checkpoint.BindingEpoch, TaskRunID: checkpoint.TaskRunID,
					QueueEntryID: checkpoint.QueueEntryID, PromptID: checkpoint.PromptID,
					Outcome: loop.ActionPromptOutcomeControlFenced, Disposition: loop.ActionDispositionPaused,
					Cause: loop.ReasonCode(loop.TransitionCausePauseBoundary),
				}, now); err != nil {
					return err
				}
				continue
			}
			if checkpoint.PromptID == "" {
				continue
			}
			lease := goalPromptLease(checkpoint)
			if !validGoalPromptLease(lease) {
				return fmt.Errorf("%w: Goal pause lease is incomplete", loop.ErrTransitionConflict)
			}
			leases = append(leases, lease)
		}
		return nil
	})
	return leases, err
}

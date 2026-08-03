package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ looppkg.NodeRequeueStore = (*LoopRepo)(nil)

// RequeueNode clears active quarantine, records provenance, and reserves one coordinator wake.
func (g *LoopRepo) RequeueNode(
	ctx context.Context,
	mutation looppkg.NodeRequeueMutation,
) (looppkg.NodeRequeueResult, error) {
	if err := g.checkReady(ctx, "requeue Loop node"); err != nil {
		return looppkg.NodeRequeueResult{}, err
	}
	if err := looppkg.ValidateNodeRequeueMutation(mutation); err != nil {
		return looppkg.NodeRequeueResult{}, err
	}
	var result looppkg.NodeRequeueResult
	err := g.withTaskImmediateTransaction(ctx, "requeue Loop node", func(exec taskSQLExecutor) error {
		prepared, err := prepareNodeRequeue(ctx, exec, mutation)
		if err != nil {
			return err
		}
		if err := applyNodeRequeue(ctx, exec, mutation, &prepared); err != nil {
			return err
		}
		result, err = g.reserveNodeRequeue(ctx, exec, mutation, &prepared)
		return err
	})
	if err != nil {
		return looppkg.NodeRequeueResult{}, err
	}
	return result, nil
}

type preparedNodeRequeue struct {
	run            looppkg.Run
	control        requeueNodeControl
	entry          json.RawMessage
	nextGeneration int
}

func prepareNodeRequeue(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
) (preparedNodeRequeue, error) {
	current, err := getLoopRunByIDWithExecutor(ctx, exec, mutation.RunID)
	if err != nil {
		return preparedNodeRequeue{}, err
	}
	if current.WorkspaceID != mutation.WorkspaceID {
		return preparedNodeRequeue{}, fmt.Errorf(
			"%w: loop run %q not found",
			looppkg.ErrRunNotFound,
			mutation.RunID,
		)
	}
	if current.Status.Terminal() {
		return preparedNodeRequeue{}, loopLifecycleReasonError(
			looppkg.ReasonCodeRunTerminal,
			fmt.Errorf("%w: terminal loop run %q cannot requeue nodes", looppkg.ErrInvalidTransition, mutation.RunID),
			string(current.Status),
			[]string{},
			runLifecycleWinner(current),
		)
	}
	if current.IterationCap > 0 && current.Generation+1 > current.IterationCap {
		return preparedNodeRequeue{}, loopLifecycleReasonError(
			looppkg.ReasonCodeInvalidStatusTransition,
			fmt.Errorf("%w: loop run %q reached its iteration cap", looppkg.ErrInvalidTransition, mutation.RunID),
			loopLifecycleStateIterationCap,
			[]string{nodeLifecycleTransitionCancel, nodeLifecycleTransitionKill},
			nil,
		)
	}
	control, err := loadQuarantinedNodeControl(ctx, exec, mutation)
	if err != nil {
		return preparedNodeRequeue{}, err
	}
	nextGeneration := current.Generation + 1
	entry, err := looppkg.AppendQuarantineRequeue(
		control.entry,
		mutation.Actor,
		mutation.Reason,
		mutation.RequestedAt,
		nextGeneration,
	)
	if err != nil {
		return preparedNodeRequeue{}, err
	}
	return preparedNodeRequeue{
		run: current, control: control, entry: entry, nextGeneration: nextGeneration,
	}, nil
}

func applyNodeRequeue(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	prepared *preparedNodeRequeue,
) error {
	if err := clearNodeQuarantine(ctx, exec, mutation, prepared); err != nil {
		return err
	}
	if err := fenceRequeuedNodeAndDependents(ctx, exec, mutation, prepared.run.Generation); err != nil {
		return err
	}
	parkedFor := mutation.RequestedAt.UTC().Sub(prepared.control.quarantinedAt)
	parkedFor = max(parkedFor, 0)
	if err := shiftQuarantinedNodeFirstScheduledAnchors(
		ctx, exec, mutation.RunID, mutation.NodeID, parkedFor,
	); err != nil {
		return err
	}
	if err := shiftLoopWallClockIfUnparked(ctx, exec, mutation.RunID, parkedFor); err != nil {
		return err
	}
	return appendNodeRequeuedEvent(ctx, exec, mutation, prepared)
}

func clearNodeQuarantine(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	prepared *preparedNodeRequeue,
) error {
	updated, err := exec.ExecContext(
		ctx,
		`UPDATE loop_node_controls SET
			quarantined = 0,
			quarantine_entry_json = ?,
			quarantined_at = NULL,
			revision = revision + 1,
			updated_at = ?
		 WHERE loop_run_id = ? AND node_id = ? AND quarantined = 1 AND revision = ?`,
		string(prepared.entry),
		mutation.RequestedAt.UTC(),
		mutation.RunID,
		mutation.NodeID,
		prepared.control.revision,
	)
	if err != nil {
		return fmt.Errorf("store: clear Loop node quarantine: %w", err)
	}
	return requireSingleRequeueMutation(updated, mutation.NodeID)
}

func fenceRequeuedNodeAndDependents(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	currentGeneration int,
) error {
	if _, err := exec.ExecContext(
		ctx,
		`UPDATE loop_generation_outputs
		 SET epoch = epoch + 1
		 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND status = 'quarantined'`,
		mutation.RunID,
		currentGeneration,
		mutation.NodeID,
	); err != nil {
		return fmt.Errorf("store: fence requeued Loop node outputs: %w", err)
	}
	attentionSuffix := " requires parked producer " + string(mutation.NodeID)
	if _, err := exec.ExecContext(
		ctx,
		`UPDATE loop_node_controls
		 SET attention_flag = '', attention_reason = '', revision = revision + 1, updated_at = ?
		 WHERE loop_run_id = ? AND attention_flag = 'dependency_quarantined'
		   AND node_id <> ?
		   AND substr(attention_reason, -length(?)) = ?`,
		mutation.RequestedAt.UTC(),
		mutation.RunID,
		mutation.NodeID,
		attentionSuffix,
		attentionSuffix,
	); err != nil {
		return fmt.Errorf("store: clear requeue dependency attention: %w", err)
	}
	return nil
}

func appendNodeRequeuedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	prepared *preparedNodeRequeue,
) error {
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		prepared.run.ID,
		prepared.run.WorkspaceID,
		loopRunEventNodeRequeued,
		map[string]any{
			loopRunEventPayloadKeyGeneration: prepared.nextGeneration,
			loopRunEventPayloadKeyNodeID:     mutation.NodeID,
			loopRunEventPayloadKeyActorKind:  mutation.Actor.Actor.Kind,
			loopRunEventPayloadKeyActorID:    mutation.Actor.Actor.Ref,
			loopRunEventPayloadKeyReason:     diagnostics.RedactAndBound(mutation.Reason, 1024),
			"entry":                          prepared.entry,
		},
		mutation.RequestedAt,
	)
}

func (g *LoopRepo) reserveNodeRequeue(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	prepared *preparedNodeRequeue,
) (looppkg.NodeRequeueResult, error) {
	idempotencyKey := fmt.Sprintf(
		"loop.requeue.%s.%s.%d",
		mutation.RunID,
		mutation.NodeID,
		prepared.control.revision+1,
	)
	coordinator, added, err := g.reserveLoopCoordinatorRunWithExecutor(
		ctx,
		exec,
		prepared.run,
		mutation.Actor.Origin,
		mutation.RequestedAt,
		"",
		idempotencyKey,
	)
	if err != nil {
		return looppkg.NodeRequeueResult{}, err
	}
	if !added {
		return looppkg.NodeRequeueResult{}, fmt.Errorf(
			"%w: Loop coordinator requeue wake was not reserved",
			looppkg.ErrTransitionConflict,
		)
	}
	return looppkg.NodeRequeueResult{
		Control: looppkg.NodeControl{
			LoopRunID: prepared.run.ID, NodeID: mutation.NodeID, Quarantined: false,
			QuarantineEntry: prepared.entry, Revision: prepared.control.revision + 1,
			UpdatedAt: mutation.RequestedAt.UTC(),
		},
		Coordinator: coordinator,
	}, nil
}

type requeueNodeControl struct {
	entry         json.RawMessage
	revision      int64
	quarantinedAt time.Time
}

func loadQuarantinedNodeControl(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
) (requeueNodeControl, error) {
	rows, err := sqlcgen.New(exec).ListLoopNodeControls(ctx, sqlcgen.ListLoopNodeControlsParams{
		WorkspaceID: string(mutation.WorkspaceID),
		LoopRunID:   string(mutation.RunID),
	})
	if err != nil {
		return requeueNodeControl{}, fmt.Errorf("store: load quarantined Loop node: %w", err)
	}
	control := looppkg.NodeControl{LoopRunID: mutation.RunID, NodeID: mutation.NodeID}
	found := false
	for _, row := range rows {
		if looppkg.NodeID(row.NodeID) != mutation.NodeID {
			continue
		}
		control, err = loopNodeControlFromGenerated(row)
		if err != nil {
			return requeueNodeControl{}, err
		}
		found = true
		break
	}
	actualState := nodeLifecycleActualState(control, found)
	if !found || !control.Quarantined || len(control.QuarantineEntry) == 0 || control.QuarantinedAt == nil {
		winner := nodeLifecycleWinner(control)
		code := looppkg.ReasonCodeNodeNotQuarantined
		cause := fmt.Errorf("%w: node %q is not quarantined", looppkg.ErrInvalidTransition, mutation.NodeID)
		if provenance := lastQuarantineRequeueProvenance(control.QuarantineEntry); provenance != nil {
			winner = provenance
			code = looppkg.ReasonCodeAlreadyDecided
			cause = fmt.Errorf("%w: node %q was already requeued", looppkg.ErrTransitionConflict, mutation.NodeID)
		}
		return requeueNodeControl{}, loopLifecycleReasonError(
			code,
			cause,
			actualState,
			nodeLifecycleAllowedTransitions(actualState),
			winner,
		)
	}
	if mutation.ExpectedRevision != nil && control.Revision != *mutation.ExpectedRevision {
		return requeueNodeControl{}, loopLifecycleReasonError(
			looppkg.ReasonCodeAlreadyDecided,
			fmt.Errorf("%w: node %q revision changed", looppkg.ErrTransitionConflict, mutation.NodeID),
			actualState,
			nodeLifecycleAllowedTransitions(actualState),
			nodeLifecycleWinner(control),
		)
	}
	return requeueNodeControl{
		entry: control.QuarantineEntry, revision: control.Revision, quarantinedAt: control.QuarantinedAt.UTC(),
	}, nil
}

func lastQuarantineRequeueProvenance(raw json.RawMessage) *looppkg.ControlProvenance {
	if len(raw) == 0 {
		return nil
	}
	var entry looppkg.QuarantineEntry
	if err := json.Unmarshal(raw, &entry); err != nil || len(entry.Requeues) == 0 {
		return nil
	}
	winner := entry.Requeues[len(entry.Requeues)-1]
	return &looppkg.ControlProvenance{
		ActorKind:   winner.ActorKind,
		ActorID:     winner.ActorID,
		Reason:      winner.Reason,
		RequestedAt: winner.RequestedAt,
	}
}

func requireSingleRequeueMutation(result sql.Result, nodeID looppkg.NodeID) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect Loop node requeue: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: node %q quarantine changed", looppkg.ErrTransitionConflict, nodeID)
	}
	return nil
}

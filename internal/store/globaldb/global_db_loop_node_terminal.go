package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type loopNodeRunMetadata struct {
	Generation int    `json:"generation"`
	NodeID     string `json:"node_id"`
	ItemIndex  int    `json:"item_index"`
	Attempt    int    `json:"attempt"`
	Epoch      int64  `json:"epoch"`
}

const (
	loopNodeOutcomeFailure  = "failure"
	loopNodeOutputFailed    = "failed"
	loopNodeOutputSucceeded = "succeeded"
)

func recordLoopNodeTerminalWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	outcome string,
	outputRef string,
	resultPayload json.RawMessage,
	terminalAt time.Time,
) error {
	loopRunID := strings.TrimSpace(run.LoopRunID)
	if !run.IsLoopWorker() {
		return nil
	}
	status := loopNodeOutputSucceeded
	if outcome == loopNodeOutcomeFailure {
		status = loopNodeOutputFailed
	}
	recorded, err := updateLoopNodeOutputStatusWithExecutor(ctx, exec, run, loopRunID, status, outputRef)
	if err != nil {
		return err
	}
	if !recorded {
		return appendLoopNodeLateArrivalWithExecutor(ctx, exec, run, outcome, terminalAt)
	}
	affected, err := sqlcgen.New(exec).UpdateLoopRunNodeTerminal(ctx, sqlcgen.UpdateLoopRunNodeTerminalParams{
		TerminalAt: store.FormatTimestamp(terminalAt), ID: loopRunID,
	})
	if err != nil {
		return fmt.Errorf("store: record loop run %q node terminal progress: %w", loopRunID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: loop run %q: %w", loopRunID, looppkg.ErrRunNotFound)
	}
	tokensUsed, err := refreshLoopTokensUsedWithExecutor(ctx, exec, loopRunID)
	if err != nil {
		return err
	}
	return appendLoopNodeTerminalEventsWithExecutor(
		ctx,
		exec,
		run,
		outcome,
		outputRef,
		normalizeTaskJSON(resultPayload),
		tokensUsed,
		terminalAt,
	)
}

func appendLoopNodeLateArrivalWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	outcome string,
	at time.Time,
) error {
	metadata, ok, err := loopNodeMetadataFromTaskRun(run.Metadata)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: loop worker %q has invalid lifecycle metadata", looppkg.ErrValidation, run.ID)
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(run.LoopRunID))
	if err != nil {
		return err
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		loopRun.ID,
		loopRun.WorkspaceID,
		loopRunEventLateArrival,
		map[string]any{
			loopRunEventPayloadKeyGeneration:  metadata.Generation,
			loopRunEventPayloadKeyNodeID:      metadata.NodeID,
			loopRunEventPayloadKeyItemIndex:   metadata.ItemIndex,
			loopRunEventPayloadKeyTaskRunID:   run.ID,
			watchEventsPayloadAttemptKey:      metadata.Attempt,
			loopRunEventPayloadKeyIssuedEpoch: metadata.Epoch,
			globalDBOutcomeKey:                strings.TrimSpace(outcome),
		},
		at,
	)
}

func updateLoopNodeOutputStatusWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	loopRunID string,
	status string,
	outputRef string,
) (bool, error) {
	metadata, ok, err := loopNodeMetadataFromTaskRun(run.Metadata)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf(
			"%w: loop worker %q has invalid lifecycle metadata",
			looppkg.ErrValidation,
			run.ID,
		)
	}
	result, err := exec.ExecContext(
		ctx,
		`UPDATE loop_generation_outputs
		 SET status = ?,
		     output_ref = CASE WHEN ? = '' THEN output_ref ELSE ? END,
		     task_run_id = ?
		 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		   AND task_run_id = ? AND epoch = ?`,
		status,
		strings.TrimSpace(outputRef),
		strings.TrimSpace(outputRef),
		run.ID,
		loopRunID,
		metadata.Generation,
		metadata.NodeID,
		metadata.ItemIndex,
		run.ID,
		metadata.Epoch,
	)
	if err != nil {
		return false, fmt.Errorf(
			"store: update loop run %q node output for task run %q: %w",
			loopRunID,
			run.ID,
			err,
		)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: inspect loop node terminal write: %w", err)
	}
	if affected > 0 {
		return true, nil
	}
	return inspectRejectedLoopNodeOutputWrite(ctx, exec, run, loopRunID, metadata)
}

func inspectRejectedLoopNodeOutputWrite(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	loopRunID string,
	metadata loopNodeRunMetadata,
) (bool, error) {
	var currentEpoch int64
	var currentTaskRunID sql.NullString
	err := exec.QueryRowContext(
		ctx,
		`SELECT epoch, task_run_id
		 FROM loop_generation_outputs
		 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
		loopRunID,
		metadata.Generation,
		metadata.NodeID,
		metadata.ItemIndex,
	).Scan(&currentEpoch, &currentTaskRunID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf(
				"%w: output %s/%d is missing for task run %q",
				looppkg.ErrValidation,
				metadata.NodeID,
				metadata.ItemIndex,
				run.ID,
			)
		}
		return false, fmt.Errorf("store: inspect current loop node output: %w", err)
	}
	if currentEpoch != metadata.Epoch || currentTaskRunID.String != run.ID {
		return false, nil
	}
	return false, fmt.Errorf(
		"%w: output %s/%d rejected task run %q at epoch %d",
		looppkg.ErrStaleGenerationOutput,
		metadata.NodeID,
		metadata.ItemIndex,
		run.ID,
		metadata.Epoch,
	)
}

func loopNodeMetadataFromTaskRun(raw json.RawMessage) (loopNodeRunMetadata, bool, error) {
	if len(raw) == 0 {
		return loopNodeRunMetadata{}, false, nil
	}
	var metadata loopNodeRunMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return loopNodeRunMetadata{}, false, fmt.Errorf(
			"store: decode loop node task run metadata: %w",
			err,
		)
	}
	metadata.NodeID = strings.TrimSpace(metadata.NodeID)
	if metadata.Generation <= 0 || metadata.NodeID == "" || metadata.ItemIndex < 0 ||
		metadata.Attempt < 1 || metadata.Epoch < 0 {
		return loopNodeRunMetadata{}, false, nil
	}
	return metadata, true, nil
}

func loopFailureOutputRef(failure taskpkg.RunFailure) string {
	if outputRef, ok := looppkg.ActionFailureOutputRefFromMetadata(failure.Metadata); ok {
		return outputRef
	}
	type reasonEnvelope struct {
		ReasonCode string `json:"reason_code"`
		Code       string `json:"code"`
	}
	var envelope reasonEnvelope
	if len(failure.Metadata) == 0 || json.Unmarshal(failure.Metadata, &envelope) != nil {
		return ""
	}
	if strings.TrimSpace(envelope.ReasonCode) != "" {
		return strings.TrimSpace(envelope.ReasonCode)
	}
	return strings.TrimSpace(envelope.Code)
}

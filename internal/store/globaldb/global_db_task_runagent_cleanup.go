package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/goal"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type runAgentBindingMetadata struct {
	NodeKind      string `json:"node_kind"`
	SessionHandle string `json:"session_handle"`
	Epoch         int64  `json:"epoch"`
}

func closeTerminalRunAgentBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	terminalAt time.Time,
) error {
	if !run.IsLoopWorker() {
		return nil
	}
	if len(run.Metadata) == 0 {
		return nil
	}
	var metadata runAgentBindingMetadata
	if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
		return fmt.Errorf("store: decode terminal run-agent metadata: %w", err)
	}
	if dsl.ActionKind(strings.TrimSpace(metadata.NodeKind)) != dsl.ActionRunAgent {
		return nil
	}
	metadata.SessionHandle = strings.TrimSpace(metadata.SessionHandle)
	if metadata.SessionHandle == "" || metadata.Epoch < 0 || metadata.Epoch == math.MaxInt64 {
		return fmt.Errorf("%w: terminal run-agent binding identity is invalid", looppkg.ErrValidation)
	}
	key := goal.BindingKey{
		WorkspaceID: looppkg.WorkspaceID(strings.TrimSpace(run.WorkspaceID)),
		LoopRunID:   looppkg.RunID(strings.TrimSpace(run.LoopRunID)),
		Handle:      metadata.SessionHandle,
	}
	if err := key.Validate(); err != nil {
		return err
	}
	binding, found, err := findSessionBindingAttemptWithExecutor(ctx, exec, key, metadata.Epoch+1)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if binding.Ownership != goal.BindingOwnershipRunOwned {
		return fmt.Errorf("%w: run-agent binding is not run-owned", looppkg.ErrTransitionConflict)
	}
	return closeGoalBindingWithCleanup(
		ctx,
		exec,
		key,
		binding.BindingEpoch,
		binding.SessionID,
		goal.SessionCleanupCauseTerminal,
		terminalAt,
	)
}

func closeSettledRunAgentBindings(
	ctx context.Context,
	exec taskSQLExecutor,
	payload looppkg.GenerationSnapshotPayload,
	terminalAt time.Time,
) error {
	for _, attempt := range payload.Attempts {
		if attempt.Disposition == looppkg.AttemptRetried || attempt.Disposition == looppkg.AttemptResumed {
			continue
		}
		taskRunID := settledAttemptTaskRunID(payload.Outputs, attempt)
		if taskRunID == "" {
			continue
		}
		run, err := (&TaskRepo{}).getTaskRunWithExecutor(ctx, exec, taskRunID)
		if err != nil {
			return err
		}
		if err := closeTerminalRunAgentBinding(ctx, exec, run, terminalAt); err != nil {
			return err
		}
	}
	return nil
}

func settledAttemptTaskRunID(outputs []looppkg.GenerationOutput, attempt looppkg.NodeAttempt) string {
	for _, output := range outputs {
		if output.NodeID == string(attempt.NodeID) && output.ItemIndex == attempt.ItemIndex &&
			output.Attempt == attempt.Attempt {
			return strings.TrimSpace(output.TaskRunID)
		}
	}
	return ""
}

func closeCanceledRunAgentLaneBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.CancellationMutation,
	generation int,
) error {
	if mutation.ItemIndex == nil {
		return fmt.Errorf("%w: run-agent lane cancellation requires item_index", looppkg.ErrValidation)
	}
	var taskRunID sql.NullString
	err := exec.QueryRowContext(
		ctx,
		`SELECT task_run_id FROM loop_generation_outputs
		 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
		mutation.RunID,
		generation,
		mutation.NodeID,
		*mutation.ItemIndex,
	).Scan(&taskRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: load canceled run-agent lane task run: %w", err)
	}
	if !taskRunID.Valid {
		return nil
	}
	run, err := (&TaskRepo{}).getTaskRunWithExecutor(ctx, exec, taskRunID.String)
	if err != nil {
		return err
	}
	return closeTerminalRunAgentBinding(ctx, exec, run, mutation.RequestedAt)
}

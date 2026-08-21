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
		return nil
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

type runAgentSnapshotSettlement struct {
	Payload    looppkg.GenerationSnapshotPayload
	Generation int
	TerminalAt time.Time
}

func (g *TaskRepo) closeSettledRunAgentBindings(
	ctx context.Context,
	exec taskSQLExecutor,
	settlement runAgentSnapshotSettlement,
) error {
	for _, attempt := range settlement.Payload.Attempts {
		if attempt.Disposition == looppkg.AttemptRetried || attempt.Disposition == looppkg.AttemptResumed {
			continue
		}
		taskRunID := settledAttemptTaskRunID(settlement.Payload.Outputs, attempt, settlement.Generation)
		if taskRunID == "" {
			continue
		}
		run, err := g.getTaskRunWithExecutor(ctx, exec, taskRunID)
		if err != nil {
			return err
		}
		if err := closeTerminalRunAgentBinding(ctx, exec, run, settlement.TerminalAt); err != nil {
			return err
		}
	}
	return nil
}

func settledAttemptTaskRunID(
	outputs []looppkg.GenerationOutput,
	attempt looppkg.NodeAttempt,
	snapshotGeneration int,
) string {
	for _, output := range outputs {
		outputGeneration := output.Generation
		if outputGeneration == 0 {
			outputGeneration = snapshotGeneration
		}
		if outputGeneration == attempt.Generation && output.NodeID == string(attempt.NodeID) &&
			output.ItemIndex == attempt.ItemIndex && output.Attempt == attempt.Attempt {
			return strings.TrimSpace(output.TaskRunID)
		}
	}
	return ""
}

func (g *TaskRepo) closeCanceledRunAgentLaneBinding(
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
	run, err := g.getTaskRunWithExecutor(ctx, exec, taskRunID.String)
	if err != nil {
		return err
	}
	return closeTerminalRunAgentBinding(ctx, exec, run, mutation.RequestedAt)
}

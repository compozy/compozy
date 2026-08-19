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
)

const (
	loopRuntimeResolvedPayloadKey  = "resolved_runtime"
	loopRuntimeProviderPayloadKey  = "provider"
	loopRuntimeModelPayloadKey     = "model"
	loopRuntimeReasoningPayloadKey = "reasoning"
	loopRuntimeSpeedPayloadKey     = "speed"
	loopRuntimeSpeedOutcomeKey     = "speed_resolution"
	loopRuntimeSourcePayloadKey    = "source"
)

// ListGenerationOutputs loads loop-owned per-node generation state in deterministic order.
func (g *LoopRepo) ListGenerationOutputs(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	generation int,
) ([]looppkg.GenerationOutput, error) {
	if err := g.checkReady(ctx, "list loop generation outputs"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || runID == "" {
		return nil, fmt.Errorf("%w: workspace and loop run id are required", looppkg.ErrValidation)
	}
	if generation <= 0 {
		return nil, fmt.Errorf("%w: generation must be positive", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopGenerationOutputs(ctx, sqlcgen.ListLoopGenerationOutputsParams{
		LoopRunID: string(runID), Generation: int64(generation), WorkspaceID: string(workspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop run %q generation %d outputs: %w", runID, generation, err)
	}
	outputs := make([]looppkg.GenerationOutput, 0, len(rows))
	for _, row := range rows {
		output, mapErr := generationOutputFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

// RecordAppliedRuntime persists binder-applied runtime truth for one workspace-owned output.
func (g *LoopRepo) RecordAppliedRuntime(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	loopRunID looppkg.RunID,
	generation int,
	nodeID looppkg.NodeID,
	itemIndex int,
	resolved looppkg.ResolvedRuntime,
) (returnErr error) {
	if err := g.checkReady(ctx, "record loop applied runtime"); err != nil {
		return err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(loopRunID)) == "" ||
		generation <= 0 || strings.TrimSpace(string(nodeID)) == "" || itemIndex < 0 {
		return fmt.Errorf("%w: applied runtime output identity is invalid", looppkg.ErrValidation)
	}
	payload, err := json.Marshal(resolved)
	if err != nil {
		return fmt.Errorf("store: marshal applied runtime: %w", err)
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin applied runtime transaction: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			returnErr = errors.Join(returnErr, fmt.Errorf("store: rollback applied runtime: %w", rollbackErr))
		}
	}()
	queries := sqlcgen.New(tx)
	rows, err := queries.RecordLoopGenerationOutputRuntime(
		ctx,
		sqlcgen.RecordLoopGenerationOutputRuntimeParams{
			ResolvedRuntimeJson: sql.NullString{String: string(payload), Valid: true},
			LoopRunID:           string(loopRunID),
			TargetGeneration:    int64(generation),
			TargetNodeID:        string(nodeID),
			TargetItemIndex:     int64(itemIndex),
			WorkspaceID:         string(workspaceID),
		},
	)
	if err != nil {
		return fmt.Errorf("store: record applied runtime: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("%w: generation output not found in workspace", looppkg.ErrValidation)
	}
	if err := appendLoopRunEventWithExecutor(
		ctx,
		tx,
		loopRunID,
		workspaceID,
		loopRunEventRuntimeApplied,
		appliedRuntimeEventPayload(generation, nodeID, itemIndex, resolved),
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("store: append applied runtime event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit applied runtime transaction: %w", err)
	}
	return nil
}

func appliedRuntimeEventPayload(
	generation int,
	nodeID looppkg.NodeID,
	itemIndex int,
	resolved looppkg.ResolvedRuntime,
) map[string]any {
	resolvedPayload := map[string]any{
		loopRuntimeProviderPayloadKey:  resolved.Runtime.Provider,
		loopRuntimeModelPayloadKey:     resolved.Runtime.Model,
		loopRuntimeReasoningPayloadKey: resolved.Runtime.Reasoning,
		loopRuntimeSpeedPayloadKey:     resolved.Runtime.Speed,
		loopRuntimeSourcePayloadKey: map[string]any{
			loopRuntimeProviderPayloadKey:  resolved.Source.Provider,
			loopRuntimeModelPayloadKey:     resolved.Source.Model,
			loopRuntimeReasoningPayloadKey: resolved.Source.Reasoning,
			loopRuntimeSpeedPayloadKey:     resolved.Source.Speed,
		},
	}
	if resolved.SpeedResolution != nil {
		resolvedPayload[loopRuntimeSpeedOutcomeKey] = resolved.SpeedResolution
	}
	return map[string]any{
		loopRunEventPayloadKeyGeneration: generation,
		loopRunEventPayloadKeyNodeID:     strings.TrimSpace(string(nodeID)),
		loopRunEventPayloadKeyItemIndex:  itemIndex,
		loopRuntimeResolvedPayloadKey:    resolvedPayload,
	}
}

// LookupLoopGenerationOutputStatus returns the output correlated with one worker task run.
func (g *LoopRepo) LookupLoopGenerationOutputStatus(
	ctx context.Context,
	workspaceID string,
	loopRunID string,
	taskRunID string,
) (string, bool, error) {
	if err := g.checkReady(ctx, "lookup loop generation output status"); err != nil {
		return "", false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	loopRunID = strings.TrimSpace(loopRunID)
	taskRunID = strings.TrimSpace(taskRunID)
	if workspaceID == "" || loopRunID == "" || taskRunID == "" {
		return "", false, fmt.Errorf(
			"%w: workspace_id, loop_run_id, and task_run_id are required",
			looppkg.ErrValidation,
		)
	}
	status, err := g.queries.GetLoopGenerationOutputStatus(ctx, sqlcgen.GetLoopGenerationOutputStatusParams{
		LoopRunID: loopRunID, TaskRunID: nullString(taskRunID), WorkspaceID: workspaceID,
	})
	if err != nil {
		if errorsIsNoRows(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf(
			"store: lookup loop run %q output for task run %q: %w",
			loopRunID,
			taskRunID,
			err,
		)
	}
	return strings.TrimSpace(status), true, nil
}

// GetGenerationOutputPayload loads an externalized payload through its workspace-owned cell.
func (g *LoopRepo) GetGenerationOutputPayload(
	ctx context.Context,
	key looppkg.GenerationOutputPayloadKey,
) (json.RawMessage, error) {
	if err := g.checkReady(ctx, "get loop generation output payload"); err != nil {
		return nil, err
	}
	workspaceID := strings.TrimSpace(string(key.WorkspaceID))
	runID := strings.TrimSpace(string(key.RunID))
	nodeID := strings.TrimSpace(string(key.NodeID))
	outputRef := strings.TrimSpace(key.OutputRef)
	if workspaceID == "" || runID == "" || key.Generation <= 0 || nodeID == "" || key.ItemIndex < 0 {
		return nil, fmt.Errorf("%w: generation output payload identity is invalid", looppkg.ErrValidation)
	}
	if !looppkg.OutputRefLooksContentAddressed(outputRef) {
		return nil, fmt.Errorf("%w: output_ref is invalid: %q", looppkg.ErrValidation, outputRef)
	}
	raw, err := g.queries.GetLoopGenerationOutputPayload(
		ctx,
		sqlcgen.GetLoopGenerationOutputPayloadParams{
			LoopRunID:   runID,
			Generation:  int64(key.Generation),
			NodeID:      nodeID,
			ItemIndex:   int64(key.ItemIndex),
			OutputRef:   outputRef,
			WorkspaceID: workspaceID,
		},
	)
	if err != nil {
		if errorsIsNoRows(err) {
			return nil, looppkg.ErrOutputRefNotFound
		}
		return nil, fmt.Errorf("store: get loop generation output payload: %w", err)
	}
	return json.RawMessage(raw), nil
}

func getLoopOutputByRefWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	outputRef string,
) (json.RawMessage, error) {
	raw, err := sqlcgen.New(exec).GetLoopOutputBlob(ctx, outputRef)
	if err != nil {
		if errorsIsNoRows(err) {
			return nil, looppkg.ErrOutputRefNotFound
		}
		return nil, fmt.Errorf("store: get loop output %q: %w", outputRef, err)
	}
	return json.RawMessage(raw), nil
}

func sweepOrphanedLoopOutputBlobsWithExecutor(ctx context.Context, exec taskSQLExecutor) error {
	if err := sqlcgen.New(exec).SweepOrphanedLoopOutputBlobs(ctx); err != nil {
		return fmt.Errorf("store: sweep orphaned loop output blobs: %w", err)
	}
	return nil
}

func upsertLoopOutputBlobWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	outputRef string,
	payload json.RawMessage,
	now time.Time,
) error {
	if !looppkg.OutputRefLooksContentAddressed(outputRef) {
		return fmt.Errorf("%w: output_ref is invalid: %q", looppkg.ErrValidation, outputRef)
	}
	if len(payload) == 0 {
		return fmt.Errorf("%w: loop output payload is required", looppkg.ErrValidation)
	}
	return store.UpsertLoopOutputBlob(ctx, exec, outputRef, payload, now)
}

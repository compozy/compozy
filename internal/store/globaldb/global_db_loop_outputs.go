package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// ListGenerationOutputs loads loop-owned per-node generation state in deterministic order.
func (g *LoopRepo) ListGenerationOutputs(
	ctx context.Context,
	runID looppkg.RunID,
	generation int,
) ([]looppkg.GenerationOutput, error) {
	if err := g.checkReady(ctx, "list loop generation outputs"); err != nil {
		return nil, err
	}
	if runID == "" {
		return nil, fmt.Errorf("%w: loop run id is required", looppkg.ErrValidation)
	}
	if generation <= 0 {
		return nil, fmt.Errorf("%w: generation must be positive", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopGenerationOutputs(ctx, sqlcgen.ListLoopGenerationOutputsParams{
		LoopRunID: string(runID), Generation: int64(generation),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop run %q generation %d outputs: %w", runID, generation, err)
	}
	outputs := make([]looppkg.GenerationOutput, 0, len(rows))
	for _, row := range rows {
		outputs = append(outputs, generationOutputFromGenerated(row))
	}
	return outputs, nil
}

// LookupLoopGenerationOutputStatus returns the output correlated with one worker task run.
func (g *LoopRepo) LookupLoopGenerationOutputStatus(
	ctx context.Context,
	loopRunID string,
	taskRunID string,
) (string, bool, error) {
	if err := g.checkReady(ctx, "lookup loop generation output status"); err != nil {
		return "", false, err
	}
	loopRunID = strings.TrimSpace(loopRunID)
	taskRunID = strings.TrimSpace(taskRunID)
	if loopRunID == "" || taskRunID == "" {
		return "", false, fmt.Errorf("%w: loop_run_id and task_run_id are required", looppkg.ErrValidation)
	}
	status, err := g.queries.GetLoopGenerationOutputStatus(ctx, sqlcgen.GetLoopGenerationOutputStatusParams{
		LoopRunID: loopRunID, TaskRunID: nullString(taskRunID),
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

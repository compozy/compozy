package loop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/gate"
)

type coordinatorGenerationHistoryReader struct {
	outputs  GenerationOutputReader
	verdicts gate.VerdictReader
	attempts NodeAttemptReader
}

func (r coordinatorGenerationHistoryReader) ListNodeAttempts(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
) ([]NodeAttempt, error) {
	if r.attempts == nil {
		return nil, nil
	}
	return r.attempts.ListNodeAttempts(ctx, workspaceID, runID)
}

func (r coordinatorGenerationHistoryReader) ListGenerationOutputs(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
	generation int,
) ([]GenerationOutput, error) {
	return r.outputs.ListGenerationOutputs(ctx, workspaceID, runID, generation)
}

func (r coordinatorGenerationHistoryReader) GetGenerationOutputPayload(
	ctx context.Context,
	key GenerationOutputPayloadKey,
) (json.RawMessage, error) {
	return r.outputs.GetGenerationOutputPayload(ctx, key)
}

func (r coordinatorGenerationHistoryReader) ApplyGenerationOutputOverlays(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
	generation int,
	outputs []GenerationOutput,
) ([]GenerationOutput, error) {
	overlays, ok := r.outputs.(GenerationOutputOverlayReader)
	if !ok {
		return cloneGenerationOutputs(outputs), nil
	}
	return overlays.ApplyGenerationOutputOverlays(ctx, workspaceID, runID, generation, outputs)
}

func (r coordinatorGenerationHistoryReader) ListGateVerdicts(
	ctx context.Context,
	workspaceID string,
	runID string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	if r.verdicts == nil {
		return nil, fmt.Errorf("%w: generation verdict history reader is unavailable", ErrValidation)
	}
	return r.verdicts.ListGateVerdicts(ctx, workspaceID, runID, generation)
}

func (r coordinatorGenerationHistoryReader) ListRouteCausingVerdicts(
	ctx context.Context,
	workspaceID string,
	runID string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	if r.verdicts == nil {
		return nil, fmt.Errorf("%w: route-causing verdict history reader is unavailable", ErrValidation)
	}
	return r.verdicts.ListRouteCausingVerdicts(ctx, workspaceID, runID, generation)
}

func (r *CoordinatorRunner) readGenerationHistory(
	ctx context.Context,
	run Run,
	generation int,
) (GenerationHistory, error) {
	if generation > 1 && r.verdicts == nil {
		return GenerationHistory{}, fmt.Errorf(
			"%w: generation verdict history is unavailable for generation %d",
			ErrValidation,
			generation,
		)
	}
	return ReadGenerationHistory(
		ctx,
		coordinatorGenerationHistoryReader{outputs: r.outputs, verdicts: r.verdicts, attempts: r.attempts},
		run,
		generation,
	)
}

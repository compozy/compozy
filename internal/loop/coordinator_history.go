package loop

import (
	"context"

	"github.com/compozy/compozy/internal/loop/gate"
)

type coordinatorGenerationHistoryReader struct {
	outputs  GenerationOutputReader
	verdicts gate.VerdictReader
}

func (r coordinatorGenerationHistoryReader) ListGenerationOutputs(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
	generation int,
) ([]GenerationOutput, error) {
	return r.outputs.ListGenerationOutputs(ctx, workspaceID, runID, generation)
}

func (r coordinatorGenerationHistoryReader) ListGateVerdicts(
	ctx context.Context,
	workspaceID string,
	runID string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	if r.verdicts == nil {
		return []gate.VerdictRecord{}, nil
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
		return []gate.VerdictRecord{}, nil
	}
	return r.verdicts.ListRouteCausingVerdicts(ctx, workspaceID, runID, generation)
}

func (r *CoordinatorRunner) readGenerationHistory(
	ctx context.Context,
	run Run,
	generation int,
) (GenerationHistory, error) {
	return ReadGenerationHistory(
		ctx,
		coordinatorGenerationHistoryReader{outputs: r.outputs, verdicts: r.verdicts},
		run,
		generation,
	)
}

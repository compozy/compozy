package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/gate"
)

func (s *daemonLoopAPIService) loopGenerations(
	ctx context.Context,
	run looppkg.Run,
) ([]contract.LoopGenerationPayload, error) {
	lineage, err := s.persistence.ListGenerations(ctx, string(run.WorkspaceID), string(run.ID))
	if err != nil {
		return nil, err
	}
	generations := make([]contract.LoopGenerationPayload, 0, len(lineage))
	for _, generation := range lineage {
		outputs, err := s.persistence.ListGenerationOutputs(
			ctx,
			run.WorkspaceID,
			run.ID,
			int(generation.Generation),
		)
		if err != nil {
			return nil, err
		}
		verdicts, err := s.persistence.ListGateVerdicts(
			ctx,
			string(run.WorkspaceID),
			string(run.ID),
			generation.Generation,
		)
		if err != nil {
			return nil, err
		}
		generations = append(generations, contract.LoopGenerationPayload{
			Generation:       int(generation.Generation),
			ParentGeneration: generation.ParentGeneration,
			Origin:           contract.LoopGenerationOrigin(generation.Origin),
			Verdicts:         loopGateVerdictsPayload(verdicts),
			Outputs:          loopGenerationOutputsPayload(outputs),
		})
	}
	return generations, nil
}

func loopGateVerdictsPayload(verdicts []gate.VerdictRecord) []contract.LoopGateVerdictPayload {
	payloads := make([]contract.LoopGateVerdictPayload, 0, len(verdicts))
	for _, verdict := range verdicts {
		payload := contract.LoopGateVerdictPayload{
			GateID:  verdict.GateID,
			Outcome: contract.LoopGateVerdictOutcome(verdict.Outcome),
		}
		if verdict.Score != nil {
			value := *verdict.Score
			payload.Score = &value
		}
		if verdict.RouteCauseRank != nil {
			value := *verdict.RouteCauseRank
			payload.RouteCauseRank = &value
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

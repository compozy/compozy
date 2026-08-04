package daemon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/gate"
)

func (s *daemonLoopAPIService) loopGenerations(
	ctx context.Context,
	run looppkg.Run,
) ([]contract.LoopGenerationPayload, error) {
	attempts := []looppkg.NodeAttempt{}
	if reader, ok := s.persistence.(looppkg.NodeAttemptReader); ok {
		loadedAttempts, attemptsErr := reader.ListNodeAttempts(ctx, run.WorkspaceID, run.ID)
		if attemptsErr != nil {
			return nil, fmt.Errorf("daemon: list attempts for loop run %s: %w", run.ID, attemptsErr)
		}
		attempts = loadedAttempts
	}
	lineage, err := s.persistence.ListGenerations(ctx, string(run.WorkspaceID), string(run.ID))
	if err != nil {
		return nil, fmt.Errorf("daemon: list generations for loop run %s: %w", run.ID, err)
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
			return nil, fmt.Errorf(
				"daemon: list outputs for loop run %s generation %d: %w",
				run.ID,
				generation.Generation,
				err,
			)
		}
		verdicts, err := s.persistence.ListGateVerdicts(
			ctx,
			string(run.WorkspaceID),
			string(run.ID),
			generation.Generation,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"daemon: list gate verdicts for loop run %s generation %d: %w",
				run.ID,
				generation.Generation,
				err,
			)
		}
		verdictPayloads, err := loopGateVerdictsPayload(verdicts)
		if err != nil {
			return nil, fmt.Errorf(
				"daemon: project gate verdicts for loop run %s generation %d: %w",
				run.ID,
				generation.Generation,
				err,
			)
		}
		generations = append(generations, contract.LoopGenerationPayload{
			Generation:       generation.Generation,
			ParentGeneration: generation.ParentGeneration,
			Origin:           contract.LoopGenerationOrigin(generation.Origin),
			Verdicts:         verdictPayloads,
			Outputs:          loopGenerationOutputsPayload(outputs, attempts),
		})
	}
	return generations, nil
}

func loopGateVerdictsPayload(verdicts []gate.VerdictRecord) ([]contract.LoopGateVerdictPayload, error) {
	payloads := make([]contract.LoopGateVerdictPayload, 0, len(verdicts))
	for _, verdict := range verdicts {
		blockingIssues, err := loopGateBlockingIssuesPayload(verdict.BlockingIssues)
		if err != nil {
			return nil, fmt.Errorf("decode gate %s blocking issues: %w", verdict.GateID, err)
		}
		criteria, err := loopGateCriteriaPayload(verdict.Criteria)
		if err != nil {
			return nil, fmt.Errorf("decode gate %s criteria: %w", verdict.GateID, err)
		}
		payload := contract.LoopGateVerdictPayload{
			GateID:         verdict.GateID,
			ItemIndex:      verdict.ItemIndex,
			Outcome:        contract.LoopGateVerdictOutcome(verdict.Outcome),
			BlockingIssues: blockingIssues,
			Criteria:       criteria,
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
	return payloads, nil
}

func loopGateBlockingIssuesPayload(raw json.RawMessage) ([]contract.LoopGateBlockingIssuePayload, error) {
	var issues []gate.BlockingIssue
	if err := json.Unmarshal(raw, &issues); err != nil {
		return nil, err
	}
	payloads := make([]contract.LoopGateBlockingIssuePayload, 0, len(issues))
	for _, issue := range issues {
		payloads = append(payloads, contract.LoopGateBlockingIssuePayload{ID: issue.ID, Note: issue.Note})
	}
	return payloads, nil
}

func loopGateCriteriaPayload(raw json.RawMessage) ([]contract.LoopGateCriterionDetailPayload, error) {
	var criteria []gate.CriterionResult
	if err := json.Unmarshal(raw, &criteria); err != nil {
		return nil, err
	}
	payloads := make([]contract.LoopGateCriterionDetailPayload, 0, len(criteria))
	for _, criterion := range criteria {
		issues := make([]contract.LoopGateBlockingIssuePayload, 0, len(criterion.BlockingIssues))
		for _, issue := range criterion.BlockingIssues {
			issues = append(issues, contract.LoopGateBlockingIssuePayload{ID: issue.ID, Note: issue.Note})
		}
		warnings := make([]contract.LoopGateDiagnosticWarning, 0, len(criterion.Warnings))
		for _, warning := range criterion.Warnings {
			warnings = append(warnings, contract.LoopGateDiagnosticWarning{
				Code: warning.Code, Message: warning.Message,
			})
		}
		payloads = append(payloads, contract.LoopGateCriterionDetailPayload{
			ID:             criterion.ID,
			Type:           string(criterion.Type),
			Outcome:        contract.LoopGateVerdictOutcome(criterion.Outcome),
			Passed:         criterion.Passed,
			Broken:         criterion.Broken,
			ExitCode:       cloneOptional(criterion.ExitCode),
			Stdout:         criterion.Stdout,
			Stderr:         criterion.Stderr,
			Score:          cloneOptional(criterion.Score),
			Evidence:       cloneRawMessage(criterion.Evidence),
			BlockingIssues: issues,
			Warnings:       warnings,
			Payload:        cloneRawMessage(criterion.Payload),
		})
	}
	return payloads, nil
}

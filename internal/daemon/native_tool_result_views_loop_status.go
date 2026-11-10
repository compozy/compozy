package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactLoopStatus struct {
	Run          compactLoopRun                    `json:"run"`
	Generations  []compactLoopGeneration           `json:"generations,omitempty"`
	NodeControls []contract.LoopNodeControlPayload `json:"node_controls"`
	Waits        []contract.LoopNodeWaitPayload    `json:"waits"`
	Requests     []compactLoopRequest              `json:"requests"`
	Amendments   []compactLoopAmendment            `json:"amendments"`
	WatchEvents  *contract.LoopWatchEventsState    `json:"watch_events,omitzero"`
}

func projectCompactLoopStatus(p *contract.LoopRunResponse) *compactLoopStatus {
	if p == nil {
		return nil
	}
	result := &compactLoopStatus{
		NodeControls: p.NodeControls,
		Waits:        p.Waits,
		WatchEvents:  p.WatchEvents,
	}
	result.Run = *projectCompactLoopRun(&p.Run)
	if p.Generations != nil {
		result.Generations = make([]compactLoopGeneration, len(p.Generations))
		for i := range p.Generations {
			result.Generations[i] = *projectCompactLoopGeneration(&p.Generations[i])
		}
	}
	if p.Requests != nil {
		result.Requests = make([]compactLoopRequest, len(p.Requests))
		for i := range p.Requests {
			result.Requests[i] = *projectCompactLoopRequest(&p.Requests[i])
		}
	}
	if p.Amendments != nil {
		result.Amendments = make([]compactLoopAmendment, len(p.Amendments))
		for i := range p.Amendments {
			result.Amendments[i] = *projectCompactLoopAmendment(&p.Amendments[i])
		}
	}
	return result
}

func viewLoopStatus(p *contract.LoopRunResponse) any { return projectCompactLoopStatus(p) }

type compactLoopGeneration struct {
	Generation       int64                            `json:"generation"`
	ParentGeneration int64                            `json:"parent_generation"`
	Origin           contract.LoopGenerationOrigin    `json:"origin"`
	RouteCauses      []contract.LoopRouteCausePayload `json:"route_causes"`
	Verdicts         []compactLoopVerdict             `json:"verdicts"`
	Outputs          []compactLoopOutput              `json:"outputs"`
}

func projectCompactLoopGeneration(p *contract.LoopGenerationPayload) *compactLoopGeneration {
	if p == nil {
		return nil
	}
	result := &compactLoopGeneration{
		Generation:       p.Generation,
		ParentGeneration: p.ParentGeneration,
		Origin:           p.Origin,
		RouteCauses:      p.RouteCauses,
	}
	if p.Verdicts != nil {
		result.Verdicts = make([]compactLoopVerdict, len(p.Verdicts))
		for i := range p.Verdicts {
			result.Verdicts[i] = *projectCompactLoopVerdict(&p.Verdicts[i])
		}
	}
	if p.Outputs != nil {
		result.Outputs = make([]compactLoopOutput, len(p.Outputs))
		for i := range p.Outputs {
			result.Outputs[i] = *projectCompactLoopOutput(&p.Outputs[i])
		}
	}
	return result
}

type compactLoopVerdict struct {
	Criteria       []compactLoopCriterion                  `json:"criteria"`
	GateID         string                                  `json:"gate_id"`
	ItemIndex      int                                     `json:"item_index"`
	Outcome        contract.LoopGateVerdictOutcome         `json:"outcome"`
	Score          *float64                                `json:"score,omitzero"`
	RouteCauseRank *int                                    `json:"route_cause_rank,omitzero"`
	BlockingIssues []contract.LoopGateBlockingIssuePayload `json:"blocking_issues"`
}

func projectCompactLoopVerdict(p *contract.LoopGateVerdictPayload) *compactLoopVerdict {
	if p == nil {
		return nil
	}
	result := &compactLoopVerdict{
		GateID:         p.GateID,
		ItemIndex:      p.ItemIndex,
		Outcome:        p.Outcome,
		Score:          p.Score,
		RouteCauseRank: p.RouteCauseRank,
		BlockingIssues: p.BlockingIssues,
	}
	if p.Criteria != nil {
		result.Criteria = make([]compactLoopCriterion, len(p.Criteria))
		for i := range p.Criteria {
			c := p.Criteria[i]
			result.Criteria[i] = compactLoopCriterion{
				ID:             c.ID,
				Type:           c.Type,
				Outcome:        c.Outcome,
				Passed:         c.Passed,
				Broken:         c.Broken,
				ExitCode:       c.ExitCode,
				Stdout:         c.Stdout,
				Stderr:         c.Stderr,
				Score:          c.Score,
				BlockingIssues: c.BlockingIssues,
				Warnings:       c.Warnings,
			}
		}
	}
	return result
}

type compactLoopCriterion struct {
	ID             string                                  `json:"id"`
	Type           string                                  `json:"type"`
	Outcome        contract.LoopGateVerdictOutcome         `json:"outcome"`
	Passed         bool                                    `json:"passed"`
	Broken         bool                                    `json:"broken,omitzero"`
	ExitCode       *int                                    `json:"exit_code,omitzero"`
	Stdout         string                                  `json:"stdout,omitempty"`
	Stderr         string                                  `json:"stderr,omitempty"`
	Score          *float64                                `json:"score,omitzero"`
	BlockingIssues []contract.LoopGateBlockingIssuePayload `json:"blocking_issues,omitempty"`
	Warnings       []contract.LoopGateDiagnosticWarning    `json:"warnings,omitempty"`
}

type compactLoopOutput struct {
	Generation     int64      `json:"generation,omitzero"`
	NodeID         string     `json:"node_id"`
	ItemIndex      int        `json:"item_index,omitzero"`
	Status         string     `json:"status"`
	OutputRef      string     `json:"output_ref,omitempty"`
	TaskRunID      string     `json:"task_run_id,omitempty"`
	SessionID      string     `json:"session_id,omitempty"`
	ChildLoopRunID string     `json:"child_loop_run_id,omitempty"`
	Attempt        int        `json:"attempt,omitzero"`
	NextAttemptAt  *time.Time `json:"next_attempt_at,omitempty"`
	FailureClass   string     `json:"failure_class,omitempty"`
	Disposition    string     `json:"disposition,omitempty"`
}

func projectCompactLoopOutput(p *contract.LoopGenerationOutput) *compactLoopOutput {
	if p == nil {
		return nil
	}
	result := &compactLoopOutput{
		Generation:     p.Generation,
		NodeID:         p.NodeID,
		ItemIndex:      p.ItemIndex,
		Status:         p.Status,
		OutputRef:      p.OutputRef,
		TaskRunID:      p.TaskRunID,
		SessionID:      p.SessionID,
		ChildLoopRunID: p.ChildLoopRunID,
		Attempt:        p.Attempt,
		NextAttemptAt:  p.NextAttemptAt,
		FailureClass:   p.FailureClass,
		Disposition:    p.Disposition,
	}
	return result
}

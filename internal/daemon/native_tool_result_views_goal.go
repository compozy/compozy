package daemon

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactGoalContext struct {
	State      contract.GoalContextState `json:"state"`
	Diagnostic string                    `json:"diagnostic,omitempty"`
}

func projectCompactGoalContext(p *contract.GoalContextSnapshot) *compactGoalContext {
	if p == nil {
		return nil
	}
	result := &compactGoalContext{
		State:      p.State,
		Diagnostic: p.Diagnostic,
	}
	return result
}

type compactGoalSnapshot struct {
	RunID           string                       `json:"run_id"`
	NodeID          string                       `json:"node_id"`
	OriginSessionID string                       `json:"origin_session_id"`
	BoundSessionID  string                       `json:"bound_session_id"`
	Status          contract.GoalStatus          `json:"status"`
	RunStatus       contract.LoopRunStatus       `json:"run_status"`
	Cause           *contract.GoalReasonCode     `json:"cause"`
	TurnsUsed       int                          `json:"turns_used"`
	TurnLimit       int                          `json:"turn_limit"`
	Live            bool                         `json:"live"`
	PauseRequested  bool                         `json:"pause_requested,omitzero"`
	LastVerdict     *contract.GoalVerdictSummary `json:"last_verdict"`
	Context         compactGoalContext           `json:"context"`
}

func projectCompactGoalSnapshot(p *contract.GoalSnapshot) *compactGoalSnapshot {
	if p == nil {
		return nil
	}
	result := &compactGoalSnapshot{
		RunID:           p.RunID,
		NodeID:          p.NodeID,
		OriginSessionID: p.OriginSessionID,
		BoundSessionID:  p.BoundSessionID,
		Status:          p.Status,
		RunStatus:       p.RunStatus,
		Cause:           p.Cause,
		TurnsUsed:       p.TurnsUsed,
		TurnLimit:       p.TurnLimit,
		Live:            p.Live,
		PauseRequested:  p.PauseRequested,
		LastVerdict:     p.LastVerdict,
	}
	result.Context = *projectCompactGoalContext(&p.Context)
	return result
}

type compactGoalControl struct {
	Outcome       contract.GoalCommandOutcome `json:"outcome"`
	ReasonCode    *contract.GoalReasonCode    `json:"reason_code"`
	Snapshot      *compactGoalSnapshot        `json:"snapshot"`
	ReplacedRunID *string                     `json:"replaced_run_id"`
}

func projectCompactGoalControl(p *contract.GoalCommandResult) *compactGoalControl {
	if p == nil {
		return nil
	}
	result := &compactGoalControl{
		Outcome:       p.Outcome,
		ReasonCode:    p.ReasonCode,
		ReplacedRunID: p.ReplacedRunID,
	}
	result.Snapshot = projectCompactGoalSnapshot(p.Snapshot)
	return result
}

func viewGoalControl(p contract.GoalCommandResult) any { return projectCompactGoalControl(&p) }

type compactGoalGet struct {
	Goal *compactGoalSnapshot `json:"goal"`
}

func projectCompactGoalGet(p *contract.SessionGoalResponse) *compactGoalGet {
	if p == nil {
		return nil
	}
	result := &compactGoalGet{}
	result.Goal = projectCompactGoalSnapshot(p.Goal)
	return result
}

func viewGoalGet(p contract.SessionGoalResponse) any { return projectCompactGoalGet(&p) }

var goalControlViews = resultViews[contract.GoalCommandResult]{
	Compact: viewGoalControl,
	Preview: func(p contract.GoalCommandResult) string { return fmt.Sprintf("Goal %s", p.Outcome) },
}

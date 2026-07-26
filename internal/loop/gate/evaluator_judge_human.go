package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

func (e *Evaluator) evaluateAgentJudge(
	ctx context.Context,
	gate Gate,
	criterion dsl.GateCriterion,
	in GateInput,
) CriterionResult {
	if e.judges == nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeError,
			Broken:  true,
			BlockingIssues: []BlockingIssue{{
				ID:   JudgeTransportFailureIssueID,
				Note: "judge runner is not configured",
			}},
		}
	}
	contract := gateInputContract(in.Contract)
	rubric, err := RenderAgentJudgeRubric(criterion, contract, in.TemplateData, in.JudgeEvidence)
	if err != nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeInvalidOutput,
			BlockingIssues: []BlockingIssue{{
				ID:   JudgeRubricInvalidIssueID,
				Note: err.Error(),
			}},
			Warnings: []DiagnosticWarning{{
				Code:    "judge_rubric_invalid",
				Message: err.Error(),
			}},
		}
	}
	model := strings.TrimSpace(criterion.Model)
	if model == "" {
		model = strings.TrimSpace(in.JudgeModel)
	}
	response, err := e.judges.Judge(ctx, JudgeRequest{
		LoopRunID:            strings.TrimSpace(in.LoopRunID),
		GateID:               gate.ID,
		CriterionID:          criterion.ID,
		Attempt:              in.Revision + 1,
		CorrelationID:        strings.TrimSpace(in.ToolCallCorrelationID),
		WorkspaceID:          in.ToolScope.WorkspaceID,
		Agent:                criterion.Agent,
		Model:                model,
		Rubric:               rubric,
		Contract:             contract,
		NetworkParticipation: in.NetworkParticipation,
	})
	if err != nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeError,
			Broken:  true,
			BlockingIssues: []BlockingIssue{{
				ID:   JudgeTransportFailureIssueID,
				Note: err.Error(),
			}},
		}
	}
	if response.TokensUsed < 0 || (!response.TokensReported && response.TokensUsed != 0) {
		return CriterionResult{
			ID: criterion.ID, Type: criterion.Type, Outcome: VerdictOutcomeInvalidOutput,
			BlockingIssues: []BlockingIssue{{
				ID: "judge_usage_invalid", Note: "judge response token usage is invalid",
			}},
			Warnings: []DiagnosticWarning{{
				Code: "judge_usage_invalid", Message: "judge response token usage is invalid",
			}},
		}
	}
	result := ParseJudgeVerdict(criterion.ID, criterion.Type, response.Raw)
	result.judgeTokensUsed = response.TokensUsed
	result.judgeTokensReported = response.TokensReported
	return result
}

func gateInputContract(contract *dsl.Contract) dsl.Contract {
	if contract == nil {
		return dsl.Contract{}
	}
	return *contract
}

func (e *Evaluator) evaluateHuman(gate Gate, criterion dsl.GateCriterion, in GateInput) CriterionResult {
	decision, exists := in.HumanDecisions[criterion.ID]
	if !exists {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeAwaitingApproval,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerHumanPending,
				Note: "human decision is pending",
			}},
			Warnings: []DiagnosticWarning{{
				Code:    blockerHumanPending,
				Message: fmt.Sprintf("gate %q criterion %q is waiting for approval", gate.ID, criterion.ID),
			}},
		}
	}
	if err := decision.Actor.Validate(); err != nil {
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeError,
			BlockingIssues: []BlockingIssue{{
				ID:   "human_actor_invalid",
				Note: err.Error(),
			}},
		}
	}
	switch decision.Decision {
	case HumanDecisionApprove:
		payload := humanDecisionPayload(decision)
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeApproved,
			Passed:  true,
			Payload: payload,
		}
	case HumanDecisionRequestChanges:
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeRejected,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerHumanRequestedChanges,
				Note: strings.TrimSpace(decision.Note),
			}},
			Payload: humanDecisionPayload(decision),
		}
	case HumanDecisionReject:
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeBlocked,
			BlockingIssues: []BlockingIssue{{
				ID:   blockerHumanRejected,
				Note: strings.TrimSpace(decision.Note),
			}},
			Payload: humanDecisionPayload(decision),
		}
	default:
		return CriterionResult{
			ID:      criterion.ID,
			Type:    criterion.Type,
			Outcome: VerdictOutcomeInvalidOutput,
			BlockingIssues: []BlockingIssue{{
				ID:   "human_decision_invalid",
				Note: fmt.Sprintf("human decision %q is unsupported", decision.Decision),
			}},
		}
	}
}

func humanDecisionPayload(decision HumanDecision) json.RawMessage {
	payload, err := json.Marshal(map[string]any{
		"decision": decision.Decision,
		"note":     strings.TrimSpace(decision.Note),
		"actor":    decision.Actor,
	})
	if err != nil {
		// The payload shape contains only JSON-marshalable primitives and ActorContext fields.
		return nil
	}
	return payload
}

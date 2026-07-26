package core

import (
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
)

// GoalSnapshotPayloadFromSession maps the session-domain projection to the canonical public contract.
func GoalSnapshotPayloadFromSession(snapshot *session.GoalSnapshot) (*contract.GoalSnapshot, error) {
	if snapshot == nil {
		return nil, nil
	}
	contextPayload, err := goalContextPayload(snapshot.Context)
	if err != nil {
		return nil, err
	}
	status, err := goalStatusPayload(snapshot.Status)
	if err != nil {
		return nil, err
	}
	payload := &contract.GoalSnapshot{
		RunID: snapshot.RunID, NodeID: snapshot.NodeID, Objective: snapshot.Objective,
		OriginSessionID: snapshot.OriginSessionID, BoundSessionID: snapshot.BoundSessionID,
		Status: status, RunStatus: contract.LoopRunStatus(snapshot.RunStatus),
		TurnsUsed: snapshot.TurnsUsed, TurnLimit: snapshot.TurnLimit, Live: snapshot.Live,
		ContractSummary: snapshot.ContractSummary, Context: contextPayload,
	}
	if snapshot.Cause != nil {
		value := contract.GoalReasonCode(*snapshot.Cause)
		payload.Cause = &value
	}
	if snapshot.LastVerdict != nil {
		payload.LastVerdict, err = goalVerdictPayload(*snapshot.LastVerdict)
		if err != nil {
			return nil, err
		}
	}
	return payload, nil
}

func goalSnapshotPayload(snapshot *session.GoalSnapshot) (*contract.GoalSnapshot, error) {
	return GoalSnapshotPayloadFromSession(snapshot)
}

func goalContextPayload(value session.GoalContextSnapshot) (contract.GoalContextSnapshot, error) {
	state := contract.GoalContextState(value.State)
	switch state {
	case contract.GoalContextKnown:
		if value.Used == nil || value.Size == nil || value.Ratio == nil || value.ReportedAt == nil ||
			*value.Used < 0 || *value.Size <= 0 || *value.Ratio < 0 || *value.Ratio > 1 {
			return contract.GoalContextSnapshot{}, goalContractValidationError(
				"known Goal context requires valid usage metrics",
			)
		}
	case contract.GoalContextUnknown, contract.GoalContextPending:
		if value.Used != nil || value.Size != nil || value.Ratio != nil || value.ReportedAt != nil {
			return contract.GoalContextSnapshot{}, goalContractValidationError(
				"unknown or pending Goal context cannot expose usage metrics",
			)
		}
	default:
		return contract.GoalContextSnapshot{}, goalContractValidationError(
			fmt.Sprintf("Goal context state is invalid: %q", value.State),
		)
	}
	if value.NudgeRatio < 0 || value.NudgeRatio > 1 {
		return contract.GoalContextSnapshot{}, goalContractValidationError(
			"Goal context nudge ratio must be between 0 and 1",
		)
	}
	return contract.GoalContextSnapshot{
		State: state, Used: value.Used, Size: value.Size, Ratio: value.Ratio,
		NudgeRatio: value.NudgeRatio, ReportedAt: value.ReportedAt,
	}, nil
}

func goalVerdictPayload(value session.GoalVerdictSummary) (*contract.GoalVerdictSummary, error) {
	outcome, err := goalVerdictOutcomePayload(value.Outcome)
	if err != nil {
		return nil, err
	}
	issues := make([]contract.GoalBlockingIssue, 0, len(value.BlockingIssues))
	for _, issue := range value.BlockingIssues {
		issues = append(issues, contract.GoalBlockingIssue{ID: issue.ID, Note: issue.Note})
	}
	return &contract.GoalVerdictSummary{
		Outcome: outcome, BlockingIssues: issues,
		EvidenceRef: value.EvidenceRef, EvaluatedAt: value.EvaluatedAt,
	}, nil
}

// GoalTurnPagePayloadFromSession maps a session-domain audit page to the canonical public contract.
func GoalTurnPagePayloadFromSession(page session.GoalTurnPage) (contract.GoalTurnPage, error) {
	turns := make([]contract.GoalTurn, 0, len(page.Turns))
	for _, turn := range page.Turns {
		payload, err := goalTurnPayload(turn)
		if err != nil {
			return contract.GoalTurnPage{}, err
		}
		turns = append(turns, payload)
	}
	return contract.GoalTurnPage{Turns: turns, NextAfterSeq: page.NextAfterSeq}, nil
}

func goalTurnPagePayload(page session.GoalTurnPage) (contract.GoalTurnPage, error) {
	return GoalTurnPagePayloadFromSession(page)
}

func goalTurnPayload(turn session.GoalTurn) (contract.GoalTurn, error) {
	if turn.ResultStatus == nil {
		if turn.EndedAt != nil || turn.StopReason != nil || turn.ReasonCode != nil ||
			turn.VerdictOutcome != nil || turn.EvidenceRef != nil || turn.TokensUsed != nil {
			return contract.GoalTurn{}, goalContractValidationError(
				"open Goal turn contains terminal fields",
			)
		}
	} else if turn.EndedAt == nil {
		return contract.GoalTurn{}, goalContractValidationError("terminal Goal turn has no ended_at")
	}
	issues := make([]contract.GoalBlockingIssue, 0, len(turn.BlockingIssues))
	for _, issue := range turn.BlockingIssues {
		issues = append(issues, contract.GoalBlockingIssue{ID: issue.ID, Note: issue.Note})
	}
	resultStatus, err := goalTurnResultStatusPayload(turn.ResultStatus)
	if err != nil {
		return contract.GoalTurn{}, err
	}
	verdictOutcome, err := optionalGoalVerdictOutcomePayload(turn.VerdictOutcome)
	if err != nil {
		return contract.GoalTurn{}, err
	}
	payload := contract.GoalTurn{
		Seq: turn.Seq, Generation: turn.Generation, NodeID: turn.NodeID, ItemIndex: turn.ItemIndex,
		Turn: turn.Turn, PromptAttempt: turn.PromptAttempt, SessionID: turn.SessionID,
		BindingHandle: turn.BindingHandle, BindingEpoch: turn.BindingEpoch, PromptID: turn.PromptID,
		ResultStatus: resultStatus, VerdictOutcome: verdictOutcome,
		BlockingIssues: issues, EvidenceRef: turn.EvidenceRef, PromptRef: turn.PromptRef,
		TokensUsed: turn.TokensUsed, ActorKind: turn.ActorKind, ActorID: turn.ActorID,
		StartedAt: turn.StartedAt, EndedAt: turn.EndedAt,
	}
	if turn.ReasonCode != nil {
		value := contract.GoalReasonCode(*turn.ReasonCode)
		payload.ReasonCode = &value
	}
	if turn.StopReason != nil {
		value := contract.ACPPromptStopReason(*turn.StopReason)
		payload.StopReason = &value
	}
	return payload, nil
}

func goalStatusPayload(value string) (contract.GoalStatus, error) {
	status := contract.GoalStatus(value)
	switch status {
	case contract.GoalStatusActive, contract.GoalStatusPaused, contract.GoalStatusBlocked,
		contract.GoalStatusUsageLimited, contract.GoalStatusBudgetLimited, contract.GoalStatusComplete:
		return status, nil
	default:
		return "", goalContractValidationError(fmt.Sprintf("Goal status is invalid: %q", value))
	}
}

func goalTurnResultStatusPayload(value *string) (*contract.GoalTurnResultStatus, error) {
	if value == nil {
		return nil, nil
	}
	status := contract.GoalTurnResultStatus(*value)
	switch status {
	case contract.GoalTurnResultCompleted, contract.GoalTurnResultInvalidResult,
		contract.GoalTurnResultFailed, contract.GoalTurnResultAmbiguous:
		return &status, nil
	default:
		return nil, goalContractValidationError(fmt.Sprintf("Goal turn result status is invalid: %q", *value))
	}
}

func optionalGoalVerdictOutcomePayload(value *string) (*contract.GoalVerdictOutcome, error) {
	if value == nil {
		return nil, nil
	}
	outcome, err := goalVerdictOutcomePayload(*value)
	if err != nil {
		return nil, err
	}
	return &outcome, nil
}

func goalVerdictOutcomePayload(value string) (contract.GoalVerdictOutcome, error) {
	outcome := contract.GoalVerdictOutcome(value)
	switch outcome {
	case contract.GoalVerdictApproved, contract.GoalVerdictRejected,
		contract.GoalVerdictAwaitingApproval, contract.GoalVerdictBlocked,
		contract.GoalVerdictError, contract.GoalVerdictTimeout, contract.GoalVerdictInvalidOutput:
		return outcome, nil
	default:
		return "", goalContractValidationError(fmt.Sprintf("Goal verdict outcome is invalid: %q", value))
	}
}

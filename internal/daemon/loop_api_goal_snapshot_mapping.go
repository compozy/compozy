package daemon

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
)

func (s *daemonLoopAPIService) goalContextSnapshot(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	projection goalpkg.SessionProjection,
) (session.GoalContextSnapshot, error) {
	result := session.GoalContextSnapshot{
		State:      loopManagedContextStateUnknown,
		NudgeRatio: projection.ContextNudgeRatio,
	}
	checkpoint := projection.Checkpoint
	if checkpoint == nil {
		return result, nil
	}
	result.State = checkpoint.ContextState
	result.NudgeRatio = checkpoint.ContextNudgeRatio
	if checkpoint.ContextState != "known" {
		return result, nil
	}
	if checkpoint.UsageSequence == nil || s.goalContext == nil {
		return session.GoalContextSnapshot{}, fmt.Errorf(
			"%w: known Goal context has no readable usage sequence",
			looppkg.ErrValidation,
		)
	}
	usage, err := s.goalContext.UsageAtSequence(ctx, looppkg.ActionSessionBinding{
		WorkspaceID:  workspaceID,
		LoopRunID:    projection.RunID,
		SessionID:    checkpoint.SessionID,
		Handle:       checkpoint.BindingHandle,
		ControlEpoch: checkpoint.ControlEpoch,
		BindingEpoch: checkpoint.BindingEpoch,
	}, *checkpoint.UsageSequence)
	if err != nil {
		return session.GoalContextSnapshot{}, fmt.Errorf(
			"daemon: read Goal usage sequence %d for session %q: %w",
			*checkpoint.UsageSequence,
			checkpoint.SessionID,
			err,
		)
	}
	if !usage.Known || usage.Size <= 0 || usage.Used < 0 {
		return session.GoalContextSnapshot{}, fmt.Errorf(
			"%w: known Goal context usage is unavailable",
			looppkg.ErrValidation,
		)
	}
	result.Used = new(usage.Used)
	result.Size = new(usage.Size)
	ratio := float64(usage.Used) / float64(usage.Size)
	result.Ratio = &ratio
	reportedAt := usage.ReportedAt.UTC()
	result.ReportedAt = &reportedAt
	return result, nil
}

func composeSessionGoalSnapshot(
	projection goalpkg.SessionProjection,
	params dsl.GoalParams,
	nodeID string,
	contractSummary string,
	contextSnapshot session.GoalContextSnapshot,
) *session.GoalSnapshot {
	checkpoint := projection.Checkpoint
	boundSessionID := projection.OriginSessionID
	status := goalStatusFromRun(projection.RunStatus)
	turnsUsed := 0
	turnLimit := params.MaxTurns
	var cause *session.GoalReasonCode
	if checkpoint != nil {
		if strings.TrimSpace(checkpoint.SessionID) != "" {
			boundSessionID = checkpoint.SessionID
		}
		status = checkpoint.Status
		turnsUsed = checkpoint.TurnsUsed
		turnLimit = checkpoint.TurnLimit
		if checkpoint.ControlCause != "" {
			value := session.GoalReasonCode(checkpoint.ControlCause)
			cause = &value
		}
	}
	return &session.GoalSnapshot{
		RunID: string(projection.RunID), NodeID: nodeID, Objective: params.Objective,
		OriginSessionID: projection.OriginSessionID, BoundSessionID: boundSessionID,
		Status: status, RunStatus: string(projection.RunStatus), Cause: cause,
		TurnsUsed: turnsUsed, TurnLimit: turnLimit, Live: projection.RunStatus.Live(),
		ContractSummary: contractSummary, LastVerdict: goalVerdictSummary(projection.LastVerdict),
		Context: contextSnapshot,
	}
}

func goalVerdictSummary(turn *goalpkg.Turn) *session.GoalVerdictSummary {
	if turn == nil || turn.VerdictOutcome == nil {
		return nil
	}
	issues := make([]session.GoalBlockingIssue, 0, len(turn.BlockingIssues))
	for _, issue := range turn.BlockingIssues {
		issues = append(issues, session.GoalBlockingIssue{ID: issue.ID, Note: issue.Note})
	}
	evaluatedAt := turn.StartedAt
	if turn.EndedAt != nil {
		evaluatedAt = turn.EndedAt.UTC()
	}
	return &session.GoalVerdictSummary{
		Outcome: string(*turn.VerdictOutcome), BlockingIssues: issues,
		EvidenceRef: cloneStringPointer(turn.EvidenceRef), EvaluatedAt: evaluatedAt,
	}
}

func goalStatusFromRun(status looppkg.Status) string {
	switch status {
	case looppkg.StatusPaused:
		return "paused"
	case looppkg.StatusDone, looppkg.StatusNoOp:
		return goalSnapshotStatusComplete
	case looppkg.StatusBlocked, looppkg.StatusFailed, looppkg.StatusExhausted, looppkg.StatusStalled:
		return goalSnapshotStatusBlocked
	default:
		return goalSnapshotStatusActive
	}
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

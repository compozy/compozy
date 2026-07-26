package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	core "github.com/compozy/agh/internal/api/core"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
)

// ListGoalTurns returns one workspace-owned run-wide Goal audit page.
func (s *daemonLoopAPIService) ListGoalTurns(
	ctx context.Context,
	workspaceID string,
	runID string,
	query core.GoalTurnListQuery,
) (session.GoalTurnPage, error) {
	if s == nil || s.goalPersistence == nil {
		return session.GoalTurnPage{}, errors.New("daemon: Goal read persistence is unavailable")
	}
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return session.GoalTurnPage{}, fmt.Errorf("daemon: normalize Goal turns workspace %q: %w", workspaceID, err)
	}
	page, err := s.goalPersistence.ListGoalTurns(ctx, goalpkg.TurnQuery{
		WorkspaceID: ws,
		LoopRunID:   looppkg.RunID(strings.TrimSpace(runID)),
		NodeID:      dsl.NodeID(strings.TrimSpace(query.NodeID)),
		ItemIndex:   query.ItemIndex,
		AfterSeq:    query.AfterSeq,
		Limit:       query.Limit,
	})
	if err != nil {
		return session.GoalTurnPage{}, fmt.Errorf("daemon: list Goal turns for run %q: %w", runID, err)
	}
	turns := make([]session.GoalTurn, 0, len(page.Turns))
	for _, turn := range page.Turns {
		turns = append(turns, sessionGoalTurn(turn))
	}
	return session.GoalTurnPage{Turns: turns, NextAfterSeq: page.NextAfterSeq}, nil
}

func sessionGoalTurn(turn goalpkg.Turn) session.GoalTurn {
	issues := make([]session.GoalBlockingIssue, 0, len(turn.BlockingIssues))
	for _, issue := range turn.BlockingIssues {
		issues = append(issues, session.GoalBlockingIssue{ID: issue.ID, Note: issue.Note})
	}
	result := session.GoalTurn{
		Seq: turn.Seq, Generation: int64(turn.Key.Generation), NodeID: string(turn.Key.NodeID),
		ItemIndex: turn.Key.ItemIndex, Turn: turn.Turn, PromptAttempt: turn.PromptAttempt,
		SessionID: turn.SessionID, BindingHandle: turn.BindingHandle, BindingEpoch: turn.BindingEpoch,
		PromptID: turn.PromptID, ResultStatus: cloneStringPointer(turn.ResultStatus),
		BlockingIssues: issues, EvidenceRef: cloneStringPointer(turn.EvidenceRef),
		PromptRef: cloneStringPointer(turn.PromptRef), TokensUsed: turn.TokensUsed,
		ActorKind: turn.ActorKind, ActorID: turn.ActorID, StartedAt: turn.StartedAt, EndedAt: turn.EndedAt,
	}
	if turn.ReasonCode != nil {
		value := session.GoalReasonCode(*turn.ReasonCode)
		result.ReasonCode = &value
	}
	if turn.StopReason != nil {
		value := string(*turn.StopReason)
		result.StopReason = &value
	}
	if turn.VerdictOutcome != nil {
		value := string(*turn.VerdictOutcome)
		result.VerdictOutcome = &value
	}
	return result
}

package daemon

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *daemonLoopAPIService) statusSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (session.GoalDispatchDecision, error) {
	snapshot, err := s.GetSessionGoal(ctx, workspaceID, sessionID)
	if err != nil {
		return session.GoalDispatchDecision{}, err
	}
	if snapshot == nil {
		return goalCommandErrorDecision(session.GoalReasonNotActive, nil), nil
	}
	return goalCommandSuccessDecision(session.GoalOutcomeStatus, snapshot, nil), nil
}

func (s *daemonLoopAPIService) pauseSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	actor taskpkg.ActorContext,
) (session.GoalDispatchDecision, error) {
	snapshot, run, decision, err := s.activeSessionGoal(ctx, workspaceID, sessionID)
	if err != nil || decision.Result != nil {
		return decision, err
	}
	if err := s.aggregate.Pause(ctx, run.WorkspaceID, run.ID, actor); err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	post, err := s.GetSessionGoal(ctx, workspaceID, sessionID)
	if err != nil {
		return session.GoalDispatchDecision{}, err
	}
	if post == nil {
		post = snapshot
	}
	return goalCommandSuccessDecision(session.GoalOutcomePaused, post, nil), nil
}

func (s *daemonLoopAPIService) resumeSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	actor taskpkg.ActorContext,
) (session.GoalDispatchDecision, error) {
	_, run, decision, err := s.activeSessionGoal(ctx, workspaceID, sessionID)
	if err != nil || decision.Result != nil {
		return decision, err
	}
	switch run.Status {
	case looppkg.StatusPaused:
		err = s.aggregate.Resume(ctx, run.WorkspaceID, run.ID, actor)
	case looppkg.StatusRunning:
		if !run.PauseRequested {
			return goalCommandErrorDecision(session.GoalReasonNotActive, nil), nil
		}
		err = s.aggregate.Resume(ctx, run.WorkspaceID, run.ID, actor)
	case looppkg.StatusNeedsApproval:
		if strings.TrimSpace(string(run.ActiveGateID)) == "" {
			return session.GoalDispatchDecision{}, fmt.Errorf(
				"%w: active Goal approval has no gate identity",
				looppkg.ErrTransitionConflict,
			)
		}
		err = s.aggregate.Approve(
			ctx,
			run.WorkspaceID,
			run.ID,
			run.ActiveGateID,
			looppkg.GateDecisionApprove,
			actor,
		)
	default:
		return goalCommandErrorDecision(session.GoalReasonNotActive, nil), nil
	}
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	post, err := s.GetSessionGoal(ctx, workspaceID, sessionID)
	if err != nil {
		return session.GoalDispatchDecision{}, err
	}
	if post == nil {
		return session.GoalDispatchDecision{}, fmt.Errorf(
			"%w: resumed Goal snapshot is unavailable",
			looppkg.ErrTransitionConflict,
		)
	}
	return goalCommandSuccessDecision(session.GoalOutcomeResumed, post, nil), nil
}

func (s *daemonLoopAPIService) clearSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	actor taskpkg.ActorContext,
) (session.GoalDispatchDecision, error) {
	err := s.aggregate.ClearInlineGoal(
		ctx,
		looppkg.WorkspaceID(strings.TrimSpace(workspaceID)),
		strings.TrimSpace(sessionID),
		actor,
	)
	if err != nil {
		return s.commandErrorDecision(ctx, workspaceID, sessionID, err)
	}
	return goalCommandSuccessDecision(session.GoalOutcomeCleared, nil, nil), nil
}

func (s *daemonLoopAPIService) activeSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (
	*session.GoalSnapshot,
	*looppkg.Run,
	session.GoalDispatchDecision,
	error,
) {
	snapshot, err := s.GetSessionGoal(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, nil, session.GoalDispatchDecision{}, err
	}
	if snapshot == nil || !snapshot.Live {
		return nil, nil, goalCommandErrorDecision(session.GoalReasonNotActive, nil), nil
	}
	run, err := s.aggregate.Get(
		ctx,
		looppkg.WorkspaceID(strings.TrimSpace(workspaceID)),
		looppkg.RunID(strings.TrimSpace(snapshot.RunID)),
	)
	if err != nil {
		return nil, nil, session.GoalDispatchDecision{}, err
	}
	return snapshot, run, session.GoalDispatchDecision{}, nil
}

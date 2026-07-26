package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

type goalCommandHandlerInstaller interface {
	SetGoalCommandHandler(session.GoalCommandHandler)
}

var _ goalCommandHandlerInstaller = (*session.Manager)(nil)
var _ session.GoalCommandHandler = (*daemonLoopAPIService)(nil)

// Handle executes one authenticated external `/goal` command against the Loop aggregate.
func (s *daemonLoopAPIService) Handle(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	caller session.PromptCaller,
	command session.GoalCommand,
) (session.GoalDispatchDecision, error) {
	if s == nil || s.aggregate == nil {
		return session.GoalDispatchDecision{}, errors.New("daemon: Goal command aggregate is unavailable")
	}
	if err := caller.Validate(); err != nil {
		return session.GoalDispatchDecision{}, err
	}
	actor, err := goalCommandActor(caller, workspaceID, sessionID)
	if err != nil {
		return goalCommandErrorDecision(session.GoalReasonCode(looppkg.ReasonCodeGoalOriginInvalid), nil), nil
	}
	switch strings.TrimSpace(command.Verb) {
	case session.GoalCommandVerbSet:
		return s.startSessionGoal(ctx, workspaceID, sessionID, command.Objective, actor)
	case session.GoalCommandVerbReplace:
		return s.replaceSessionGoal(
			ctx,
			workspaceID,
			sessionID,
			command.ExpectedRunID,
			command.Objective,
			actor,
		)
	case session.GoalCommandVerbStatus:
		return s.statusSessionGoal(ctx, workspaceID, sessionID)
	case session.GoalCommandVerbPause:
		return s.pauseSessionGoal(ctx, workspaceID, sessionID, actor)
	case session.GoalCommandVerbResume:
		return s.resumeSessionGoal(ctx, workspaceID, sessionID, actor)
	case session.GoalCommandVerbClear:
		return s.clearSessionGoal(ctx, workspaceID, sessionID, actor)
	case session.GoalCommandVerbDraft:
		return session.GoalDispatchDecision{
			Kind:             session.GoalDispatchPrompt,
			RewrittenMessage: goalDraftPrompt(command.Objective),
			BypassGoalParse:  true,
			BusyPolicy:       "reject-if-busy",
			BusyReason:       session.GoalReasonDraftRequiresIdle,
		}, nil
	default:
		return goalCommandErrorDecision(session.GoalReasonCommandInvalid, nil), nil
	}
}

func goalCommandActor(
	caller session.PromptCaller,
	workspaceID string,
	sessionID string,
) (taskpkg.ActorContext, error) {
	originKind := taskpkg.OriginKind(strings.TrimSpace(caller.Source)).Normalize()
	originRef := fmt.Sprintf("goal.command:%s", strings.TrimSpace(sessionID))
	switch taskpkg.ActorKind(strings.TrimSpace(caller.Kind)).Normalize() {
	case taskpkg.ActorKindHuman:
		return taskpkg.DeriveHumanActorContextForWorkspace(caller.ID, workspaceID, originKind, originRef)
	case taskpkg.ActorKindAgentSession:
		return taskpkg.DeriveAgentSessionActorContextForOrigin(caller.ID, workspaceID, originKind, originRef)
	default:
		return taskpkg.ActorContext{}, fmt.Errorf("%w: Goal caller kind is invalid", taskpkg.ErrValidation)
	}
}

func goalDraftPrompt(draft string) string {
	return "Turn this draft into a specific Goal objective. Return the objective first, followed only by optional " +
		"line-oriented `verify:` and `constraints:` clauses. Do not activate the Goal.\n\nDraft:\n" +
		strings.TrimSpace(draft)
}

func goalCommandSuccessDecision(
	outcome session.GoalCommandOutcome,
	snapshot *session.GoalSnapshot,
	replacedRunID *string,
) session.GoalDispatchDecision {
	return session.GoalDispatchDecision{
		Kind: session.GoalDispatchRespond,
		Result: &session.GoalCommandResult{
			Outcome: outcome, Snapshot: snapshot, ReplacedRunID: replacedRunID,
		},
	}
}

func goalCommandErrorDecision(
	reason session.GoalReasonCode,
	snapshot *session.GoalSnapshot,
) session.GoalDispatchDecision {
	return session.GoalDispatchDecision{
		Kind: session.GoalDispatchRespond,
		Result: &session.GoalCommandResult{
			Outcome: session.GoalOutcomeError, ReasonCode: &reason, Snapshot: snapshot,
		},
	}
}

func (s *daemonLoopAPIService) commandErrorDecision(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	err error,
) (session.GoalDispatchDecision, error) {
	if commandErr, ok := errors.AsType[*session.GoalCommandError](err); ok {
		return goalCommandErrorDecision(commandErr.Code, nil), nil
	}
	reasonErr, ok := errors.AsType[*looppkg.ReasonError](err)
	if !ok {
		if errors.Is(err, looppkg.ErrInvalidTransition) || errors.Is(err, looppkg.ErrTransitionConflict) {
			return goalCommandErrorDecision(
				session.GoalReasonCode(looppkg.ReasonCodeGoalControlStale),
				nil,
			), nil
		}
		return session.GoalDispatchDecision{}, err
	}
	reason := session.GoalReasonCode(reasonErr.Code)
	if reasonErr.Code != looppkg.ReasonCodeGoalReplaceRequired &&
		reasonErr.Code != looppkg.ReasonCodeGoalReplaceStale {
		return goalCommandErrorDecision(reason, nil), nil
	}
	snapshot, snapshotErr := s.GetSessionGoal(ctx, workspaceID, sessionID)
	if snapshotErr != nil {
		return session.GoalDispatchDecision{}, snapshotErr
	}
	if snapshot == nil {
		return goalCommandErrorDecision(session.GoalReasonNotActive, nil), nil
	}
	return goalCommandErrorDecision(reason, snapshot), nil
}

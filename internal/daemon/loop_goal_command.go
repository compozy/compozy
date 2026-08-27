package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type goalCommandHandlerInstaller interface {
	SetGoalCommandHandler(session.GoalCommandHandler)
}

var _ goalCommandHandlerInstaller = (*session.Manager)(nil)
var _ session.GoalCommandHandler = (*daemonLoopAPIService)(nil)

var errGoalCallerDenied = errors.New("goal caller is not authorized")

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
	verb := strings.TrimSpace(command.Verb)
	if err := s.authorizeGoalCaller(ctx, workspaceID, sessionID, caller); err != nil {
		if !errors.Is(err, errGoalCallerDenied) {
			s.logGoalCommandWarning(ctx, "Goal caller authorization failed", workspaceID, sessionID, err)
			return session.GoalDispatchDecision{}, err
		}
		s.logGoalCommandWarning(ctx, "Goal caller denied", workspaceID, sessionID, err)
		return goalCommandErrorDecision(session.GoalReasonCallerUnauthorized, nil), nil
	}
	if verb == session.GoalCommandVerbDraft {
		return session.GoalDispatchDecision{
			Kind:             session.GoalDispatchPrompt,
			RewrittenMessage: goalDraftPrompt(command.Objective),
			BypassGoalParse:  true,
			BusyPolicy:       "reject-if-busy",
			BusyReason:       session.GoalReasonDraftRequiresIdle,
		}, nil
	}
	runtime, err := goalCommandRuntime(command.Runtime)
	if err != nil {
		s.logGoalCommandWarning(ctx, "Goal runtime selection invalid", workspaceID, sessionID, err)
		return goalCommandErrorDecision(session.GoalReasonRuntimeInvalid, nil), nil
	}
	actor, err := goalCommandActor(caller, workspaceID, sessionID)
	if err != nil {
		s.logGoalCommandWarning(ctx, "Goal caller actor invalid", workspaceID, sessionID, err)
		return goalCommandErrorDecision(session.GoalReasonCode(looppkg.ReasonCodeGoalOriginInvalid), nil), nil
	}
	switch verb {
	case session.GoalCommandVerbSet:
		return s.startSessionGoal(ctx, workspaceID, sessionID, command.Objective, runtime, actor)
	case session.GoalCommandVerbReplace:
		return s.replaceSessionGoal(
			ctx,
			workspaceID,
			sessionID,
			command.ExpectedRunID,
			command.Objective,
			runtime,
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
	default:
		return goalCommandErrorDecision(session.GoalReasonCommandInvalid, nil), nil
	}
}

func goalCommandRuntime(runtime *session.RuntimeSelection) (*looppkg.RuntimeSpec, error) {
	if runtime == nil {
		return nil, nil
	}
	normalized, err := session.NormalizeRuntimeSelection(*runtime)
	if err != nil {
		return nil, err
	}
	return &looppkg.RuntimeSpec{
		Provider:   normalized.Provider,
		Model:      normalized.Model,
		Reasoning:  normalized.ReasoningEffort,
		Speed:      normalized.Speed,
		ACPOptions: loopACPOptionsFromSession(normalized.ACPOptions),
	}, nil
}

func (s *daemonLoopAPIService) authorizeGoalCaller(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	caller session.PromptCaller,
) error {
	if taskpkg.ActorKind(strings.TrimSpace(caller.Kind)).Normalize() != taskpkg.ActorKindAgentSession {
		return nil
	}
	target, callerInfo, err := s.loadGoalCallerSessions(ctx, sessionID, caller.ID)
	if err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if !goalSessionInWorkspace(target, workspaceID) {
		return fmt.Errorf("%w: target session is outside the caller workspace", errGoalCallerDenied)
	}
	if strings.TrimSpace(target.ID) != strings.TrimSpace(sessionID) {
		return fmt.Errorf("%w: target session identity is inconsistent", errGoalCallerDenied)
	}
	if !goalSessionIdentityMatches(callerInfo, caller.ID, workspaceID) {
		return fmt.Errorf("%w: agent caller session is not in the target workspace", errGoalCallerDenied)
	}
	if strings.TrimSpace(target.ID) == strings.TrimSpace(caller.ID) {
		return nil
	}
	return s.authorizeGoalDescendant(ctx, workspaceID, target, caller.ID)
}

func (s *daemonLoopAPIService) loadGoalCallerSessions(
	ctx context.Context,
	targetSessionID string,
	callerSessionID string,
) (*session.Info, *session.Info, error) {
	if s == nil || s.sessionStatus == nil {
		return nil, nil, errors.New("daemon: Goal caller authorization session reader is unavailable")
	}
	target, err := s.sessionStatus.Status(ctx, strings.TrimSpace(targetSessionID))
	if err != nil {
		return nil, nil, fmt.Errorf("load Goal target session: %w", err)
	}
	callerInfo, err := s.sessionStatus.Status(ctx, strings.TrimSpace(callerSessionID))
	if err != nil {
		return nil, nil, fmt.Errorf("load Goal caller session: %w", err)
	}
	return target, callerInfo, nil
}

func goalSessionIdentityMatches(info *session.Info, id string, workspaceID string) bool {
	return goalSessionInWorkspace(info, workspaceID) && strings.TrimSpace(info.ID) == strings.TrimSpace(id)
}

func goalSessionInWorkspace(info *session.Info, workspaceID string) bool {
	return info != nil && strings.TrimSpace(info.ID) != "" &&
		strings.TrimSpace(info.WorkspaceID) == strings.TrimSpace(workspaceID)
}

func (s *daemonLoopAPIService) authorizeGoalDescendant(
	ctx context.Context,
	workspaceID string,
	target *session.Info,
	callerID string,
) error {
	visited := map[string]struct{}{strings.TrimSpace(target.ID): {}}
	current := target
	for range 256 {
		if !goalSessionInWorkspace(current, workspaceID) || current.Lineage == nil {
			return fmt.Errorf("%w: target session is not a child of the caller", errGoalCallerDenied)
		}
		parentID := strings.TrimSpace(current.Lineage.ParentSessionID)
		if parentID == "" {
			return fmt.Errorf("%w: target session is not a child of the caller", errGoalCallerDenied)
		}
		if parentID == strings.TrimSpace(callerID) {
			return nil
		}
		if _, seen := visited[parentID]; seen {
			return fmt.Errorf("%w: target session lineage contains a cycle", errGoalCallerDenied)
		}
		current, err := s.sessionStatus.Status(ctx, parentID)
		if err != nil {
			return fmt.Errorf("load Goal target parent session: %w", err)
		}
		if !goalSessionIdentityMatches(current, parentID, workspaceID) {
			return fmt.Errorf("%w: target session parent is outside the caller workspace", errGoalCallerDenied)
		}
		visited[parentID] = struct{}{}
	}
	return fmt.Errorf("%w: target session lineage is too deep", errGoalCallerDenied)
}

func (s *daemonLoopAPIService) logGoalCommandWarning(
	ctx context.Context,
	message string,
	workspaceID string,
	sessionID string,
	err error,
) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.WarnContext(
		ctx,
		message,
		slog.String("workspace_id", strings.TrimSpace(workspaceID)),
		slog.String("session_id", strings.TrimSpace(sessionID)),
		slog.Any("error", err),
	)
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
		runtimeValidation, runtimeErr := looppkg.AsRuntimeValidationError(err)
		if runtimeErr && runtimeValidation != nil {
			return goalCommandErrorDecision(session.GoalReasonRuntimeInvalid, nil), nil
		}
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

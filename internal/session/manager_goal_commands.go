package session

import (
	"context"
	"errors"
	"strings"
)

const (
	goalBusyPolicyInherit      = "inherit"
	goalBusyPolicyRejectIfBusy = "reject-if-busy"
)

// SetGoalCommandHandler installs the late-bound daemon dispatcher before API servers start.
func (m *Manager) SetGoalCommandHandler(handler GoalCommandHandler) {
	if m == nil {
		return
	}
	m.goalCommandMu.Lock()
	m.goalCommandHandler = handler
	m.goalCommandMu.Unlock()
}

func (m *Manager) dispatchGoalCommand(
	ctx context.Context,
	req *promptRequest,
	opts SendPromptOpts,
) (GoalDispatchDecision, bool, error) {
	command, matched, parseErr := ParseGoalCommand(req.authoredMessage)
	if !matched {
		return GoalDispatchDecision{}, false, nil
	}
	workspaceID, lookupErr := m.goalCommandWorkspaceID(ctx, req.target)
	if lookupErr != nil {
		return GoalDispatchDecision{}, true, lookupErr
	}
	if err := opts.Caller.Validate(); err != nil {
		return GoalDispatchDecision{}, true, err
	}
	if err := m.recordPromptInputEvent(ctx, nil, req); err != nil {
		return GoalDispatchDecision{}, true, err
	}
	if parseErr != nil {
		var commandErr *GoalCommandError
		if !errors.As(parseErr, &commandErr) {
			return GoalDispatchDecision{}, true, parseErr
		}
		return goalErrorDecision(commandErr.Code), true, nil
	}
	handler := m.currentGoalCommandHandler()
	if handler == nil {
		return GoalDispatchDecision{}, true, errors.New("session: Goal command handler is unavailable")
	}
	decision, err := handler.Handle(ctx, workspaceID, req.target, opts.Caller, command)
	if err != nil {
		return GoalDispatchDecision{}, true, err
	}
	if err := validateGoalDispatchDecision(decision); err != nil {
		return GoalDispatchDecision{}, true, err
	}
	return decision, true, nil
}

func (m *Manager) goalCommandWorkspaceID(ctx context.Context, sessionID string) (string, error) {
	if session, ok := m.Get(sessionID); ok {
		info := session.Info()
		if info != nil && strings.TrimSpace(info.WorkspaceID) != "" {
			return strings.TrimSpace(info.WorkspaceID), nil
		}
		return "", errors.New("session: Goal command workspace is unavailable")
	}
	meta, err := m.readMetaWithContext(ctx, sessionID)
	if err != nil {
		return "", err
	}
	workspaceID := strings.TrimSpace(meta.WorkspaceID)
	if workspaceID == "" {
		return "", errors.New("session: Goal command workspace is unavailable")
	}
	return workspaceID, nil
}

func (m *Manager) currentGoalCommandHandler() GoalCommandHandler {
	if m == nil {
		return nil
	}
	m.goalCommandMu.RLock()
	defer m.goalCommandMu.RUnlock()
	return m.goalCommandHandler
}

func validateGoalDispatchDecision(decision GoalDispatchDecision) error {
	switch decision.Kind {
	case GoalDispatchRespond:
		if decision.Result == nil {
			return errors.New("session: Goal response decision has no result")
		}
		return nil
	case GoalDispatchPrompt:
		if strings.TrimSpace(decision.RewrittenMessage) == "" || !decision.BypassGoalParse ||
			decision.BusyPolicy != goalBusyPolicyRejectIfBusy || decision.BusyReason != GoalReasonDraftRequiresIdle {
			return errors.New("session: Goal prompt decision is incomplete")
		}
		return nil
	default:
		return errors.New("session: Goal dispatch kind is invalid")
	}
}

func goalErrorDecision(reason GoalReasonCode) GoalDispatchDecision {
	return GoalDispatchDecision{
		Kind: GoalDispatchRespond,
		Result: &GoalCommandResult{
			Outcome: GoalOutcomeError, ReasonCode: &reason,
		},
	}
}

func goalDraftBusyResult() SendPromptResult {
	reason := GoalReasonDraftRequiresIdle
	return SendPromptResult{
		Status: managedInputOwnerGoal,
		Goal:   &GoalCommandResult{Outcome: GoalOutcomeError, ReasonCode: &reason},
	}
}

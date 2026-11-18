package daemon

import (
	"context"
	"errors"
	"strings"

	sessionpkg "github.com/compozy/compozy/internal/session"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// nativeTerminalActor rechecks live run authority independently of an accepted tool approval.
func (n *daemonNativeTools) nativeTerminalActor(
	ctx context.Context,
	workspaceID string,
	profileID string,
	req toolspkg.CallRequest,
) (terminalpkg.Actor, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	runID := strings.TrimSpace(req.RunID)
	if sessionID == "" || runID == "" || req.Generation <= 0 {
		return terminalpkg.Actor{}, terminalSessionDeniedError(
			req.ToolID,
			"terminal action has incomplete prompt run identity",
		)
	}
	if n.deps.Sessions == nil {
		return terminalpkg.Actor{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"terminal session service is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonBackendUnhealthy,
		)
	}
	info, err := n.deps.Sessions.Status(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sessionpkg.ErrSessionNotFound) {
			return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID, "terminal session is unavailable")
		}
		return terminalpkg.Actor{}, err
	}
	if info == nil {
		return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID, "terminal session is unavailable")
	}
	active, err := n.deps.Sessions.ActivePromptRun(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sessionpkg.ErrPromptNotActive) {
			return terminalpkg.Actor{}, terminalSessionDeniedError(
				req.ToolID,
				"terminal action requires an active prompt run",
			)
		}
		return terminalpkg.Actor{}, err
	}
	if active.WorkspaceID != workspaceID || active.ProfileID != profileID || active.SessionID != sessionID {
		return terminalpkg.Actor{}, terminalSessionDeniedError(
			req.ToolID,
			"terminal action does not match the active session scope",
		)
	}
	if active.RunID != runID {
		return terminalpkg.Actor{}, terminalSessionDeniedError(
			req.ToolID,
			"terminal action belongs to a different prompt run",
		)
	}
	if active.Generation != req.Generation || info.RuntimeGeneration != req.Generation {
		return terminalpkg.Actor{}, &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeGenerationFenced,
			Message: "terminal agent run identity is stale",
			Err:     terminalpkg.ErrGenerationFenced,
		}
	}
	agentName := strings.TrimSpace(req.AgentName)
	if agentName == "" {
		agentName = strings.TrimSpace(info.AgentName)
	}
	if agentName == "" {
		return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID, "terminal agent identity is unavailable")
	}
	return terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: agentName, ProfileID: profileID,
		SessionID: sessionID, RunID: runID, Generation: req.Generation,
	}, nil
}

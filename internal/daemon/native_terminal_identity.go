package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sessionpkg "github.com/compozy/compozy/internal/session"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) nativeTerminalActor(
	ctx context.Context,
	workspaceID string,
	profileID string,
	req toolspkg.CallRequest,
) (terminalpkg.Actor, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	runID := strings.TrimSpace(req.RunID)
	if sessionID == "" || runID == "" || req.Generation <= 0 {
		return terminalpkg.Actor{}, fmt.Errorf(
			"terminal agent run identity is incomplete: %w", terminalpkg.ErrRunIdentityIncomplete,
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
			return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID)
		}
		return terminalpkg.Actor{}, err
	}
	if info == nil {
		return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID)
	}
	active, err := n.deps.Sessions.ActivePromptRun(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sessionpkg.ErrPromptNotActive) {
			return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID)
		}
		return terminalpkg.Actor{}, err
	}
	if active.WorkspaceID != workspaceID || active.ProfileID != profileID || active.SessionID != sessionID ||
		active.RunID != runID {
		return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID)
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
		return terminalpkg.Actor{}, terminalSessionDeniedError(req.ToolID)
	}
	return terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: agentName, ProfileID: profileID,
		SessionID: sessionID, RunID: runID, Generation: req.Generation,
	}, nil
}

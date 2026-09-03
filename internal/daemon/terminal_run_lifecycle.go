package daemon

import (
	"context"
	"errors"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
)

type terminalSessionRunEnder interface {
	SessionRunEnded(context.Context, string, string, string, string, int64) int
}

type terminalRunLifecycleObserver struct {
	terminals terminalSessionRunEnder
	sessions  core.SessionManager
}

func registerTerminalPromptRunEnd(sessions SessionManager, terminals terminalSessionRunEnder) error {
	if terminals == nil {
		return nil
	}
	registrar, ok := sessions.(turnEndNotifierRegistrar)
	if !ok {
		return errors.New("daemon: terminal prompt lifecycle requires turn-end registration")
	}
	registrar.AddTurnEndNotifier(terminalPromptRunEndNotifier(terminals))
	return nil
}

func terminalPromptRunEndNotifier(terminals terminalSessionRunEnder) session.TurnEndNotifier {
	return func(ctx context.Context, identity session.PromptRunIdentity) {
		if terminals == nil {
			return
		}
		terminals.SessionRunEnded(
			ctx,
			identity.WorkspaceID,
			identity.ProfileID,
			identity.SessionID,
			identity.RunID,
			identity.Generation,
		)
	}
}

func (o *terminalRunLifecycleObserver) OnTaskRunTerminal(
	ctx context.Context,
	payload hookspkg.TaskRunLeasePayload,
) error {
	if o == nil || o.terminals == nil || o.sessions == nil {
		return errors.New("daemon: terminal run lifecycle dependencies are unavailable")
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(payload.TargetSessionID)
	}
	if sessionID == "" || strings.TrimSpace(payload.ProfileID) == "" ||
		strings.TrimSpace(payload.WorkspaceID) == "" || strings.TrimSpace(payload.RunID) == "" {
		return errors.New("daemon: terminal run lifecycle identity is incomplete")
	}
	info, err := o.sessions.Status(ctx, sessionID)
	if err != nil {
		return err
	}
	if info == nil || info.RuntimeGeneration <= 0 {
		return nil
	}
	o.terminals.SessionRunEnded(
		ctx,
		strings.TrimSpace(payload.WorkspaceID),
		strings.TrimSpace(payload.ProfileID),
		sessionID,
		strings.TrimSpace(payload.RunID),
		info.RuntimeGeneration,
	)
	return nil
}

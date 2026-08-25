package daemon

import (
	"context"
	"errors"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

type terminalSessionRunEnder interface {
	SessionRunEnded(context.Context, string, string, int64) int
}

type terminalRunLifecycleObserver struct {
	terminals terminalSessionRunEnder
	sessions  core.SessionManager
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
	if sessionID == "" || strings.TrimSpace(payload.ProfileID) == "" {
		return nil
	}
	info, err := o.sessions.Status(ctx, sessionID)
	if err != nil {
		return err
	}
	if info == nil || info.RuntimeGeneration <= 0 {
		return nil
	}
	o.terminals.SessionRunEnded(ctx, payload.ProfileID, sessionID, info.RuntimeGeneration)
	return nil
}

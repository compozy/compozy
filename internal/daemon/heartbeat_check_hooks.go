package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func beforeInstanceCheck(ctx context.Context, hooks *hooksNotifier, target heartbeat.CheckTarget, source string) (string, error) {
	if hooks == nil {
		return "", nil
	}
	_, err := hooks.DispatchAgentHeartbeatWakeBefore(ctx, hookspkg.AgentHeartbeatWakeBeforePayload{
		Event:        hookspkg.HookAgentHeartbeatWakeBefore,
		Timestamp:    hooks.timestamp(),
		SessionID:    target.Instance.MainSessionID,
		AgentName:    target.Instance.AgentName,
		WorkspaceID:  target.Instance.WorkspaceID,
		ProfileID:    target.Instance.ProfileID,
		Source:       source,
		PolicyDigest: target.Policy.Digest,
		ConfigDigest: target.Policy.ConfigDigest,
	})
	if errors.Is(err, hookspkg.ErrHeartbeatWakeCancelled) {
		return "hook_cancelled", nil
	}
	if err != nil {
		return "", err
	}
	return "", nil
}

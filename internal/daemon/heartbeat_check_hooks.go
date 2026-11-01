package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func beforeInstanceCheck(
	ctx context.Context,
	hooks *hooksNotifier,
	target *heartbeat.CheckTarget,
	source string,
) (string, error) {
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
		if denied, ok := errors.AsType[*hookspkg.DeniedError](err); ok && strings.TrimSpace(denied.Reason) != "" {
			return strings.TrimSpace(denied.Reason), nil
		}
		return "The check was canceled by a hook.", nil
	}
	if err != nil {
		return "", err
	}
	return "", nil
}

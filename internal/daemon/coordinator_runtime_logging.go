package daemon

import (
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (r *coordinatorRuntime) logCoordinatorError(
	message string,
	err error,
	payload hookspkg.TaskRunEnqueuedPayload,
) {
	if r == nil {
		return
	}
	args := []any{
		coordinatorRuntimeTaskIDKey, strings.TrimSpace(payload.TaskID),
		daemonLogRunIDKey, strings.TrimSpace(payload.RunID),
		coordinatorRuntimeWorkspaceIDKey, strings.TrimSpace(payload.WorkspaceID),
		daemonNetworkChannelKey, strings.TrimSpace(payload.NetworkSpecSnapshot().ChannelID),
	}
	if err != nil {
		args = append(args, "error", err)
	}
	r.logger.Warn(message, args...)
}

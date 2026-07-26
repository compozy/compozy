package daemon

import (
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func nativeExtensionActorContext(req toolspkg.CallRequest) (taskpkg.ActorContext, error) {
	if sessionID := strings.TrimSpace(req.SessionID); sessionID != "" {
		return taskpkg.DeriveAgentSessionActorContextForOrigin(
			sessionID,
			req.WorkspaceID,
			taskpkg.OriginKindAgentSession,
			strings.TrimSpace(string(req.ToolID)),
		)
	}
	return taskpkg.DeriveDaemonActorContext("native-tools", strings.TrimSpace(string(req.ToolID)))
}

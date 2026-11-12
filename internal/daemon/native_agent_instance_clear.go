package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const nativeInstanceActionClearMain = "clear_main"

// clearNativeInstanceMain is the operator recovery for a main session that cannot resume: it
// reuses the session clear operation, so the session id, queued inputs, agent memory and the
// instance survive while the conversation and its provider thread are replaced.
func (n *daemonNativeTools) clearNativeInstanceMain(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	req toolspkg.CallRequest,
	instance agentinstance.Instance,
	input nativeInstanceInput,
) (toolspkg.ToolResult, error) {
	if input.DryRun {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID,
			errors.New("clear_main does not support dry_run; use agent_instance_get to inspect the instance"))
	}
	if instance.ArchivedAt != nil {
		return toolspkg.ToolResult{}, agentinstance.ErrNotFound
	}
	mainSessionID := strings.TrimSpace(instance.MainSessionID)
	if mainSessionID == "" {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID,
			errors.New("instance has no main session; use agent_instance_ensure_main to start one"))
	}
	if n.deps.Sessions == nil {
		return toolspkg.ToolResult{}, errors.New("sessions unavailable")
	}
	if _, err := n.deps.Sessions.ClearConversation(ctx, mainSessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err := api.Payload(ctx, instance)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return agentInstanceRecordViews.result(req.View, payload)
}

package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// instanceLifecycleHooks completes an instance lifecycle payload with its workspace root before dispatch.
// Configured hooks are scoped by workspace id and root, so a payload carrying only the id would never
// match a user's agent.instance.* declaration. The hooks describe an already committed transition, so they
// run on an execution lifetime independent of the request that caused it.
type instanceLifecycleHooks struct {
	dispatcher *hookspkg.Hooks
	resolve    func(context.Context, string, string) (workspacepkg.ResolvedWorkspace, error)
}

var _ agentinstance.LifecycleHooks = instanceLifecycleHooks{}

func newInstanceLifecycleHooks(dispatcher *hookspkg.Hooks, definitions instanceDefinitions) instanceLifecycleHooks {
	return instanceLifecycleHooks{dispatcher: dispatcher, resolve: definitions.resolveWorkspace}
}

func (h instanceLifecycleHooks) DispatchAgentInstance(
	ctx context.Context,
	payload hookspkg.AgentInstancePayload,
) (hookspkg.AgentInstancePayload, error) {
	ctx = context.WithoutCancel(ctx)
	var resolveErr error
	if strings.TrimSpace(payload.Workspace) == "" && strings.TrimSpace(payload.WorkspaceID) != "" {
		resolved, err := h.resolve(ctx, payload.ProfileID, payload.WorkspaceID)
		if err != nil {
			resolveErr = fmt.Errorf("daemon: resolve instance hook workspace %q: %w", payload.WorkspaceID, err)
		} else {
			payload.Workspace = resolved.RootDir
		}
	}
	dispatched, err := h.dispatcher.DispatchAgentInstance(ctx, payload)
	return dispatched, errors.Join(resolveErr, err)
}

func proposalPickHook(instance agentinstance.Instance, pick agentinstance.Pick) hookspkg.AgentInstancePayload {
	return hookspkg.AgentInstancePayload{
		Event: hookspkg.HookAgentInstanceProposalPicked, Timestamp: pick.PickedAt,
		ProfileID:     instance.ProfileID,
		WorkspaceID:   instance.WorkspaceID,
		SessionID:     pick.TargetSession,
		AgentName:     instance.AgentName,
		InstanceID:    instance.ID,
		MainSessionID: pick.TargetSession,
		Actor:         pick.Actor,
		ActionID:      pick.ActionID,
		ReportID:      pick.ReportID,
		ProposalIndex: new(pick.ProposalIndex),
	}
}

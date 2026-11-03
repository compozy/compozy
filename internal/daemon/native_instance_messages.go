package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) resolveNamedInstanceMessage(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	input *sessionPromptInput,
) (*store.InstancePromptTarget, error) {
	if strings.TrimSpace(input.Agent) == "" {
		return nil, nil
	}
	if n.deps.AgentInstances == nil || n.deps.AgentInstances() == nil {
		return nil, nativeUnavailableError(id, "agent instances unavailable")
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, id, input.Workspace, scope)
	if err != nil {
		return nil, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return nil, nativeInputError(id, err)
	}
	target := &store.InstancePromptTarget{
		ProfileID:   scope.ProfileID,
		WorkspaceID: workspaceID,
		AgentName:   strings.TrimSpace(input.Agent),
	}
	rows, err := n.deps.AgentInstances().Store.List(ctx, agentinstance.ListQuery{
		ProfileID: target.ProfileID, WorkspaceID: target.WorkspaceID, AgentName: target.AgentName, Limit: 1,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, toolspkg.NewToolError(
			"agent_instance_not_found",
			id,
			"No instance exists for this agent in this scope.",
			agentinstance.ErrNotFound,
		)
	}
	if rows[0].MainSessionID == "" {
		return nil, toolspkg.NewToolError(
			"agent_instance_starting",
			id,
			"The instance has no main session yet.",
			agentinstance.ErrStarting,
		)
	}
	if !scope.Operator {
		active, ok := n.deps.Sessions.(interface{ IsPrompting(string) bool })
		if !ok || !active.IsPrompting(scope.SessionID) {
			return nil, nativeInputError(id, errors.New("agent messages require an active work turn"))
		}
	}
	input.SessionID = rows[0].MainSessionID
	return target, nil
}

package daemon

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (n *daemonNativeTools) nativeAgentActivity(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input nativeInstanceInput,
	actor workspaceaccess.ActorRef,
) (toolspkg.ToolResult, error) {
	if input.Days == 0 {
		input.Days = 30
	}
	if input.Days < 1 || input.Days > 365 {
		return toolspkg.ToolResult{}, errors.New("days must be from 1 to 365")
	}
	if input.AllWorkspaces {
		return toolspkg.ToolResult{}, errors.New("choose one workspace for agent activity")
	}
	target, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&target)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if !scope.Operator {
		if n.deps.WorkspaceAccess == nil {
			return toolspkg.ToolResult{}, agentinstance.ErrForbidden
		}
		decision, err := n.deps.WorkspaceAccess.Authorize(
			ctx,
			workspaceaccess.Request{
				Actor:    actor,
				Target:   workspaceaccess.ScopeTarget(workspaceID, scope.ProfileID),
				ReadOnly: true,
				Seam:     workspaceaccess.SeamTool,
			},
		)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if !decision.Allowed {
			return toolspkg.ToolResult{}, agentinstance.ErrForbidden
		}
	}
	reader, ok := n.deps.AgentInstances().Store.(agentinstance.ActivityReader)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("agent activity unavailable")
	}
	result, err := reader.ReadActivity(
		ctx,
		agentinstance.ActivityQuery{
			ProfileID:   scope.ProfileID,
			WorkspaceID: workspaceID,
			AgentName:   input.Agent,
			Since:       store.LocalDayStart(time.Now(), input.Days-1),
		},
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	label := target.Name
	if workspaceID == "" {
		label = "Global"
	}
	for index := range result.CostByProject {
		result.CostByProject[index].Scope = label
	}
	return structuredResult(result, "Agent activity")
}

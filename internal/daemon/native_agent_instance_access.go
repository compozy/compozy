package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (n *daemonNativeTools) authorizeNativeInstanceApproval(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input nativeInstanceInput,
	action string,
) error {
	if !scope.Operator && (action == "create" || action == "archive" || (action == "check" && !input.DryRun)) &&
		!req.ApprovalGranted {
		approved := false
		if n.deps.Sessions != nil {
			info, err := n.deps.Sessions.Status(ctx, scope.SessionID)
			if err != nil {
				return err
			}
			approved = info != nil && info.EffectivePermissions == string(toolspkg.PermissionModeApproveAll)
		}
		if !approved {
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeApprovalRequired,
				req.ToolID,
				"instance operation requires operator approval",
				toolspkg.ErrToolApprovalRequired,
				toolspkg.ReasonApprovalRequired,
			)
		}
	}
	return nil
}

func (n *daemonNativeTools) createNativeInstance(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input nativeInstanceInput,
	actor workspaceaccess.ActorRef,
) (toolspkg.ToolResult, error) {
	target, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&target)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	instance, created, err := api.Service.Create(
		ctx,
		agentinstance.CreateRequest{
			Agent:     input.Agent,
			Workspace: workspaceID,
			Profile:   scope.ProfileID,
			Actor:     actor,
		},
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err := api.Payload(ctx, instance)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload.Created = created
	return structuredResult(contract.AgentInstanceResponse{Instance: payload}, "Instance created")
}

func (n *daemonNativeTools) nativeInstanceQuery(
	ctx context.Context,
	scope toolspkg.Scope,
	input *nativeInstanceInput,
	actor workspaceaccess.ActorRef,
	action string,
) (agentinstance.ListQuery, error) {
	if input.Limit == 0 {
		input.Limit = 50
	}
	if input.Limit < 1 || input.Limit > 100 {
		return agentinstance.ListQuery{}, errors.New("limit must be from 1 to 100")
	}
	workspaceID := scope.WorkspaceID
	if input.Workspace == "@user" {
		workspaceID = ""
	} else if input.Workspace != "" {
		resolved, err := n.deps.WorkspaceResolver.Resolve(ctx, input.Workspace)
		if err != nil {
			return agentinstance.ListQuery{}, err
		}
		workspaceID = resolved.ID
	}
	if input.AllWorkspaces && !scope.Operator {
		decision, err := n.deps.WorkspaceAccess.Authorize(
			ctx,
			workspaceaccess.Request{
				Actor:    actor,
				Target:   workspaceaccess.TargetRef{Kind: workspaceaccess.TargetAggregate, ProfileID: scope.ProfileID},
				ReadOnly: true,
				Seam:     workspaceaccess.SeamCatalog,
			},
		)
		if err != nil {
			return agentinstance.ListQuery{}, err
		}
		if !decision.Allowed {
			return agentinstance.ListQuery{}, agentinstance.ErrForbidden
		}
	}
	query := agentinstance.ListQuery{
		AgentName:     input.Agent,
		ProfileID:     scope.ProfileID,
		WorkspaceID:   workspaceID,
		AllWorkspaces: input.AllWorkspaces,
		Limit:         input.Limit,
	}
	if action == "list" {
		query.AfterID = input.Cursor
		query.State = input.State
	}
	return query, nil
}

func (n *daemonNativeTools) authorizeNativeInstanceAction(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	scope toolspkg.Scope,
	actor workspaceaccess.ActorRef,
	instance agentinstance.Instance,
	input nativeInstanceInput,
	action string,
) error {
	if instance.ProfileID != scope.ProfileID {
		return agentinstance.ErrNotFound
	}
	if (action == "away" || action == "mark_seen") && !scope.Operator {
		return agentinstance.ErrForbidden
	}
	verb := action
	if action == "get" || action == "check_ins" {
		verb = "reports"
	}
	if action == "checks" {
		if input.Enabled == nil {
			return errors.New("enabled is required")
		}
		verb = "checks_off"
		if *input.Enabled {
			verb = "checks_on"
		}
	}
	if action == "check" && input.DryRun {
		verb = "check_dry_run"
	}
	if err := api.Authority.Authorize(ctx, actor, instance, verb); err != nil {
		return err
	}
	if !scope.Operator {
		if n.deps.WorkspaceAccess == nil {
			return agentinstance.ErrWorkspaceAccessDenied
		}
		decision, accessErr := n.deps.WorkspaceAccess.Authorize(
			ctx,
			workspaceaccess.Request{
				Actor:    actor,
				Target:   workspaceaccess.ScopeTarget(instance.WorkspaceID, instance.ProfileID),
				ReadOnly: action == "reports" || action == "get" || action == "check_ins",
				Seam:     workspaceaccess.SeamTool,
			},
		)
		if accessErr != nil {
			return accessErr
		}
		if !decision.Allowed {
			return agentinstance.ErrWorkspaceAccessDenied
		}
	}
	return nil
}

func nativeInstanceList(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	profileID string,
	actor workspaceaccess.ActorRef,
	query agentinstance.ListQuery,
) (toolspkg.ToolResult, error) {
	rows, err := api.Store.List(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result := contract.AgentInstanceListResponse{Instances: []contract.AgentInstancePayload{}}
	for _, row := range rows {
		if err := api.Authority.Authorize(ctx, actor, row, "list"); err != nil {
			if errors.Is(err, agentinstance.ErrForbidden) {
				continue
			}
			return toolspkg.ToolResult{}, err
		}
		payload, err := api.Payload(ctx, row)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		result.Instances = append(result.Instances, payload)
	}
	if len(rows) == query.Limit {
		result.NextCursor = rows[len(rows)-1].ID
	}
	metadata, err := api.ListResponse(ctx, profileID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result.MaxActiveInstances, result.ActiveCount = metadata.MaxActiveInstances, metadata.ActiveCount
	return structuredResult(result, "Agent instances")
}

func nativeInstanceTarget(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	actor workspaceaccess.ActorRef,
	id string,
	query agentinstance.ListQuery,
) (agentinstance.Instance, error) {
	if id != "" {
		return api.Store.Get(ctx, id)
	}
	for {
		rows, err := api.Store.List(ctx, query)
		if err != nil {
			return agentinstance.Instance{}, err
		}
		for _, row := range rows {
			if err := api.Authority.Authorize(ctx, actor, row, "list"); err != nil {
				if errors.Is(err, agentinstance.ErrForbidden) {
					continue
				}
				return agentinstance.Instance{}, err
			}
			return row, nil
		}
		if len(rows) != query.Limit {
			return agentinstance.Instance{}, agentinstance.ErrNotFound
		}
		query.AfterID = rows[len(rows)-1].ID
	}
}

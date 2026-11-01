package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"strings"
	"time"
)

type nativeInstanceInput struct {
	Agent         string `json:"agent"`
	Workspace     string `json:"workspace"`
	ID            string `json:"id"`
	AllWorkspaces bool   `json:"all_workspaces"`
	State         string `json:"state"`
	Cursor        string `json:"cursor"`
	Limit         int    `json:"limit"`
}

func (n *daemonNativeTools) agentInstanceBindings() map[toolspkg.ToolID]nativeToolBinding {
	available := n.dependencyAvailability(func() bool { return n.deps.AgentInstances != nil && n.deps.AgentInstances() != nil })
	bindings := make(map[toolspkg.ToolID]nativeToolBinding)
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstanceCreate, toolspkg.ToolIDAgentInstanceList,
		toolspkg.ToolIDAgentInstancePause, toolspkg.ToolIDAgentInstanceResume, toolspkg.ToolIDAgentInstanceArchive,
		toolspkg.ToolIDAgentInstanceEnsureMain, toolspkg.ToolIDAgentInstanceReports} {
		bindings[id] = nativeToolBinding{call: n.agentInstanceCall, availability: available}
	}
	return bindings
}
func (n *daemonNativeTools) agentInstanceCall(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
	var input nativeInstanceInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	api := n.deps.AgentInstances()
	if api == nil {
		return toolspkg.ToolResult{}, errors.New("agent instances unavailable")
	}
	actor := workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: scope.SessionID,
		WorkspaceID: scope.WorkspaceID, AgentName: scope.AgentName, Operator: scope.Operator}
	if scope.Operator {
		actor.Kind = workspaceaccess.ActorHuman
	}
	if input.Agent == "" {
		input.Agent = scope.AgentName
	}
	if input.Workspace == "" {
		input.Workspace = scope.WorkspaceID
	}
	action := strings.TrimPrefix(req.ToolID.String(), "compozy__agent_instance_")
	if !scope.Operator && (action == "create" || action == "archive") && !req.ApprovalGranted {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(toolspkg.ErrorCodeApprovalRequired, req.ToolID,
			"instance operation requires operator approval", toolspkg.ErrToolApprovalRequired, toolspkg.ReasonApprovalRequired)
	}
	if action == "create" {
		target, err := n.authoredAgentTarget(ctx, req.ToolID, scope.ProfileID, input.Workspace, input.Agent)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		instance, _, err := api.Service.Create(ctx, agentinstance.CreateRequest{Agent: input.Agent, Workspace: target.workspaceID, Profile: scope.ProfileID, Actor: actor})
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(contract.AgentInstanceResponse{Instance: core.AgentInstancePayload(instance)}, "Instance created")
	}
	if input.Limit == 0 {
		input.Limit = 50
	}
	if input.Limit < 1 || input.Limit > 100 {
		return toolspkg.ToolResult{}, errors.New("limit must be from 1 to 100")
	}
	workspaceID := scope.WorkspaceID
	if input.Workspace == "@user" {
		workspaceID = ""
	} else if input.Workspace != "" {
		resolved, err := n.deps.WorkspaceResolver.Resolve(ctx, input.Workspace)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		workspaceID = resolved.ID
	}
	if input.AllWorkspaces && !scope.Operator {
		decision, err := n.deps.WorkspaceAccess.Authorize(ctx, workspaceaccess.Request{Actor: actor, Target: workspaceaccess.TargetRef{Kind: workspaceaccess.TargetAggregate, ProfileID: scope.ProfileID}, ReadOnly: true, Seam: workspaceaccess.SeamCatalog})
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if !decision.Allowed {
			return toolspkg.ToolResult{}, agentinstance.ErrForbidden
		}
	}
	query := agentinstance.ListQuery{ProfileID: scope.ProfileID, WorkspaceID: workspaceID, AllWorkspaces: input.AllWorkspaces, Limit: input.Limit}
	if action == "list" {
		query.AfterID = input.Cursor
	}
	var instance agentinstance.Instance
	if input.ID != "" && action != "list" {
		var err error
		instance, err = api.Store.Get(ctx, input.ID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
	} else {
		for {
			rows, err := api.Store.List(ctx, query)
			if err != nil {
				return toolspkg.ToolResult{}, err
			}
			result := contract.AgentInstanceListResponse{Instances: []contract.AgentInstancePayload{}}
			for _, row := range rows {
				if input.Agent != "" && row.AgentName != input.Agent {
					continue
				}
				if err := api.Authority.Authorize(ctx, actor, row, "list"); err != nil {
					if errors.Is(err, agentinstance.ErrForbidden) {
						continue
					}
					return toolspkg.ToolResult{}, err
				}
				if action != "list" {
					instance = row
					break
				}
				if input.State == "" || row.State == input.State {
					result.Instances = append(result.Instances, core.AgentInstancePayload(row))
				}
			}
			if len(rows) == query.Limit {
				result.NextCursor = rows[len(rows)-1].ID
			}
			if action == "list" {
				return structuredResult(result, "Agent instances")
			}
			if instance.ID != "" {
				break
			}
			if result.NextCursor == "" {
				return toolspkg.ToolResult{}, agentinstance.ErrNotFound
			}
			query.AfterID = result.NextCursor
		}
	}
	if err := api.Authority.Authorize(ctx, actor, instance, action); err != nil {
		return toolspkg.ToolResult{}, err
	}
	var err error
	switch action {
	case "pause":
		instance, err = api.Service.Pause(ctx, instance.ID, "manual")
	case "resume":
		instance, err = api.Service.Resume(ctx, instance.ID)
	case "archive":
		err = api.Service.Archive(ctx, instance.ID)
	case "ensure_main":
		instance, err = api.Service.EnsureMain(ctx, instance.ID)
	case "reports":
		return nativeInstanceReports(ctx, api, instance.ID, input)
	default:
		return toolspkg.ToolResult{}, errors.New("unsupported instance operation")
	}
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.AgentInstanceResponse{Instance: core.AgentInstancePayload(instance)}, "Instance updated")
}
func nativeInstanceReports(ctx context.Context, api *core.AgentInstanceAPI, id string, input nativeInstanceInput) (toolspkg.ToolResult, error) {
	switch input.State {
	case "", "pending", "reserved", "delivered", "indeterminate":
	default:
		return toolspkg.ToolResult{}, errors.New("invalid report state")
	}
	query := agentinstance.ReportQuery{InstanceID: id, State: input.State, Limit: input.Limit}
	if input.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		parts := strings.Split(string(raw), "\n")
		if len(parts) != 4 || parts[0] != id || parts[3] != input.State {
			return toolspkg.ToolResult{}, errors.New("invalid report cursor")
		}
		query.AfterCreatedAt, err = time.Parse(time.RFC3339Nano, parts[1])
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		query.AfterID = parts[2]
	}
	rows, err := api.Reports.ListReports(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result := contract.AgentInstanceReportsResponse{Reports: []contract.AgentInstanceReportPayload{}}
	for _, report := range rows {
		var payload contract.AgentInstanceReportPayload
		if err := json.Unmarshal(report.Payload, &payload); err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload.ID, payload.Kind, payload.State, payload.CreatedAt = report.ID, report.Kind, report.State, report.CreatedAt
		result.Reports = append(result.Reports, payload)
	}
	if len(rows) == input.Limit {
		last := rows[len(rows)-1]
		result.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(id + "\n" + last.CreatedAt.Format(time.RFC3339Nano) + "\n" + last.ID + "\n" + input.State))
	}
	return structuredResult(result, "Instance reports")
}

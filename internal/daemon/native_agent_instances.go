package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"strings"
	"time"
)

type nativeInstanceInput struct {
	DryRun        bool   `json:"dry_run"`
	Enabled       *bool  `json:"enabled"`
	OccurrenceID  string `json:"occurrence_id"`
	Days          int    `json:"days"`
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
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstanceCheck, toolspkg.ToolIDAgentInstanceChecks, toolspkg.ToolIDAgentInstanceOccurrence, toolspkg.ToolIDAgentInstanceCreate, toolspkg.ToolIDAgentInstanceList,
		toolspkg.ToolIDAgentInstancePause, toolspkg.ToolIDAgentInstanceResume, toolspkg.ToolIDAgentInstanceArchive,
		toolspkg.ToolIDAgentInstanceEnsureMain, toolspkg.ToolIDAgentInstanceReports, toolspkg.ToolIDAgentActivity, toolspkg.ToolIDAgentInstanceGet, toolspkg.ToolIDAgentInstanceAway, toolspkg.ToolIDAgentInstanceSeen, toolspkg.ToolIDAgentInstanceCheckIns} {
		bindings[id] = nativeToolBinding{call: n.agentInstanceCall, availability: available}
	}
	return bindings
}
func (n *daemonNativeTools) agentInstanceCall(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (result toolspkg.ToolResult, callErr error) {
	var input nativeInstanceInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	api := n.deps.AgentInstances()
	if api == nil {
		return toolspkg.ToolResult{}, errors.New("agent instances unavailable")
	}
	defer func() {
		if message := agentinstance.ErrorMessage(callErr, api.MaxActiveInstances); message != "" {
			callErr = fmt.Errorf("%w: %s", callErr, message)
		}
	}()
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
	if req.ToolID == toolspkg.ToolIDAgentActivity {
		return n.nativeAgentActivity(ctx, scope, req, input, actor)
	}
	action := strings.TrimPrefix(req.ToolID.String(), "compozy__agent_instance_")
	if !scope.Operator && (action == "create" || action == "archive" || (action == "check" && !input.DryRun)) && !req.ApprovalGranted {
		approved := false
		if n.deps.Sessions != nil {
			info, err := n.deps.Sessions.Status(ctx, scope.SessionID)
			if err != nil {
				return toolspkg.ToolResult{}, err
			}
			approved = info != nil && info.EffectivePermissions == string(toolspkg.PermissionModeApproveAll)
		}
		if !approved {
			return toolspkg.ToolResult{}, toolspkg.NewToolError(toolspkg.ErrorCodeApprovalRequired, req.ToolID,
				"instance operation requires operator approval", toolspkg.ErrToolApprovalRequired, toolspkg.ReasonApprovalRequired)
		}
	}
	if action == "create" {
		target, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		workspaceID, err := nativeResolvedRegistryWorkspaceID(&target)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		instance, created, err := api.Service.Create(ctx, agentinstance.CreateRequest{Agent: input.Agent, Workspace: workspaceID, Profile: scope.ProfileID, Actor: actor})
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
	query := agentinstance.ListQuery{AgentName: input.Agent, ProfileID: scope.ProfileID, WorkspaceID: workspaceID, AllWorkspaces: input.AllWorkspaces, Limit: input.Limit}
	if action == "list" {
		query.AfterID = input.Cursor
		query.State = input.State
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
				payload, err := api.Payload(ctx, row)
				if err != nil {
					return toolspkg.ToolResult{}, err
				}
				result.Instances = append(result.Instances, payload)
			}
			if len(rows) == query.Limit {
				result.NextCursor = rows[len(rows)-1].ID
			}
			if action == "list" {
				metadata, err := api.ListResponse(ctx, scope.ProfileID)
				if err != nil {
					return toolspkg.ToolResult{}, err
				}
				result.MaxActiveInstances, result.ActiveCount = metadata.MaxActiveInstances, metadata.ActiveCount
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
	if instance.ProfileID != scope.ProfileID {
		return toolspkg.ToolResult{}, agentinstance.ErrNotFound
	}
	if (action == "away" || action == "mark_seen") && !scope.Operator {
		return toolspkg.ToolResult{}, agentinstance.ErrForbidden
	}
	verb := action
	if action == "get" || action == "check_ins" {
		verb = "reports"
	}
	if action == "checks" {
		if input.Enabled == nil {
			return toolspkg.ToolResult{}, errors.New("enabled is required")
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
		return toolspkg.ToolResult{}, err
	}
	if !scope.Operator {
		if n.deps.WorkspaceAccess == nil {
			return toolspkg.ToolResult{}, agentinstance.ErrWorkspaceAccessDenied
		}
		decision, accessErr := n.deps.WorkspaceAccess.Authorize(ctx, workspaceaccess.Request{Actor: actor, Target: workspaceaccess.ScopeTarget(instance.WorkspaceID, instance.ProfileID), ReadOnly: action == "reports" || action == "get" || action == "check_ins", Seam: workspaceaccess.SeamTool})
		if accessErr != nil {
			return toolspkg.ToolResult{}, accessErr
		}
		if !decision.Allowed {
			return toolspkg.ToolResult{}, agentinstance.ErrWorkspaceAccessDenied
		}
	}
	var err error
	switch action {
	case "check":
		if api.Checks == nil {
			return toolspkg.ToolResult{}, errors.New("instance checks unavailable")
		}
		result, err := api.Checks.Check(ctx, instance.ID, input.DryRun)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		response := contract.AgentInstanceCheckResponse{Gate: result.Gate, OccurrenceID: result.OccurrenceID}
		if result.Digest != nil {
			response.Digest = &contract.AgentInstanceCheckDigest{ItemKeys: []string{}, Fingerprint: result.Digest.Fingerprint, Diagnostics: result.Digest.Diagnostics}
			for _, item := range result.Digest.Items {
				response.Digest.ItemKeys = append(response.Digest.ItemKeys, item.Key)
			}
		}
		return structuredResult(response, "Instance check")
	case "checks":
		if api.CheckSettings == nil {
			return toolspkg.ToolResult{}, errors.New("instance check settings unavailable")
		}
		instance, err = api.CheckSettings.SetChecks(ctx, instance.ID, *input.Enabled)
	case "occurrence":
		reader, ok := api.Store.(core.AgentInstanceOccurrenceReader)
		if !ok {
			return toolspkg.ToolResult{}, errors.New("instance occurrences unavailable")
		}
		if strings.TrimSpace(input.OccurrenceID) == "" {
			return toolspkg.ToolResult{}, errors.New("occurrence_id is required")
		}
		result, err := reader.Occurrence(ctx, instance.ID, input.OccurrenceID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(contract.AgentInstanceOccurrenceResponse{State: result.State, Result: result.Result, Reason: result.Reason, ReportID: result.ReportID}, "Instance occurrence")
	case "get":
	case "away":
		result, err := api.Away(ctx, instance.ID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(result, "Instance away")
	case "mark_seen":
		reader, ok := api.Store.(agentinstance.SeenStore)
		if !ok {
			return toolspkg.ToolResult{}, errors.New("instance seen storage unavailable")
		}
		at, err := reader.MarkSeen(ctx, instance.ID, agentinstance.LocalOperator, time.Now())
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(contract.AgentInstanceSeenResponse{LastSeenAt: at}, "Instance marked seen")
	case "check_ins":
		result, err := api.CheckIns(ctx, instance.ID, input.Cursor, input.Limit)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(result, "Instance check-ins")
	case "pause":
		instance, err = api.Service.Pause(ctx, instance.ID, "manual")
	case "resume":
		instance, err = api.Service.Resume(ctx, instance.ID, actor)
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
	payload, err := api.Payload(ctx, instance)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.AgentInstanceResponse{Instance: payload}, "Instance updated")
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

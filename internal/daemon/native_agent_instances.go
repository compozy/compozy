package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
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
	available := n.dependencyAvailability(
		func() bool { return n.deps.AgentInstances != nil && n.deps.AgentInstances() != nil },
	)
	bindings := make(map[toolspkg.ToolID]nativeToolBinding)
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDAgentInstanceCheck, toolspkg.ToolIDAgentInstanceChecks, toolspkg.ToolIDAgentInstanceOccurrence, toolspkg.ToolIDAgentInstanceCreate, toolspkg.ToolIDAgentInstanceList,
		toolspkg.ToolIDAgentInstancePause, toolspkg.ToolIDAgentInstanceResume, toolspkg.ToolIDAgentInstanceArchive,
		toolspkg.ToolIDAgentInstanceEnsureMain, toolspkg.ToolIDAgentInstanceReports, toolspkg.ToolIDAgentActivity, toolspkg.ToolIDAgentInstanceGet, toolspkg.ToolIDAgentInstanceAway, toolspkg.ToolIDAgentInstanceSeen, toolspkg.ToolIDAgentInstanceCheckIns} {
		bindings[id] = nativeToolBinding{call: n.agentInstanceCall, availability: available}
	}
	return bindings
}

func (n *daemonNativeTools) agentInstanceCall(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (result toolspkg.ToolResult, callErr error) {
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
	if err := n.authorizeNativeInstanceApproval(ctx, scope, req, input, action); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if action == "create" {
		return n.createNativeInstance(ctx, api, scope, req, input, actor)
	}
	query, err := n.nativeInstanceQuery(ctx, scope, &input, actor, action)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if action == "list" {
		return nativeInstanceList(ctx, api, scope.ProfileID, actor, query)
	}
	instance, err := nativeInstanceTarget(ctx, api, actor, input.ID, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := n.authorizeNativeInstanceAction(ctx, api, scope, actor, instance, input, action); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return executeNativeInstanceAction(ctx, api, actor, instance, input, action)
}

func executeNativeInstanceAction(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	actor workspaceaccess.ActorRef,
	instance agentinstance.Instance,
	input nativeInstanceInput,
	action string,
) (toolspkg.ToolResult, error) {
	var err error
	switch action {
	case "check":
		return nativeInstanceCheck(ctx, api, instance.ID, input)
	case "checks":
		if api.CheckSettings == nil {
			return toolspkg.ToolResult{}, errors.New("instance check settings unavailable")
		}
		instance, err = api.CheckSettings.SetChecks(ctx, instance.ID, *input.Enabled)
	case "occurrence":
		return nativeInstanceOccurrence(ctx, api, instance.ID, input)
	case "get":
	case "away":
		result, err := api.Away(ctx, instance.ID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(result, "Instance away")
	case "mark_seen":
		return nativeInstanceMarkSeen(ctx, api, instance.ID)
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

func nativeInstanceReports(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	id string,
	input nativeInstanceInput,
) (toolspkg.ToolResult, error) {
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
		result.NextCursor = base64.RawURLEncoding.EncodeToString(
			[]byte(id + "\n" + last.CreatedAt.Format(time.RFC3339Nano) + "\n" + last.ID + "\n" + input.State),
		)
	}
	return structuredResult(result, "Instance reports")
}

func nativeInstanceCheck(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	id string,
	input nativeInstanceInput,
) (toolspkg.ToolResult, error) {
	if api.Checks == nil {
		return toolspkg.ToolResult{}, errors.New("instance checks unavailable")
	}
	result, err := api.Checks.Check(ctx, id, input.DryRun)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response := contract.AgentInstanceCheckResponse{Gate: result.Gate, OccurrenceID: result.OccurrenceID}
	if result.Digest != nil {
		response.Digest = &contract.AgentInstanceCheckDigest{
			ItemKeys:    []string{},
			Fingerprint: result.Digest.Fingerprint,
			Diagnostics: result.Digest.Diagnostics,
		}
		for _, item := range result.Digest.Items {
			response.Digest.ItemKeys = append(response.Digest.ItemKeys, item.Key)
		}
	}
	return structuredResult(response, "Instance check")
}

func nativeInstanceOccurrence(
	ctx context.Context,
	api *core.AgentInstanceAPI,
	id string,
	input nativeInstanceInput,
) (toolspkg.ToolResult, error) {
	reader, ok := api.Store.(core.AgentInstanceOccurrenceReader)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("instance occurrences unavailable")
	}
	if strings.TrimSpace(input.OccurrenceID) == "" {
		return toolspkg.ToolResult{}, errors.New("occurrence_id is required")
	}
	result, err := reader.Occurrence(ctx, id, input.OccurrenceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		contract.AgentInstanceOccurrenceResponse{
			State:    result.State,
			Result:   result.Result,
			Reason:   result.Reason,
			ReportID: result.ReportID,
		},
		"Instance occurrence",
	)
}

func nativeInstanceMarkSeen(ctx context.Context, api *core.AgentInstanceAPI, id string) (toolspkg.ToolResult, error) {
	reader, ok := api.Store.(agentinstance.SeenStore)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("instance seen storage unavailable")
	}
	at, err := reader.MarkSeen(ctx, id, agentinstance.LocalOperator, time.Now())
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.AgentInstanceSeenResponse{LastSeenAt: at}, "Instance marked seen")
}

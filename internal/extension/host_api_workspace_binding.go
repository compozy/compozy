package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/resources"
)

// HostAPIWorkspaceBinding identifies how a workspace-bound Host API call is constrained.
type HostAPIWorkspaceBinding uint8

const (
	HostAPIWorkspaceBindingNone HostAPIWorkspaceBinding = iota
	HostAPIWorkspaceBindingActor
	HostAPIWorkspaceBindingPath
	HostAPIWorkspaceBindingID
	HostAPIWorkspaceBindingTask
	HostAPIWorkspaceBindingMemory
	HostAPIWorkspaceBindingAutomation
	HostAPIWorkspaceBindingResource
)

type hostAPIWorkspaceBinding = HostAPIWorkspaceBinding

const (
	hostAPIWorkspaceBindingNone       = HostAPIWorkspaceBindingNone
	hostAPIWorkspaceBindingActor      = HostAPIWorkspaceBindingActor
	hostAPIWorkspaceBindingPath       = HostAPIWorkspaceBindingPath
	hostAPIWorkspaceBindingID         = HostAPIWorkspaceBindingID
	hostAPIWorkspaceBindingTask       = HostAPIWorkspaceBindingTask
	hostAPIWorkspaceBindingMemory     = HostAPIWorkspaceBindingMemory
	hostAPIWorkspaceBindingAutomation = HostAPIWorkspaceBindingAutomation
	hostAPIWorkspaceBindingResource   = HostAPIWorkspaceBindingResource
	hostAPIWorkspaceScopeKindKey      = "kind"
)

// Every Host API method must have an explicit workspace-binding decision.
var hostAPIWorkspaceBindings = map[extensionprotocol.HostAPIMethod]hostAPIWorkspaceBinding{
	extensionprotocol.HostAPIMethodSessionsList:                hostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodSessionsCreate:              hostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodSessionsPrompt:              hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsStop:                hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsStatus:              hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsEvents:              hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsSoulRefresh:         hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsHealthGet:           hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsStatusGet:           hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSandboxList:                 hostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodSandboxInfo:                 hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSandboxExec:                 hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodMemoryRecall:                hostAPIWorkspaceBindingMemory,
	extensionprotocol.HostAPIMethodMemoryStore:                 hostAPIWorkspaceBindingMemory,
	extensionprotocol.HostAPIMethodMemoryForget:                hostAPIWorkspaceBindingMemory,
	extensionprotocol.HostAPIMethodObserveHealth:               hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodListLogs:                    hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSkillsList:                  hostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodModelsList:                  hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodModelsRefresh:               hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodModelsStatus:                hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAgentsSoulGet:               hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulValidate:          hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulPut:               hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulDelete:            hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulHistory:           hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulRollback:          hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatGet:          hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatValidate:     hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatPut:          hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatDelete:       hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatHistory:      hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatRollback:     hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatStatus:       hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatWake:         hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAutomationJobs:              hostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationJobsGet:           hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsCreate:        hostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationJobsUpdate:        hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsDelete:        hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsTrigger:       hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsRuns:          hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggers:          hostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationTriggersGet:       hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersCreate:    hostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationTriggersUpdate:    hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersDelete:    hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersRuns:      hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersFire:      hostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationRuns:              hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasks:                       hostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksGet:                    hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksTimeline:               hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksTree:                   hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksDashboard:              hostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksInbox:                  hostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksCreate:                 hostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksUpdate:                 hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksCancel:                 hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRuns:                   hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsGet:                hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsEnqueue:            hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsStart:              hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsAttachSession:      hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsComplete:           hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsFail:               hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsCancel:             hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodNetworkStatus:               hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodNetworkUsage:                hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkChannels:             hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkPeers:                hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkThreads:              hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkThreadGet:            hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkThreadMessages:       hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkDirects:              hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkDirectResolve:        hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkDirectMessages:       hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkWorkGet:              hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkSend:                 hostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodResourcesList:               hostAPIWorkspaceBindingResource,
	extensionprotocol.HostAPIMethodResourcesGet:                hostAPIWorkspaceBindingActor,
	extensionprotocol.HostAPIMethodResourcesSnapshot:           hostAPIWorkspaceBindingResource,
	extensionprotocol.HostAPIMethodBridgesInstancesList:        hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodBridgesMessagesIngest:       hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodBridgesInstancesGet:         hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodBridgesInstancesReportState: hostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodClarifyAsk:                  hostAPIWorkspaceBindingNone,
}

// HostAPIWorkspaceBindingFor returns the canonical workspace-binding decision for one Host API method.
func HostAPIWorkspaceBindingFor(method extensionprotocol.HostAPIMethod) (HostAPIWorkspaceBinding, bool) {
	binding, ok := hostAPIWorkspaceBindings[method]
	return binding, ok
}

func (h *HostAPIHandler) bindWorkspaceScopedParams(
	ctx context.Context,
	method string,
	raw json.RawMessage,
) (json.RawMessage, error) {
	workspaceID, bound, err := hostAPIBoundWorkspaceID(ctx)
	if err != nil || !bound {
		return raw, err
	}
	binding, ok := hostAPIWorkspaceBindings[extensionprotocol.HostAPIMethod(method)]
	if !ok {
		return nil, unavailableRPCError(fmt.Errorf("workspace binding is not declared for %q", method))
	}
	if binding == hostAPIWorkspaceBindingNone || binding == hostAPIWorkspaceBindingActor {
		return raw, nil
	}
	if h.workspaces == nil {
		return nil, unavailableRPCError(errors.New("workspace resolver is not configured"))
	}
	resolved, err := h.workspaces.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, unavailableRPCError(fmt.Errorf("resolve bound workspace %q: %w", workspaceID, err))
	}

	params, err := decodeWorkspaceBindingParams(raw)
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	switch binding {
	case hostAPIWorkspaceBindingPath:
		err = h.bindWorkspaceReference(ctx, params, "workspace", resolved.ID, resolved.RootDir)
	case hostAPIWorkspaceBindingID:
		err = h.bindWorkspaceReference(ctx, params, "workspace_id", resolved.ID, resolved.ID)
	case hostAPIWorkspaceBindingTask, hostAPIWorkspaceBindingMemory:
		err = errors.Join(
			bindWorkspaceScopeLiteral(params),
			h.bindWorkspaceReference(ctx, params, "workspace", resolved.ID, resolved.RootDir),
		)
	case hostAPIWorkspaceBindingAutomation:
		err = errors.Join(
			bindWorkspaceScopeLiteral(params),
			h.bindWorkspaceReference(ctx, params, "workspace_id", resolved.ID, resolved.ID),
		)
	case hostAPIWorkspaceBindingResource:
		err = h.bindResourceWorkspaceScope(ctx, params, resolved.ID)
	default:
		err = fmt.Errorf("unknown workspace binding %d", binding)
	}
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, unavailableRPCError(fmt.Errorf("encode workspace-bound params: %w", err))
	}
	return encoded, nil
}

func hostAPIBoundWorkspaceID(ctx context.Context) (string, bool, error) {
	resourceSession, ok := hostAPIResourceSessionFromContext(ctx)
	if !ok {
		return "", false, nil
	}
	scope := resourceSession.Actor.MaxScope.Normalize()
	if scope.Kind != resources.ResourceScopeKindWorkspace {
		return "", false, nil
	}
	if err := scope.Validate("extension.max_scope"); err != nil {
		return "", false, invalidParamsRPCError(err)
	}
	return scope.ID, true, nil
}

func decodeWorkspaceBindingParams(raw json.RawMessage) (map[string]any, error) {
	params := make(map[string]any)
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == manifestNullKey {
		return params, nil
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("params must be an object: %w", err)
	}
	return params, nil
}

func (h *HostAPIHandler) bindWorkspaceReference(
	ctx context.Context,
	params map[string]any,
	key string,
	workspaceID string,
	canonical string,
) error {
	if value, ok := params[key]; ok && value != nil {
		provided, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", key)
		}
		provided = strings.TrimSpace(provided)
		if provided != "" {
			resolved, err := h.workspaces.Resolve(ctx, provided)
			if err != nil {
				return fmt.Errorf("resolve %s %q: %w", key, provided, err)
			}
			if strings.TrimSpace(resolved.ID) != strings.TrimSpace(workspaceID) {
				return fmt.Errorf("%s conflicts with the bound workspace", key)
			}
		}
	}
	params[key] = canonical
	return nil
}

func bindWorkspaceScopeLiteral(params map[string]any) error {
	if value, ok := params["scope"]; ok && value != nil {
		provided, ok := value.(string)
		if !ok || (strings.TrimSpace(provided) != "" &&
			strings.TrimSpace(provided) != hostAPIAuthoredContextWorkspaceKey) {
			return errors.New("scope conflicts with the bound workspace")
		}
	}
	params["scope"] = hostAPIAuthoredContextWorkspaceKey
	return nil
}

func (h *HostAPIHandler) bindResourceWorkspaceScope(
	ctx context.Context,
	params map[string]any,
	workspaceID string,
) error {
	scope := map[string]any{
		hostAPIWorkspaceScopeKindKey: hostAPIAuthoredContextWorkspaceKey,
		"id":                         workspaceID,
	}
	if records, ok := params["records"].([]any); ok {
		for index, value := range records {
			record, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("records[%d] must be an object", index)
			}
			if err := h.validateResourceWorkspaceScope(ctx, record["scope"], workspaceID); err != nil {
				return fmt.Errorf("records[%d].scope: %w", index, err)
			}
			record["scope"] = scope
		}
		return nil
	}
	if err := h.validateResourceWorkspaceScope(ctx, params["scope"], workspaceID); err != nil {
		return err
	}
	params["scope"] = scope
	return nil
}

func (h *HostAPIHandler) validateResourceWorkspaceScope(
	ctx context.Context,
	value any,
	workspaceID string,
) error {
	if value == nil {
		return nil
	}
	scope, ok := value.(map[string]any)
	if !ok {
		return errors.New("scope must be an object")
	}
	kind, kindOK := scope[hostAPIWorkspaceScopeKindKey].(string)
	id, idOK := scope["id"].(string)
	if !kindOK || !idOK ||
		strings.TrimSpace(kind) != hostAPIAuthoredContextWorkspaceKey ||
		strings.TrimSpace(id) == "" {
		return errors.New("scope conflicts with the bound workspace")
	}
	resolved, err := h.workspaces.Resolve(ctx, id)
	if err != nil {
		return fmt.Errorf("resolve scope workspace %q: %w", id, err)
	}
	if strings.TrimSpace(resolved.ID) != strings.TrimSpace(workspaceID) {
		return errors.New("scope conflicts with the bound workspace")
	}
	return nil
}

package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/resources"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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

// Every Host API method must have an explicit workspace-binding decision.
var hostAPIWorkspaceBindings = map[extensionprotocol.HostAPIMethod]HostAPIWorkspaceBinding{
	extensionprotocol.HostAPIMethodSessionsList:                HostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodSessionsCreate:              HostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodSessionsPrompt:              HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsRuntimeSet:          HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsRuntimeClear:        HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsInputsList:          HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsInputsReplace:       HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsInputsCancel:        HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsInputsPromote:       HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsStop:                HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsArchive:             HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsUnarchive:           HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsStatus:              HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsEvents:              HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsSoulRefresh:         HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsHealthGet:           HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSessionsStatusGet:           HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSandboxList:                 HostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodSandboxInfo:                 HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSandboxExec:                 HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodMemoryRecall:                HostAPIWorkspaceBindingMemory,
	extensionprotocol.HostAPIMethodMemoryStore:                 HostAPIWorkspaceBindingMemory,
	extensionprotocol.HostAPIMethodMemoryForget:                HostAPIWorkspaceBindingMemory,
	extensionprotocol.HostAPIMethodObserveHealth:               HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodListLogs:                    HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodSkillsList:                  HostAPIWorkspaceBindingPath,
	extensionprotocol.HostAPIMethodModelsList:                  HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodModelsRefresh:               HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodModelsStatus:                HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAgentsSoulGet:               HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulValidate:          HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulPut:               HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulDelete:            HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulHistory:           HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsSoulRollback:          HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatGet:          HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatValidate:     HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatPut:          HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatDelete:       HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatHistory:      HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatRollback:     HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatStatus:       HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAgentsHeartbeatWake:         HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodAutomationJobs:              HostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationJobsGet:           HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsCreate:        HostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationJobsUpdate:        HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsDelete:        HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsTrigger:       HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationJobsRuns:          HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggers:          HostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationTriggersGet:       HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersCreate:    HostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationTriggersUpdate:    HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersDelete:    HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersRuns:      HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodAutomationTriggersFire:      HostAPIWorkspaceBindingAutomation,
	extensionprotocol.HostAPIMethodAutomationRuns:              HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasks:                       HostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksGet:                    HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksTimeline:               HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksTree:                   HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksDashboard:              HostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksInbox:                  HostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksCreate:                 HostAPIWorkspaceBindingTask,
	extensionprotocol.HostAPIMethodTasksUpdate:                 HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksCancel:                 HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRuns:                   HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsGet:                HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsEnqueue:            HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsStart:              HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsAttachSession:      HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsComplete:           HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsFail:               HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodTasksRunsCancel:             HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodNetworkStatus:               HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodNetworkUsage:                HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkChannels:             HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkPeers:                HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkThreads:              HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkThreadGet:            HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkThreadMessages:       HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkDirects:              HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkDirectResolve:        HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkDirectMessages:       HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkWorkGet:              HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodNetworkSend:                 HostAPIWorkspaceBindingID,
	extensionprotocol.HostAPIMethodResourcesList:               HostAPIWorkspaceBindingResource,
	extensionprotocol.HostAPIMethodResourcesGet:                HostAPIWorkspaceBindingActor,
	extensionprotocol.HostAPIMethodResourcesSnapshot:           HostAPIWorkspaceBindingResource,
	extensionprotocol.HostAPIMethodBridgesInstancesList:        HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodBridgesMessagesIngest:       HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodBridgesInstancesGet:         HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodBridgesInstancesReportState: HostAPIWorkspaceBindingNone,
	extensionprotocol.HostAPIMethodClarifyAsk:                  HostAPIWorkspaceBindingNone,
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
	if binding == HostAPIWorkspaceBindingNone || binding == HostAPIWorkspaceBindingActor {
		return raw, nil
	}
	if h.workspaces == nil {
		return nil, unavailableRPCError(errors.New("workspace resolver is not configured"))
	}
	resolved, err := h.workspaces.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, unavailableRPCError(fmt.Errorf("resolve bound workspace %q: %w", workspaceID, err))
	}

	encoded, err := BindHostAPIWorkspaceParams(
		ctx,
		raw,
		binding,
		resolved.ID,
		resolved.RootDir,
		h.workspaces,
	)
	if err != nil {
		return nil, invalidParamsRPCError(err)
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

// BindHostAPIWorkspaceParams canonicalizes Host API workspace fields against a bound workspace.
func BindHostAPIWorkspaceParams(
	ctx context.Context,
	raw json.RawMessage,
	binding HostAPIWorkspaceBinding,
	workspaceID string,
	workspaceRoot string,
	workspaces workspacepkg.RuntimeResolver,
) (json.RawMessage, error) {
	if binding == HostAPIWorkspaceBindingActor {
		return raw, nil
	}
	if binding == HostAPIWorkspaceBindingNone {
		return nil, errors.New("projected method has no workspace binding")
	}
	params, err := decodeWorkspaceBindingParams(raw)
	if err != nil {
		return nil, err
	}
	switch binding {
	case HostAPIWorkspaceBindingPath:
		err = bindWorkspaceReference(ctx, params, "workspace", workspaceID, workspaceRoot, workspaces)
	case HostAPIWorkspaceBindingID:
		err = bindWorkspaceReference(ctx, params, "workspace_id", workspaceID, workspaceID, workspaces)
	case HostAPIWorkspaceBindingTask, HostAPIWorkspaceBindingMemory:
		err = errors.Join(
			bindWorkspaceScopeLiteral(params),
			bindWorkspaceReference(ctx, params, "workspace", workspaceID, workspaceRoot, workspaces),
		)
	case HostAPIWorkspaceBindingAutomation:
		err = errors.Join(
			bindWorkspaceScopeLiteral(params),
			bindWorkspaceReference(ctx, params, "workspace_id", workspaceID, workspaceID, workspaces),
		)
	case HostAPIWorkspaceBindingResource:
		err = bindResourceWorkspaceScope(ctx, params, workspaceID, workspaces)
	default:
		err = fmt.Errorf("unknown workspace binding %d", binding)
	}
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode workspace-bound params: %w", err)
	}
	return encoded, nil
}

func bindWorkspaceReference(
	ctx context.Context,
	params map[string]any,
	key string,
	workspaceID string,
	canonical string,
	workspaces workspacepkg.RuntimeResolver,
) error {
	if value, ok := params[key]; ok && value != nil {
		provided, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", key)
		}
		provided = strings.TrimSpace(provided)
		if provided != "" && provided != canonical && provided != workspaceID {
			if workspaces == nil {
				return fmt.Errorf("%s conflicts with the bound workspace", key)
			}
			resolved, err := workspaces.Resolve(ctx, provided)
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

func bindResourceWorkspaceScope(
	ctx context.Context,
	params map[string]any,
	workspaceID string,
	workspaces workspacepkg.RuntimeResolver,
) error {
	scope := map[string]any{
		hostAPIKindKey: hostAPIAuthoredContextWorkspaceKey,
		"id":           workspaceID,
	}
	if records, ok := params["records"].([]any); ok {
		for index, value := range records {
			record, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("records[%d] must be an object", index)
			}
			if err := validateResourceWorkspaceScope(ctx, record["scope"], workspaceID, workspaces); err != nil {
				return fmt.Errorf("records[%d].scope: %w", index, err)
			}
			record["scope"] = scope
		}
		return nil
	}
	if err := validateResourceWorkspaceScope(ctx, params["scope"], workspaceID, workspaces); err != nil {
		return err
	}
	params["scope"] = scope
	return nil
}

func validateResourceWorkspaceScope(
	ctx context.Context,
	value any,
	workspaceID string,
	workspaces workspacepkg.RuntimeResolver,
) error {
	if value == nil {
		return nil
	}
	scope, ok := value.(map[string]any)
	if !ok {
		return errors.New("scope must be an object")
	}
	kind, kindOK := scope[hostAPIKindKey].(string)
	id, idOK := scope["id"].(string)
	if !kindOK || !idOK ||
		strings.TrimSpace(kind) != hostAPIAuthoredContextWorkspaceKey ||
		strings.TrimSpace(id) == "" {
		return errors.New("scope conflicts with the bound workspace")
	}
	id = strings.TrimSpace(id)
	if id == strings.TrimSpace(workspaceID) {
		return nil
	}
	if workspaces == nil {
		return errors.New("scope conflicts with the bound workspace")
	}
	resolved, err := workspaces.Resolve(ctx, id)
	if err != nil {
		return fmt.Errorf("resolve scope workspace %q: %w", id, err)
	}
	if strings.TrimSpace(resolved.ID) != strings.TrimSpace(workspaceID) {
		return errors.New("scope conflicts with the bound workspace")
	}
	return nil
}

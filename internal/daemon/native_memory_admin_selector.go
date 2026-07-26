package daemon

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) memoryAdminLocation(
	ctx context.Context,
	callerScope toolspkg.Scope,
	id toolspkg.ToolID,
	input memoryAdminSelectorInput,
	defaultScope memcontract.Scope,
) (memoryToolLocation, error) {
	return n.memoryStoreFor(ctx, callerScope, id, input.memoryToolSelector(), defaultScope)
}

func (n *daemonNativeTools) memoryAdminResolvedSelector(
	ctx context.Context,
	callerScope toolspkg.Scope,
	input memoryAdminSelectorInput,
) (contract.MemoryScopeSelectorPayload, error) {
	scope, err := core.ParseOptionalMemoryScope(input.Scope)
	if err != nil {
		return contract.MemoryScopeSelectorPayload{}, err
	}
	workspaceID := ""
	workspaceRef := firstNonEmpty(input.WorkspaceID, input.Workspace, callerScope.WorkspaceID)
	if workspaceRef != "" {
		resolvedID, _, err := n.memoryWorkspaceIdentity(ctx, workspaceRef)
		if err != nil {
			return contract.MemoryScopeSelectorPayload{}, err
		}
		workspaceID = resolvedID
	}
	agentTier := memcontract.AgentTier(input.AgentTier).Normalize()
	if strings.TrimSpace(input.AgentName) != "" && agentTier == "" {
		agentTier = memcontract.AgentTierWorkspace
	}
	if agentTier != "" {
		if err := agentTier.Validate(); err != nil {
			return contract.MemoryScopeSelectorPayload{}, core.NewMemoryValidationError(err)
		}
	}
	return contract.MemoryScopeSelectorPayload{
		Scope:       scope,
		WorkspaceID: workspaceID,
		AgentName:   strings.TrimSpace(firstNonEmpty(input.AgentName, callerScope.AgentName)),
		AgentTier:   agentTier,
	}, nil
}

func (n *daemonNativeTools) memoryAdminOperationHistoryQuery(
	ctx context.Context,
	callerScope toolspkg.Scope,
	input memoryAdminHistoryInput,
) (memcontract.OperationHistoryQuery, error) {
	scope, err := core.ParseOptionalMemoryScope(input.Scope)
	if err != nil {
		return memcontract.OperationHistoryQuery{}, err
	}
	workspace := ""
	workspaceRef := firstNonEmpty(input.WorkspaceID, input.Workspace, callerScope.WorkspaceID)
	if workspaceRef != "" || scope == memcontract.ScopeWorkspace {
		_, resolvedWorkspace, err := n.memoryWorkspaceIdentity(ctx, workspaceRef)
		if err != nil {
			return memcontract.OperationHistoryQuery{}, err
		}
		workspace = resolvedWorkspace
	}
	return memcontract.OperationHistoryQuery{
		Scope:     scope,
		Workspace: workspace,
		Operation: memcontract.Operation(strings.TrimSpace(input.Operation)),
		Limit:     input.Limit,
	}, nil
}

func (i memoryAdminSelectorInput) memoryToolSelector() memoryToolSelector {
	return memoryToolSelector{
		Scope:     i.Scope,
		Workspace: firstNonEmpty(i.WorkspaceID, i.Workspace),
		AgentName: i.AgentName,
		AgentTier: i.AgentTier,
	}
}

func (n *daemonNativeTools) memoryAdminDefaultScope(
	callerScope toolspkg.Scope,
	input memoryAdminSelectorInput,
) memcontract.Scope {
	if scope := memcontract.Scope(input.Scope).Normalize(); scope != "" {
		return scope
	}
	if strings.TrimSpace(firstNonEmpty(input.AgentName, callerScope.AgentName)) != "" {
		return memcontract.ScopeAgent
	}
	if strings.TrimSpace(firstNonEmpty(input.WorkspaceID, input.Workspace, callerScope.WorkspaceID)) != "" {
		return memcontract.ScopeWorkspace
	}
	return memcontract.ScopeGlobal
}

func memoryAdminSelectorPayload(location memoryToolLocation) contract.MemoryScopeSelectorPayload {
	return contract.MemoryScopeSelectorPayload{
		Scope:       location.Scope.Normalize(),
		WorkspaceID: strings.TrimSpace(location.WorkspaceID),
		AgentName:   strings.TrimSpace(location.AgentName),
		AgentTier:   location.AgentTier.Normalize(),
	}
}

func memoryAdminPrecedencePayloads(location memoryToolLocation) []contract.MemoryScopeSelectorPayload {
	payloads := []contract.MemoryScopeSelectorPayload{{Scope: memcontract.ScopeGlobal}}
	if strings.TrimSpace(location.WorkspaceID) != "" {
		payloads = append(payloads, contract.MemoryScopeSelectorPayload{
			Scope:       memcontract.ScopeWorkspace,
			WorkspaceID: location.WorkspaceID,
		})
	}
	if strings.TrimSpace(location.AgentName) != "" {
		payloads = append(payloads, memoryAdminSelectorPayload(location))
	}
	return payloads
}

func (n *daemonNativeTools) memoryAdminRoots(location memoryToolLocation) map[string]string {
	roots := map[string]string{}
	if global := strings.TrimSpace(n.deps.Config.Memory.GlobalDir); global != "" {
		roots["global"] = global
	}
	if workspace := strings.TrimSpace(location.Workspace); workspace != "" {
		roots["workspace"] = workspace
	}
	if agent := strings.TrimSpace(location.AgentName); agent != "" {
		roots["agent"] = "memory://agent/" + agent
	}
	return roots
}

func memoryAdminSelectorFromPayload(payload contract.MemoryScopeSelectorPayload) memoryToolSelector {
	return memoryToolSelector{
		Scope:     string(payload.Scope),
		Workspace: payload.WorkspaceID,
		AgentName: payload.AgentName,
		AgentTier: string(payload.AgentTier),
	}
}

func memoryAdminSelectorInputFromPayload(payload contract.MemoryScopeSelectorPayload) memoryAdminSelectorInput {
	return memoryAdminSelectorInput{
		Scope:       string(payload.Scope),
		WorkspaceID: payload.WorkspaceID,
		AgentName:   payload.AgentName,
		AgentTier:   string(payload.AgentTier),
	}
}

package daemon

import (
	"context"

	"errors"
	"fmt"

	"os"

	"strings"

	core "github.com/compozy/agh/internal/api/core"

	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"

	toolspkg "github.com/compozy/agh/internal/tools"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (n *daemonNativeTools) resolveMemoryLocation(
	ctx context.Context,
	callerScope toolspkg.Scope,
	id toolspkg.ToolID,
	filename string,
	selector memoryToolSelector,
) (memoryToolLocation, error) {
	trimmedFilename, err := requiredNativeString(id, "filename", filename)
	if err != nil {
		return memoryToolLocation{}, err
	}
	scope, err := core.ParseOptionalMemoryScope(selector.Scope)
	if err != nil {
		return memoryToolLocation{}, err
	}
	if scope != "" {
		location, err := n.memoryStoreFor(ctx, callerScope, id, selector, scope)
		if err != nil {
			return memoryToolLocation{}, err
		}
		exists, err := location.Store.Exists(location.Scope, trimmedFilename)
		if err != nil {
			return memoryToolLocation{}, err
		}
		if !exists {
			return memoryToolLocation{}, fmt.Errorf("%w: memory %q not found", os.ErrNotExist, trimmedFilename)
		}
		location.Filename = trimmedFilename
		return location, nil
	}
	candidates := []memoryToolLocation{
		{Store: n.deps.MemoryStore, Scope: memcontract.ScopeGlobal, Filename: trimmedFilename},
	}
	if strings.TrimSpace(firstNonEmpty(selector.Workspace, callerScope.WorkspaceID)) != "" {
		workspaceSelector := selector
		workspaceSelector.Scope = string(memcontract.ScopeWorkspace)
		location, err := n.memoryStoreFor(ctx, callerScope, id, workspaceSelector, memcontract.ScopeWorkspace)
		if err != nil {
			return memoryToolLocation{}, err
		}
		location.Filename = trimmedFilename
		candidates = append(candidates, location)
	}
	if strings.TrimSpace(firstNonEmpty(selector.AgentName, callerScope.AgentName)) != "" {
		agentSelector := selector
		agentSelector.Scope = string(memcontract.ScopeAgent)
		location, err := n.memoryStoreFor(ctx, callerScope, id, agentSelector, memcontract.ScopeAgent)
		if err != nil {
			return memoryToolLocation{}, err
		}
		location.Filename = trimmedFilename
		candidates = append(candidates, location)
	}
	matches := make([]memoryToolLocation, 0, len(candidates))
	for _, candidate := range candidates {
		exists, err := candidate.Store.Exists(candidate.Scope, trimmedFilename)
		if err != nil {
			return memoryToolLocation{}, err
		}
		if exists {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return memoryToolLocation{}, fmt.Errorf("%w: memory %q not found", os.ErrNotExist, trimmedFilename)
	case 1:
		return matches[0], nil
	default:
		return memoryToolLocation{}, core.NewMemoryValidationError(
			fmt.Errorf("memory %q exists in multiple scopes; set scope explicitly", trimmedFilename),
		)
	}
}

func (n *daemonNativeTools) memoryStoreFor(
	ctx context.Context,
	callerScope toolspkg.Scope,
	id toolspkg.ToolID,
	selector memoryToolSelector,
	defaultScope memcontract.Scope,
) (memoryToolLocation, error) {
	scope, err := core.ParseOptionalMemoryScope(selector.Scope)
	if err != nil {
		return memoryToolLocation{}, err
	}
	if scope == "" {
		scope = defaultScope.Normalize()
	}
	if scope == "" {
		scope = memcontract.ScopeGlobal
	}
	workspaceRef := firstNonEmpty(selector.Workspace, callerScope.WorkspaceID)
	switch scope.Normalize() {
	case memcontract.ScopeGlobal:
		return memoryToolLocation{Store: n.deps.MemoryStore, Scope: memcontract.ScopeGlobal}, nil
	case memcontract.ScopeWorkspace:
		workspaceID, workspace, err := n.memoryWorkspaceIdentity(ctx, workspaceRef)
		if err != nil {
			return memoryToolLocation{}, err
		}
		if workspace == "" {
			return memoryToolLocation{}, core.NewMemoryValidationError(
				errors.New("workspace is required for workspace memory scope"),
			)
		}
		return memoryToolLocation{
			Store:       n.deps.MemoryStore.ForWorkspace(workspace),
			Scope:       memcontract.ScopeWorkspace,
			Workspace:   workspace,
			WorkspaceID: workspaceID,
		}, nil
	case memcontract.ScopeAgent:
		return n.agentMemoryStoreFor(ctx, callerScope, id, selector, workspaceRef)
	default:
		return memoryToolLocation{}, core.NewMemoryValidationError(fmt.Errorf("unsupported scope %q", scope))
	}
}

func (n *daemonNativeTools) memoryRecallStore(
	ctx context.Context,
	callerScope toolspkg.Scope,
	id toolspkg.ToolID,
	selector memoryToolSelector,
) (memoryToolLocation, error) {
	scope, err := core.ParseOptionalMemoryScope(selector.Scope)
	if err != nil {
		return memoryToolLocation{}, err
	}
	if scope == "" {
		if strings.TrimSpace(firstNonEmpty(selector.AgentName, callerScope.AgentName)) != "" {
			scope = memcontract.ScopeAgent
		} else if strings.TrimSpace(firstNonEmpty(selector.Workspace, callerScope.WorkspaceID)) != "" {
			scope = memcontract.ScopeWorkspace
		}
	}
	return n.memoryStoreFor(ctx, callerScope, id, selector, scope)
}

func (n *daemonNativeTools) memoryWriteStore(
	ctx context.Context,
	callerScope toolspkg.Scope,
	id toolspkg.ToolID,
	selector memoryToolSelector,
	rawType string,
) (memoryToolLocation, error) {
	scope, err := core.ParseOptionalMemoryScope(selector.Scope)
	if err != nil {
		return memoryToolLocation{}, err
	}
	if scope == "" {
		if strings.TrimSpace(firstNonEmpty(selector.AgentName, callerScope.AgentName)) != "" {
			scope = memcontract.ScopeAgent
		} else if inferred, inferErr := memcontract.DefaultScopeForType(memcontract.Type(rawType)); inferErr == nil {
			scope = inferred
		} else if strings.TrimSpace(firstNonEmpty(selector.Workspace, callerScope.WorkspaceID)) != "" {
			scope = memcontract.ScopeWorkspace
		}
	}
	return n.memoryStoreFor(ctx, callerScope, id, selector, scope)
}

func (n *daemonNativeTools) memoryCallerActorKind(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (nativeMemoryActorKind, error) {
	if actorKind := normalizeNativeMemoryActorKind(firstNonEmpty(req.ActorKind, scope.ActorKind)); actorKind != "" {
		return actorKind, nil
	}
	sessionID := strings.TrimSpace(firstNonEmpty(req.SessionID, scope.SessionID))
	if sessionID == "" || n == nil || n.deps == nil || n.deps.Sessions == nil {
		return nativeMemoryActorKind(""), nil
	}
	info, err := n.deps.Sessions.Status(ctx, sessionID)
	if err != nil {
		return nativeMemoryActorKind(""), fmt.Errorf(
			"daemon: resolve memory tool caller session %q: %w",
			sessionID,
			err,
		)
	}
	if info != nil && info.Lineage != nil && strings.TrimSpace(info.Lineage.ParentSessionID) != "" {
		return nativeMemoryActorKindSubagent, nil
	}
	return nativeMemoryActorKindRoot, nil
}

func (n *daemonNativeTools) denySubagentMemoryWrite(
	ctx context.Context,
	req toolspkg.CallRequest,
	location memoryToolLocation,
	actorKind nativeMemoryActorKind,
	targetID string,
) error {
	if actorKind != nativeMemoryActorKindSubagent {
		return nil
	}
	cause := fmt.Errorf("%w: sub-agent memory writes are denied", toolspkg.ErrToolDenied)
	if location.Store != nil {
		if err := location.Store.RecordMemoryWriteRejected(ctx, memorypkg.WriteRejectedEvent{
			Scope:       location.Scope,
			WorkspaceID: location.WorkspaceID,
			AgentName:   location.AgentName,
			AgentTier:   location.AgentTier,
			SessionID:   strings.TrimSpace(req.SessionID),
			ActorKind:   string(actorKind),
			TargetID:    targetID,
			Reason:      string(toolspkg.ReasonMemorySubagentWriteDenied),
			ToolID:      string(req.ToolID),
		}); err != nil {
			cause = errors.Join(cause, err)
		}
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		req.ToolID,
		"sub-agent memory writes are denied",
		cause,
		toolspkg.ReasonMemorySubagentWriteDenied,
	)
}

func (n *daemonNativeTools) recordMemoryToolWrite(
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	actorKind nativeMemoryActorKind,
) {
	if n == nil || n.deps == nil || n.deps.MemoryToolWrites == nil {
		return
	}
	if actorKind != nativeMemoryActorKindRoot {
		return
	}
	sessionID := strings.TrimSpace(firstNonEmpty(req.SessionID, scope.SessionID))
	if sessionID == "" {
		return
	}
	n.deps.MemoryToolWrites.RecordToolWrite(sessionID, 0)
}

func (n *daemonNativeTools) agentMemoryStoreFor(
	ctx context.Context,
	callerScope toolspkg.Scope,
	id toolspkg.ToolID,
	selector memoryToolSelector,
	workspaceRef string,
) (memoryToolLocation, error) {
	agentName := strings.TrimSpace(firstNonEmpty(selector.AgentName, callerScope.AgentName))
	if agentName == "" {
		return memoryToolLocation{}, nativeRequiredInputError(id, "agent_name")
	}
	tier, err := parseNativeOptionalAgentTier(id, selector.AgentTier, memcontract.AgentTierWorkspace)
	if err != nil {
		return memoryToolLocation{}, err
	}
	base := n.deps.MemoryStore
	location := memoryToolLocation{
		Scope:     memcontract.ScopeAgent,
		AgentName: agentName,
		AgentTier: tier,
	}
	if tier == memcontract.AgentTierWorkspace {
		workspaceID, workspace, err := n.memoryWorkspaceIdentity(ctx, workspaceRef)
		if err != nil {
			return memoryToolLocation{}, err
		}
		if workspace == "" {
			return memoryToolLocation{}, core.NewMemoryValidationError(
				errors.New("workspace is required for workspace-tier agent memory"),
			)
		}
		base = base.ForWorkspace(workspace)
		location.Workspace = workspace
		location.WorkspaceID = workspaceID
	}
	location.Store = base.ForAgent(location.WorkspaceID, agentName, tier)
	return location, nil
}

func parseNativeOptionalAgentTier(
	id toolspkg.ToolID,
	raw string,
	defaultTier memcontract.AgentTier,
) (memcontract.AgentTier, error) {
	tier := memcontract.AgentTier(strings.TrimSpace(raw)).Normalize()
	if tier == "" {
		tier = defaultTier.Normalize()
	}
	if err := tier.Validate(); err != nil {
		return "", toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return tier, nil
}

func (n *daemonNativeTools) memoryWorkspaceIdentity(ctx context.Context, ref string) (string, string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed != "" && n.deps.Workspaces != nil {
		resolved, err := n.deps.Workspaces.Resolve(ctx, trimmed)
		switch {
		case err == nil:
			root := firstNonEmpty(resolved.RootDir, trimmed)
			workspaceRoot, resolveErr := core.ResolveMemoryWorkspace(root)
			workspaceID := strings.TrimSpace(resolved.WorkspaceID)
			if workspaceID == "" {
				return "", "", errors.New("daemon: resolved workspace_id is empty")
			}
			return workspaceID, workspaceRoot, resolveErr
		case !errors.Is(err, workspacepkg.ErrWorkspaceNotFound):
			return "", "", err
		}
		if workspacepkg.IsWorkspaceID(trimmed) {
			return "", "", err
		}
	}
	workspaceRoot, err := core.ResolveMemoryWorkspace(trimmed)
	if err != nil {
		return "", "", err
	}
	identity, err := workspacepkg.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		return "", "", fmt.Errorf("daemon: resolve memory workspace identity: %w", err)
	}
	return identity.WorkspaceID, workspaceRoot, nil
}

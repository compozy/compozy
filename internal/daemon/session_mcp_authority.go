package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspace"
)

var _ session.MCPAuthorityResolver = (*nativeToolPolicyResolver)(nil)

func (r *nativeToolPolicyResolver) EffectiveMCPAuthority(
	ctx context.Context,
	id string,
) (store.SessionPermissionPolicy, error) {
	ctx = r.PolicyReadContext(ctx)
	read, ok := ctx.Value(toolPolicyReadKey{}).(*toolPolicyRead)
	if !ok {
		return r.resolveEffectiveMCPAuthority(ctx, id)
	}
	key := toolPolicyAuthorityKey{resolver: r, id: id, read: mcpAuthorityRead(ctx)}
	if info := session.MCPAuthoritySession(ctx, id); info != nil {
		// Synthetic spawn/start/adoption rows may share an ID but carry different ceilings.
		row, err := json.Marshal(struct {
			Profile, Workspace, Agent string
			Type                      session.Type
			Lineage                   *store.SessionLineage
		}{info.ProfileID, info.WorkspaceID, info.AgentName, info.Type, info.Lineage})
		if err != nil {
			return store.SessionPermissionPolicy{}, err
		}
		key.row = string(row)
	}
	read.authorityMu.Lock()
	defer read.authorityMu.Unlock()
	if cached, ok := read.authorities[key]; ok {
		return store.NormalizeSessionPermissionPolicy(cached.policy), cached.err
	}
	policy, err := r.resolveEffectiveMCPAuthority(ctx, id)
	if read.authorities == nil {
		read.authorities = make(map[toolPolicyAuthorityKey]toolPolicyAuthority)
	}
	read.authorities[key] = toolPolicyAuthority{policy, err}
	return store.NormalizeSessionPermissionPolicy(policy), err
}

func (r *nativeToolPolicyResolver) resolveEffectiveMCPAuthority(
	ctx context.Context, id string,
) (store.SessionPermissionPolicy, error) {
	info := session.MCPAuthoritySession(ctx, id)
	if info == nil {
		var err error
		info, err = r.sessionInfo(ctx, id)
		if err != nil {
			return store.SessionPermissionPolicy{}, err
		}
	}
	if info == nil {
		return store.SessionPermissionPolicy{}, fmt.Errorf("%w: %s", session.ErrSessionNotFound, id)
	}
	scope := toolspkg.Scope{
		SessionID:   id,
		ProfileID:   info.ProfileID,
		WorkspaceID: info.WorkspaceID,
		AgentName:   info.AgentName,
	}
	base, err := r.resolveMCPAuthorityScope(ctx, scope)
	if err != nil {
		return store.SessionPermissionPolicy{}, err
	}
	inputs := base.inputs
	if scope.AgentName != "" && !mcpAuthorityRead(ctx) {
		if err := r.applyAgentToolPolicy(&inputs, scope.AgentName, base.workspace, base.cfg); err != nil {
			// Existing delegated sessions retain their stored ceiling if the agent was removed.
			if info.Lineage == nil || !errors.Is(err, workspace.ErrAgentNotAvailable) {
				return store.SessionPermissionPolicy{}, err
			}
		}
	}
	lineage := store.NormalizeSessionLineage(id, info.Lineage)
	policy := store.DecodeIntent(lineage.PermissionPolicy, lineage.Kind, string(info.Type))
	policy.MCPServers = store.BoundMCPServers(policy, base.names)
	if r.scopeTools != nil && !mcpAuthorityRead(ctx) {
		policy.Tools, err = r.effectiveScopeTools(
			ctx,
			scope,
			inputs,
			policy,
			// Only governed sessions retain a recorded native-tool ceiling.
			lineage.Kind != store.LineageKindSpawn &&
				info.Type != session.SessionTypeSpawned && info.Type != session.SessionTypeCoordinator,
		)
		if err != nil {
			return store.SessionPermissionPolicy{}, err
		}
	}
	return store.NormalizeSessionPermissionPolicy(policy), nil
}

func (r *nativeToolPolicyResolver) admittedMCPNames(
	inputs toolspkg.PolicyInputs,
	servers []compozyconfig.MCPServer,
) []string {
	names := make([]string, 0)
	if inputs.ExternalDefault == toolspkg.ExternalDefaultEnabled {
		for _, server := range servers {
			names = append(names, server.Name)
		}
	}
	resources := map[string]string{}
	if r.mcpResourceNames != nil {
		resources = r.mcpResourceNames()
	}
	for _, grant := range inputs.AllowSources {
		if grant.Kind != toolspkg.SourceMCP {
			continue
		}
		name := resources[grant.ResourceID]
		if grant.ResourceID == "" {
			name = grant.Owner
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (r *nativeToolPolicyResolver) effectiveScopeTools(
	ctx context.Context,
	scope toolspkg.Scope,
	inputs toolspkg.PolicyInputs,
	policy store.SessionPermissionPolicy,
	root bool,
) ([]string, error) {
	scope.SessionID, scope.AgentName = "", ""
	// Supply the already resolved scope policy to avoid reentering the registry policy cache.
	views, err := r.scopeTools(ctx, scope, inputs)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	if r.mcpResourceNames != nil {
		names = r.mcpResourceNames()
	}
	denied := make(map[string]bool)
	for i := range views {
		view := &views[i]
		if view.Decision.AgentPolicyResult == hookPermissionDecisionDenied ||
			view.Decision.RegistryPolicyResult == hookPermissionDecisionDenied {
			denied[view.Descriptor.ID.String()] = true
		}
	}
	if policy.ToolsMode == store.PermissionList {
		return listedMCPAuthorityTools(policy, views, names, denied), nil
	}
	ids := slices.DeleteFunc(slices.Clone(policy.Tools), func(id string) bool {
		return !strings.HasPrefix(id, "compozy__") || denied[id]
	})
	if root {
		ids = nil
	}
	for i := range views {
		view := &views[i]
		if view.Decision.AgentPolicyResult == hookPermissionDecisionDenied ||
			view.Decision.RegistryPolicyResult == hookPermissionDecisionDenied {
			continue
		}
		source := view.Descriptor.Source
		if root && source.Kind == toolspkg.SourceBuiltin &&
			strings.HasPrefix(view.Descriptor.ID.String(), "compozy__") {
			ids = append(ids, view.Descriptor.ID.String())
		}
		if source.Kind == toolspkg.SourceExtension && sourcePolicyAdmitted(view.Decision.SourcePolicyResult) {
			ids = append(ids, view.Descriptor.ID.String())
		}
		if source.Kind == toolspkg.SourceMCP &&
			slices.Contains(policy.MCPServers, mcpAuthoritySourceName(source, names)) {
			ids = append(ids, view.Descriptor.ID.String())
		}
	}
	return ids, nil
}

func mcpAuthoritySourceName(source toolspkg.SourceRef, names map[string]string) string {
	if source.ResourceID != "" {
		return names[source.ResourceID]
	}
	if source.RawServerName != "" {
		return source.RawServerName
	}
	return source.Owner
}

func sourcePolicyAdmitted(result string) bool {
	return result == "allowed" || result == "trusted" || result == "approval_required"
}

func bindSessionMCPAuthority(state *bootState, resolver *nativeToolPolicyResolver, registry *toolspkg.RuntimeRegistry) {
	resolver.scopeTools = registry.PolicyProjection
	bindSessionMCPReader(state, resolver, registry)
	if manager, ok := state.sessions.(interface {
		SetMCPAuthorityResolver(session.MCPAuthorityResolver)
	}); ok {
		manager.SetMCPAuthorityResolver(resolver)
	}
}

func listedMCPAuthorityTools(
	policy store.SessionPermissionPolicy, views []toolspkg.ToolView, names map[string]string, denied map[string]bool,
) []string {
	allowedMCP := make(map[string]bool)
	for i := range views {
		view := &views[i]
		if view.Descriptor.Source.Kind == toolspkg.SourceMCP {
			name := mcpAuthoritySourceName(view.Descriptor.Source, names)
			allowedMCP[view.Descriptor.ID.String()] = slices.Contains(policy.MCPServers, name)
		}
	}
	return slices.DeleteFunc(slices.Clone(policy.Tools), func(id string) bool {
		return denied[id] || strings.HasPrefix(id, "mcp__") && !allowedMCP[id]
	})
}

package daemon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var _ session.MCPAuthorityResolver = (*nativeToolPolicyResolver)(nil)

func (r *nativeToolPolicyResolver) EffectiveMCPAuthority(
	ctx context.Context,
	id string,
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
	workspace, cfg, err := r.resolveWorkspaceConfig(ctx, scope)
	if err != nil {
		return store.SessionPermissionPolicy{}, err
	}
	inputs, err := nativeToolPolicyInputs(cfg)
	if err != nil {
		return store.SessionPermissionPolicy{}, err
	}
	if err := r.applyScopedSourceGrants(ctx, &inputs, scope, workspace); err != nil {
		return store.SessionPermissionPolicy{}, err
	}
	names := r.admittedMCPNames(inputs, cfg.MCPServers)
	lineage := store.NormalizeSessionLineage(id, info.Lineage)
	policy := store.DecodeIntent(lineage.PermissionPolicy, lineage.Kind, string(info.Type))
	policy.MCPServers = store.BoundMCPServers(policy, names)
	if r.scopeTools != nil {
		policy.Tools, err = r.effectiveScopeTools(ctx, scope, inputs, policy)
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
	if r.mcpResourceNames != nil {
		resources := r.mcpResourceNames()
		for _, grant := range inputs.AllowSources {
			if grant.Kind == toolspkg.SourceMCP {
				if name := resources[grant.ResourceID]; name != "" {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

func (r *nativeToolPolicyResolver) effectiveScopeTools(
	ctx context.Context, scope toolspkg.Scope, inputs toolspkg.PolicyInputs, policy store.SessionPermissionPolicy,
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
	if policy.ToolsMode == store.PermissionList {
		allowedMCP := make(map[string]bool)
		for _, view := range views {
			if view.Descriptor.Source.Kind == toolspkg.SourceMCP {
				name := mcpAuthoritySourceName(view.Descriptor.Source, names)
				allowedMCP[view.Descriptor.ID.String()] = slices.Contains(policy.MCPServers, name)
			}
		}
		return slices.DeleteFunc(slices.Clone(policy.Tools), func(id string) bool {
			return strings.HasPrefix(id, "mcp__") && !allowedMCP[id]
		}), nil
	}
	ids := slices.DeleteFunc(slices.Clone(policy.Tools), func(id string) bool {
		return !strings.HasPrefix(id, "compozy__")
	})
	for _, view := range views {
		source := view.Descriptor.Source
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

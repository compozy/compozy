package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func newDaemonMCPServerResolver(state *bootState) mcppkg.ServerResolver {
	return mcppkg.ServerResolverFunc(func(
		ctx context.Context,
		source toolspkg.SourceRef,
	) (mcppkg.ResolvedServer, error) {
		return resolveDaemonMCPServer(ctx, state, source)
	})
}

func resolveDaemonMCPServer(
	ctx context.Context,
	state *bootState,
	source toolspkg.SourceRef,
) (mcppkg.ResolvedServer, error) {
	if state == nil {
		return mcppkg.ResolvedServer{}, errors.New("daemon: MCP runtime state is unavailable")
	}
	name := strings.TrimSpace(firstNonEmpty(source.RawServerName, source.Owner))
	if name == "" {
		return mcppkg.ResolvedServer{}, errors.New("daemon: MCP source server name is required")
	}

	if resourceID := strings.TrimSpace(source.ResourceID); resourceID != "" {
		if state.mcpServerCatalog == nil {
			return mcppkg.ResolvedServer{}, fmt.Errorf("daemon: MCP resource %q is unavailable", resourceID)
		}
		for _, record := range state.mcpServerCatalog.Snapshot() {
			if strings.TrimSpace(record.ID) != resourceID {
				continue
			}
			if strings.TrimSpace(record.Spec.Name) != name {
				return mcppkg.ResolvedServer{}, errors.New("daemon: MCP resource identity does not match source")
			}
			target, err := mcpAuthTargetForResource(record.Scope, name)
			if err != nil {
				return mcppkg.ResolvedServer{}, err
			}
			server, err := projectExtensionSecretHeaders(ctx, state, record, source.ProfileID)
			if err != nil {
				return mcppkg.ResolvedServer{}, err
			}
			return mcppkg.ResolvedServer{
				Server: server, Target: target, HealthKey: extensionMCPHealthKey(state, record),
			}, nil
		}
		return mcppkg.ResolvedServer{}, fmt.Errorf("daemon: MCP resource %q is unavailable", resourceID)
	}

	for _, server := range state.cfg.MCPServers {
		if strings.TrimSpace(server.Name) == name {
			return userResolvedMCPServer(server), nil
		}
	}
	providerNames := make([]string, 0, len(state.cfg.Providers))
	for providerName := range state.cfg.Providers {
		providerNames = append(providerNames, providerName)
	}
	slices.Sort(providerNames)
	for _, providerName := range providerNames {
		for _, server := range state.cfg.Providers[providerName].MCPServers {
			if strings.TrimSpace(server.Name) == name {
				return userResolvedMCPServer(server), nil
			}
		}
	}
	return mcppkg.ResolvedServer{}, fmt.Errorf("daemon: MCP server %q is unavailable", name)
}

func projectExtensionSecretHeaders(
	ctx context.Context,
	state *bootState,
	record resources.Record[compozyconfig.MCPServer],
	activeProfileID string,
) (compozyconfig.MCPServer, error) {
	server := cloneDaemonMCPServer(record.Spec)
	owner := record.Owner.Normalize()
	if owner.Kind != extensionResourceOwnerKind || state.extensionEnvBindings == nil ||
		server.EffectiveTransport() != compozyconfig.MCPServerTransportHTTP {
		return server, nil
	}
	profileID, workspaceID, err := mcpExtensionBindingOwner(ctx, state, record.Scope)
	if err != nil {
		return compozyconfig.MCPServer{}, err
	}
	if projectedProfileID := strings.TrimSpace(activeProfileID); projectedProfileID != "" {
		profileID = projectedProfileID
	}
	bindings, err := state.extensionEnvBindings.ResolveEnvBindings(
		ctx,
		owner.ID,
		profileID,
		workspaceID,
	)
	if err != nil {
		return compozyconfig.MCPServer{}, fmt.Errorf(
			"daemon: list extension MCP header bindings for %q: %w",
			owner.ID,
			err,
		)
	}
	for _, binding := range bindings {
		if strings.TrimSpace(binding.MCPServer) != strings.TrimSpace(server.Name) {
			continue
		}
		if server.SecretHeaders == nil {
			server.SecretHeaders = make(map[string]string)
		}
		server.SecretHeaders[strings.TrimSpace(binding.HeaderName)] = strings.TrimSpace(binding.SecretRef)
	}
	if err := server.Validate("extension.mcp_server"); err != nil {
		return compozyconfig.MCPServer{}, fmt.Errorf("daemon: validate extension MCP header bindings: %w", err)
	}
	return server, nil
}

func mcpExtensionBindingOwner(
	ctx context.Context,
	state *bootState,
	scope resources.ResourceScope,
) (string, string, error) {
	normalized := scope.Normalize()
	switch normalized.Kind {
	case resources.ResourceScopeKindProfile:
		return normalized.ID, "", nil
	case resources.ResourceScopeKindWorkspaceProfile:
		workspaceID, profileName, ok := strings.Cut(normalized.ID, "@pf:")
		if !ok || state == nil || state.profiles == nil {
			return "", "", errors.New("daemon: workspace-profile MCP resource requires profile catalog")
		}
		profile, err := state.profiles.GetByName(ctx, profileName)
		if err != nil {
			return "", "", fmt.Errorf("daemon: resolve MCP resource profile %q: %w", profileName, err)
		}
		return strings.TrimSpace(profile.ID), strings.TrimSpace(workspaceID), nil
	case resources.ResourceScopeKindWorkspace:
		return store.DefaultProfileID, normalized.ID, nil
	case resources.ResourceScopeKindUser:
		return store.DefaultProfileID, "", nil
	}
	return "", "", fmt.Errorf("daemon: unsupported MCP resource scope %q", normalized.Kind)
}

func userResolvedMCPServer(server compozyconfig.MCPServer) mcppkg.ResolvedServer {
	name := strings.TrimSpace(server.Name)
	return mcppkg.ResolvedServer{
		Server: cloneDaemonMCPServer(server),
		Target: mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: name},
	}
}

func mcpAuthTargetForResource(scope resources.ResourceScope, serverName string) (mcpauth.Target, error) {
	scope = scope.Normalize()
	target := mcpauth.Target{ServerName: strings.TrimSpace(serverName)}
	switch scope.Kind {
	case resources.ResourceScopeKindUser:
		target.Scope = mcpauth.ScopeUser
	case resources.ResourceScopeKindWorkspace:
		target.Scope = mcpauth.ScopeWorkspace
		target.WorkspaceID = scope.ID
	case resources.ResourceScopeKindProfile:
		target.Scope = mcpauth.ScopeProfile
		target.WorkspaceID = scope.ID
	case resources.ResourceScopeKindWorkspaceProfile:
		target.Scope = mcpauth.ScopeWorkspaceProfile
		target.WorkspaceID = scope.ID
	default:
		return mcpauth.Target{}, fmt.Errorf("daemon: unsupported MCP resource scope %q", scope.Kind)
	}
	if err := target.Validate(); err != nil {
		return mcpauth.Target{}, fmt.Errorf("daemon: invalid MCP auth target: %w", err)
	}
	return target, nil
}

func daemonMCPSources(ctx context.Context, state *bootState) ([]toolspkg.SourceRef, error) {
	if state == nil {
		return nil, nil
	}
	sources := make([]toolspkg.SourceRef, 0, len(state.cfg.MCPServers))
	seen := map[string]struct{}{}
	add := func(server compozyconfig.MCPServer, source toolspkg.SourceRef) {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			return
		}
		key := strings.TrimSpace(source.Scope) + "\x00" + strings.TrimSpace(source.ProfileID) + "\x00" +
			strings.TrimSpace(source.WorkspaceID) + "\x00" + name
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		source.Kind = toolspkg.SourceMCP
		source.Owner = name
		source.RawServerName = name
		sources = append(sources, source)
	}
	for _, server := range state.cfg.MCPServers {
		add(server, toolspkg.SourceRef{Scope: string(mcpauth.ScopeUser)})
	}
	providerNames := make([]string, 0, len(state.cfg.Providers))
	for name := range state.cfg.Providers {
		providerNames = append(providerNames, name)
	}
	slices.Sort(providerNames)
	for _, name := range providerNames {
		for _, server := range state.cfg.Providers[name].MCPServers {
			add(server, toolspkg.SourceRef{Scope: string(mcpauth.ScopeUser)})
		}
	}
	if state.mcpServerCatalog != nil {
		for _, record := range state.mcpServerCatalog.Snapshot() {
			profileID, workspaceID, err := mcpResourceProjectionOwner(ctx, state, record.Scope)
			if err != nil {
				return nil, err
			}
			add(record.Spec, toolspkg.SourceRef{
				ResourceID:      record.ID,
				ResourceVersion: fmt.Sprint(record.Version),
				ProfileID:       profileID,
				WorkspaceID:     workspaceID,
				Scope:           string(record.Scope.Kind),
			})
		}
	}
	return sources, nil
}

func mcpResourceProjectionOwner(
	ctx context.Context,
	state *bootState,
	scope resources.ResourceScope,
) (string, string, error) {
	normalized := scope.Normalize()
	switch normalized.Kind {
	case resources.ResourceScopeKindUser:
		return "", "", nil
	case resources.ResourceScopeKindProfile:
		return normalized.ID, "", nil
	case resources.ResourceScopeKindWorkspace:
		return "", normalized.ID, nil
	case resources.ResourceScopeKindWorkspaceProfile:
		workspaceID, profileName, ok := strings.Cut(normalized.ID, "@pf:")
		if !ok || state == nil || state.profiles == nil {
			return "", "", errors.New("daemon: workspace-profile MCP resource requires profile catalog")
		}
		profile, err := state.profiles.GetByName(ctx, profileName)
		if err != nil {
			return "", "", fmt.Errorf("daemon: resolve MCP resource profile %q: %w", profileName, err)
		}
		return strings.TrimSpace(profile.ID), strings.TrimSpace(workspaceID), nil
	default:
		return "", "", fmt.Errorf("daemon: unsupported MCP resource scope %q", normalized.Kind)
	}
}

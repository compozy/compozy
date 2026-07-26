package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	mcppkg "github.com/compozy/agh/internal/mcp"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/resources"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func newDaemonMCPServerResolver(state *bootState) mcppkg.ServerResolver {
	return mcppkg.ServerResolverFunc(func(
		_ context.Context,
		source toolspkg.SourceRef,
	) (mcppkg.ResolvedServer, error) {
		return resolveDaemonMCPServer(state, source)
	})
}

func resolveDaemonMCPServer(state *bootState, source toolspkg.SourceRef) (mcppkg.ResolvedServer, error) {
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
			return mcppkg.ResolvedServer{Server: cloneDaemonMCPServer(record.Spec), Target: target}, nil
		}
		return mcppkg.ResolvedServer{}, fmt.Errorf("daemon: MCP resource %q is unavailable", resourceID)
	}

	for _, server := range state.cfg.MCPServers {
		if strings.TrimSpace(server.Name) == name {
			return globalResolvedMCPServer(server), nil
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
				return globalResolvedMCPServer(server), nil
			}
		}
	}
	return mcppkg.ResolvedServer{}, fmt.Errorf("daemon: MCP server %q is unavailable", name)
}

func globalResolvedMCPServer(server aghconfig.MCPServer) mcppkg.ResolvedServer {
	name := strings.TrimSpace(server.Name)
	return mcppkg.ResolvedServer{
		Server: cloneDaemonMCPServer(server),
		Target: mcpauth.Target{Scope: mcpauth.ScopeGlobal, ServerName: name},
	}
}

func mcpAuthTargetForResource(scope resources.ResourceScope, serverName string) (mcpauth.Target, error) {
	scope = scope.Normalize()
	target := mcpauth.Target{ServerName: strings.TrimSpace(serverName)}
	switch scope.Kind {
	case resources.ResourceScopeKindGlobal:
		target.Scope = mcpauth.ScopeGlobal
	case resources.ResourceScopeKindWorkspace:
		target.Scope = mcpauth.ScopeWorkspace
		target.WorkspaceID = scope.ID
	default:
		return mcpauth.Target{}, fmt.Errorf("daemon: unsupported MCP resource scope %q", scope.Kind)
	}
	if err := target.Validate(); err != nil {
		return mcpauth.Target{}, fmt.Errorf("daemon: invalid MCP auth target: %w", err)
	}
	return target, nil
}

func daemonMCPSources(state *bootState) []toolspkg.SourceRef {
	if state == nil {
		return nil
	}
	sources := make([]toolspkg.SourceRef, 0, len(state.cfg.MCPServers))
	seen := map[string]struct{}{}
	add := func(server aghconfig.MCPServer, source toolspkg.SourceRef) {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			return
		}
		key := strings.TrimSpace(source.Scope) + "\x00" + strings.TrimSpace(source.WorkspaceID) + "\x00" + name
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
		add(server, toolspkg.SourceRef{Scope: string(mcpauth.ScopeGlobal)})
	}
	providerNames := make([]string, 0, len(state.cfg.Providers))
	for name := range state.cfg.Providers {
		providerNames = append(providerNames, name)
	}
	slices.Sort(providerNames)
	for _, name := range providerNames {
		for _, server := range state.cfg.Providers[name].MCPServers {
			add(server, toolspkg.SourceRef{Scope: string(mcpauth.ScopeGlobal)})
		}
	}
	if state.mcpServerCatalog != nil {
		for _, record := range state.mcpServerCatalog.Snapshot() {
			add(record.Spec, toolspkg.SourceRef{
				ResourceID:      record.ID,
				ResourceVersion: fmt.Sprint(record.Version),
				WorkspaceID:     record.Scope.ID,
				Scope:           string(record.Scope.Kind),
			})
		}
	}
	return sources
}

package settings

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/vault"
)

type MCPRuntimeChecker interface {
	CheckMCPServerRuntime(context.Context, mcpauth.Target, compozyconfig.MCPServer) (MCPServerRuntimeStatus, error)
}

func (s *service) CheckMCPServer(ctx context.Context, req MCPAuthTargetRequest) (MCPServerItem, error) {
	checker, ok := s.mcpRuntime.(MCPRuntimeChecker)
	if !ok {
		return MCPServerItem{}, unavailableError(errors.New("settings: MCP check runtime is unavailable"))
	}
	resolved, err := s.resolveMCPAuthDefinition(ctx, req)
	if err != nil {
		return MCPServerItem{}, err
	}
	collection, err := s.ListCollection(ctx, CollectionRequest{
		Collection: CollectionMCPServers, Scope: req.Scope, WorkspaceID: req.WorkspaceID,
		ProfileName: req.ProfileName, SkipRuntimeStatus: true,
	})
	if err != nil {
		return MCPServerItem{}, err
	}
	for i := range collection.MCPServers {
		item := &collection.MCPServers[i]
		if item.Name != req.Name || vault.NormalizeMCPOwner(item.Owner) != vault.NormalizeMCPOwner(req.Owner) {
			continue
		}
		ctx = mcpauth.WithConsentScope(
			ctx,
			mcpauth.ConsentScope{ProfileName: req.ProfileName, WorkspaceID: req.WorkspaceID},
		)
		status, err := checker.CheckMCPServerRuntime(ctx, resolved.target, resolved.server)
		if err != nil {
			return MCPServerItem{}, err
		}
		setMCPRuntimeObservation(item, resolved.server, status)
		return *item, nil
	}
	return MCPServerItem{}, notFoundError(fmt.Errorf("settings: MCP server %q is unavailable", req.Name))
}

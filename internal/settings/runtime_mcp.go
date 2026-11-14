package settings

import (
	"context"
	"errors"
	"os"

	"github.com/compozy/compozy/internal/runtimemcp"
)

type RuntimeMCPInventory interface {
	List(context.Context, runtimemcp.Request) (runtimemcp.Inventory, error)
}

func (s *service) ListRuntimeMCP(ctx context.Context, req CollectionRequest, runtime string) (runtimemcp.Inventory, error) {
	if s.runtimeMCP == nil {
		return runtimemcp.Inventory{}, errors.New("runtime MCP inventory unavailable")
	}
	cfg, resolved, err := s.loadConfig(ctx, req.Scope, req.WorkspaceID, req.ProfileName)
	if err != nil {
		return runtimemcp.Inventory{}, err
	}
	var readContext runtimemcp.ReadContext
	if s.runtimeMCPContext != nil {
		readContext, err = s.runtimeMCPContext()
	} else {
		readContext.Home, err = os.UserHomeDir()
		readContext.Env = runtimemcp.Environment(os.Environ())
	}
	if err != nil {
		return runtimemcp.Inventory{}, err
	}
	readContext.Profile = req.ProfileName
	if resolved != nil {
		readContext.CWD = resolved.RootDir
		readContext.WorkspaceRoot = resolved.RootDir
	}
	modes := map[string]string{}
	for name, provider := range cfg.Providers {
		modes[name] = provider.EffectiveRuntimeMCP()
	}
	req.Collection = CollectionMCPServers
	req.SkipRuntimeStatus = true
	collection, err := s.ListCollection(ctx, req)
	if err != nil {
		return runtimemcp.Inventory{}, err
	}
	names := make([]string, 0, len(collection.MCPServers))
	for _, server := range collection.MCPServers {
		names = append(names, server.Name)
	}
	return s.runtimeMCP.List(ctx, runtimemcp.Request{Context: readContext, Runtime: runtime, Modes: modes, CompozyNames: names})
}

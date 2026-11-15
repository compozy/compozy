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

func (s *service) ListRuntimeMCP(
	ctx context.Context,
	req CollectionRequest,
	runtime string,
) (runtimemcp.Inventory, error) {
	if s.runtimeMCP == nil {
		return runtimemcp.Inventory{}, errors.New("runtime MCP inventory unavailable")
	}
	cfg, resolved, err := s.loadConfig(ctx, req.Scope, req.WorkspaceID, req.ProfileName)
	if err != nil {
		return runtimemcp.Inventory{}, err
	}
	readContext, err := s.operatorRuntimeMCPContext()
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
	for i := range collection.MCPServers {
		names = append(names, collection.MCPServers[i].Name)
	}
	return s.runtimeMCP.List(
		ctx,
		runtimemcp.Request{Context: readContext, Runtime: runtime, Modes: modes, CompozyNames: names},
	)
}

func (s *service) operatorRuntimeMCPContext() (runtimemcp.ReadContext, error) {
	if s.runtimeMCPContext != nil {
		return s.runtimeMCPContext()
	}
	home, err := os.UserHomeDir()
	return runtimemcp.ReadContext{Home: home, Env: runtimemcp.Environment(os.Environ())}, err
}

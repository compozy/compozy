package settings

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

// MCPExtensionDefinitionResolver reads published extension definitions without probing servers.
type MCPExtensionDefinitionResolver interface {
	ResolveMCPExtensionDefinition(
		context.Context,
		MCPAuthTargetRequest,
	) (mcpauth.Target, compozyconfig.MCPServer, bool, error)
}

func (s *service) resolveExtensionMCPAuthTarget(
	ctx context.Context, req MCPAuthTargetRequest,
) (mcpauth.Target, compozyconfig.MCPServer, error) {
	if s.mcpExtensions != nil {
		target, server, found, err := s.mcpExtensions.ResolveMCPExtensionDefinition(ctx, req)
		if err != nil {
			return mcpauth.Target{}, compozyconfig.MCPServer{}, err
		}
		if found {
			return target, server, nil
		}
	}
	return mcpauth.Target{}, compozyconfig.MCPServer{}, notFoundError(
		fmt.Errorf(
			"settings: MCP server %q has no definition for owner %q in %s scope",
			req.Name,
			req.Owner,
			req.Scope,
		),
	)
}

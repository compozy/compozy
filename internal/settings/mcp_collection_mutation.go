package settings

import (
	"context"
	"errors"
)

func (s *service) putMCPCollectionItem(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	req CollectionItemPutRequest,
) (MutationResult, error) {
	if req.MCPServer == nil {
		return MutationResult{}, validationError(errors.New("settings: MCP server payload is required"))
	}
	return s.putMCPServer(
		ctx,
		scope,
		workspaceID,
		name,
		req.Target,
		*req.MCPServer,
		req.MCPSecrets,
		req.MCPSecretPreservation,
		req.MCPEnvPreservation,
	)
}

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
	target, owned, err := s.extensionMCPMutationTarget(
		ctx,
		MCPAuthTargetRequest{
			Scope:       scope,
			WorkspaceID: workspaceID,
			ProfileName: req.ProfileName,
			Name:        name,
			Owner:       req.Owner,
		},
	)
	if err != nil {
		return MutationResult{}, err
	}
	if owned {
		return s.putExtensionMCPOverride(ctx, target, req)
	}
	return s.putMCPServer(
		ctx,
		scope,
		workspaceID,
		req.ProfileName,
		name,
		req.Target,
		*req.MCPServer,
		req.MCPSecrets,
		req.MCPSecretPreservation,
		req.MCPEnvPreservation,
	)
}

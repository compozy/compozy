package settings

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *service) resolveCollectionConfigTarget(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
) (string, compozyconfig.WriteTarget, error) {
	root := ""
	if scope == ScopeWorkspace || (scope == ScopeProfile && workspaceID != "") {
		resolved, err := s.resolveWorkspace(ctx, scope, workspaceID)
		if err != nil {
			return "", compozyconfig.WriteTarget{}, err
		}
		if resolved == nil {
			return "", compozyconfig.WriteTarget{}, errors.New("settings: resolved workspace is required")
		}
		root = resolved.RootDir
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(
		s.homePaths, root, scope.configWriteScope(), profileName,
	)
	if err != nil {
		return "", compozyconfig.WriteTarget{}, fmt.Errorf("settings: resolve collection write target: %w", err)
	}
	return root, target, nil
}

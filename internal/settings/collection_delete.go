package settings

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func (s *service) deleteProvider(name string) (MutationResult, error) {
	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			path := []string{"providers", name}
			if !editor.HasPath(path) {
				return notFoundError(fmt.Errorf("settings: provider %q overlay not found", name))
			}
			return editor.Delete(path)
		},
	); err != nil {
		return MutationResult{}, fmt.Errorf("settings: delete provider %q: %w", name, err)
	}

	return mutationResultAtPath(
		mutationResultForCollection(CollectionProviders, ScopeUser, "", target.Kind()),
		target.Path(),
	), nil
}

func (s *service) putSandbox(name string, profile compozyconfig.SandboxProfile) (MutationResult, error) {
	values := sandboxProfileMap(profile)
	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return editor.SetTable([]string{"sandboxes", name}, values)
		},
	); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write sandbox %q: %w", name, err)
	}

	return mutationResultAtPath(
		mutationResultForCollection(CollectionSandboxes, ScopeUser, "", target.Kind()),
		target.Path(),
	), nil
}

func (s *service) deleteSandbox(name string) (MutationResult, error) {
	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			path := []string{"sandboxes", name}
			if !editor.HasPath(path) {
				return notFoundError(fmt.Errorf("settings: sandbox %q not found", name))
			}
			return editor.Delete(path)
		},
	); err != nil {
		return MutationResult{}, fmt.Errorf("settings: delete sandbox %q: %w", name, err)
	}

	return mutationResultAtPath(
		mutationResultForCollection(CollectionSandboxes, ScopeUser, "", target.Kind()),
		target.Path(),
	), nil
}

func (s *service) putHook(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
	name string,
	declaration hookspkg.HookDecl,
) (MutationResult, error) {
	normalized, err := normalizeHookDeclaration(name, declaration)
	if err != nil {
		return MutationResult{}, err
	}

	root, target, err := s.resolveCollectionConfigTarget(ctx, scope, workspaceID, profileName)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		root,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return editor.UpsertArrayTableItem(
				[]string{"hooks", "declarations"},
				"name",
				name,
				hookDeclarationMap(normalized),
			)
		},
	); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write hook %q: %w", name, err)
	}

	result := mutationResultForCollection(CollectionHooks, scope, workspaceID, target.Kind())
	result.ProfileName = profileName
	result.writePath = target.Path()
	return result, nil
}

func (s *service) deleteHook(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
	name string,
) (MutationResult, error) {
	root, target, err := s.resolveCollectionConfigTarget(ctx, scope, workspaceID, profileName)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		root,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			deleted, deleteErr := editor.DeleteArrayTableItem([]string{"hooks", "declarations"}, "name", name)
			if deleteErr != nil {
				return deleteErr
			}
			if !deleted {
				return notFoundError(fmt.Errorf("settings: hook %q not found", name))
			}
			return nil
		},
	); err != nil {
		return MutationResult{}, fmt.Errorf("settings: delete hook %q: %w", name, err)
	}

	result := mutationResultForCollection(CollectionHooks, scope, workspaceID, target.Kind())
	result.ProfileName = profileName
	result.writePath = target.Path()
	return result, nil
}

func mutationResultForCollection(
	collection CollectionName,
	scope ScopeKind,
	workspaceID string,
	target WriteTargetKind,
) MutationResult {
	classification := restartRequiredClassification()
	return MutationResult{
		Section:         SectionName(collection),
		Scope:           scope,
		WriteTarget:     target,
		WorkspaceID:     workspaceID,
		Behavior:        classification.Behavior,
		Applied:         classification.Applied,
		RestartRequired: classification.RestartRequired,
		RestartScope:    classification.RestartScope,
		Lifecycle:       lifecycle.RestartRequired,
		DiffClass:       lifecycle.DiffClassForRoot(string(collection)),
	}
}

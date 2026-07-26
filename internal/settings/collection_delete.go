package settings

import (
	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (s *service) deleteProvider(name string) (MutationResult, error) {
	target, err := aghconfig.ResolveConfigWriteTarget(s.homePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := aghconfig.EditConfigOverlay(s.homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
		path := []string{"providers", name}
		if !editor.HasPath(path) {
			return notFoundError(fmt.Errorf("settings: provider %q overlay not found", name))
		}
		return editor.Delete(path)
	}); err != nil {
		return MutationResult{}, fmt.Errorf("settings: delete provider %q: %w", name, err)
	}

	return mutationResultForCollection(CollectionProviders, ScopeGlobal, "", target.Kind()), nil
}

func (s *service) putSandbox(name string, profile aghconfig.SandboxProfile) (MutationResult, error) {
	values := sandboxProfileMap(profile)
	target, err := aghconfig.ResolveConfigWriteTarget(s.homePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := aghconfig.EditConfigOverlay(s.homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
		return editor.SetTable([]string{"sandboxes", name}, values)
	}); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write sandbox %q: %w", name, err)
	}

	return mutationResultForCollection(CollectionSandboxes, ScopeGlobal, "", target.Kind()), nil
}

func (s *service) deleteSandbox(name string) (MutationResult, error) {
	target, err := aghconfig.ResolveConfigWriteTarget(s.homePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := aghconfig.EditConfigOverlay(s.homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
		path := []string{"sandboxes", name}
		if !editor.HasPath(path) {
			return notFoundError(fmt.Errorf("settings: sandbox %q not found", name))
		}
		return editor.Delete(path)
	}); err != nil {
		return MutationResult{}, fmt.Errorf("settings: delete sandbox %q: %w", name, err)
	}

	return mutationResultForCollection(CollectionSandboxes, ScopeGlobal, "", target.Kind()), nil
}

func (s *service) putHook(name string, declaration hookspkg.HookDecl) (MutationResult, error) {
	normalized, err := normalizeHookDeclaration(name, declaration)
	if err != nil {
		return MutationResult{}, err
	}

	target, err := aghconfig.ResolveConfigWriteTarget(s.homePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := aghconfig.EditConfigOverlay(s.homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
		return editor.UpsertArrayTableItem(
			[]string{"hooks", "declarations"},
			"name",
			name,
			hookDeclarationMap(normalized),
		)
	}); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write hook %q: %w", name, err)
	}

	return mutationResultForCollection(CollectionHooks, ScopeGlobal, "", target.Kind()), nil
}

func (s *service) deleteHook(name string) (MutationResult, error) {
	target, err := aghconfig.ResolveConfigWriteTarget(s.homePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := aghconfig.EditConfigOverlay(s.homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
		deleted, deleteErr := editor.DeleteArrayTableItem([]string{"hooks", "declarations"}, "name", name)
		if deleteErr != nil {
			return deleteErr
		}
		if !deleted {
			return notFoundError(fmt.Errorf("settings: hook %q not found", name))
		}
		return nil
	}); err != nil {
		return MutationResult{}, fmt.Errorf("settings: delete hook %q: %w", name, err)
	}

	return mutationResultForCollection(CollectionHooks, ScopeGlobal, "", target.Kind()), nil
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

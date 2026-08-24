package settings

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *service) updateWindowManagerSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	loaded, err := s.loadScopedSectionUpdate(
		ctx,
		req.Section,
		req.Scope,
		req.WorkspaceID,
		ScopeUser,
		ScopeWorkspace,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if !hasWindowManagerMutation(req) {
		return MutationResult{}, validationError(
			errors.New("settings: window-manager section payload is required"),
		)
	}
	desired, desiredAliases, err := s.normalizeWindowManagerRequest(ctx, &loaded, req)
	if err != nil {
		return MutationResult{}, err
	}
	if err := desired.Validate(); err != nil {
		return MutationResult{}, unprocessableError(err)
	}
	desiredCmdPalette := loaded.config.CmdPalette
	desiredCmdPalette.Aliases = desiredAliases
	if err := desiredCmdPalette.Validate(); err != nil {
		return MutationResult{}, unprocessableError(err)
	}
	changed := diffWindowManagerSettings(
		loaded.config.WindowManager,
		loaded.config.CmdPalette.Aliases,
		desired,
		desiredAliases,
	)
	bindingChanges := changedBindingCommandIDs(loaded.config.WindowManager, desired)
	aliasChanges := changedAliasCommandIDs(loaded.config.CmdPalette.Aliases, desiredAliases)
	eventWorkspaces, err := s.cmdPaletteEventWorkspaces(
		ctx,
		loaded.scope,
		loaded.workspaceID,
		len(bindingChanges)+len(aliasChanges) > 0,
	)
	if err != nil {
		return MutationResult{}, err
	}
	result, err := s.updateScopedConfigSection(
		req.Section,
		changed,
		loaded.target,
		loaded.scope,
		loaded.workspaceID,
		loaded.workspaceRoot,
		func(editor *compozyconfig.OverlayEditor) error {
			return applyWindowManagerSettings(editor, desired, desiredAliases)
		},
	)
	if err != nil {
		return MutationResult{}, err
	}
	s.emitCmdPaletteSettingsEvents(ctx, eventWorkspaces, bindingChanges, aliasChanges)
	return result, nil
}

func (s *service) normalizeWindowManagerRequest(
	ctx context.Context,
	loaded *scopedSectionUpdate,
	req SectionUpdateRequest,
) (compozyconfig.WindowManagerConfig, map[string]string, error) {
	desired, desiredAliases := mergeWindowManagerRequest(
		loaded.config.WindowManager,
		loaded.config.CmdPalette.Aliases,
		req,
	)
	bindableIDs, err := s.windowManagerBindableIDs(ctx, loaded.workspaceID)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, nil, err
	}
	desired.Shortcuts, err = normalizeShortcutMutation(
		loaded.config.WindowManager.Shortcuts,
		desired.Shortcuts,
		bindableIDs,
		req.Overwrite,
	)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, nil, err
	}
	desired.GlobalShortcuts, err = normalizeGlobalShortcutMutation(
		loaded.config.WindowManager.GlobalShortcuts,
		desired.GlobalShortcuts,
		bindableIDs,
		req.Overwrite,
	)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, nil, err
	}
	desiredAliases, err = normalizeAliasMutation(
		loaded.config.CmdPalette.Aliases,
		desiredAliases,
		bindableIDs,
		req.Overwrite,
	)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, nil, err
	}
	return desired, desiredAliases, nil
}

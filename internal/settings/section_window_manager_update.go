package settings

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
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
		ScopeGlobal,
		ScopeWorkspace,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if req.WindowManager == nil && req.WindowManagerShortcuts == nil &&
		req.WindowManagerGlobalShortcuts == nil && req.WindowManagerAliases == nil {
		return MutationResult{}, validationError(
			errors.New("settings: window-manager section payload is required"),
		)
	}
	desired := cloneWindowManagerConfig(loaded.config.WindowManager)
	if req.WindowManager != nil {
		desired = cloneWindowManagerConfig(*req.WindowManager)
	}
	if req.WindowManagerShortcuts != nil {
		desired.Shortcuts = windowmanager.CloneShortcutMap(*req.WindowManagerShortcuts)
	}
	if req.WindowManagerGlobalShortcuts != nil {
		desired.GlobalShortcuts = windowmanager.CloneGlobalShortcutMap(
			*req.WindowManagerGlobalShortcuts,
		)
	}
	desiredAliases := cloneAliases(loaded.config.CmdPalette.Aliases)
	if req.WindowManagerAliases != nil {
		desiredAliases = cloneAliases(*req.WindowManagerAliases)
	}
	bindableIDs, err := s.windowManagerBindableIDs(ctx, loaded.workspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	desired.Shortcuts, err = normalizeShortcutMutation(
		loaded.config.WindowManager.Shortcuts,
		desired.Shortcuts,
		bindableIDs,
		req.Overwrite,
	)
	if err != nil {
		return MutationResult{}, err
	}
	desired.GlobalShortcuts, err = normalizeGlobalShortcutMutation(
		loaded.config.WindowManager.GlobalShortcuts,
		desired.GlobalShortcuts,
		bindableIDs,
		req.Overwrite,
	)
	if err != nil {
		return MutationResult{}, err
	}
	desiredAliases, err = normalizeAliasMutation(
		loaded.config.CmdPalette.Aliases,
		desiredAliases,
		bindableIDs,
		req.Overwrite,
	)
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
	return s.updateScopedConfigSection(
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
}

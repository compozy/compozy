package settings

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *service) updateCmdPaletteSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	loaded, err := s.loadScopedSectionUpdateForProfile(
		ctx,
		req.Section,
		req.Scope,
		req.WorkspaceID,
		req.ProfileName,
		ScopeUser,
		ScopeProfile,
		ScopeWorkspace,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if req.CmdPalette == nil {
		return MutationResult{}, validationError(
			errors.New("settings: cmd-palette section payload is required"),
		)
	}
	if req.CmdPalette.Aliases != nil && len(*req.CmdPalette.Aliases) == 0 {
		return MutationResult{}, validationError(errors.New(
			"settings: cmd-palette aliases cannot be empty; omit aliases to preserve the current map",
		))
	}
	desired := desiredCmdPaletteSection(loaded.config.CmdPalette, *req.CmdPalette)
	changed := diffCmdPaletteSettings(loaded.config.CmdPalette, desired)
	aliasChanges := changedAliasCommandIDs(loaded.config.CmdPalette.Aliases, desired.Aliases)
	eventWorkspaces, err := s.cmdPaletteEventWorkspaces(
		ctx,
		loaded.scope,
		loaded.workspaceID,
		len(aliasChanges) > 0,
	)
	if err != nil {
		return MutationResult{}, err
	}
	result, err := s.updateScopedConfigSectionForProfile(
		req.Section,
		changed,
		loaded.target,
		loaded.scope,
		loaded.workspaceID,
		loaded.profileName,
		loaded.workspaceRoot,
		func(editor *compozyconfig.OverlayEditor) error {
			return applyCmdPaletteSettings(editor, *req.CmdPalette)
		},
	)
	if err != nil {
		return MutationResult{}, err
	}
	s.emitCmdPaletteSettingsEvents(ctx, eventWorkspaces, nil, aliasChanges)
	return result, nil
}

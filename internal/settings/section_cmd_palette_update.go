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
	desired := desiredCmdPaletteSection(loaded.config.CmdPalette, *req.CmdPalette)
	changed := diffCmdPaletteSettings(loaded.config.CmdPalette, desired)
	return s.updateScopedConfigSectionForProfile(
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
}

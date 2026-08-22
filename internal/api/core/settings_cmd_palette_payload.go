package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func settingsCmdPaletteSectionResponse(envelope settingspkg.SectionEnvelope) (any, error) {
	if envelope.CmdPalette == nil {
		return nil, errors.New("settings cmd-palette section is required")
	}
	return contract.SettingsCmdPaletteResponse{
		SettingsLayeredSectionResponseMetaPayload: settingsGlobalWorkspaceSectionMetaPayload(envelope),
		FallbackAgentEnabled:                      envelope.CmdPalette.FallbackAgentEnabled,
		Personalization:                           envelope.CmdPalette.Personalization,
		Aliases:                                   cloneSettingsAliases(envelope.CmdPalette.Aliases),
	}, nil
}

package settings

import (
	"reflect"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const cmdPaletteConfigRoot = "cmd_palette"

func buildCmdPaletteSection(cfg *compozyconfig.Config) CmdPaletteSection {
	return CmdPaletteSection{Personalization: cfg.CmdPalette.Personalization}
}

func diffCmdPaletteSettings(current compozyconfig.CmdPaletteConfig, desired CmdPaletteSection) []string {
	if current.Personalization == desired.Personalization {
		return nil
	}
	return []string{"cmd_palette.personalization"}
}

func diffCmdPaletteConfig(
	current compozyconfig.CmdPaletteConfig,
	desired compozyconfig.CmdPaletteConfig,
) []string {
	changed := make([]string, 0, 2)
	if !reflect.DeepEqual(current.FallbackTargets, desired.FallbackTargets) {
		changed = append(changed, "cmd_palette.fallback_targets")
	}
	if current.Personalization != desired.Personalization {
		changed = append(changed, "cmd_palette.personalization")
	}
	return changed
}

func applyCmdPaletteSettings(
	editor *compozyconfig.OverlayEditor,
	settings CmdPaletteSection,
) error {
	return editor.SetValue([]string{cmdPaletteConfigRoot, "personalization"}, settings.Personalization)
}

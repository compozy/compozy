package settings

import (
	"reflect"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const cmdPaletteConfigRoot = "cmd_palette"

func buildCmdPaletteSection(cfg *compozyconfig.Config) CmdPaletteSection {
	return CmdPaletteSection{
		FallbackAgentEnabled: slices.Contains(
			cfg.CmdPalette.FallbackTargets,
			compozyconfig.CmdPaletteFallbackAgent,
		),
		Personalization: cfg.CmdPalette.Personalization,
	}
}

func desiredCmdPaletteSection(
	current compozyconfig.CmdPaletteConfig,
	update CmdPaletteUpdate,
) CmdPaletteSection {
	desired := CmdPaletteSection{
		FallbackAgentEnabled: slices.Contains(
			current.FallbackTargets,
			compozyconfig.CmdPaletteFallbackAgent,
		),
		Personalization: current.Personalization,
	}
	if update.FallbackAgentEnabled != nil {
		desired.FallbackAgentEnabled = *update.FallbackAgentEnabled
	}
	if update.Personalization != nil {
		desired.Personalization = *update.Personalization
	}
	return desired
}

func diffCmdPaletteSettings(
	current compozyconfig.CmdPaletteConfig,
	desired CmdPaletteSection,
) []string {
	changed := make([]string, 0, 2)
	currentFallbackEnabled := slices.Contains(
		current.FallbackTargets,
		compozyconfig.CmdPaletteFallbackAgent,
	)
	if currentFallbackEnabled != desired.FallbackAgentEnabled {
		changed = append(changed, "cmd_palette.fallback_targets")
	}
	if current.Personalization != desired.Personalization {
		changed = append(changed, "cmd_palette.personalization")
	}
	return changed
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
	update CmdPaletteUpdate,
) error {
	if update.FallbackAgentEnabled != nil {
		targets := []string{}
		if *update.FallbackAgentEnabled {
			targets = []string{compozyconfig.CmdPaletteFallbackAgent}
		}
		if err := editor.SetValue([]string{cmdPaletteConfigRoot, "fallback_targets"}, targets); err != nil {
			return err
		}
	}
	if update.Personalization != nil {
		if err := editor.SetValue(
			[]string{cmdPaletteConfigRoot, "personalization"},
			*update.Personalization,
		); err != nil {
			return err
		}
	}
	return nil
}

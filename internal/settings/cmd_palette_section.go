package settings

import (
	"reflect"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const cmdPaletteConfigRoot = "cmd_palette"

func cmdPaletteSectionFromConfig(current compozyconfig.CmdPaletteConfig) CmdPaletteSection {
	return CmdPaletteSection{
		FallbackAgentEnabled: slices.Contains(
			current.FallbackTargets,
			compozyconfig.CmdPaletteFallbackAgent,
		),
		Personalization: current.Personalization,
	}
}

func buildCmdPaletteSection(cfg *compozyconfig.Config) CmdPaletteSection {
	return cmdPaletteSectionFromConfig(cfg.CmdPalette)
}

func desiredCmdPaletteSection(
	current compozyconfig.CmdPaletteConfig,
	update CmdPaletteUpdate,
) CmdPaletteSection {
	desired := cmdPaletteSectionFromConfig(current)
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
	currentSection := cmdPaletteSectionFromConfig(current)
	if currentSection.FallbackAgentEnabled != desired.FallbackAgentEnabled {
		changed = append(changed, "cmd_palette.fallback_targets")
	}
	if currentSection.Personalization != desired.Personalization {
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

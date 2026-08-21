package config

import "fmt"

// ApplyCmdPaletteOverlayFile applies the optional workspace [cmd_palette] overlay.
func ApplyCmdPaletteOverlayFile(path string, defaults CmdPaletteConfig) (CmdPaletteConfig, error) {
	overlay, err := loadConfigOverlayFile(path)
	if err != nil {
		return CmdPaletteConfig{}, err
	}
	result := CloneCmdPaletteConfig(defaults)
	overlay.CmdPalette.Apply(&result)
	result.normalizeFallbackTargets()
	if err := result.Validate(); err != nil {
		return CmdPaletteConfig{}, fmt.Errorf("validate command palette overlay %q: %w", path, err)
	}
	return result, nil
}

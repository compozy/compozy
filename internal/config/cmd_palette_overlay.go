package config

import (
	"fmt"
	"os"
)

// ApplyCmdPaletteOverlayFile applies the optional workspace [cmd_palette] overlay.
func ApplyCmdPaletteOverlayFile(path string, defaults CmdPaletteConfig) (CmdPaletteConfig, error) {
	overlay, err := loadConfigOverlayFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cloneCmdPaletteConfig(defaults), nil
		}
		return CmdPaletteConfig{}, err
	}
	result := cloneCmdPaletteConfig(defaults)
	overlay.CmdPalette.Apply(&result)
	if err := result.Validate(); err != nil {
		return CmdPaletteConfig{}, fmt.Errorf("validate command palette overlay %q: %w", path, err)
	}
	return result, nil
}

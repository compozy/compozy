package config

import (
	"fmt"
	"maps"
)

// ApplyWindowManagerOverlayFile applies only the optional [window_manager]
// section at path onto the supplied active defaults.
func ApplyWindowManagerOverlayFile(
	path string,
	defaults WindowManagerConfig,
) (WindowManagerConfig, error) {
	overlay, err := loadConfigOverlayFile(path)
	if err != nil {
		return WindowManagerConfig{}, err
	}
	result := cloneWindowManagerConfig(defaults)
	overlay.WindowManager.Apply(&result)
	if err := result.Validate(); err != nil {
		return WindowManagerConfig{}, fmt.Errorf("validate window manager overlay %q: %w", path, err)
	}
	return result, nil
}

func cloneWindowManagerConfig(source WindowManagerConfig) WindowManagerConfig {
	cloned := source
	cloned.Snap.RepeatRatios = append([]float64(nil), source.Snap.RepeatRatios...)
	cloned.Shortcuts = maps.Clone(source.Shortcuts)
	return cloned
}

package config

import (
	"maps"
	"strings"
)

type cmdPaletteOverlay struct {
	FallbackTargets *[]string         `toml:"fallback_targets"`
	Personalization *bool             `toml:"personalization"`
	Aliases         map[string]string `toml:"aliases"`
}

func (o cmdPaletteOverlay) Apply(dst *CmdPaletteConfig) {
	if o.FallbackTargets != nil {
		targets := make([]string, 0, len(*o.FallbackTargets))
		for _, target := range *o.FallbackTargets {
			if trimmed := strings.TrimSpace(target); trimmed != "" {
				targets = append(targets, trimmed)
			}
		}
		dst.FallbackTargets = targets
	}
	if o.Personalization != nil {
		dst.Personalization = *o.Personalization
	}
	if len(o.Aliases) == 0 {
		return
	}
	if dst.Aliases == nil {
		dst.Aliases = make(map[string]string, len(o.Aliases))
	}
	maps.Copy(dst.Aliases, o.Aliases)
}

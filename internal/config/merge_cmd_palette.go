package config

type cmdPaletteOverlay struct {
	FallbackTargets *[]string         `toml:"fallback_targets"`
	Personalization *bool             `toml:"personalization"`
	Aliases         map[string]string `toml:"aliases"`
}

func (o cmdPaletteOverlay) Apply(dst *CmdPaletteConfig) {
	if o.FallbackTargets != nil {
		dst.FallbackTargets = append([]string(nil), (*o.FallbackTargets)...)
	}
	if o.Personalization != nil {
		dst.Personalization = *o.Personalization
	}
	for commandID, alias := range o.Aliases {
		if dst.Aliases == nil {
			dst.Aliases = make(map[string]string)
		}
		dst.Aliases[commandID] = alias
	}
}

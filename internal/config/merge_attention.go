package config

type attentionOverlay struct {
	Toasts *bool `toml:"toasts"`
	Sound  *bool `toml:"sound"`
	System *bool `toml:"system"`
}

func (o attentionOverlay) Apply(dst *AttentionConfig) {
	applyBoolPointer(o.Toasts, &dst.Toasts)
	applyBoolPointer(o.Sound, &dst.Sound)
	applyBoolPointer(o.System, &dst.System)
}

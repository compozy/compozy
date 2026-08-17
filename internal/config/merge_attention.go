package config

type attentionOverlay struct {
	Toasts          *bool     `toml:"toasts"`
	Sound           *bool     `toml:"sound"`
	System          *bool     `toml:"system"`
	MutedWorkspaces *[]string `toml:"muted_workspaces"`
}

func (o attentionOverlay) Apply(dst *AttentionConfig) {
	applyBoolPointer(o.Toasts, &dst.Toasts)
	applyBoolPointer(o.Sound, &dst.Sound)
	applyBoolPointer(o.System, &dst.System)
	if o.MutedWorkspaces != nil {
		dst.MutedWorkspaces = append([]string(nil), (*o.MutedWorkspaces)...)
	}
}

package config

type shellOverlay struct {
	Sessions shellSessionsOverlay `toml:"sessions"`
}

type shellSessionsOverlay struct {
	Sort  *ShellSessionSort  `toml:"sort"`
	Scope *ShellSessionScope `toml:"scope"`
}

func (o shellOverlay) Apply(dst *ShellConfig) {
	if o.Sessions.Sort != nil {
		dst.Sessions.Sort = *o.Sessions.Sort
	}
	if o.Sessions.Scope != nil {
		dst.Sessions.Scope = *o.Sessions.Scope
	}
}

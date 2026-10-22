package config

type shellOverlay struct {
	Sessions shellSessionsOverlay `toml:"sessions"`
}

type shellSessionsOverlay struct {
	GroupBy *ShellSessionGroupBy `toml:"group_by"`
	Sort    *ShellSessionSort    `toml:"sort"`
	Scope   *ShellSessionScope   `toml:"scope"`
}

func (o shellOverlay) Apply(dst *ShellConfig) {
	if o.Sessions.Sort != nil {
		dst.Sessions.Sort = *o.Sessions.Sort
	}
	if o.Sessions.Scope != nil {
		dst.Sessions.Scope = *o.Sessions.Scope
		dst.Sessions.ScopeExplicit = true
	}
	if o.Sessions.GroupBy != nil {
		dst.Sessions.GroupBy = *o.Sessions.GroupBy
		dst.Sessions.GroupByExplicit = true
	}
	if o.Sessions.Scope != nil || o.Sessions.GroupBy != nil {
		dst.Sessions.ResolveGroupBy()
	}
}

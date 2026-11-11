package config

type shellOverlay struct {
	Sessions   shellSessionsOverlay   `toml:"sessions"`
	Appearance shellAppearanceOverlay `toml:"appearance"`
}

type shellAppearanceOverlay struct {
	UIFont        *string             `toml:"ui_font"`
	MonoFont      *string             `toml:"mono_font"`
	TextSize      *ShellTextSize      `toml:"text_size"`
	LetterSpacing *ShellLetterSpacing `toml:"letter_spacing"`
}

type shellSessionsOverlay struct {
	GroupBy *ShellSessionGroupBy `toml:"group_by"`
	Sort    *ShellSessionSort    `toml:"sort"`
	Scope   *ShellSessionScope   `toml:"scope"`
}

func (o shellOverlay) Apply(dst *ShellConfig) {
	if o.Appearance.UIFont != nil {
		dst.Appearance.UIFont = *o.Appearance.UIFont
		dst.Appearance.UIFontExplicit = true
	}
	if o.Appearance.MonoFont != nil {
		dst.Appearance.MonoFont = *o.Appearance.MonoFont
		dst.Appearance.MonoFontExplicit = true
	}
	if o.Appearance.TextSize != nil {
		dst.Appearance.TextSize = *o.Appearance.TextSize
		dst.Appearance.TextSizeExplicit = true
	}
	if o.Appearance.LetterSpacing != nil {
		dst.Appearance.LetterSpacing = *o.Appearance.LetterSpacing
		dst.Appearance.LetterSpacingExplicit = true
	}
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

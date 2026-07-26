package config

import "maps"

type windowManagerOverlay struct {
	NewWindowPolicy     *string                     `toml:"new_window_policy"`
	SmallViewportPolicy *string                     `toml:"small_viewport_policy"`
	FocusPolicy         *string                     `toml:"focus_policy"`
	FocusWrap           *bool                       `toml:"focus_wrap"`
	FocusFollowsPointer *bool                       `toml:"focus_follows_pointer"`
	RaiseOnFocus        *bool                       `toml:"raise_on_focus"`
	DragAwayPolicy      *string                     `toml:"drag_away_policy"`
	GroupMoveModifier   *string                     `toml:"group_move_modifier"`
	SwapModifier        *string                     `toml:"swap_modifier"`
	HistoryLimit        *int                        `toml:"history_limit"`
	DesktopTransition   *string                     `toml:"desktop_transition"`
	Gaps                windowManagerGapsOverlay    `toml:"gaps"`
	Snap                windowManagerSnapOverlay    `toml:"snap"`
	Bindings            windowManagerBindingOverlay `toml:"bindings"`
	Shortcuts           map[string]string           `toml:"shortcuts,omitempty"`
}

type windowManagerGapsOverlay struct {
	Inner  *int `toml:"inner"`
	Top    *int `toml:"top"`
	Right  *int `toml:"right"`
	Bottom *int `toml:"bottom"`
	Left   *int `toml:"left"`
}

type windowManagerSnapOverlay struct {
	EdgeBand     *int      `toml:"edge_band"`
	CornerReach  *int      `toml:"corner_reach"`
	ExitSlack    *int      `toml:"exit_slack"`
	RepeatRatios []float64 `toml:"repeat_ratios"`
}

type windowManagerBindingOverlay struct {
	TopCenter    *string `toml:"top_center"`
	BottomCenter *string `toml:"bottom_center"`
}

func (o windowManagerOverlay) Apply(dst *WindowManagerConfig) {
	applyStringPointer(o.NewWindowPolicy, &dst.NewWindowPolicy)
	applyStringPointer(o.SmallViewportPolicy, &dst.SmallViewportPolicy)
	applyStringPointer(o.FocusPolicy, &dst.FocusPolicy)
	applyBoolPointer(o.FocusWrap, &dst.FocusWrap)
	applyBoolPointer(o.FocusFollowsPointer, &dst.FocusFollowsPointer)
	applyBoolPointer(o.RaiseOnFocus, &dst.RaiseOnFocus)
	applyStringPointer(o.DragAwayPolicy, &dst.DragAwayPolicy)
	applyStringPointer(o.GroupMoveModifier, &dst.GroupMoveModifier)
	applyStringPointer(o.SwapModifier, &dst.SwapModifier)
	applyIntPointer(o.HistoryLimit, &dst.HistoryLimit)
	applyStringPointer(o.DesktopTransition, &dst.DesktopTransition)
	o.Gaps.Apply(&dst.Gaps)
	o.Snap.Apply(&dst.Snap)
	o.Bindings.Apply(&dst.Bindings)
	if o.Shortcuts != nil {
		dst.Shortcuts = maps.Clone(o.Shortcuts)
	}
}

func (o windowManagerGapsOverlay) Apply(dst *WindowManagerGapsConfig) {
	applyIntPointer(o.Inner, &dst.Inner)
	applyIntPointer(o.Top, &dst.Top)
	applyIntPointer(o.Right, &dst.Right)
	applyIntPointer(o.Bottom, &dst.Bottom)
	applyIntPointer(o.Left, &dst.Left)
}

func (o windowManagerSnapOverlay) Apply(dst *WindowManagerSnapConfig) {
	applyIntPointer(o.EdgeBand, &dst.EdgeBand)
	applyIntPointer(o.CornerReach, &dst.CornerReach)
	applyIntPointer(o.ExitSlack, &dst.ExitSlack)
	if o.RepeatRatios != nil {
		dst.RepeatRatios = append([]float64(nil), o.RepeatRatios...)
	}
}

func (o windowManagerBindingOverlay) Apply(dst *WindowManagerBindingConfig) {
	applyStringPointer(o.TopCenter, &dst.TopCenter)
	applyStringPointer(o.BottomCenter, &dst.BottomCenter)
}

func applyStringPointer(src *string, dst *string) {
	if src != nil {
		*dst = *src
	}
}

func applyBoolPointer(src *bool, dst *bool) {
	if src != nil {
		*dst = *src
	}
}

func applyIntPointer(src *int, dst *int) {
	if src != nil {
		*dst = *src
	}
}

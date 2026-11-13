package config

import "github.com/compozy/compozy/internal/windowmanager"

type windowManagerOverlay struct {
	NewWindowPolicy     *string                                  `toml:"new_window_policy"`
	SmallViewportPolicy *string                                  `toml:"small_viewport_policy"`
	FocusPolicy         *string                                  `toml:"focus_policy"`
	FocusWrap           *bool                                    `toml:"focus_wrap"`
	FocusFollowsPointer *bool                                    `toml:"focus_follows_pointer"`
	RaiseOnFocus        any                                      `toml:"raise_on_focus"`
	DragAwayPolicy      any                                      `toml:"drag_away_policy"`
	GroupMoveModifier   any                                      `toml:"group_move_modifier"`
	SwapModifier        *string                                  `toml:"swap_modifier"`
	HistoryLimit        *int                                     `toml:"history_limit"`
	NavStackLimit       *int                                     `toml:"nav_stack_limit"`
	ClosedEntryLimit    *int                                     `toml:"closed_entry_limit"`
	DesktopTransition   *string                                  `toml:"desktop_transition"`
	Gaps                windowManagerGapsOverlay                 `toml:"gaps"`
	Snap                windowManagerSnapOverlay                 `toml:"snap"`
	Bindings            windowManagerBindingOverlay              `toml:"bindings"`
	Shortcuts           map[string]windowmanager.ShortcutBinding `toml:"shortcuts,omitempty"`
	GlobalShortcuts     map[string]string                        `toml:"global_shortcuts,omitempty"`
}

type windowManagerGapsOverlay struct {
	Inner  *int `toml:"inner"`
	Top    *int `toml:"top"`
	Right  *int `toml:"right"`
	Bottom *int `toml:"bottom"`
	Left   *int `toml:"left"`
}

type windowManagerSnapOverlay struct {
	EdgeBand     any `toml:"edge_band"`
	CornerReach  any `toml:"corner_reach"`
	ExitSlack    any `toml:"exit_slack"`
	RepeatRatios any `toml:"repeat_ratios"`
}

type windowManagerBindingOverlay struct {
	TopCenter    any `toml:"top_center"`
	BottomCenter any `toml:"bottom_center"`
}

func (o windowManagerOverlay) Apply(dst *WindowManagerConfig) {
	dst.RaiseOnFocus, dst.DragAwayPolicy, dst.GroupMoveModifier = false, "", ""
	dst.Snap, dst.Bindings = WindowManagerSnapConfig{}, WindowManagerBindingConfig{}
	applyStringPointer(o.NewWindowPolicy, &dst.NewWindowPolicy)
	if dst.NewWindowPolicy == WindowNewPolicyFloating {
		dst.warnRetiredKey("new_window_policy", true)
		dst.NewWindowPolicy = WindowNewPolicyTab
	}
	warnRetiredWindowManagerOverlay(o, dst)
	applyStringPointer(o.SmallViewportPolicy, &dst.SmallViewportPolicy)
	applyStringPointer(o.FocusPolicy, &dst.FocusPolicy)
	applyBoolPointer(o.FocusWrap, &dst.FocusWrap)
	applyBoolPointer(o.FocusFollowsPointer, &dst.FocusFollowsPointer)
	applyStringPointer(o.SwapModifier, &dst.SwapModifier)
	applyIntPointer(o.HistoryLimit, &dst.HistoryLimit)
	applyIntPointer(o.NavStackLimit, &dst.NavStackLimit)
	applyIntPointer(o.ClosedEntryLimit, &dst.ClosedEntryLimit)
	applyStringPointer(o.DesktopTransition, &dst.DesktopTransition)
	o.Gaps.Apply(&dst.Gaps)
	if o.Shortcuts != nil {
		dst.Shortcuts = windowmanager.CloneShortcutMap(o.Shortcuts)
	}
	if o.GlobalShortcuts != nil {
		dst.GlobalShortcuts = windowmanager.CloneGlobalShortcutMap(o.GlobalShortcuts)
	}
}

func (o windowManagerGapsOverlay) Apply(dst *WindowManagerGapsConfig) {
	applyIntPointer(o.Inner, &dst.Inner)
	applyIntPointer(o.Top, &dst.Top)
	applyIntPointer(o.Right, &dst.Right)
	applyIntPointer(o.Bottom, &dst.Bottom)
	applyIntPointer(o.Left, &dst.Left)
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

package config

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/windowmanager"
)

const (
	WindowNewPolicyFloating    = "floating"
	WindowNewPolicyBesideFocus = "beside_focus"

	WindowSmallViewportStack  = "stack"
	WindowSmallViewportReject = "reject"

	WindowFocusClickDirectional = "click_directional"
	WindowFocusDirectional      = "directional"

	WindowDragAwayWindow = "window"
	WindowDragAwayGroup  = "group"

	WindowDesktopTransitionSlide     = "slide"
	WindowDesktopTransitionCrossfade = "crossfade"
	WindowDesktopTransitionInstant   = "instant"

	WindowBindingNone     = "none"
	WindowBindingReserved = "reserved"
	WindowBindingZoom     = "zoom"

	WindowDragModifierAlt     = "alt"
	WindowDragModifierControl = "control"
	WindowDragModifierMeta    = "meta"
	WindowDragModifierShift   = "shift"
	WindowDragModifierNone    = "none"
)

// windowDragModifiers lists the modifier keys accepted by group_move_modifier and swap_modifier.
func windowDragModifiers() []string {
	return []string{
		WindowDragModifierAlt,
		WindowDragModifierControl,
		WindowDragModifierMeta,
		WindowDragModifierShift,
		WindowDragModifierNone,
	}
}

// WindowManagerConfig controls window topology behavior and browser projection defaults.
type WindowManagerConfig struct {
	NewWindowPolicy     string                     `toml:"new_window_policy"`
	SmallViewportPolicy string                     `toml:"small_viewport_policy"`
	FocusPolicy         string                     `toml:"focus_policy"`
	FocusWrap           bool                       `toml:"focus_wrap"`
	FocusFollowsPointer bool                       `toml:"focus_follows_pointer"`
	RaiseOnFocus        bool                       `toml:"raise_on_focus"`
	DragAwayPolicy      string                     `toml:"drag_away_policy"`
	GroupMoveModifier   string                     `toml:"group_move_modifier"`
	SwapModifier        string                     `toml:"swap_modifier"`
	HistoryLimit        int                        `toml:"history_limit"`
	DesktopTransition   string                     `toml:"desktop_transition"`
	Gaps                WindowManagerGapsConfig    `toml:"gaps"`
	Snap                WindowManagerSnapConfig    `toml:"snap"`
	Bindings            WindowManagerBindingConfig `toml:"bindings"`
	Shortcuts           map[string]string          `toml:"shortcuts,omitempty"`
}

// WindowManagerGapsConfig controls the shared inner gap and work-area insets.
type WindowManagerGapsConfig struct {
	Inner  int `toml:"inner"`
	Top    int `toml:"top"`
	Right  int `toml:"right"`
	Bottom int `toml:"bottom"`
	Left   int `toml:"left"`
}

// WindowManagerSnapConfig controls pointer target resolution and repeated side ratios.
type WindowManagerSnapConfig struct {
	EdgeBand     int       `toml:"edge_band"`
	CornerReach  int       `toml:"corner_reach"`
	ExitSlack    int       `toml:"exit_slack"`
	RepeatRatios []float64 `toml:"repeat_ratios"`
}

// WindowManagerBindingConfig maps each edge-center snap action; "reserved" blocks that edge-center snap.
type WindowManagerBindingConfig struct {
	TopCenter string `toml:"top_center"`
	// BottomCenter defaults to reserved to keep the approach strip above the Dock clear of snaps.
	BottomCenter string `toml:"bottom_center"`
}

// DefaultWindowManagerConfig returns the built-in behavior shared by daemon and web clients.
func DefaultWindowManagerConfig() WindowManagerConfig {
	return WindowManagerConfig{
		NewWindowPolicy:     WindowNewPolicyFloating,
		SmallViewportPolicy: WindowSmallViewportStack,
		FocusPolicy:         WindowFocusClickDirectional,
		FocusWrap:           false,
		FocusFollowsPointer: false,
		RaiseOnFocus:        true,
		DragAwayPolicy:      WindowDragAwayWindow,
		GroupMoveModifier:   WindowDragModifierAlt,
		SwapModifier:        WindowDragModifierShift,
		HistoryLimit:        50,
		DesktopTransition:   WindowDesktopTransitionSlide,
		Gaps: WindowManagerGapsConfig{
			Inner: 8, Top: 8, Right: 10, Bottom: 8, Left: 10,
		},
		Snap: WindowManagerSnapConfig{
			EdgeBand: 32, CornerReach: 150, ExitSlack: 16,
			RepeatRatios: []float64{0.5, 0.666667, 0.333333},
		},
		Bindings: WindowManagerBindingConfig{
			TopCenter: WindowBindingZoom, BottomCenter: WindowBindingReserved,
		},
		Shortcuts: map[string]string{},
	}
}

// Validate rejects incomplete or internally conflicting window-manager behavior.
func (c WindowManagerConfig) Validate() error {
	if err := validateWindowManagerEnum(
		"window_manager.new_window_policy",
		c.NewWindowPolicy,
		WindowNewPolicyFloating,
		WindowNewPolicyBesideFocus,
	); err != nil {
		return err
	}
	if err := validateWindowManagerEnum(
		"window_manager.small_viewport_policy",
		c.SmallViewportPolicy,
		WindowSmallViewportStack,
		WindowSmallViewportReject,
	); err != nil {
		return err
	}
	if err := validateWindowManagerEnum(
		"window_manager.focus_policy",
		c.FocusPolicy,
		WindowFocusClickDirectional,
		WindowFocusDirectional,
	); err != nil {
		return err
	}
	if err := validateWindowManagerEnum(
		"window_manager.drag_away_policy",
		c.DragAwayPolicy,
		WindowDragAwayWindow,
		WindowDragAwayGroup,
	); err != nil {
		return err
	}
	if err := validateWindowManagerEnum(
		"window_manager.desktop_transition",
		c.DesktopTransition,
		WindowDesktopTransitionSlide,
		WindowDesktopTransitionCrossfade,
		WindowDesktopTransitionInstant,
	); err != nil {
		return err
	}
	if err := validateWindowManagerEnum(
		"window_manager.group_move_modifier",
		c.GroupMoveModifier,
		windowDragModifiers()...,
	); err != nil {
		return err
	}
	if err := validateWindowManagerEnum(
		"window_manager.swap_modifier",
		c.SwapModifier,
		windowDragModifiers()...,
	); err != nil {
		return err
	}
	if c.HistoryLimit < 1 || c.HistoryLimit > 500 {
		return ValidationError{
			Path: "window_manager.history_limit", Message: "must be between 1 and 500",
		}
	}
	if err := c.Gaps.Validate(); err != nil {
		return err
	}
	if err := c.Snap.Validate(); err != nil {
		return err
	}
	if err := c.Bindings.Validate(); err != nil {
		return err
	}
	return validateWindowManagerShortcuts(c.Shortcuts)
}

// Validate ensures projection gaps stay within a practical viewport-safe range.
func (c WindowManagerGapsConfig) Validate() error {
	values := []struct {
		name  string
		value int
	}{
		{name: "inner", value: c.Inner},
		{name: "top", value: c.Top},
		{name: "right", value: c.Right},
		{name: "bottom", value: c.Bottom},
		{name: "left", value: c.Left},
	}
	for _, field := range values {
		if field.value < 0 || field.value > 64 {
			return ValidationError{
				Path: "window_manager.gaps." + field.name, Message: "must be between 0 and 64",
			}
		}
	}
	return nil
}

// Validate ensures snap thresholds and ratios can resolve deterministic targets.
func (c WindowManagerSnapConfig) Validate() error {
	if c.EdgeBand < 4 || c.EdgeBand > 128 {
		return ValidationError{
			Path: "window_manager.snap.edge_band", Message: "must be between 4 and 128",
		}
	}
	if c.CornerReach < 16 || c.CornerReach > 512 {
		return ValidationError{
			Path: "window_manager.snap.corner_reach", Message: "must be between 16 and 512",
		}
	}
	if c.ExitSlack < 0 || c.ExitSlack > 64 {
		return ValidationError{
			Path: "window_manager.snap.exit_slack", Message: "must be between 0 and 64",
		}
	}
	if len(c.RepeatRatios) == 0 || len(c.RepeatRatios) > 8 {
		return ValidationError{
			Path: "window_manager.snap.repeat_ratios", Message: "must contain between 1 and 8 ratios",
		}
	}
	seen := make(map[int64]struct{}, len(c.RepeatRatios))
	for index, ratio := range c.RepeatRatios {
		if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0.1 || ratio > 0.9 {
			return ValidationError{
				Path:    fmt.Sprintf("window_manager.snap.repeat_ratios[%d]", index),
				Message: "must be finite and between 0.1 and 0.9",
			}
		}
		key := int64(math.Round(ratio * 1_000_000))
		if _, duplicate := seen[key]; duplicate {
			return ValidationError{
				Path: "window_manager.snap.repeat_ratios", Message: "must not contain duplicates",
			}
		}
		seen[key] = struct{}{}
	}
	return nil
}

// Validate ensures reserved edge centers resolve to supported semantic actions.
func (c WindowManagerBindingConfig) Validate() error {
	allowed := []string{WindowBindingNone, WindowBindingReserved, WindowBindingZoom}
	bindings := []struct {
		path  string
		value string
	}{
		{path: "window_manager.bindings.top_center", value: c.TopCenter},
		{path: "window_manager.bindings.bottom_center", value: c.BottomCenter},
	}
	for _, binding := range bindings {
		if !slices.Contains(allowed, strings.TrimSpace(binding.value)) {
			return ValidationError{
				Path: binding.path, Message: "must be one of none, reserved, or zoom",
			}
		}
	}
	return nil
}

func validateWindowManagerEnum(path string, value string, allowed ...string) error {
	trimmed := strings.TrimSpace(value)
	if slices.Contains(allowed, trimmed) {
		return nil
	}
	return ValidationError{Path: path, Message: "must be one of " + strings.Join(allowed, ", ")}
}

func validateWindowManagerShortcuts(shortcuts map[string]string) error {
	if _, err := windowmanager.CanonicalShortcuts(shortcuts); err != nil {
		return ValidationError{Path: "window_manager.shortcuts", Message: err.Error()}
	}
	return nil
}

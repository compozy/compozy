package config

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/windowmanager"
)

const (
	windowManagerNavStackLimitPath    = "window_manager.nav_stack_limit"
	windowManagerClosedEntryLimitPath = "window_manager.closed_entry_limit"

	WindowNewPolicyBesideFocus = "beside_focus"
	WindowNewPolicyTab         = "tab"

	WindowSmallViewportStack  = "stack"
	WindowSmallViewportReject = "reject"

	WindowFocusClickDirectional = "click_directional"
	WindowFocusDirectional      = "directional"

	WindowDesktopTransitionSlide     = "slide"
	WindowDesktopTransitionCrossfade = "crossfade"
	WindowDesktopTransitionInstant   = "instant"

	WindowDragModifierAlt     = "alt"
	WindowDragModifierControl = "control"
	WindowDragModifierMeta    = "meta"
	WindowDragModifierShift   = "shift"
	WindowDragModifierNone    = "none"
)

// windowDragModifiers lists the modifier keys accepted by swap_modifier.
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
	retiredKeyWarning   func(string, bool)
	NewWindowPolicy     string                                   `toml:"new_window_policy"`
	SmallViewportPolicy string                                   `toml:"small_viewport_policy"`
	FocusPolicy         string                                   `toml:"focus_policy"`
	FocusWrap           bool                                     `toml:"focus_wrap"`
	FocusFollowsPointer bool                                     `toml:"focus_follows_pointer"`
	SwapModifier        string                                   `toml:"swap_modifier"`
	HistoryLimit        int                                      `toml:"history_limit"`
	NavStackLimit       int                                      `toml:"nav_stack_limit"`
	ClosedEntryLimit    int                                      `toml:"closed_entry_limit"`
	DesktopTransition   string                                   `toml:"desktop_transition"`
	Gaps                WindowManagerGapsConfig                  `toml:"gaps"`
	Shortcuts           map[string]windowmanager.ShortcutBinding `toml:"shortcuts,omitempty"`
	GlobalShortcuts     map[string]string                        `toml:"global_shortcuts,omitempty"`
}

// WindowManagerGapsConfig controls the shared inner gap and work-area insets.
type WindowManagerGapsConfig struct {
	Inner  int `toml:"inner"`
	Top    int `toml:"top"`
	Right  int `toml:"right"`
	Bottom int `toml:"bottom"`
	Left   int `toml:"left"`
}

// DefaultWindowManagerConfig returns the built-in behavior shared by daemon and web clients.
func DefaultWindowManagerConfig() WindowManagerConfig {
	return WindowManagerConfig{
		NewWindowPolicy:     WindowNewPolicyTab,
		SmallViewportPolicy: WindowSmallViewportStack,
		FocusPolicy:         WindowFocusClickDirectional,
		FocusWrap:           false,
		FocusFollowsPointer: false,
		SwapModifier:        WindowDragModifierShift,
		HistoryLimit:        50,
		NavStackLimit:       50,
		ClosedEntryLimit:    20,
		DesktopTransition:   WindowDesktopTransitionSlide,
		Gaps:                WindowManagerGapsConfig{},
		Shortcuts:           map[string]windowmanager.ShortcutBinding{},
		GlobalShortcuts: map[string]string{
			windowmanager.DefaultGlobalSummonCommandID: windowmanager.DefaultGlobalSummonChord,
		},
	}
}

// Validate rejects incomplete or internally conflicting window-manager behavior.
func (c WindowManagerConfig) Validate() error {
	if err := c.validateEnums(); err != nil {
		return err
	}
	if c.HistoryLimit < 1 || c.HistoryLimit > 500 {
		return ValidationError{
			Path: "window_manager.history_limit", Message: "must be between 1 and 500",
		}
	}
	if c.NavStackLimit < 1 || c.NavStackLimit > 200 {
		return ValidationError{Path: windowManagerNavStackLimitPath, Message: "must be between 1 and 200"}
	}
	if c.ClosedEntryLimit < 1 || c.ClosedEntryLimit > 100 {
		return ValidationError{Path: windowManagerClosedEntryLimitPath, Message: "must be between 1 and 100"}
	}
	if err := c.Gaps.Validate(); err != nil {
		return err
	}
	if err := validateWindowManagerShortcuts(c.Shortcuts); err != nil {
		return err
	}
	if _, err := windowmanager.CanonicalStoredGlobalShortcuts(c.GlobalShortcuts); err != nil {
		return ValidationError{Path: "window_manager.global_shortcuts", Message: err.Error()}
	}
	return nil
}

func (c WindowManagerConfig) validateEnums() error {
	if err := validateWindowManagerEnum(
		"window_manager.new_window_policy",
		c.NewWindowPolicy,
		WindowNewPolicyBesideFocus,
		WindowNewPolicyTab,
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
		"window_manager.desktop_transition",
		c.DesktopTransition,
		WindowDesktopTransitionSlide,
		WindowDesktopTransitionCrossfade,
		WindowDesktopTransitionInstant,
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
	return nil
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

func validateWindowManagerEnum(path string, value string, allowed ...string) error {
	trimmed := strings.TrimSpace(value)
	if slices.Contains(allowed, trimmed) {
		return nil
	}
	return ValidationError{Path: path, Message: "must be one of " + strings.Join(allowed, ", ")}
}

func validateWindowManagerShortcuts(shortcuts map[string]windowmanager.ShortcutBinding) error {
	if _, err := windowmanager.CanonicalStoredShortcutsV2(shortcuts); err != nil {
		return ValidationError{Path: "window_manager.shortcuts", Message: err.Error()}
	}
	return nil
}

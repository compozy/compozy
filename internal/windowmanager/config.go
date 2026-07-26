package windowmanager

import (
	"fmt"
	"math"
)

// NewWindowPolicy controls how newly opened windows enter a desktop.
type NewWindowPolicy string

// SmallViewportPolicy controls layout behavior when the viewport cannot fit a tiled arrangement.
type SmallViewportPolicy string

// FocusPolicy controls how pointer and directional focus interact.
type FocusPolicy string

// DragAwayPolicy controls whether dragging detaches one window or its tiled group.
type DragAwayPolicy string

// DesktopTransition controls the client presentation used when switching desktops.
type DesktopTransition string

const (
	NewWindowFloating          NewWindowPolicy     = "floating"
	NewWindowInsert            NewWindowPolicy     = "beside_focus"
	SmallViewportStack         SmallViewportPolicy = "stack"
	SmallViewportReject        SmallViewportPolicy = "reject"
	FocusClickDirectional      FocusPolicy         = "click_directional"
	FocusDirectional           FocusPolicy         = "directional"
	DragAwayWindow             DragAwayPolicy      = "window"
	DragAwayGroup              DragAwayPolicy      = "group"
	DesktopTransitionSlide     DesktopTransition   = "slide"
	DesktopTransitionCrossfade DesktopTransition   = "crossfade"
	DesktopTransitionInstant   DesktopTransition   = "instant"
	dragModifierMeta                               = "meta"
	dragModifierControl                            = "control"
	dragModifierAlt                                = "alt"
	dragModifierShift                              = "shift"
	configValueNone                                = "none"
	bindingReserved                                = "reserved"
)

// GapsConfig defines the inner and outer layout gaps in CSS pixels.
type GapsConfig struct {
	Inner  float64 `json:"inner"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

// SnapConfig defines pointer thresholds and repeated edge-snap ratios.
type SnapConfig struct {
	EdgeBand     float64   `json:"edge_band"`
	CornerReach  float64   `json:"corner_reach"`
	ExitSlack    float64   `json:"exit_slack"`
	RepeatRatios []float64 `json:"repeat_ratios"`
}

// BindingsConfig maps special snap regions to window-manager actions.
type BindingsConfig struct {
	TopCenter    string `json:"top_center"`
	BottomCenter string `json:"bottom_center"`
}

// Config is the effective global window-manager configuration.
type Config struct {
	NewWindowPolicy     NewWindowPolicy     `json:"new_window_policy"`
	SmallViewportPolicy SmallViewportPolicy `json:"small_viewport_policy"`
	FocusPolicy         FocusPolicy         `json:"focus_policy"`
	FocusWrap           bool                `json:"focus_wrap"`
	FocusFollowsPointer bool                `json:"focus_follows_pointer"`
	RaiseOnFocus        bool                `json:"raise_on_focus"`
	DragAwayPolicy      DragAwayPolicy      `json:"drag_away_policy"`
	GroupMoveModifier   string              `json:"group_move_modifier"`
	SwapModifier        string              `json:"swap_modifier"`
	HistoryLimit        int                 `json:"history_limit"`
	DesktopTransition   DesktopTransition   `json:"desktop_transition"`
	Gaps                GapsConfig          `json:"gaps"`
	Snap                SnapConfig          `json:"snap"`
	Bindings            BindingsConfig      `json:"bindings"`
	Shortcuts           map[string]string   `json:"shortcuts,omitempty"`
}

// WorkspaceConfig contains optional workspace overrides.
type WorkspaceConfig struct {
	NewWindowPolicy     *NewWindowPolicy     `json:"new_window_policy,omitempty"`
	SmallViewportPolicy *SmallViewportPolicy `json:"small_viewport_policy,omitempty"`
	FocusPolicy         *FocusPolicy         `json:"focus_policy,omitempty"`
	FocusWrap           *bool                `json:"focus_wrap,omitempty"`
	FocusFollowsPointer *bool                `json:"focus_follows_pointer,omitempty"`
	RaiseOnFocus        *bool                `json:"raise_on_focus,omitempty"`
	DragAwayPolicy      *DragAwayPolicy      `json:"drag_away_policy,omitempty"`
	GroupMoveModifier   *string              `json:"group_move_modifier,omitempty"`
	SwapModifier        *string              `json:"swap_modifier,omitempty"`
	HistoryLimit        *int                 `json:"history_limit,omitempty"`
	DesktopTransition   *DesktopTransition   `json:"desktop_transition,omitempty"`
	Gaps                *GapsConfig          `json:"gaps,omitempty"`
	Snap                *SnapConfig          `json:"snap,omitempty"`
	Bindings            *BindingsConfig      `json:"bindings,omitempty"`
	Shortcuts           map[string]string    `json:"shortcuts,omitempty"`
}

// DefaultConfig returns production defaults from the accepted contract.
func DefaultConfig() Config {
	return Config{
		NewWindowPolicy:     NewWindowFloating,
		SmallViewportPolicy: SmallViewportStack,
		FocusPolicy:         FocusClickDirectional,
		RaiseOnFocus:        true,
		DragAwayPolicy:      DragAwayWindow,
		GroupMoveModifier:   dragModifierAlt,
		SwapModifier:        dragModifierShift,
		HistoryLimit:        50,
		DesktopTransition:   DesktopTransitionSlide,
		Gaps:                GapsConfig{Inner: 8, Top: 8, Right: 10, Bottom: 8, Left: 10},
		Snap: SnapConfig{
			EdgeBand:     32,
			CornerReach:  150,
			ExitSlack:    16,
			RepeatRatios: []float64{0.5, 0.666667, 0.333333},
		},
		Bindings:  BindingsConfig{TopCenter: "zoom", BottomCenter: bindingReserved},
		Shortcuts: map[string]string{},
	}
}

func validateConfig(config Config) error {
	if err := validateBehaviorConfig(config); err != nil {
		return err
	}
	if err := validateGeometryConfig(config); err != nil {
		return err
	}
	return validateBindingsConfig(config)
}

func validateBehaviorConfig(config Config) error {
	if config.NewWindowPolicy != NewWindowFloating && config.NewWindowPolicy != NewWindowInsert {
		return fmt.Errorf("new window policy %q: %w", config.NewWindowPolicy, ErrInvalidCommand)
	}
	if config.SmallViewportPolicy != SmallViewportStack && config.SmallViewportPolicy != SmallViewportReject {
		return fmt.Errorf("small viewport policy %q: %w", config.SmallViewportPolicy, ErrInvalidCommand)
	}
	if config.FocusPolicy != FocusClickDirectional && config.FocusPolicy != FocusDirectional {
		return fmt.Errorf("focus policy %q: %w", config.FocusPolicy, ErrInvalidCommand)
	}
	if config.DragAwayPolicy != DragAwayWindow && config.DragAwayPolicy != DragAwayGroup {
		return fmt.Errorf("drag away policy %q: %w", config.DragAwayPolicy, ErrInvalidCommand)
	}
	if config.HistoryLimit <= 0 || config.HistoryLimit > 500 {
		return fmt.Errorf("history limit %d must be between 1 and 500: %w", config.HistoryLimit, ErrInvalidCommand)
	}
	if config.DesktopTransition != DesktopTransitionSlide && config.DesktopTransition != DesktopTransitionCrossfade &&
		config.DesktopTransition != DesktopTransitionInstant {
		return fmt.Errorf("desktop transition %q: %w", config.DesktopTransition, ErrInvalidCommand)
	}
	if err := validateDragModifier("group move modifier", config.GroupMoveModifier); err != nil {
		return err
	}
	return validateDragModifier("swap modifier", config.SwapModifier)
}

func validateDragModifier(label, modifier string) error {
	switch modifier {
	case dragModifierAlt, dragModifierControl, dragModifierMeta, dragModifierShift, configValueNone:
		return nil
	default:
		return fmt.Errorf("%s %q: %w", label, modifier, ErrInvalidCommand)
	}
}

func validateGeometryConfig(config Config) error {
	gaps := []float64{
		config.Gaps.Inner,
		config.Gaps.Top,
		config.Gaps.Right,
		config.Gaps.Bottom,
		config.Gaps.Left,
	}
	for _, gap := range gaps {
		if !finite(gap) || gap < 0 {
			return fmt.Errorf("gaps must be finite and non-negative: %w", ErrInvalidCommand)
		}
	}
	if !finite(config.Snap.EdgeBand) || config.Snap.EdgeBand <= 0 || !finite(config.Snap.CornerReach) ||
		config.Snap.CornerReach <= 0 ||
		!finite(config.Snap.ExitSlack) ||
		config.Snap.ExitSlack < 0 {
		return fmt.Errorf("snap thresholds are invalid: %w", ErrInvalidCommand)
	}
	if len(config.Snap.RepeatRatios) == 0 {
		return fmt.Errorf("repeat ratios are required: %w", ErrInvalidCommand)
	}
	seen := make(map[float64]struct{}, len(config.Snap.RepeatRatios))
	for _, ratio := range config.Snap.RepeatRatios {
		if !finite(ratio) || ratio < 0.1 || ratio > 0.9 {
			return fmt.Errorf("repeat ratio %v is invalid: %w", ratio, ErrInvalidCommand)
		}
		if _, exists := seen[ratio]; exists {
			return fmt.Errorf("repeat ratio %v is duplicated: %w", ratio, ErrInvalidCommand)
		}
		seen[ratio] = struct{}{}
	}
	return nil
}

func validateBindingsConfig(config Config) error {
	for _, binding := range []string{config.Bindings.TopCenter, config.Bindings.BottomCenter} {
		switch binding {
		case configValueNone, bindingReserved, "zoom":
		default:
			return fmt.Errorf("binding %q: %w", binding, ErrInvalidCommand)
		}
	}
	_, err := CanonicalShortcuts(config.Shortcuts)
	return err
}

func canonicalConfig(config Config) (Config, error) {
	result := cloneConfig(config)
	shortcuts, err := CanonicalShortcuts(result.Shortcuts)
	if err != nil {
		return Config{}, err
	}
	result.Shortcuts = shortcuts
	if err := validateConfig(result); err != nil {
		return Config{}, err
	}
	return result, nil
}

func effectiveConfig(defaults Config, overrides WorkspaceConfig) (Config, error) {
	result := cloneConfig(defaults)
	if overrides.NewWindowPolicy != nil {
		result.NewWindowPolicy = *overrides.NewWindowPolicy
	}
	if overrides.SmallViewportPolicy != nil {
		result.SmallViewportPolicy = *overrides.SmallViewportPolicy
	}
	if overrides.FocusPolicy != nil {
		result.FocusPolicy = *overrides.FocusPolicy
	}
	if overrides.FocusWrap != nil {
		result.FocusWrap = *overrides.FocusWrap
	}
	if overrides.FocusFollowsPointer != nil {
		result.FocusFollowsPointer = *overrides.FocusFollowsPointer
	}
	if overrides.RaiseOnFocus != nil {
		result.RaiseOnFocus = *overrides.RaiseOnFocus
	}
	if overrides.DragAwayPolicy != nil {
		result.DragAwayPolicy = *overrides.DragAwayPolicy
	}
	if overrides.GroupMoveModifier != nil {
		result.GroupMoveModifier = *overrides.GroupMoveModifier
	}
	if overrides.SwapModifier != nil {
		result.SwapModifier = *overrides.SwapModifier
	}
	if overrides.HistoryLimit != nil {
		result.HistoryLimit = *overrides.HistoryLimit
	}
	if overrides.DesktopTransition != nil {
		result.DesktopTransition = *overrides.DesktopTransition
	}
	if overrides.Gaps != nil {
		result.Gaps = *overrides.Gaps
	}
	if overrides.Snap != nil {
		result.Snap = cloneSnapConfig(*overrides.Snap)
	}
	if overrides.Bindings != nil {
		result.Bindings = *overrides.Bindings
	}
	if overrides.Shortcuts != nil {
		result.Shortcuts = cloneStringMap(overrides.Shortcuts)
	}
	return canonicalConfig(result)
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

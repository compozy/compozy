package contract

type SettingsWindowNewPolicy string

const (
	SettingsWindowNewPolicyFloating    SettingsWindowNewPolicy = "floating"
	SettingsWindowNewPolicyBesideFocus SettingsWindowNewPolicy = "beside_focus"
)

type SettingsWindowSmallViewportPolicy string

const (
	SettingsWindowSmallViewportPolicyStack  SettingsWindowSmallViewportPolicy = "stack"
	SettingsWindowSmallViewportPolicyReject SettingsWindowSmallViewportPolicy = "reject"
)

type SettingsWindowFocusPolicy string

const (
	SettingsWindowFocusPolicyClickDirectional SettingsWindowFocusPolicy = "click_directional"
	SettingsWindowFocusPolicyDirectional      SettingsWindowFocusPolicy = "directional"
)

type SettingsWindowDragAwayPolicy string

const (
	SettingsWindowDragAwayPolicyWindow SettingsWindowDragAwayPolicy = "window"
	SettingsWindowDragAwayPolicyGroup  SettingsWindowDragAwayPolicy = "group"
)

type SettingsWindowDragModifier string

const (
	SettingsWindowDragModifierAlt     SettingsWindowDragModifier = "alt"
	SettingsWindowDragModifierControl SettingsWindowDragModifier = "control"
	SettingsWindowDragModifierMeta    SettingsWindowDragModifier = "meta"
	SettingsWindowDragModifierShift   SettingsWindowDragModifier = "shift"
	SettingsWindowDragModifierNone    SettingsWindowDragModifier = "none"
)

type SettingsWindowDesktopTransition string

const (
	SettingsWindowDesktopTransitionSlide     SettingsWindowDesktopTransition = "slide"
	SettingsWindowDesktopTransitionCrossfade SettingsWindowDesktopTransition = "crossfade"
	SettingsWindowDesktopTransitionInstant   SettingsWindowDesktopTransition = "instant"
)

type SettingsWindowBindingAction string

const (
	SettingsWindowBindingActionNone     SettingsWindowBindingAction = "none"
	SettingsWindowBindingActionReserved SettingsWindowBindingAction = "reserved"
	SettingsWindowBindingActionZoom     SettingsWindowBindingAction = "zoom"
)

type SettingsWindowManagerConfigPayload struct {
	NewWindowPolicy     SettingsWindowNewPolicy             `json:"new_window_policy"`
	SmallViewportPolicy SettingsWindowSmallViewportPolicy   `json:"small_viewport_policy"`
	FocusPolicy         SettingsWindowFocusPolicy           `json:"focus_policy"`
	FocusWrap           bool                                `json:"focus_wrap"`
	FocusFollowsPointer bool                                `json:"focus_follows_pointer"`
	RaiseOnFocus        bool                                `json:"raise_on_focus"`
	DragAwayPolicy      SettingsWindowDragAwayPolicy        `json:"drag_away_policy"`
	GroupMoveModifier   SettingsWindowDragModifier          `json:"group_move_modifier"`
	SwapModifier        SettingsWindowDragModifier          `json:"swap_modifier"`
	HistoryLimit        int                                 `json:"history_limit"`
	DesktopTransition   SettingsWindowDesktopTransition     `json:"desktop_transition"`
	Gaps                SettingsWindowManagerGapsPayload    `json:"gaps"`
	Snap                SettingsWindowManagerSnapPayload    `json:"snap"`
	Bindings            SettingsWindowManagerBindingPayload `json:"bindings"`
	Shortcuts           map[string]string                   `json:"shortcuts"`
}

type SettingsWindowManagerGapsPayload struct {
	Inner  int `json:"inner"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
}

type SettingsWindowManagerSnapPayload struct {
	EdgeBand     int       `json:"edge_band"`
	CornerReach  int       `json:"corner_reach"`
	ExitSlack    int       `json:"exit_slack"`
	RepeatRatios []float64 `json:"repeat_ratios"`
}

type SettingsWindowManagerBindingPayload struct {
	TopCenter    SettingsWindowBindingAction `json:"top_center"`
	BottomCenter SettingsWindowBindingAction `json:"bottom_center"`
}

type UpdateSettingsWindowManagerRequest struct {
	Config SettingsWindowManagerConfigPayload `json:"config"`
}

type SettingsWindowManagerResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Config SettingsWindowManagerConfigPayload `json:"config"`
}

package contract

import "github.com/compozy/compozy/internal/windowmanager"

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
	NewWindowPolicy     SettingsWindowNewPolicy                  `json:"new_window_policy"`
	SmallViewportPolicy SettingsWindowSmallViewportPolicy        `json:"small_viewport_policy"`
	FocusPolicy         SettingsWindowFocusPolicy                `json:"focus_policy"`
	FocusWrap           bool                                     `json:"focus_wrap"`
	FocusFollowsPointer bool                                     `json:"focus_follows_pointer"`
	RaiseOnFocus        bool                                     `json:"raise_on_focus"`
	DragAwayPolicy      SettingsWindowDragAwayPolicy             `json:"drag_away_policy"`
	GroupMoveModifier   SettingsWindowDragModifier               `json:"group_move_modifier"`
	SwapModifier        SettingsWindowDragModifier               `json:"swap_modifier"`
	HistoryLimit        int                                      `json:"history_limit"`
	NavStackLimit       int                                      `json:"nav_stack_limit"`
	ClosedEntryLimit    int                                      `json:"closed_entry_limit"`
	DesktopTransition   SettingsWindowDesktopTransition          `json:"desktop_transition"`
	Gaps                SettingsWindowManagerGapsPayload         `json:"gaps"`
	Snap                SettingsWindowManagerSnapPayload         `json:"snap"`
	Bindings            SettingsWindowManagerBindingPayload      `json:"bindings"`
	Shortcuts           map[string]windowmanager.ShortcutBinding `json:"shortcuts"`
	GlobalShortcuts     map[string]string                        `json:"global_shortcuts"`
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
	Config          *SettingsWindowManagerConfigPayload       `json:"config,omitempty"`
	Shortcuts       *map[string]windowmanager.ShortcutBinding `json:"shortcuts,omitempty"`
	Aliases         *map[string]string                        `json:"aliases,omitempty"`
	GlobalShortcuts *map[string]string                        `json:"global_shortcuts,omitempty"`
	Overwrite       bool                                      `json:"overwrite,omitempty"`
}

type SettingsWindowManagerResponse struct {
	SettingsGlobalWorkspaceSectionResponseMetaPayload
	Config             SettingsWindowManagerConfigPayload       `json:"config"`
	Defaults           map[string]windowmanager.ShortcutBinding `json:"defaults"`
	EffectiveShortcuts map[string]windowmanager.ShortcutBinding `json:"effective_shortcuts"`
	Aliases            map[string]string                        `json:"aliases"`
	Commands           []SettingsWindowManagerCommandPayload    `json:"commands"`
	ExtensionDefaults  []SettingsWindowManagerDefaultPayload    `json:"extension_defaults"`
	Diagnostics        []SettingsWindowManagerDiagnosticPayload `json:"diagnostics"`
	GlobalShortcuts    []SettingsGlobalShortcutPayload          `json:"global_shortcuts"`
}

type SettingsGlobalShortcutPayload struct {
	CommandID     string                       `json:"command_id"`
	IntendedChord string                       `json:"intended_chord"`
	ActiveChord   string                       `json:"active_chord,omitempty"`
	Status        SettingsGlobalShortcutStatus `json:"status,omitempty"`
	Reason        string                       `json:"reason,omitempty"`
	SettingsURL   string                       `json:"settings_url,omitempty"`
}

type SettingsGlobalShortcutStatus string

const (
	SettingsGlobalShortcutRegistered       SettingsGlobalShortcutStatus = "registered"
	SettingsGlobalShortcutFailedInUse      SettingsGlobalShortcutStatus = "failed_in_use"
	SettingsGlobalShortcutFailedPermission SettingsGlobalShortcutStatus = "failed_permission"
	SettingsGlobalShortcutUnsupported      SettingsGlobalShortcutStatus = "unsupported"
)

type SettingsWindowManagerCommandPayload struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Section string `json:"section"`
	Source  string `json:"source"`
}

type SettingsWindowManagerDefaultPayload struct {
	CommandID    string                        `json:"command"`
	Binding      windowmanager.ShortcutBinding `json:"binding"`
	Dormant      bool                          `json:"dormant"`
	ConflictWith string                        `json:"conflict_with,omitempty"`
}

type SettingsWindowManagerDiagnosticPayload struct {
	CommandID string `json:"command_id"`
	Message   string `json:"message"`
}

type SettingsWindowManagerMutationError struct {
	Error   string `json:"error"`
	Owner   string `json:"owner,omitempty"`
	Chord   string `json:"chord,omitempty"`
	Alias   string `json:"alias,omitempty"`
	Message string `json:"message,omitempty"`
}

package contract

// SettingsShellSessionSort identifies the operator's preferred session ordering.
type SettingsShellSessionSort string

const (
	SettingsShellSessionSortCreated      SettingsShellSessionSort = "created"
	SettingsShellSessionSortName         SettingsShellSessionSort = "name"
	SettingsShellSessionSortLastActivity SettingsShellSessionSort = "last_activity"
	SettingsShellSessionSortAttention    SettingsShellSessionSort = "attention"
	SettingsShellSessionSortManual       SettingsShellSessionSort = "manual"
)

// SettingsShellSessionScope identifies the operator's preferred session list breadth.
type SettingsShellSessionScope string

const (
	SettingsShellSessionScopeWorkspace     SettingsShellSessionScope = "workspace"
	SettingsShellSessionScopeAllWorkspaces SettingsShellSessionScope = "all-workspaces"
)

// SettingsShellSessionsPayload carries persistent session-list preferences.
type SettingsShellSessionsPayload struct {
	GroupBy SettingsShellSessionGroupBy `json:"group_by,omitempty"`
	Sort    SettingsShellSessionSort    `json:"sort,omitempty"`
	Scope   SettingsShellSessionScope   `json:"scope,omitempty"`
}

// SettingsShellTextSize is a named step that scales every shell text size together.
type SettingsShellTextSize string

const (
	SettingsShellTextSizeSmall   SettingsShellTextSize = "small"
	SettingsShellTextSizeDefault SettingsShellTextSize = "default"
	SettingsShellTextSizeLarge   SettingsShellTextSize = "large"
	SettingsShellTextSizeLarger  SettingsShellTextSize = "larger"
)

// SettingsShellLetterSpacing is a named step for the interface text tracking.
type SettingsShellLetterSpacing string

const (
	SettingsShellLetterSpacingTight   SettingsShellLetterSpacing = "tight"
	SettingsShellLetterSpacingDefault SettingsShellLetterSpacing = "default"
	SettingsShellLetterSpacingRelaxed SettingsShellLetterSpacing = "relaxed"
	SettingsShellLetterSpacingWide    SettingsShellLetterSpacing = "wide"
)

// SettingsShellAppearancePayload carries shell text preferences: font families,
// text size and letter spacing. On update, an omitted field preserves the stored
// value; a "" family and the "default" size or spacing reset it to the default.
// Responses always populate every field.
type SettingsShellAppearancePayload struct {
	UIFont        *string                     `json:"ui_font,omitempty"`
	MonoFont      *string                     `json:"mono_font,omitempty"`
	TextSize      *SettingsShellTextSize      `json:"text_size,omitempty"`
	LetterSpacing *SettingsShellLetterSpacing `json:"letter_spacing,omitempty"`
}

// SettingsShellPayload is the stable operator shell settings contract.
type SettingsShellPayload struct {
	Sessions   SettingsShellSessionsPayload    `json:"sessions"`
	Appearance *SettingsShellAppearancePayload `json:"appearance,omitempty"`
}

// UpdateSettingsShellRequest updates supplied shell preferences and preserves omitted fields.
type UpdateSettingsShellRequest struct {
	Config SettingsShellPayload `json:"config"`
}

// SettingsShellResponse returns the effective global shell settings.
type SettingsShellResponse struct {
	Warnings    []string                        `json:"warnings"`
	LegacyScope SettingsShellLegacyScopePayload `json:"legacy_scope"`
	SettingsUserSectionResponseMetaPayload
	Config SettingsShellPayload `json:"config"`
}

type SettingsShellSessionGroupBy string

const (
	SettingsShellSessionGroupByNone    SettingsShellSessionGroupBy = "none"
	SettingsShellSessionGroupByProject SettingsShellSessionGroupBy = "project"
	SettingsShellSessionGroupByProfile SettingsShellSessionGroupBy = "profile"
)

type SettingsShellLegacyScopePayload struct {
	Value    SettingsShellSessionScope `json:"value"`
	Explicit bool                      `json:"explicit"`
	Revision uint64                    `json:"revision"`
}

// SettingsShellMutationResponse preserves apply receipts and returns the updated preferences.
type SettingsShellMutationResponse struct {
	SettingsApplyResponse
	Config      SettingsShellPayload            `json:"config"`
	LegacyScope SettingsShellLegacyScopePayload `json:"legacy_scope"`
}

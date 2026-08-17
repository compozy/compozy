package contract

// SettingsShellSessionSort identifies the operator's preferred session ordering.
type SettingsShellSessionSort string

const (
	SettingsShellSessionSortLastActivity SettingsShellSessionSort = "last_activity"
	SettingsShellSessionSortAttention    SettingsShellSessionSort = "attention"
)

// SettingsShellSessionScope identifies the operator's preferred session list breadth.
type SettingsShellSessionScope string

const (
	SettingsShellSessionScopeWorkspace     SettingsShellSessionScope = "workspace"
	SettingsShellSessionScopeAllWorkspaces SettingsShellSessionScope = "all-workspaces"
)

// SettingsShellSessionsPayload carries persistent session-list preferences.
type SettingsShellSessionsPayload struct {
	Sort  SettingsShellSessionSort  `json:"sort"`
	Scope SettingsShellSessionScope `json:"scope"`
}

// SettingsShellPayload is the stable operator shell settings contract.
type SettingsShellPayload struct {
	Sessions SettingsShellSessionsPayload `json:"sessions"`
}

// UpdateSettingsShellRequest replaces the complete shell section.
type UpdateSettingsShellRequest struct {
	Config SettingsShellPayload `json:"config"`
}

// SettingsShellResponse returns the effective global shell settings.
type SettingsShellResponse struct {
	SettingsGlobalSectionResponseMetaPayload
	Config SettingsShellPayload `json:"config"`
}

package contract

type UpdateSettingsCmdPaletteRequest struct {
	FallbackAgentEnabled *bool `json:"fallback_agent_enabled,omitempty"`
	Personalization      *bool `json:"personalization,omitempty"`
}

type SettingsCmdPaletteResponse struct {
	SettingsGlobalWorkspaceSectionResponseMetaPayload
	FallbackAgentEnabled bool `json:"fallback_agent_enabled"`
	Personalization      bool `json:"personalization"`
}

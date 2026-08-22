package contract

type UpdateSettingsCmdPaletteRequest struct {
	FallbackAgentEnabled *bool              `json:"fallback_agent_enabled,omitempty"`
	Personalization      *bool              `json:"personalization,omitempty"`
	Aliases              *map[string]string `json:"aliases,omitempty"`
}

type SettingsCmdPaletteResponse struct {
	SettingsLayeredSectionResponseMetaPayload
	FallbackAgentEnabled bool              `json:"fallback_agent_enabled"`
	Personalization      bool              `json:"personalization"`
	Aliases              map[string]string `json:"aliases"`
}

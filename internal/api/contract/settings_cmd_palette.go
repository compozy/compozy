package contract

type UpdateSettingsCmdPaletteRequest struct {
	FallbackAgentEnabled *bool              `json:"fallback_agent_enabled,omitzero"`
	Personalization      *bool              `json:"personalization,omitzero"`
	Aliases              *map[string]string `json:"aliases,omitzero"`
}

type SettingsCmdPaletteResponse struct {
	SettingsLayeredSectionResponseMetaPayload
	FallbackAgentEnabled bool              `json:"fallback_agent_enabled"`
	Personalization      bool              `json:"personalization"`
	Aliases              map[string]string `json:"aliases"`
}

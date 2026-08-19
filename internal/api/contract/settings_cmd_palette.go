package contract

type UpdateSettingsCmdPaletteRequest struct {
	Personalization *bool `json:"personalization"`
}

type SettingsCmdPaletteResponse struct {
	SettingsGlobalWorkspaceSectionResponseMetaPayload
	Personalization bool `json:"personalization"`
}

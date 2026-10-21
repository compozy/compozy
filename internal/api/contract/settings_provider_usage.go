package contract

type SettingsProviderUsageProviderPayload struct {
	Enabled        *bool `json:"enabled,omitempty"`
	KeychainAccess *bool `json:"keychain_access,omitempty"`
}

type SettingsProviderUsagePayload struct {
	Enabled         bool                                            `json:"enabled"`
	RefreshInterval string                                          `json:"refresh_interval"`
	Providers       map[string]SettingsProviderUsageProviderPayload `json:"providers"`
}

type SettingsProviderUsageUpdateRequest struct {
	Config SettingsProviderUsagePayload `json:"config"`
}

type SettingsProviderUsageResponse struct {
	SettingsUserSectionResponseMetaPayload
	Config SettingsProviderUsagePayload `json:"config"`
}

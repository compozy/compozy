package contract

// ProviderUsageCollectionProfileQuery selects the login to collect after a user-scoped consent change.
const ProviderUsageCollectionProfileQuery = "collection_profile"

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
	KeychainSupported bool `json:"keychain_supported"`
	SettingsUserSectionResponseMetaPayload
	Config SettingsProviderUsagePayload `json:"config"`
}

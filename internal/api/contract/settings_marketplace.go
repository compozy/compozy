package contract

type SettingsMarketplaceCatalogPayload struct {
	BaseURL string `json:"base_url"`
	TTL     string `json:"ttl"`
	Timeout string `json:"timeout"`
}

type SettingsMarketplaceResponse struct {
	SettingsUserSectionResponseMetaPayload
	Config SettingsMarketplaceCatalogPayload `json:"config"`
}

type UpdateSettingsMarketplaceRequest struct {
	Config SettingsMarketplaceCatalogPayload `json:"config"`
}

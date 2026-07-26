package config

type extensionsMarketplaceOverlay struct {
	Registry        *string `toml:"registry"`
	BaseURL         *string `toml:"base_url"`
	AllowUnverified *bool   `toml:"allow_unverified"`
}

func (o extensionsMarketplaceOverlay) Apply(dst *ExtensionsMarketplaceConfig) {
	if o.Registry != nil {
		dst.Registry = *o.Registry
	}
	if o.BaseURL != nil {
		dst.BaseURL = *o.BaseURL
	}
	if o.AllowUnverified != nil {
		dst.AllowUnverified = *o.AllowUnverified
	}
}

package config

// ExtensionsMarketplaceConfig controls extension registry access and trust policy.
type ExtensionsMarketplaceConfig struct {
	Registry        string `toml:"registry"`
	BaseURL         string `toml:"base_url,omitempty"`
	AllowUnverified bool   `toml:"allow_unverified,omitempty"`
}

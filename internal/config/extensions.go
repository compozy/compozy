package config

import "time"

// ExtensionsTrustConfig controls operator policy for unverified published extensions.
type ExtensionsTrustConfig struct {
	AllowUnverified bool `toml:"allow_unverified"`
}

// ExtensionsSourcesConfig controls published extension source availability.
type ExtensionsSourcesConfig struct {
	GitHub ExtensionsGitHubSourceConfig `toml:"github"`
	Git    ExtensionsGitSourceConfig    `toml:"git"`
}

// ExtensionsGitHubSourceConfig controls the GitHub extension source.
type ExtensionsGitHubSourceConfig struct {
	Enabled bool   `toml:"enabled"`
	BaseURL string `toml:"base_url"`
}

// ExtensionsGitSourceConfig controls direct Git extension installs.
type ExtensionsGitSourceConfig struct {
	Enabled bool `toml:"enabled"`
}

// ExtensionsDevConfig controls the CLI-side extension development loop.
type ExtensionsDevConfig struct {
	WatchInterval time.Duration `toml:"watch_interval"`
}

func defaultExtensionsConfig() ExtensionsConfig {
	return ExtensionsConfig{
		Trust: ExtensionsTrustConfig{AllowUnverified: true},
		Sources: ExtensionsSourcesConfig{
			GitHub: ExtensionsGitHubSourceConfig{
				Enabled: true,
				BaseURL: "https://api.github.com",
			},
			Git: ExtensionsGitSourceConfig{Enabled: true},
		},
		Dev: ExtensionsDevConfig{WatchInterval: 2 * time.Second},
	}
}

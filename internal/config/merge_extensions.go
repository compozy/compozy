package config

import "time"

type extensionsTrustOverlay struct {
	AllowUnverified *bool `toml:"allow_unverified"`
}

type extensionsSourcesOverlay struct {
	GitHub extensionsGitHubSourceOverlay `toml:"github"`
	Git    extensionsGitSourceOverlay    `toml:"git"`
}

type extensionsGitHubSourceOverlay struct {
	Enabled *bool   `toml:"enabled"`
	BaseURL *string `toml:"base_url"`
}

type extensionsGitSourceOverlay struct {
	Enabled *bool `toml:"enabled"`
}

type extensionsDevOverlay struct {
	WatchInterval *time.Duration `toml:"watch_interval"`
}

func (o extensionsTrustOverlay) Apply(dst *ExtensionsTrustConfig) {
	if o.AllowUnverified != nil {
		dst.AllowUnverified = *o.AllowUnverified
	}
}

func (o extensionsSourcesOverlay) Apply(dst *ExtensionsSourcesConfig) {
	o.GitHub.Apply(&dst.GitHub)
	o.Git.Apply(&dst.Git)
}

func (o extensionsGitHubSourceOverlay) Apply(dst *ExtensionsGitHubSourceConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.BaseURL != nil {
		dst.BaseURL = *o.BaseURL
	}
}

func (o extensionsGitSourceOverlay) Apply(dst *ExtensionsGitSourceConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
}

func (o extensionsDevOverlay) Apply(dst *ExtensionsDevConfig) {
	if o.WatchInterval != nil {
		dst.WatchInterval = *o.WatchInterval
	}
}

package config

import "time"

type mcpOverlay struct {
	OAuth              mcpOAuthGlobalOverlay `toml:"oauth"`
	Stdio              mcpStdioOverlay       `toml:"stdio"`
	StatusProbeTimeout *time.Duration        `toml:"status_probe_timeout"`
}

type mcpStdioOverlay struct {
	IdleTimeout        *time.Duration `toml:"idle_timeout"`
	ConnectTimeout     *time.Duration `toml:"connect_timeout"`
	MaxConcurrentCalls *int           `toml:"max_concurrent_calls"`
}

type mcpOAuthGlobalOverlay struct {
	ClientMetadataURL *string `toml:"client_metadata_url"`
	RedirectURI       *string `toml:"redirect_uri"`
}

func (o mcpOverlay) Apply(dst *MCPConfig) {
	o.OAuth.Apply(&dst.OAuth)
	o.Stdio.Apply(&dst.Stdio)
	if o.StatusProbeTimeout != nil {
		dst.StatusProbeTimeout = *o.StatusProbeTimeout
	}
}

func (o mcpStdioOverlay) Apply(dst *MCPStdioConfig) {
	if o.IdleTimeout != nil {
		dst.IdleTimeout = *o.IdleTimeout
	}
	if o.ConnectTimeout != nil {
		dst.ConnectTimeout = *o.ConnectTimeout
	}
	if o.MaxConcurrentCalls != nil {
		dst.MaxConcurrentCalls = *o.MaxConcurrentCalls
	}
}

func (o mcpOAuthGlobalOverlay) Apply(dst *MCPOAuthConfig) {
	if o.ClientMetadataURL != nil {
		dst.ClientMetadataURL = *o.ClientMetadataURL
	}
	if o.RedirectURI != nil {
		dst.RedirectURI = *o.RedirectURI
	}
}

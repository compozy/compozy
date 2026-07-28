package config

import "time"

type toolsOverlay struct {
	Enabled               *bool                 `toml:"enabled"`
	HostedMCPEnabled      *bool                 `toml:"hosted_mcp_enabled"`
	DefaultMaxResultBytes *int64                `toml:"default_max_result_bytes"`
	HostedMCP             toolsHostedMCPOverlay `toml:"hosted_mcp"`
	Clarify               toolsClarifyOverlay   `toml:"clarify"`
	Artifacts             toolsArtifactsOverlay `toml:"artifacts"`
	Policy                toolsPolicyOverlay    `toml:"policy"`
}

type toolsHostedMCPOverlay struct {
	BindNonceTTLSeconds *int `toml:"bind_nonce_ttl_seconds"`
}

type toolsClarifyOverlay struct {
	Timeout *time.Duration `toml:"timeout"`
}

type toolsArtifactsOverlay struct {
	MaxCount *int           `toml:"max_count"`
	MaxBytes *int64         `toml:"max_bytes"`
	MaxAge   *time.Duration `toml:"max_age"`
}

type toolsPolicyOverlay struct {
	ExternalDefault        *ToolsExternalDefault `toml:"external_default"`
	ApprovalTimeoutSeconds *int                  `toml:"approval_timeout_seconds"`
	TrustedSources         *[]string             `toml:"trusted_sources"`
}

func (o toolsOverlay) Apply(dst *ToolsConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.HostedMCPEnabled != nil {
		dst.HostedMCPEnabled = *o.HostedMCPEnabled
	}
	if o.DefaultMaxResultBytes != nil {
		dst.DefaultMaxResultBytes = *o.DefaultMaxResultBytes
	}
	o.HostedMCP.Apply(&dst.HostedMCP)
	o.Clarify.Apply(&dst.Clarify)
	o.Artifacts.Apply(&dst.Artifacts)
	o.Policy.Apply(&dst.Policy)
}

func (o toolsHostedMCPOverlay) Apply(dst *ToolsHostedMCPConfig) {
	if o.BindNonceTTLSeconds != nil {
		dst.BindNonceTTLSeconds = *o.BindNonceTTLSeconds
	}
}

func (o toolsClarifyOverlay) Apply(dst *ToolsClarifyConfig) {
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
}

func (o toolsArtifactsOverlay) Apply(dst *ToolsArtifactsConfig) {
	if o.MaxCount != nil {
		dst.MaxCount = *o.MaxCount
	}
	if o.MaxBytes != nil {
		dst.MaxBytes = *o.MaxBytes
	}
	if o.MaxAge != nil {
		dst.MaxAge = *o.MaxAge
	}
}

func (o toolsPolicyOverlay) Apply(dst *ToolsPolicyConfig) {
	if o.ExternalDefault != nil {
		dst.ExternalDefault = *o.ExternalDefault
	}
	if o.ApprovalTimeoutSeconds != nil {
		dst.ApprovalTimeoutSeconds = *o.ApprovalTimeoutSeconds
	}
	if o.TrustedSources != nil {
		dst.TrustedSources = append([]string(nil), (*o.TrustedSources)...)
	}
}

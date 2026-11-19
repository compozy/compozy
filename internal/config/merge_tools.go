package config

import (
	"slices"
	"time"
)

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
	ToolListThreshold   *int `toml:"tool_list_threshold"`
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
	if o.ToolListThreshold != nil {
		dst.ToolListThreshold = *o.ToolListThreshold
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

func (o *toolsPolicyOverlay) restrictTo(inherited ToolsPolicyConfig) {
	if o.ExternalDefault != nil {
		loosens := (*o.ExternalDefault == ToolsExternalDefaultEnabled &&
			inherited.ExternalDefault != ToolsExternalDefaultEnabled) ||
			(*o.ExternalDefault == ToolsExternalDefaultAsk &&
				inherited.ExternalDefault == ToolsExternalDefaultDisabled)
		if loosens {
			warnWorkspaceToolConsentPolicy()
			o.ExternalDefault = nil
		}
	}
	if o.TrustedSources != nil {
		trusted := make([]string, 0, len(*o.TrustedSources))
		for _, source := range *o.TrustedSources {
			if !slices.Contains(inherited.TrustedSources, source) {
				warnWorkspaceToolConsentPolicy()
				continue
			}
			trusted = append(trusted, source)
		}
		o.TrustedSources = &trusted
	}
}

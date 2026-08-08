package gateway

import (
	"context"
	"strings"
	"time"
)

const ProviderInstallSourceBundled = "bundled"

// ProviderTrust is re-derived from the live extension artifact before every reachability attempt.
type ProviderTrust struct {
	InstallSource   string
	ControlDigest   string
	ConfirmedDigest string
	ConfirmedBy     string
	ConfirmedAt     time.Time
	ChannelScopes   []string
}

// ProviderChannelScope returns the manifest scope that authorizes one gateway tier.
func ProviderChannelScope(tier Tier) string {
	return "gateway." + strings.TrimSpace(string(tier))
}

// ProviderTrustResolver derives source and control consent from the live extension registry.
type ProviderTrustResolver interface {
	ResolveProviderTrust(context.Context, string) (ProviderTrust, error)
}

// ProviderIdentityStore reads the separate gateway consent row without trusting it as live evidence.
type ProviderIdentityStore interface {
	ProviderIdentity(context.Context, string) (ProviderIdentity, error)
}

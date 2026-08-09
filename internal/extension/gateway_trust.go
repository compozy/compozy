package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/gateway"
)

// ConnectivityProviderTrustResolver derives provider authority from the live global registry.
type ConnectivityProviderTrustResolver struct {
	registry *Registry
}

// NewConnectivityProviderTrustResolver constructs the gateway-facing live trust adapter.
func NewConnectivityProviderTrustResolver(registry *Registry) (*ConnectivityProviderTrustResolver, error) {
	if registry == nil {
		return nil, errors.New("extension: connectivity trust registry is required")
	}
	return &ConnectivityProviderTrustResolver{registry: registry}, nil
}

// ResolveProviderTrust reloads the installed artifact digest on every call.
func (r *ConnectivityProviderTrustResolver) ResolveProviderTrust(
	ctx context.Context,
	name string,
) (gateway.ProviderTrust, error) {
	if ctx == nil {
		return gateway.ProviderTrust{}, errors.New("extension: connectivity trust context is required")
	}
	if err := ctx.Err(); err != nil {
		return gateway.ProviderTrust{}, err
	}
	if r == nil || r.registry == nil {
		return gateway.ProviderTrust{}, errors.New("extension: connectivity trust resolver is required")
	}
	trimmed := strings.TrimSpace(name)
	info, err := r.registry.Get(trimmed)
	if err != nil {
		return gateway.ProviderTrust{}, fmt.Errorf("extension: resolve connectivity provider %q: %w", trimmed, err)
	}
	manifest, _, checksum, err := loadVerifiedInstallManifest(
		&Manifest{Name: info.Name, Version: info.Version},
		info.ManifestPath,
		info.Checksum,
	)
	if err != nil {
		return gateway.ProviderTrust{}, fmt.Errorf("extension: verify live connectivity provider %q: %w", trimmed, err)
	}
	if checksum != info.Checksum {
		return gateway.ProviderTrust{}, fmt.Errorf("%w: live provider checksum changed", gateway.ErrProviderTrustStale)
	}
	if !providesCapability(manifest.Capabilities.Provides, extensionprotocol.CapabilityProvideConnectivityProvider) {
		return gateway.ProviderTrust{}, fmt.Errorf(
			"extension: provider %q does not declare connectivity.provider",
			trimmed,
		)
	}
	if info.Source == SourceWorkspace {
		return gateway.ProviderTrust{}, fmt.Errorf(
			"%w: workspace-scoped connectivity providers are forbidden",
			gateway.ErrExposureRefused,
		)
	}
	liveRequirement := manifest.NetworkParticipation.Normalize()
	liveDigest, err := NetworkParticipationRequirementDigest(liveRequirement)
	if err != nil {
		return gateway.ProviderTrust{}, fmt.Errorf("extension: derive live connectivity control digest: %w", err)
	}
	if liveDigest == "" || liveDigest != info.NetworkRequirementDigest {
		return gateway.ProviderTrust{}, fmt.Errorf(
			"%w: live provider control digest changed",
			gateway.ErrProviderTrustStale,
		)
	}
	confirmation, err := r.registry.NetworkConfirmation(GlobalInstanceKey(trimmed))
	if err != nil {
		return gateway.ProviderTrust{}, fmt.Errorf("extension: derive connectivity control digest: %w", err)
	}
	trust := gateway.ProviderTrust{
		InstallSource: info.Source.String(), ControlDigest: confirmation.Digest,
		ConfirmedBy: confirmation.ConfirmedBy, ConfirmedAt: confirmation.ConfirmedAt,
		ChannelScopes: slices.Clone(liveRequirement.ChannelScopes),
	}
	if confirmation.ConfirmedBy != "" && !confirmation.ConfirmedAt.IsZero() {
		trust.ConfirmedDigest = confirmation.Digest
	}
	return trust, nil
}

var _ gateway.ProviderTrustResolver = (*ConnectivityProviderTrustResolver)(nil)

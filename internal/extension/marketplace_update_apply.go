package extensionpkg

import (
	"context"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func applyResolvedMarketplaceUpdate(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	info ExtensionInfo,
	req MarketplaceUpdateRequest,
	reload MutationReload,
	resolution marketplaceUpdateResolution,
) (marketplaceUpdateApplyResult, error) {
	if err := validateMarketplaceTrustGate(
		dereferenceOptionalString(info.RegistrySlug),
		resolution.registryName,
		req.PolicyAllowsUnverified,
		req.AllowUnverified,
		resolution.trust,
	); err != nil {
		return marketplaceUpdateApplyResult{}, err
	}

	return applyMarketplaceExtensionUpdate(
		ctx,
		homePaths,
		registry,
		resolution.downloader,
		info,
		resolution.latestVersion,
		resolution.registryName,
		req.AllowUnverified,
		req.InstalledBy,
		resolution.trust,
		req.ObserveDigestVerification,
		req.PreflightCandidate,
		req.CommitCandidate,
		reload,
		marketplaceUpdateCleanupForRequest(req),
	)
}

package extensionpkg

import (
	"context"
	"errors"

	registrypkg "github.com/compozy/compozy/internal/registry"
)

func committedMarketplaceUpdateResult(
	cleanup marketplaceUpdateCleanup,
	extensionName string,
	remoteVersion string,
	change *stagedExtensionDirChange,
) marketplaceUpdateApplyResult {
	out := marketplaceUpdateApplyResult{remoteVersion: remoteVersion, committed: true}
	if cleanupErr := cleanup.commitChange(change); cleanupErr != nil {
		out.warnings = append(out.warnings, marketplaceUpdateCleanupWarning(
			extensionName,
			marketplaceUpdateCleanupBackup,
			change.backupDir,
			cleanupErr,
		))
	}
	return out
}

func commitMarketplaceUpdateCandidate(
	ctx context.Context,
	registry LifecycleRegistry,
	info ExtensionInfo,
	installDir string,
	result *registrypkg.InstallResult,
	manifest *Manifest,
	change *stagedExtensionDirChange,
	slug string,
	registryName string,
	latestVersion string,
	allowUnverified bool,
	installedBy string,
	trust *MarketplaceTrustEvidence,
	commitCandidate func(ExtensionInfo, *Manifest) error,
	reload MutationReload,
) (string, error) {
	remoteVersion := firstNonEmpty(result.Version, latestVersion, manifest.Version)
	provenance := marketplaceUpdateProvenance(info, result, manifest, registryName, allowUnverified, installedBy, trust)
	if err := installMarketplaceExtensionUpdateRecord(
		registry,
		manifest,
		installDir,
		result.Checksum,
		slug,
		registryName,
		remoteVersion,
		provenance,
	); err != nil {
		return "", errors.Join(err, change.Rollback())
	}
	if commitCandidate != nil {
		if err := commitCandidate(info, manifest); err != nil {
			return "", errors.Join(err, restoreUpdatedExtensionRecord(registry, info, installDir, change))
		}
	}
	if err := reloadMarketplaceExtensionUpdate(ctx, reload, registry, info, installDir, change); err != nil {
		return "", err
	}
	return remoteVersion, nil
}

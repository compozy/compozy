package extensionpkg

import (
	"context"
	"errors"

	registrypkg "github.com/compozy/compozy/internal/registry"
)

type marketplaceUpdateCommitInput struct {
	registry        LifecycleRegistry
	info            ExtensionInfo
	installDir      string
	result          *registrypkg.InstallResult
	manifest        *Manifest
	change          *stagedExtensionDirChange
	slug            string
	registryName    string
	latestVersion   string
	allowUnverified bool
	installedBy     string
	trust           *MarketplaceTrustEvidence
	commitCandidate MarketplaceUpdateCommit
	reload          MutationReload
}

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
	input *marketplaceUpdateCommitInput,
) (string, error) {
	remoteVersion := firstNonEmpty(input.result.Version, input.latestVersion, input.manifest.Version)
	provenance := marketplaceUpdateProvenance(
		input.info,
		input.result,
		input.manifest,
		input.registryName,
		input.allowUnverified,
		input.installedBy,
		input.trust,
	)
	if err := installMarketplaceExtensionUpdateRecord(
		input.registry,
		input.manifest,
		input.installDir,
		input.result.Checksum,
		input.slug,
		input.registryName,
		remoteVersion,
		provenance,
	); err != nil {
		return "", errors.Join(err, input.change.Rollback())
	}
	if input.commitCandidate != nil {
		if err := input.commitCandidate(input.info, input.manifest); err != nil {
			return "", errors.Join(
				err,
				restoreUpdatedExtensionRecord(input.registry, input.info, input.installDir, input.change),
			)
		}
	}
	if err := reloadMarketplaceExtensionUpdate(
		ctx,
		input.reload,
		input.registry,
		input.info,
		input.installDir,
		input.change,
	); err != nil {
		return "", err
	}
	return remoteVersion, nil
}

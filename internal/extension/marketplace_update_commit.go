package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"time"

	registrypkg "github.com/compozy/compozy/internal/registry"
)

type marketplaceUpdateCommitInput struct {
	registry          LifecycleRegistry
	info              ExtensionInfo
	installDir        string
	result            *registrypkg.InstallResult
	manifest          *Manifest
	change            *stagedExtensionDirChange
	slug              string
	registryName      string
	latestVersion     string
	allowUnverified   bool
	installedBy       string
	trust             *MarketplaceTrustEvidence
	commitCandidate   MarketplaceUpdateCommit
	rollbackCandidate MarketplaceUpdateRollback
	reload            MutationReload
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
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			defer cancel()
			return "", errors.Join(err, restoreMarketplaceUpdateCandidate(
				rollbackCtx, input.registry, input.info, input.installDir, input.change, input.rollbackCandidate,
			))
		}
	}
	if err := reloadMarketplaceExtensionUpdate(
		ctx,
		input.reload,
		input.registry,
		input.info,
		input.installDir,
		input.change,
		input.rollbackCandidate,
	); err != nil {
		return "", err
	}
	return remoteVersion, nil
}

func reloadMarketplaceExtensionUpdate(
	ctx context.Context,
	reload MutationReload,
	registry LifecycleRegistry,
	info ExtensionInfo,
	installDir string,
	change *stagedExtensionDirChange,
	rollbackCandidate MarketplaceUpdateRollback,
) error {
	if reload == nil {
		return nil
	}
	if err := reload(ctx); err != nil {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		restoreErr := restoreMarketplaceUpdateCandidate(
			rollbackCtx,
			registry,
			info,
			installDir,
			change,
			rollbackCandidate,
		)
		if restoreErr == nil {
			restoreErr = reload(rollbackCtx)
		}
		return errors.Join(
			fmt.Errorf("extension: reload after update %q: %w", info.Name, err),
			restoreErr,
		)
	}
	return nil
}

// Restore the package before releasing candidate-only resources. A failed package restoration
// retains its resources and never reloads a partially restored installation.
func restoreMarketplaceUpdateCandidate(
	ctx context.Context, registry LifecycleRegistry, info ExtensionInfo, installDir string,
	change *stagedExtensionDirChange, rollbackCandidate MarketplaceUpdateRollback,
) error {
	if err := restoreUpdatedExtensionRecord(registry, info, installDir, change); err != nil {
		return err
	}
	if rollbackCandidate != nil {
		return rollbackCandidate(ctx, info)
	}
	return nil
}

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
	provenance        ExtensionProvenance
	commitCandidate   MarketplaceUpdateCommit
	rollbackCandidate MarketplaceUpdateRollback
	reload            MutationReload
	afterReload       MutationReload
}

// Both update and reinstall publish one already validated artifact through this transaction.
func applyMarketplaceUpdateCandidate(
	ctx context.Context, input *marketplaceUpdateCommitInput,
	preflight MarketplaceUpdatePreflight, cleanup marketplaceUpdateCleanup,
) (marketplaceUpdateApplyResult, error) {
	if preflight != nil {
		if err := preflight(input.info, input.manifest); err != nil {
			return marketplaceUpdateApplyResult{}, err
		}
	}
	change, err := stageExtensionDirReplacement(input.result.InstallPath, input.installDir)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	input.change = change
	remoteVersion, err := commitMarketplaceUpdateCandidate(ctx, input)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	return committedMarketplaceUpdateResult(cleanup, input.info.Name, remoteVersion, change), nil
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
	if err := installMarketplaceExtensionUpdateRecord(
		input.registry,
		input.manifest,
		input.installDir,
		input.result.Checksum,
		input.slug,
		input.registryName,
		remoteVersion,
		input.provenance,
	); err != nil {
		return "", errors.Join(err, input.change.Rollback())
	}
	if input.commitCandidate != nil {
		if err := input.commitCandidate(input.info, input.manifest); err != nil {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			defer cancel()
			return "", errors.Join(err, restoreMarketplaceUpdateCandidate(
				rollbackCtx, input.registry, &input.info, input.installDir, input.change, input.rollbackCandidate,
			))
		}
	}
	if err := reloadMarketplaceExtensionUpdate(ctx, input); err != nil {
		return "", err
	}
	return remoteVersion, nil
}

func reloadMarketplaceExtensionUpdate(ctx context.Context, input *marketplaceUpdateCommitInput) error {
	var err error
	if input.reload != nil {
		err = input.reload(ctx)
	}
	if err == nil && input.afterReload != nil {
		err = input.afterReload(ctx)
	}
	if err == nil {
		return nil
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	restoreErr := restoreMarketplaceUpdateCandidate(
		rollbackCtx,
		input.registry,
		&input.info,
		input.installDir,
		input.change,
		input.rollbackCandidate,
	)
	if restoreErr == nil && input.reload != nil {
		restoreErr = input.reload(rollbackCtx)
	}
	return errors.Join(fmt.Errorf("extension: publish update %q: %w", input.info.Name, err), restoreErr)
}

// Restore the package before releasing candidate-only resources. A failed package restoration
// retains its resources and never reloads a partially restored installation.
func restoreMarketplaceUpdateCandidate(
	ctx context.Context, registry LifecycleRegistry, info *ExtensionInfo, installDir string,
	change *stagedExtensionDirChange, rollbackCandidate MarketplaceUpdateRollback,
) error {
	if err := restoreUpdatedExtensionRecord(registry, info, installDir, change); err != nil {
		return err
	}
	if rollbackCandidate != nil {
		return rollbackCandidate(ctx, *info)
	}
	return nil
}

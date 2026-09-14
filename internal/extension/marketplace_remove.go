package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// RemovalRegistry includes profile records needed to compensate an uninstall.
type RemovalRegistry interface {
	LifecycleRegistry
	SnapshotRemovalState(context.Context, string) (RemovalState, error)
	RestoreRemovalState(context.Context, string, RemovalState) error
}

type managedRemovalSnapshot struct {
	info  ExtensionInfo
	state RemovalState
}

// ManagedRemovalCommit follows reversible unpublication/staging: failure preserves state; success commits removal and later cleanup only warns.
type ManagedRemovalCommit func(context.Context) error

// RemoveManagedExtension rolls back registry and filesystem state when reloading after removal fails.
func RemoveManagedExtension(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry RemovalRegistry,
	name string,
	reload MutationReload,
	commit ManagedRemovalCommit,
) (_ ManagedRemoveResult, err error) {
	return removeManagedExtensionWithDataOps(
		ctx,
		homePaths,
		registry,
		name,
		reload,
		defaultExtensionDataRemovalOps(),
		commit,
	)
}

func removeManagedExtensionWithDataOps(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry RemovalRegistry,
	name string,
	reload MutationReload,
	dataRemovalOps extensionDataRemovalOps,
	commit ManagedRemovalCommit,
) (_ ManagedRemoveResult, err error) {
	if registry == nil {
		return ManagedRemoveResult{}, errors.New("extension: registry is required")
	}
	info, err := registry.Get(name)
	if err != nil {
		return ManagedRemoveResult{}, err
	}
	state, err := registry.SnapshotRemovalState(ctx, info.Name)
	if err != nil {
		return ManagedRemoveResult{}, err
	}
	snapshot := managedRemovalSnapshot{info: *info, state: state}
	installDir, err := InstalledExtensionDir(info)
	if err != nil {
		return ManagedRemoveResult{}, err
	}
	change, err := stageExtensionDirRemoval(installDir)
	if err != nil {
		return ManagedRemoveResult{}, err
	}

	if err := registry.Uninstall(info.Name); err != nil {
		return ManagedRemoveResult{}, errors.Join(err, change.Rollback())
	}
	if reload != nil {
		if err := reload(ctx); err != nil {
			return ManagedRemoveResult{}, rollbackManagedRemoval(
				ctx, registry, &snapshot, installDir, change, reload,
				fmt.Errorf("extension: reload after remove %q: %w", info.Name, err),
			)
		}
	}
	data, err := stageAgentPluginDataForRemoval(info, homePaths, dataRemovalOps)
	if err != nil {
		return ManagedRemoveResult{}, rollbackManagedRemoval(ctx, registry, &snapshot, installDir, change, reload, err)
	}
	if commit != nil {
		if err := commit(ctx); err != nil {
			restoreDataErr := data.rollback()
			return ManagedRemoveResult{}, rollbackManagedRemoval(
				ctx,
				registry, &snapshot, installDir,
				change,
				reload,
				errors.Join(err, restoreDataErr),
			)
		}
	}
	dataCleanup, dataCleanupErr := data.commit()
	result := finalizeManagedExtensionRemoval(info.Name, installDir, change)
	result.DataPath = dataCleanup.dataPath
	result.QuarantinePath = dataCleanup.quarantinePath
	if dataCleanupErr != nil {
		result.Warnings = append(result.Warnings, extensionDataCleanupWarning(
			info.Name,
			dataCleanup.quarantinePath,
			dataCleanupErr,
		))
	}
	return result, nil
}

// InstalledExtensionDir returns the root directory for a persisted extension
// registry row after validating the manifest path shape.
func InstalledExtensionDir(info *ExtensionInfo) (string, error) {
	manifestPath := filepath.Clean(strings.TrimSpace(info.ManifestPath))
	if manifestPath == "" || manifestPath == "." {
		return "", fmt.Errorf("extension: extension %q has an invalid manifest path %q", info.Name, info.ManifestPath)
	}
	if !filepath.IsAbs(manifestPath) {
		return "", fmt.Errorf(
			"extension: extension %q has a non-absolute manifest path %q",
			info.Name,
			info.ManifestPath,
		)
	}
	switch filepath.Base(manifestPath) {
	case "extension.toml", "extension.json", agentPluginManifestFileName:
	default:
		return "", fmt.Errorf("extension: extension %q has an invalid manifest path %q", info.Name, info.ManifestPath)
	}
	installDir := PackageRootFromManifest(manifestPath)
	if installDir == "." || installDir == string(filepath.Separator) {
		return "", fmt.Errorf("extension: extension %q has an invalid install directory %q", info.Name, installDir)
	}
	return installDir, nil
}

func rollbackManagedRemoval(
	ctx context.Context, registry RemovalRegistry, snapshot *managedRemovalSnapshot, installDir string,
	change *stagedExtensionDirChange, reload MutationReload, cause error,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	restoreErr := restoreRemovedExtensionRecord(registry, &snapshot.info, installDir, change)
	if restoreErr == nil {
		restoreErr = registry.RestoreRemovalState(rollbackCtx, snapshot.info.Name, snapshot.state)
	}
	if restoreErr == nil && reload != nil {
		restoreErr = reload(rollbackCtx)
	}
	return errors.Join(cause, restoreErr)
}

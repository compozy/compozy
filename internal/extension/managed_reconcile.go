package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	managedInstallStagingPrefix = ".compozy-extension-stage-"
	managedInstallBackupMarker  = ".compozy-backup-"
)

type managedInstallReconcileRegistry interface {
	List() ([]ExtensionInfo, error)
}

// ReconcileManagedExtensionArtifacts restores the managed install tree to the
// state committed in the registry before extension traffic starts.
func ReconcileManagedExtensionArtifacts(
	homePaths compozyconfig.HomePaths,
	registry managedInstallReconcileRegistry,
) error {
	if registry == nil {
		return errors.New("extension: registry is required to reconcile managed installs")
	}
	root := filepath.Clean(ManagedInstallRoot(homePaths))
	if strings.TrimSpace(homePaths.HomeDir) == "" || root == managedInstallDirName {
		return errors.New("extension: managed install home path is required")
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("extension: read managed install root %q: %w", root, err)
	}
	infos, err := registry.List()
	if err != nil {
		return fmt.Errorf("extension: list registry rows for managed install reconciliation: %w", err)
	}
	owners, err := managedInstallOwners(root, infos)
	if err != nil {
		return err
	}
	backups := make(map[string][]string)
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(root, name)
		switch {
		case strings.HasPrefix(name, managedInstallStagingPrefix):
			if err := removeReconcilePath(path); err != nil {
				return err
			}
		case strings.Contains(name, managedInstallBackupMarker):
			base, _, _ := strings.Cut(name, managedInstallBackupMarker)
			backups[base] = append(backups[base], path)
		case owners[name] == nil:
			if err := removeReconcilePath(path); err != nil {
				return err
			}
		}
	}
	for name, paths := range backups {
		if err := reconcileManagedInstallBackups(root, name, paths, owners[name]); err != nil {
			return err
		}
	}
	return nil
}

func managedInstallOwners(root string, infos []ExtensionInfo) (map[string]*ExtensionInfo, error) {
	owners := make(map[string]*ExtensionInfo)
	for idx := range infos {
		installDir, err := InstalledExtensionDir(infos[idx])
		if err != nil {
			return nil, fmt.Errorf("extension: resolve registered install %q during reconciliation: %w", infos[idx].Name, err)
		}
		relative, err := filepath.Rel(root, installDir)
		if err != nil {
			return nil, fmt.Errorf("extension: relate registered install %q to managed root: %w", infos[idx].Name, err)
		}
		if relative == "." || filepath.IsAbs(relative) || strings.Contains(relative, string(filepath.Separator)) ||
			relative == ".." {
			continue
		}
		owners[relative] = &infos[idx]
	}
	return owners, nil
}

func reconcileManagedInstallBackups(
	root string,
	name string,
	backups []string,
	owner *ExtensionInfo,
) error {
	if owner == nil {
		return removeReconcilePaths(backups)
	}
	target := filepath.Join(root, name)
	if checksumMatchesManagedInstall(target, owner.Checksum) {
		return removeReconcilePaths(backups)
	}
	matching := ""
	for _, backup := range backups {
		if checksumMatchesManagedInstall(backup, owner.Checksum) {
			if matching != "" {
				return fmt.Errorf("extension: multiple backups match registered extension %q", name)
			}
			matching = backup
		}
	}
	if matching == "" {
		return fmt.Errorf("extension: no managed artifact matches registered checksum for %q", name)
	}
	if err := removeReconcilePath(target); err != nil {
		return err
	}
	if err := os.Rename(matching, target); err != nil {
		return fmt.Errorf("extension: restore managed install backup %q: %w", matching, err)
	}
	remaining := make([]string, 0, len(backups)-1)
	for _, backup := range backups {
		if backup != matching {
			remaining = append(remaining, backup)
		}
	}
	return removeReconcilePaths(remaining)
}

func checksumMatchesManagedInstall(path string, expected string) bool {
	checksum, err := ComputeDirectoryChecksum(path)
	return err == nil && checksum == strings.TrimSpace(expected)
}

func removeReconcilePaths(paths []string) error {
	for _, path := range paths {
		if err := removeReconcilePath(path); err != nil {
			return err
		}
	}
	return nil
}

func removeReconcilePath(path string) error {
	if err := os.RemoveAll(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("extension: remove orphaned managed install path %q: %w", path, err)
	}
	return nil
}

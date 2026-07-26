package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func restoreRemovedExtensionRecord(
	registry LifecycleRegistry,
	info ExtensionInfo,
	installDir string,
	change *stagedExtensionDirChange,
) error {
	if err := change.Rollback(); err != nil {
		return err
	}
	return reinstallExtensionInfo(registry, info, installDir)
}

func restoreUpdatedExtensionRecord(
	registry LifecycleRegistry,
	info ExtensionInfo,
	installDir string,
	change *stagedExtensionDirChange,
) error {
	if err := change.Rollback(); err != nil {
		return err
	}
	return reinstallExtensionInfo(registry, info, installDir, WithInstallReplaceExisting())
}

func reinstallExtensionInfo(
	registry LifecycleRegistry,
	info ExtensionInfo,
	installDir string,
	opts ...InstallOption,
) error {
	manifest, err := LoadManifest(installDir)
	if err != nil {
		return fmt.Errorf("extension: reload manifest from %q after rollback: %w", installDir, err)
	}
	installOpts := []InstallOption{
		WithInstallSource(info.Source),
		WithInstallProvenance(info.Provenance),
	}
	if info.Source == SourceMarketplace {
		installOpts = append(installOpts, WithInstallRegistryMetadata(
			dereferenceOptionalString(info.RegistrySlug),
			dereferenceOptionalString(info.RegistryName),
			dereferenceOptionalString(info.RemoteVersion),
		))
	}
	installOpts = append(installOpts, opts...)
	if err := registry.Install(manifest, installDir, info.Checksum, installOpts...); err != nil {
		return fmt.Errorf("extension: restore registry record for %q: %w", info.Name, err)
	}
	if !info.Enabled {
		if err := registry.Disable(info.Name); err != nil {
			return fmt.Errorf("extension: restore disabled state for %q: %w", info.Name, err)
		}
	}
	return nil
}

func stageExtensionDirRemoval(targetDir string) (*stagedExtensionDirChange, error) {
	change := &stagedExtensionDirChange{targetDir: targetDir}
	info, err := os.Stat(targetDir)
	if errors.Is(err, os.ErrNotExist) {
		return change, nil
	}
	if err != nil {
		return nil, fmt.Errorf("extension: stat extension directory %q: %w", targetDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("extension: extension path %q is not a directory", targetDir)
	}
	backupDir := uniqueExtensionBackupPath(targetDir)
	if err := os.Rename(targetDir, backupDir); err != nil {
		return nil, fmt.Errorf("extension: stage extension directory removal %q: %w", targetDir, err)
	}
	change.backupDir = backupDir
	return change, nil
}

func stageExtensionDirReplacement(stagingDir string, targetDir string) (*stagedExtensionDirChange, error) {
	change := &stagedExtensionDirChange{targetDir: targetDir}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return nil, fmt.Errorf("extension: create install parent %q: %w", filepath.Dir(targetDir), err)
	}
	if _, err := os.Stat(targetDir); err == nil {
		backupDir := uniqueExtensionBackupPath(targetDir)
		if err := os.Rename(targetDir, backupDir); err != nil {
			return nil, fmt.Errorf("extension: backup extension directory %q: %w", targetDir, err)
		}
		change.backupDir = backupDir
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("extension: stat extension directory %q: %w", targetDir, err)
	}
	if err := os.Rename(stagingDir, targetDir); err != nil {
		return nil, errors.Join(
			fmt.Errorf("extension: move staged extension into %q: %w", targetDir, err),
			change.Rollback(),
		)
	}
	return change, nil
}

func (c *stagedExtensionDirChange) Commit() error {
	if c == nil || strings.TrimSpace(c.backupDir) == "" {
		return nil
	}
	removeBackup := c.removeBackup
	if removeBackup == nil {
		removeBackup = os.RemoveAll
	}
	if err := removeBackup(c.backupDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("extension: remove extension backup %q: %w", c.backupDir, err)
	}
	return nil
}

func (c *stagedExtensionDirChange) Rollback() error {
	if c == nil {
		return nil
	}
	if strings.TrimSpace(c.targetDir) != "" {
		if err := os.RemoveAll(c.targetDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("extension: remove failed extension target %q: %w", c.targetDir, err)
		}
	}
	if strings.TrimSpace(c.backupDir) == "" {
		return nil
	}
	if err := os.Rename(c.backupDir, c.targetDir); err != nil {
		return fmt.Errorf("extension: restore extension backup %q: %w", c.targetDir, err)
	}
	return nil
}

func uniqueExtensionBackupPath(targetDir string) string {
	return fmt.Sprintf("%s.agh-backup-%d", targetDir, time.Now().UTC().UnixNano())
}

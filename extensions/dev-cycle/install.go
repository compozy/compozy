package devcycle

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	registrypkg "github.com/compozy/agh/internal/registry"
)

const Name = "dev-cycle"

// EnsureManagedInstall enrolls the first-party dev-cycle extension without
// overwriting a user-installed extension with the same name.
func EnsureManagedInstall(homePaths aghconfig.HomePaths, registry *extensionpkg.Registry) (err error) {
	if registry == nil {
		return errors.New("dev-cycle: extension registry is required")
	}

	stagingDir, err := materializeEmbeddedExtension(homePaths)
	if err != nil {
		return err
	}
	cleanupStaging := true
	defer func() {
		if !cleanupStaging {
			return
		}
		if removeErr := os.RemoveAll(stagingDir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("dev-cycle: remove staging dir %q: %w", stagingDir, removeErr))
		}
	}()

	manifest, err := extensionpkg.LoadManifest(stagingDir)
	if err != nil {
		return fmt.Errorf("dev-cycle: load embedded manifest: %w", err)
	}
	if strings.TrimSpace(manifest.Name) != Name {
		return fmt.Errorf("dev-cycle: embedded manifest name %q does not match %q", manifest.Name, Name)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(stagingDir)
	if err != nil {
		return fmt.Errorf("dev-cycle: compute embedded checksum: %w", err)
	}

	existing, err := registry.Get(Name)
	switch {
	case errors.Is(err, extensionpkg.ErrExtensionNotFound):
		return extensionpkg.InstallLocalManaged(
			homePaths,
			registry,
			manifest,
			stagingDir,
			checksum,
			extensionpkg.WithInstallSource(extensionpkg.SourceBundled),
		)
	case err != nil:
		return fmt.Errorf("dev-cycle: inspect existing extension: %w", err)
	case existing.Source != extensionpkg.SourceBundled:
		return nil
	case strings.EqualFold(existing.Checksum, checksum) && strings.TrimSpace(existing.Version) == manifest.Version:
		return nil
	default:
		return replaceBundledInstall(homePaths, registry, manifest, stagingDir, existing)
	}
}

func materializeEmbeddedExtension(homePaths aghconfig.HomePaths) (string, error) {
	stagingDir, err := extensionpkg.NewManagedInstallStagingDir(homePaths)
	if err != nil {
		return "", err
	}
	if err := fs.WalkDir(FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(stagingDir, filepath.FromSlash(path))
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("dev-cycle: create embedded dir %q: %w", target, err)
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("dev-cycle: create embedded parent %q: %w", filepath.Dir(target), err)
		}
		content, err := fs.ReadFile(FS(), path)
		if err != nil {
			return fmt.Errorf("dev-cycle: read embedded file %q: %w", path, err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return fmt.Errorf("dev-cycle: write embedded file %q: %w", target, err)
		}
		return nil
	}); err != nil {
		return "", errors.Join(err, os.RemoveAll(stagingDir))
	}
	return stagingDir, nil
}

func replaceBundledInstall(
	homePaths aghconfig.HomePaths,
	registry *extensionpkg.Registry,
	manifest *extensionpkg.Manifest,
	stagingDir string,
	existing *extensionpkg.ExtensionInfo,
) error {
	finalDir, err := extensionpkg.ManagedInstallPathChecked(homePaths, Name)
	if err != nil {
		return err
	}
	if err := registrypkg.MoveInstalledDir(stagingDir, finalDir, true); err != nil {
		return fmt.Errorf("dev-cycle: replace managed install: %w", err)
	}
	installedChecksum, err := extensionpkg.ComputeDirectoryChecksum(finalDir)
	if err != nil {
		return fmt.Errorf("dev-cycle: compute installed checksum: %w", err)
	}
	if err := registry.Install(
		manifest,
		finalDir,
		installedChecksum,
		extensionpkg.WithInstallSource(extensionpkg.SourceBundled),
		extensionpkg.WithInstallReplaceExisting(),
	); err != nil {
		return fmt.Errorf("dev-cycle: persist bundled install: %w", err)
	}
	if existing != nil && !existing.Enabled {
		if err := registry.Disable(Name); err != nil {
			return fmt.Errorf("dev-cycle: preserve disabled state: %w", err)
		}
	}
	return nil
}

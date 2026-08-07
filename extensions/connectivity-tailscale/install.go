package connectivitytailscale

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

const Name = "connectivity-tailscale"

// EnsureManagedInstall enrolls the first-party provider without replacing an operator install.
func EnsureManagedInstall(homePaths compozyconfig.HomePaths, registry *extensionpkg.Registry) (err error) {
	if registry == nil {
		return errors.New("connectivity-tailscale: extension registry is required")
	}
	stagingDir, err := materializeEmbeddedExtension(homePaths)
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.RemoveAll(stagingDir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("connectivity-tailscale: remove staging directory: %w", removeErr))
		}
	}()
	manifest, err := extensionpkg.LoadManifest(stagingDir)
	if err != nil {
		return fmt.Errorf("connectivity-tailscale: load embedded manifest: %w", err)
	}
	if strings.TrimSpace(manifest.Name) != Name {
		return fmt.Errorf("connectivity-tailscale: embedded manifest name %q does not match %q", manifest.Name, Name)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(stagingDir)
	if err != nil {
		return fmt.Errorf("connectivity-tailscale: compute embedded checksum: %w", err)
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
			extensionpkg.WithInstallEnabled(true),
		)
	case err != nil:
		return fmt.Errorf("connectivity-tailscale: inspect existing extension: %w", err)
	case existing.Source != extensionpkg.SourceBundled:
		return nil
	case strings.EqualFold(existing.Checksum, checksum) && strings.TrimSpace(existing.Version) == manifest.Version:
		return nil
	default:
		return replaceBundledInstall(homePaths, registry, manifest, stagingDir, existing)
	}
}

func materializeEmbeddedExtension(homePaths compozyconfig.HomePaths) (string, error) {
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
			return os.MkdirAll(target, 0o755)
		}
		content, err := fs.ReadFile(FS(), path)
		if err != nil {
			return fmt.Errorf("connectivity-tailscale: read embedded file %q: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("connectivity-tailscale: create embedded parent: %w", err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return fmt.Errorf("connectivity-tailscale: write embedded file %q: %w", path, err)
		}
		return nil
	}); err != nil {
		return "", errors.Join(err, os.RemoveAll(stagingDir))
	}
	return stagingDir, nil
}

func replaceBundledInstall(
	homePaths compozyconfig.HomePaths,
	registry *extensionpkg.Registry,
	manifest *extensionpkg.Manifest,
	stagingDir string,
	existing *extensionpkg.ExtensionInfo,
) error {
	finalDir, err := extensionpkg.ManagedInstallPathChecked(homePaths, Name)
	if err != nil {
		return err
	}
	moveResult, err := registrypkg.MoveInstalledDir(stagingDir, finalDir, true)
	if err != nil {
		return fmt.Errorf("connectivity-tailscale: replace managed install: %w", err)
	}
	for _, diagnostic := range moveResult.CleanupDiagnostics {
		slog.Warn("bundled extension cleanup degraded", "operation", diagnostic.Operation, "extension", Name)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(finalDir)
	if err != nil {
		return fmt.Errorf("connectivity-tailscale: compute installed checksum: %w", err)
	}
	if err := registry.Install(
		manifest,
		finalDir,
		checksum,
		extensionpkg.WithInstallSource(extensionpkg.SourceBundled),
		extensionpkg.WithInstallReplaceExisting(),
	); err != nil {
		return fmt.Errorf("connectivity-tailscale: persist bundled install: %w", err)
	}
	if existing != nil && !existing.Enabled {
		if err := registry.Disable(Name); err != nil {
			return fmt.Errorf("connectivity-tailscale: preserve disabled state: %w", err)
		}
	}
	return nil
}

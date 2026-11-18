package extensionpkg

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

// BundledInstallSpec identifies one first-party extension embedded in the Compozy binary.
type BundledInstallSpec struct {
	Name string
	FS   fs.FS
}

// InstallBundledExtension enrolls or refreshes a first-party managed extension without
// replacing an operator-owned extension with the same name.
func InstallBundledExtension(
	homePaths compozyconfig.HomePaths,
	registry *Registry,
	spec BundledInstallSpec,
) (err error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return errors.New("extension: bundled extension name is required")
	}
	if spec.FS == nil {
		return fmt.Errorf("%s: embedded extension filesystem is required", name)
	}
	if registry == nil {
		return fmt.Errorf("%s: extension registry is required", name)
	}

	stagingDir, err := MaterializeBundledExtension(homePaths, name, spec.FS)
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.RemoveAll(stagingDir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("%s: remove staging directory %q: %w", name, stagingDir, removeErr))
		}
	}()

	manifest, err := LoadManifest(stagingDir)
	if err != nil {
		return fmt.Errorf("%s: load embedded manifest: %w", name, err)
	}
	if strings.TrimSpace(manifest.Name) != name {
		return fmt.Errorf("%s: embedded manifest name %q does not match %q", name, manifest.Name, name)
	}
	checksum, err := ComputeDirectoryChecksum(stagingDir)
	if err != nil {
		return fmt.Errorf("%s: compute embedded checksum: %w", name, err)
	}

	existing, err := registry.Get(name)
	switch {
	case errors.Is(err, ErrExtensionNotFound):
		if err := InstallLocalManaged(
			homePaths,
			registry,
			manifest,
			stagingDir,
			checksum,
			WithInstallSource(SourceBundled),
			WithInstallEnabled(true),
		); err != nil {
			return fmt.Errorf("%s: install bundled extension: %w", name, err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("%s: inspect existing extension: %w", name, err)
	case existing.Source != SourceBundled:
		return nil
	case strings.EqualFold(existing.Checksum, checksum) &&
		strings.TrimSpace(existing.Version) == manifest.Version:
		if err := registry.ReconcileBundledProvenance(manifest); err != nil {
			return fmt.Errorf("%s: reconcile bundled provenance: %w", name, err)
		}
		return nil
	default:
		return replaceBundledExtension(homePaths, registry, name, manifest, stagingDir, existing)
	}
}

// MaterializeBundledExtension copies one embedded extension into a managed staging directory.
func MaterializeBundledExtension(
	homePaths compozyconfig.HomePaths,
	name string,
	embedded fs.FS,
) (string, error) {
	stagingDir, err := NewManagedInstallStagingDir(homePaths)
	if err != nil {
		return "", fmt.Errorf("%s: create managed install staging directory: %w", name, err)
	}
	if err := fs.WalkDir(embedded, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("%s: walk embedded path %q: %w", name, path, walkErr)
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(stagingDir, filepath.FromSlash(path))
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("%s: create embedded directory %q: %w", name, target, err)
			}
			return nil
		}
		content, err := fs.ReadFile(embedded, path)
		if err != nil {
			return fmt.Errorf("%s: read embedded file %q: %w", name, path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("%s: create embedded parent %q: %w", name, filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return fmt.Errorf("%s: write embedded file %q: %w", name, target, err)
		}
		return nil
	}); err != nil {
		cleanupErr := os.RemoveAll(stagingDir)
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("%s: remove failed staging directory %q: %w", name, stagingDir, cleanupErr)
		}
		return "", errors.Join(err, cleanupErr)
	}
	return stagingDir, nil
}

func replaceBundledExtension(
	homePaths compozyconfig.HomePaths,
	registry *Registry,
	name string,
	manifest *Manifest,
	stagingDir string,
	existing *ExtensionInfo,
) error {
	finalDir, err := ManagedInstallPathChecked(homePaths, name)
	if err != nil {
		return fmt.Errorf("%s: resolve managed install path: %w", name, err)
	}
	moveResult, err := registrypkg.MoveInstalledDir(stagingDir, finalDir, true)
	if err != nil {
		return fmt.Errorf("%s: replace managed install: %w", name, err)
	}
	for _, diagnostic := range moveResult.CleanupDiagnostics {
		slog.Warn(
			"bundled extension install cleanup degraded",
			"operation", diagnostic.Operation,
			"extension", name,
		)
	}
	checksum, err := ComputeDirectoryChecksum(finalDir)
	if err != nil {
		return fmt.Errorf("%s: compute installed checksum: %w", name, err)
	}
	if err := registry.Install(
		manifest,
		finalDir,
		checksum,
		WithInstallSource(SourceBundled),
		WithInstallReplaceExisting(),
	); err != nil {
		return fmt.Errorf("%s: persist bundled install: %w", name, err)
	}
	if existing != nil && !existing.Enabled {
		if err := registry.Disable(name); err != nil {
			return fmt.Errorf("%s: preserve disabled state: %w", name, err)
		}
	}
	return nil
}

package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"os"
	"path/filepath"

	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

const managedInstallDirName = "extensions"
const invalidManagedInstallName = "_invalid-extension-name"

type installPackageManifest struct {
	Dependencies         map[string]string `json:"dependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

// ManagedInstallRoot returns the Compozy-managed root directory for installed extensions.
func ManagedInstallRoot(homePaths compozyconfig.HomePaths) string {
	return filepath.Join(strings.TrimSpace(homePaths.HomeDir), managedInstallDirName)
}

// ManagedInstallPath returns the Compozy-managed directory for one installed extension.
func ManagedInstallPath(homePaths compozyconfig.HomePaths, name string) string {
	path, err := ManagedInstallPathChecked(homePaths, name)
	if err != nil {
		return filepath.Join(ManagedInstallRoot(homePaths), invalidManagedInstallName)
	}
	return path
}

// ManagedInstallPathChecked returns the contained managed directory for one installed extension.
func ManagedInstallPathChecked(homePaths compozyconfig.HomePaths, name string) (string, error) {
	root := filepath.Clean(ManagedInstallRoot(homePaths))
	if strings.TrimSpace(root) == "" || root == managedInstallDirName {
		return "", errors.New("extension: managed install home path is required")
	}

	safeName, err := validateManagedInstallName(name)
	if err != nil {
		return "", err
	}

	finalDir := filepath.Join(root, safeName)
	rel, err := filepath.Rel(root, finalDir)
	if err != nil {
		return "", fmt.Errorf("extension: resolve managed install path for %q: %w", safeName, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("extension: managed extension name %q escapes install root", name)
	}
	return finalDir, nil
}

// RemoveManagedInstall removes one extension directory after proving its resolved path stays
// beneath the managed install root.
func RemoveManagedInstall(homePaths compozyconfig.HomePaths, name string) error {
	root := ManagedInstallRoot(homePaths)
	target, err := ManagedInstallPathChecked(homePaths, name)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("extension: inspect managed install path %q: %w", target, err)
	}
	canonicalRoot, err := canonicalizeInstallPath(root)
	if err != nil {
		return fmt.Errorf("extension: resolve managed install root %q: %w", root, err)
	}
	canonicalTarget, err := canonicalizeInstallPath(target)
	if err != nil {
		return fmt.Errorf("extension: resolve managed install path %q: %w", target, err)
	}
	rel, err := filepath.Rel(canonicalRoot, canonicalTarget)
	if err != nil {
		return fmt.Errorf("extension: relate managed install path %q to root %q: %w", target, root, err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("extension: managed install path %q escapes install root", target)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("extension: remove managed install path %q: %w", target, err)
	}
	return nil
}

// NewManagedInstallStagingDir creates an empty staging directory under the managed extension root.
func NewManagedInstallStagingDir(homePaths compozyconfig.HomePaths) (string, error) {
	root := ManagedInstallRoot(homePaths)
	if strings.TrimSpace(root) == "" || root == managedInstallDirName {
		return "", errors.New("extension: managed install home path is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("extension: create managed install root %q: %w", root, err)
	}
	return os.MkdirTemp(root, ".compozy-extension-stage-*")
}

// InstallLocalManaged copies one local extension into the managed install root and persists the registry record there.
// With WithInstallReplaceExisting an installed extension is replaced through the same transaction published sources
// use: the previous directory and registry record survive until every post-commit step succeeds.
func InstallLocalManaged(
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	manifest *Manifest,
	sourceDir string,
	checksum string,
	opts ...InstallOption,
) (err error) {
	installConfig := installConfig{}
	applyInstallOptions(&installConfig, opts...)

	if err := validateManagedInstallInput(
		registry,
		manifest,
		checksum,
		installConfig.replaceExisting,
	); err != nil {
		return err
	}
	prepared, err := PrepareLocalManagedInstall(homePaths, registry, manifest, sourceDir, checksum, opts...)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := prepared.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if !installConfig.replaceExisting {
		return prepared.Commit()
	}

	installed, getErr := registry.Get(manifest.Name)
	switch {
	case errors.Is(getErr, ErrExtensionNotFound):
		return prepared.Commit()
	case getErr != nil:
		return getErr
	}
	warnings, reinstallErr := prepared.Reinstall(
		context.Background(),
		installed,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	for _, warning := range warnings {
		slog.Warn(
			"managed extension local reinstall cleanup degraded",
			"extension", manifest.Name,
			"code", warning.Code,
		)
	}
	return reinstallErr
}

// validateManagedInstallInput fails fast on a duplicate name before the bundle is staged.
func validateManagedInstallInput(
	registry LifecycleRegistry,
	manifest *Manifest,
	checksum string,
	replaceExisting bool,
) error {
	if registry == nil {
		return errors.New("extension: registry is required")
	}
	if _, err := normalizeManagedInstallChecksum(manifest, checksum); err != nil {
		return err
	}
	if _, err := registry.Get(manifest.Name); err == nil {
		if !replaceExisting {
			return &ExtensionExistsError{Name: manifest.Name}
		}
	} else if !errors.Is(err, ErrExtensionNotFound) {
		return err
	}
	return nil
}

func normalizeManagedInstallChecksum(manifest *Manifest, checksum string) (string, error) {
	if manifest == nil {
		return "", errors.New("extension: manifest is required")
	}
	if _, err := validateManagedInstallName(manifest.Name); err != nil {
		return "", err
	}
	normalizedChecksum := strings.ToLower(strings.TrimSpace(checksum))
	if normalizedChecksum == "" {
		return "", errors.New("extension: checksum is required")
	}
	return normalizedChecksum, nil
}

func verifyManagedInstallSource(sourceDir string, expectedChecksum string) error {
	actualChecksum, err := ComputeDirectoryChecksum(sourceDir)
	if err != nil {
		return fmt.Errorf("extension: compute source checksum %q: %w", sourceDir, err)
	}
	if actualChecksum != expectedChecksum {
		return &ExtensionChecksumMismatchError{
			ExpectedChecksum: expectedChecksum,
			ActualChecksum:   actualChecksum,
		}
	}
	return nil
}

func observeManagedInstallBoundary(
	observer func(managedInstallBoundary) error,
	boundary managedInstallBoundary,
) error {
	if observer == nil {
		return nil
	}
	if err := observer(boundary); err != nil {
		return fmt.Errorf("extension: managed install interrupted after %s: %w", boundary, err)
	}
	return nil
}

func logLocalInstallCleanupDiagnostics(diagnostics []registrypkg.CleanupDiagnostic) {
	for _, diagnostic := range diagnostics {
		slog.Warn(
			"managed extension install cleanup degraded",
			"operation", diagnostic.Operation,
			"install_source", "local",
		)
	}
}

func validateManagedInstallName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return "", errors.New("extension: managed extension name is required")
	case trimmed == "." || trimmed == ".." || trimmed == sourceMarketplaceName:
		return "", fmt.Errorf("extension: managed extension name %q is reserved", name)
	case filepath.IsAbs(trimmed):
		return "", fmt.Errorf("extension: managed extension name %q must be relative", name)
	case strings.Contains(trimmed, "/") || strings.Contains(trimmed, `\`):
		return "", fmt.Errorf("extension: managed extension name %q must be a single path segment", name)
	case filepath.Clean(trimmed) != trimmed:
		return "", fmt.Errorf("extension: managed extension name %q is not normalized", name)
	default:
		return trimmed, nil
	}
}

func removeManagedInstallOnError(finalDir string, baseErr error, contextSuffix string) error {
	removeErr := os.RemoveAll(finalDir)
	if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
		return baseErr
	}

	message := fmt.Sprintf("extension: remove failed local install %q", finalDir)
	if strings.TrimSpace(contextSuffix) != "" {
		message += " " + strings.TrimSpace(contextSuffix)
	}
	return errors.Join(baseErr, fmt.Errorf("%s: %w", message, removeErr))
}

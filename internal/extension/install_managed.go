package extensionpkg

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	registrypkg "github.com/compozy/agh/internal/registry"
)

const managedInstallDirName = "extensions"
const invalidManagedInstallName = "_invalid-extension-name"

type installPackageManifest struct {
	Dependencies         map[string]string `json:"dependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

type managedInstallRegistry interface {
	Get(name string) (*ExtensionInfo, error)
	Install(manifest *Manifest, path string, checksum string, opts ...InstallOption) error
}

// ManagedInstallRoot returns the AGH-managed root directory for installed extensions.
func ManagedInstallRoot(homePaths aghconfig.HomePaths) string {
	return filepath.Join(strings.TrimSpace(homePaths.HomeDir), managedInstallDirName)
}

// ManagedInstallPath returns the AGH-managed directory for one installed extension.
func ManagedInstallPath(homePaths aghconfig.HomePaths, name string) string {
	path, err := ManagedInstallPathChecked(homePaths, name)
	if err != nil {
		return filepath.Join(ManagedInstallRoot(homePaths), invalidManagedInstallName)
	}
	return path
}

// ManagedInstallPathChecked returns the contained managed directory for one installed extension.
func ManagedInstallPathChecked(homePaths aghconfig.HomePaths, name string) (string, error) {
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

// NewManagedInstallStagingDir creates an empty staging directory under the managed extension root.
func NewManagedInstallStagingDir(homePaths aghconfig.HomePaths) (string, error) {
	root := ManagedInstallRoot(homePaths)
	if strings.TrimSpace(root) == "" || root == managedInstallDirName {
		return "", errors.New("extension: managed install home path is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("extension: create managed install root %q: %w", root, err)
	}
	return os.MkdirTemp(root, ".agh-extension-stage-*")
}

// InstallLocalManaged copies one local extension into the managed install root and persists the registry record there.
func InstallLocalManaged(
	homePaths aghconfig.HomePaths,
	registry managedInstallRegistry,
	manifest *Manifest,
	sourceDir string,
	checksum string,
	opts ...InstallOption,
) (err error) {
	normalizedChecksum, err := validateManagedInstallInput(registry, manifest, checksum)
	if err != nil {
		return err
	}
	finalDir, err := ManagedInstallPathChecked(homePaths, manifest.Name)
	if err != nil {
		return err
	}

	actualSourceChecksum, err := ComputeDirectoryChecksum(sourceDir)
	if err != nil {
		return fmt.Errorf("extension: compute source checksum %q: %w", sourceDir, err)
	}
	if actualSourceChecksum != normalizedChecksum {
		return &ExtensionChecksumMismatchError{
			ExpectedChecksum: normalizedChecksum,
			ActualChecksum:   actualSourceChecksum,
		}
	}

	stagingDir, err := NewManagedInstallStagingDir(homePaths)
	if err != nil {
		return err
	}

	cleanupStaging := true
	defer func() {
		if !cleanupStaging {
			return
		}
		if removeErr := os.RemoveAll(stagingDir); removeErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("extension: remove managed install staging dir %q: %w", stagingDir, removeErr),
			)
		}
	}()

	if err := copyInstallTree(sourceDir, stagingDir); err != nil {
		return err
	}

	if err := registrypkg.MoveInstalledDir(stagingDir, finalDir, false); err != nil {
		return fmt.Errorf("extension: move local extension %q into managed install path: %w", manifest.Name, err)
	}
	cleanupStaging = false

	installedChecksum, err := ComputeDirectoryChecksum(finalDir)
	if err != nil {
		return removeManagedInstallOnError(
			finalDir,
			fmt.Errorf("extension: compute installed checksum %q: %w", finalDir, err),
			"after checksum error",
		)
	}

	if err := registry.Install(manifest, finalDir, installedChecksum, opts...); err != nil {
		return removeManagedInstallOnError(
			finalDir,
			fmt.Errorf("extension: persist managed extension %q: %w", manifest.Name, err),
			"",
		)
	}

	return nil
}

func validateManagedInstallInput(
	registry managedInstallRegistry,
	manifest *Manifest,
	checksum string,
) (string, error) {
	if registry == nil {
		return "", errors.New("extension: registry is required")
	}
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

	if _, err := registry.Get(manifest.Name); err == nil {
		return "", &ExtensionExistsError{Name: manifest.Name}
	} else if !errors.Is(err, ErrExtensionNotFound) {
		return "", err
	}

	return normalizedChecksum, nil
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

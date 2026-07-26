package cli

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
)

func prepareExtensionInstall(path string) (preparedExtensionInstall, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return preparedExtensionInstall{}, errors.New("extension: install path is required")
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return preparedExtensionInstall{}, fmt.Errorf("extension: resolve install path %q: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return preparedExtensionInstall{}, fmt.Errorf("extension: stat install path %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return preparedExtensionInstall{}, fmt.Errorf("extension: install path %q must be a directory", absPath)
	}

	manifest, err := extensionpkg.LoadManifest(absPath)
	if err != nil {
		return preparedExtensionInstall{}, err
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(absPath)
	if err != nil {
		return preparedExtensionInstall{}, err
	}

	return preparedExtensionInstall{
		Path:     absPath,
		Manifest: manifest,
		Checksum: checksum,
	}, nil
}

func prepareLocalExtensionInstallIfPresent(path string) (preparedExtensionInstall, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return preparedExtensionInstall{}, false, errors.New("extension: install path or registry slug is required")
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return preparedExtensionInstall{}, false, fmt.Errorf("extension: resolve install path %q: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return preparedExtensionInstall{}, false, nil
	}
	if err != nil {
		return preparedExtensionInstall{}, false, fmt.Errorf("extension: stat install path %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return preparedExtensionInstall{}, false, fmt.Errorf("extension: install path %q must be a directory", absPath)
	}

	prepared, err := prepareExtensionInstall(absPath)
	if err != nil {
		return preparedExtensionInstall{}, false, err
	}
	return prepared, true, nil
}

func installPreparedExtension(
	homePaths aghconfig.HomePaths,
	registry localExtensionRegistry,
	prepared preparedExtensionInstall,
	installedAt time.Time,
	allowUnverified bool,
) error {
	if registry == nil {
		return errors.New("extension: registry is required")
	}
	if prepared.Manifest == nil {
		return errors.New("extension: manifest is required")
	}
	if !allowUnverified {
		return extensionpkg.NewExtensionChecksumUnverifiedError(prepared.Manifest.Name, prepared.Path)
	}
	return extensionpkg.InstallLocalManaged(
		homePaths,
		registry,
		prepared.Manifest,
		prepared.Path,
		prepared.Checksum,
		extensionpkg.WithInstallProvenance(extensionpkg.LocalPathProvenance(
			prepared.Manifest,
			prepared.Path,
			prepared.Checksum,
			installedAt,
			allowUnverified,
		)),
	)
}

func localExtensionRecord(
	info extensionpkg.ExtensionInfo,
	now func() time.Time,
	getenv func(string) string,
) ExtensionRecord {
	ext := &extensionpkg.Extension{
		Info: info,
		Status: extensionpkg.ExtensionStatus{
			Name:    info.Name,
			Version: info.Version,
			Source:  info.Source,
			Enabled: info.Enabled,
		},
	}
	if manifest, err := extensionpkg.LoadManifest(filepath.Dir(info.ManifestPath)); err == nil {
		ext.Manifest = manifest
		ext.Status.MissingEnv = manifest.MissingEnv(getenv)
		ext.Status.MissingEnvChecked = len(manifest.RequiresEnv) > 0
	}
	return extensionpkg.DescribeExtension(ext, false, now())
}

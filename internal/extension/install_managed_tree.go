package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"sort"
	"strings"
)

func copyInstallTree(sourceDir string, targetDir string) error {
	sourceRoot := strings.TrimSpace(sourceDir)
	if sourceRoot == "" {
		return errors.New("extension: source directory is required")
	}

	absSourceRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return fmt.Errorf("extension: resolve source directory %q: %w", sourceDir, err)
	}
	canonicalSourceRoot, err := canonicalizeInstallPath(absSourceRoot)
	if err != nil {
		return fmt.Errorf("extension: canonicalize source directory %q: %w", absSourceRoot, err)
	}

	info, err := os.Stat(absSourceRoot)
	if err != nil {
		return fmt.Errorf("extension: stat source directory %q: %w", absSourceRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("extension: source directory %q is not a directory", absSourceRoot)
	}

	if strings.TrimSpace(targetDir) == "" {
		return errors.New("extension: target directory is required")
	}
	if err := os.MkdirAll(targetDir, info.Mode().Perm()); err != nil {
		return fmt.Errorf("extension: create target directory %q: %w", targetDir, err)
	}
	if err := os.Chmod(targetDir, info.Mode().Perm()); err != nil {
		return fmt.Errorf("extension: set target directory mode %q: %w", targetDir, err)
	}

	return copyInstallDirectoryContents(canonicalSourceRoot, absSourceRoot, targetDir, map[string]struct{}{
		canonicalSourceRoot: {},
	})
}

func copyInstallDirectoryContents(
	sourceRoot string,
	sourceDir string,
	targetDir string,
	activeDirs map[string]struct{},
) error {
	runtimeDeps, hasPackageManifest, err := loadInstallRuntimeDependencies(sourceDir)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("extension: read source directory %q: %w", sourceDir, err)
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(sourceDir, entry.Name())
		targetPath := filepath.Join(targetDir, entry.Name())
		if hasPackageManifest && entry.Name() == "node_modules" {
			if err := copyInstallNodeModules(sourceRoot, sourcePath, targetPath, activeDirs, runtimeDeps); err != nil {
				return err
			}
			continue
		}
		if err := copyInstallEntry(sourceRoot, sourcePath, targetPath, activeDirs); err != nil {
			return err
		}
	}

	return nil
}

func loadInstallRuntimeDependencies(sourceDir string) (map[string]struct{}, bool, error) {
	manifestPath := filepath.Join(sourceDir, "package.json")
	info, err := os.Stat(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("extension: stat package manifest %q: %w", manifestPath, err)
	}
	if info.IsDir() {
		return nil, false, fmt.Errorf("extension: package manifest %q is a directory", manifestPath)
	}

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, false, fmt.Errorf("extension: read package manifest %q: %w", manifestPath, err)
	}

	var manifest installPackageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, false, fmt.Errorf("extension: decode package manifest %q: %w", manifestPath, err)
	}

	runtimeDeps := make(map[string]struct{}, len(manifest.Dependencies)+len(manifest.OptionalDependencies))
	for name := range manifest.Dependencies {
		name = strings.TrimSpace(name)
		if name != "" {
			runtimeDeps[name] = struct{}{}
		}
	}
	for name := range manifest.OptionalDependencies {
		name = strings.TrimSpace(name)
		if name != "" {
			runtimeDeps[name] = struct{}{}
		}
	}

	return runtimeDeps, true, nil
}

func copyInstallNodeModules(
	sourceRoot string,
	sourceDir string,
	targetDir string,
	activeDirs map[string]struct{},
	runtimeDeps map[string]struct{},
) error {
	if len(runtimeDeps) == 0 {
		return nil
	}

	names := make([]string, 0, len(runtimeDeps))
	for name := range runtimeDeps {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sourcePath, err := installNodeModulePath(sourceDir, name)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(sourcePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("extension: runtime dependency %q missing from %q", name, sourceDir)
			}
			return fmt.Errorf("extension: stat runtime dependency %q in %q: %w", name, sourceDir, err)
		}
		targetPath := filepath.Join(targetDir, filepath.FromSlash(name))
		if err := copyInstallRuntimeDependency(sourceRoot, sourcePath, targetPath, activeDirs); err != nil {
			return err
		}
	}

	return nil
}

func installNodeModulePath(nodeModulesDir string, packageName string) (string, error) {
	name := strings.TrimSpace(packageName)
	if name == "" {
		return "", errors.New("extension: runtime dependency name is required")
	}
	if filepath.IsAbs(name) || strings.Contains(name, "\\") {
		return "", fmt.Errorf("extension: invalid runtime dependency name %q", packageName)
	}

	parts := strings.Split(name, "/")
	switch {
	case len(parts) == 1:
		if !validInstallPackageSegment(parts[0], false) {
			return "", fmt.Errorf("extension: invalid runtime dependency name %q", packageName)
		}
	case len(parts) == 2 && strings.HasPrefix(parts[0], "@"):
		if !validInstallPackageSegment(parts[0], true) || !validInstallPackageSegment(parts[1], false) {
			return "", fmt.Errorf("extension: invalid runtime dependency name %q", packageName)
		}
	default:
		return "", fmt.Errorf("extension: invalid runtime dependency name %q", packageName)
	}

	return filepath.Join(nodeModulesDir, filepath.FromSlash(name)), nil
}

func validInstallPackageSegment(segment string, scoped bool) bool {
	if scoped {
		return len(segment) > 1 && segment != "." && segment != ".." && !strings.Contains(segment, "/") &&
			!strings.Contains(segment, "\\")
	}
	return segment != "" && segment != "." && segment != ".." && !strings.Contains(segment, "/") &&
		!strings.Contains(segment, "\\")
}

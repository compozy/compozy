package extensionpkg

import (
	"fmt"

	"os"
	"path/filepath"

	"strings"
)

func pushInstallCopyDir(
	activeDirs map[string]struct{},
	resolvedPath string,
	sourcePath string,
) (map[string]struct{}, error) {
	canonicalPath, err := canonicalizeInstallPath(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("extension: resolve directory %q: %w", resolvedPath, err)
	}
	if _, exists := activeDirs[canonicalPath]; exists {
		return nil, fmt.Errorf("extension: symlink directory cycle detected from %q to %q", sourcePath, canonicalPath)
	}

	nextActiveDirs := make(map[string]struct{}, len(activeDirs)+1)
	for path := range activeDirs {
		nextActiveDirs[path] = struct{}{}
	}
	nextActiveDirs[canonicalPath] = struct{}{}
	return nextActiveDirs, nil
}

func ensureInstallPathWithinRoot(sourceRoot string, resolvedPath string) error {
	canonicalRoot, err := canonicalizeInstallPath(sourceRoot)
	if err != nil {
		return fmt.Errorf("resolve source root %q: %w", sourceRoot, err)
	}
	canonicalPath, err := canonicalizeInstallPath(resolvedPath)
	if err != nil {
		return fmt.Errorf("resolve source path %q: %w", resolvedPath, err)
	}

	relToRoot, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil {
		return fmt.Errorf("relate %q to source root %q: %w", canonicalPath, canonicalRoot, err)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("symlink target %q escapes source root %q", canonicalPath, canonicalRoot)
	}
	return nil
}

func canonicalizeInstallPath(path string) (string, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absPath)
}

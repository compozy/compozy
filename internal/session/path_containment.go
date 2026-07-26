package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveContainedDirectory(root string, requested string) (string, error) {
	rootPath := strings.TrimSpace(root)
	if rootPath == "" || filepath.Clean(rootPath) == "." {
		return "", fmt.Errorf("contained directory root is required")
	}

	canonicalRoot, err := canonicalDirectory(rootPath)
	if err != nil {
		return "", fmt.Errorf("canonicalize root %q: %w", rootPath, err)
	}

	targetPath := strings.TrimSpace(requested)
	if targetPath == "" {
		targetPath = rootPath
	} else if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(rootPath, targetPath)
	}
	canonicalTarget, err := canonicalDirectory(targetPath)
	if err != nil {
		return "", fmt.Errorf("canonicalize target %q: %w", targetPath, err)
	}

	contained, err := directoryContains(canonicalRoot, canonicalTarget)
	if err != nil {
		return "", fmt.Errorf("compare root and target: %w", err)
	}
	if !contained {
		return "", fmt.Errorf("target %q escapes root %q", canonicalTarget, canonicalRoot)
	}
	return canonicalTarget, nil
}

func directoryContains(root string, target string) (bool, error) {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false, err
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func canonicalDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("inspect directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", canonical)
	}
	return canonical, nil
}

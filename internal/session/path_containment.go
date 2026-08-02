package session

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
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

	contained, err := fileutil.PathWithinRoot(canonicalRoot, canonicalTarget)
	if err != nil {
		return "", fmt.Errorf("compare root and target: %w", err)
	}
	if !contained {
		return "", fmt.Errorf("target %q escapes root %q", canonicalTarget, canonicalRoot)
	}
	return canonicalTarget, nil
}

func canonicalDirectory(path string) (string, error) {
	canonical, err := fileutil.CanonicalExistingDirectory(path)
	if err != nil {
		return "", err
	}
	return canonical, nil
}

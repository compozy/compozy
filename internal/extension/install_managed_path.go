package extensionpkg

import (
	"path/filepath"
	"strings"
)

func canonicalizeInstallPath(path string) (string, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absPath)
}

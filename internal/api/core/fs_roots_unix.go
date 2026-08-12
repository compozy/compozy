//go:build !windows

package core

import "path/filepath"

func filesystemRoots() ([]string, error) {
	return []string{string(filepath.Separator)}, nil
}

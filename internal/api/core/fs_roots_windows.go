//go:build windows

package core

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func filesystemRoots() ([]string, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, fmt.Errorf("api: enumerate filesystem roots: %w", err)
	}

	roots := make([]string, 0, 26)
	for drive := 0; drive < 26; drive++ {
		if mask&(1<<drive) == 0 {
			continue
		}
		roots = append(roots, string(rune('A'+drive))+":\\")
	}
	return roots, nil
}

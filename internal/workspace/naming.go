package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

// UniqueWorkspaceName derives a stable workspace name from the root path and
// appends numeric suffixes until it no longer collides with the taken set.
func UniqueWorkspaceName(rootDir string, taken map[string]struct{}) string {
	baseName := filepath.Base(filepath.Clean(strings.TrimSpace(rootDir)))
	switch baseName {
	case "", ".", string(filepath.Separator):
		baseName = "workspace"
	}

	candidate := baseName
	for suffix := 2; ; suffix++ {
		if _, ok := taken[candidate]; !ok {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", baseName, suffix)
	}
}

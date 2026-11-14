package runtimemcp

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/compozy/compozy/internal/fileutil"
)

// Codex core-plugin-common/installed.rs prefers local, otherwise the greatest
// semver when both names parse, and lexical ordering for non-semver names.
func activeCodexPluginVersion(root string, candidates []string) string {
	versions := make([]string, 0, len(candidates))
	for _, version := range candidates {
		if !validPluginVersion(version) {
			continue
		}
		dir, err := fileutil.OpenDirectory(filepath.Join(root, version))
		if err != nil {
			continue
		}
		if dir.Close() != nil {
			continue
		}
		versions = append(versions, version)
	}
	if slices.Contains(versions, scopeLocal) {
		return scopeLocal
	}
	slices.SortFunc(versions, func(a, b string) int {
		left, leftErr := semver.StrictNewVersion(a)
		right, rightErr := semver.StrictNewVersion(b)
		if leftErr == nil && rightErr == nil {
			if result := left.Compare(right); result != 0 {
				return result
			}
		}
		return strings.Compare(a, b)
	})
	if len(versions) == 0 {
		return ""
	}
	return versions[len(versions)-1]
}

func validPluginVersion(version string) bool {
	if version == "" || version == "." || version == ".." {
		return false
	}
	for _, ch := range version {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			continue
		}
		if !strings.ContainsRune("-_.+", ch) {
			return false
		}
	}
	return true
}

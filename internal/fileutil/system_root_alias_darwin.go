//go:build darwin

package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// normalizeUnixSystemRootAlias resolves only the first component below the
// filesystem root. On macOS, aliases such as /var and /tmp are maintained by
// the operating system; every component below that boundary is still opened
// descriptor-relative with O_NOFOLLOW.
func normalizeUnixSystemRootAlias(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return path, nil
	}

	components := splitUnixPath(strings.TrimPrefix(path, string(filepath.Separator)))
	if len(components) == 0 {
		return path, nil
	}

	rootComponent := filepath.Join(string(filepath.Separator), components[0])
	expectedTarget, knownAlias := darwinSystemRootAliasTarget(rootComponent)
	if !knownAlias {
		return path, nil
	}
	info, err := os.Lstat(rootComponent)
	if err != nil {
		return "", fmt.Errorf("fileutil: inspect system root component %q: %w", rootComponent, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}

	resolved, err := filepath.EvalSymlinks(rootComponent)
	if err != nil {
		return "", fmt.Errorf("fileutil: resolve system root alias %q: %w", rootComponent, err)
	}
	if filepath.Clean(resolved) != expectedTarget {
		return "", fmt.Errorf(
			"%w: system root alias %q resolved to unexpected target %q",
			ErrSymlink,
			rootComponent,
			resolved,
		)
	}
	if len(components) == 1 {
		return resolved, nil
	}
	return filepath.Join(resolved, filepath.Join(components[1:]...)), nil
}

func darwinSystemRootAliasTarget(path string) (string, bool) {
	switch path {
	case "/etc":
		return "/private/etc", true
	case "/tmp":
		return "/private/tmp", true
	case "/var":
		return "/private/var", true
	default:
		return "", false
	}
}

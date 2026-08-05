//go:build aix || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

func normalizeUnixSystemRootAlias(path string) (string, error) {
	return path, nil
}

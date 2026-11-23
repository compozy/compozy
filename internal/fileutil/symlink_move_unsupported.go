//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package fileutil

func moveSymlinkNoReplace(*Directory, string, string) error {
	return ErrAtomicNoReplaceUnsupported
}

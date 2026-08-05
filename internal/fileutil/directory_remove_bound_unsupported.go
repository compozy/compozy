//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package fileutil

func removeBoundEmptyDirectory(*Directory, *Directory) (bool, error) {
	return false, ErrAtomicNoReplaceUnsupported
}

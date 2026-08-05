//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package fileutil

import "os"

func moveRegularFileNoReplace(*Directory, *os.File, os.FileInfo, string, string) (bool, error) {
	return false, ErrAtomicNoReplaceUnsupported
}

func removeBoundRegularFile(*Directory, *os.File, os.FileInfo, string) error {
	return ErrAtomicNoReplaceUnsupported
}

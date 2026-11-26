//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package fileutil

import (
	"errors"
	"os"
)

func tryLockFile(*os.File) (bool, error) {
	return false, errors.New("fileutil: file locks are unsupported on this platform")
}

func unlockFile(*os.File) error {
	return errors.New("fileutil: file locks are unsupported on this platform")
}

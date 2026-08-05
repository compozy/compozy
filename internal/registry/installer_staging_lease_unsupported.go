//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package registry

import (
	"errors"
	"os"
)

func tryLockInstallStagingLease(*os.File) (bool, error) {
	return false, errors.New("registry: install staging leases are unsupported on this platform")
}

func unlockInstallStagingLease(*os.File) error {
	return errors.New("registry: install staging leases are unsupported on this platform")
}

//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package fileutil

func locateDirectoryMoveRecoveryEntry(*Directory, directoryMoveIdentity) (string, error) {
	return boundDirectoryRemovalCandidate, nil
}

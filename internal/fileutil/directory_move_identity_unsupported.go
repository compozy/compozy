//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package fileutil

import "os"

type directoryMoveIdentity struct{}

func directoryMoveIdentityFromFile(*os.File) (directoryMoveIdentity, error) {
	return directoryMoveIdentity{}, nil
}

func directoryMoveIdentityMatches(directoryMoveIdentity, *os.File) (bool, error) {
	return true, nil
}

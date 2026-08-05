//go:build windows

package fileutil

import "os"

// Windows moves the exact held source handle, so a second pathname identity
// comparison is neither needed nor meaningful after the handle-bound rename.
type directoryMoveIdentity struct{}

func directoryMoveIdentityFromFile(*os.File) (directoryMoveIdentity, error) {
	return directoryMoveIdentity{}, nil
}

func directoryMoveIdentityMatches(directoryMoveIdentity, *os.File) (bool, error) {
	return true, nil
}

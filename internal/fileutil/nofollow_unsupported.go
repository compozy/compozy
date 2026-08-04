//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package fileutil

import (
	"errors"
	"os"
)

var errNoFollowUnsupported = errors.New("fileutil: no-follow filesystem access is unsupported on this platform")

func openNoFollow(string, entryExpectation, openIntent) (*os.File, error) {
	return nil, errNoFollowUnsupported
}

func openNoFollowAt(*os.File, string, entryExpectation, openIntent) (*os.File, error) {
	return nil, errNoFollowUnsupported
}

func openOrCreateDirectoryNoFollow(string, os.FileMode) (*os.File, error) {
	return nil, errNoFollowUnsupported
}

func createDirectoryNoFollowAt(*os.File, string, os.FileMode) (*os.File, error) {
	return nil, errNoFollowUnsupported
}

func createFileNoFollowAt(*os.File, string, os.FileMode) (*os.File, error) {
	return nil, errNoFollowUnsupported
}

func createFileForMoveNoFollowAt(*os.File, string, os.FileMode) (*os.File, error) {
	return nil, errNoFollowUnsupported
}

func publishNoFollowAt(*os.File, *os.File, string, string, bool) error {
	return errNoFollowUnsupported
}

func removeNoFollowAt(*os.File, string, bool) error {
	return errNoFollowUnsupported
}

func isDirectoryUnlinkRejection(error) bool {
	return false
}

func syncDirectoryHandle(*os.File) error {
	return errNoFollowUnsupported
}

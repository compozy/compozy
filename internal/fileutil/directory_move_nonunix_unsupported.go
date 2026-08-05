//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package fileutil

func moveBoundDirectory(*Directory, *Directory, string, bool) (DirectoryMoveResult, error) {
	return DirectoryMoveResult{State: DirectoryMovePreCommit}, ErrAtomicNoReplaceUnsupported
}

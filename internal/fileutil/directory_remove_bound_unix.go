//go:build aix || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

func removeBoundEmptyDirectory(directory *Directory, transaction *Directory) (bool, error) {
	beforeLinks, err := unixFileLinkCount(directory.file)
	if err != nil {
		return false, err
	}
	if err := removeNoFollowAt(transaction.file, boundDirectoryRemovalCandidate, true); err != nil {
		return false, err
	}
	afterLinks, err := unixFileLinkCount(directory.file)
	if err != nil {
		return false, err
	}
	if afterLinks >= beforeLinks {
		return false, ErrMoveSourceIdentityChanged
	}
	return true, transaction.Sync()
}

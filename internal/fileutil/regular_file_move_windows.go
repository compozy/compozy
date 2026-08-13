//go:build windows

package fileutil

import (
	"errors"
	"fmt"
	"os"
)

func moveRegularFileNoReplace(
	directory *Directory,
	_ *os.File,
	sourceInfo os.FileInfo,
	sourceName string,
	targetName string,
) (bool, error) {
	bound, err := openBoundWindowsRegularFile(directory, sourceInfo, sourceName)
	if err != nil {
		return false, err
	}
	moveErr := moveDirectoryNoFollow(
		directory.file,
		bound,
		sourceName,
		directory.file,
		targetName,
		false,
	)
	closeErr := bound.Close()
	if moveErr != nil {
		return false, errors.Join(
			fmt.Errorf("fileutil: move regular file %q to %q: %w", sourceName, targetName, moveErr),
			wrapDirectoryMovePostCommitError("close rejected regular-file move handle", closeErr),
		)
	}
	return true, errors.Join(
		wrapDirectoryMovePostCommitError("close committed regular-file move handle", closeErr),
		wrapDirectoryMovePostCommitError("sync regular-file move parent", directory.Sync()),
	)
}

func removeBoundRegularFile(
	directory *Directory,
	_ *os.File,
	sourceInfo os.FileInfo,
	sourceName string,
) error {
	bound, err := openBoundWindowsRegularFile(directory, sourceInfo, sourceName)
	if err != nil {
		return err
	}
	removeErr := removeBoundWindowsHandle(bound)
	closeErr := bound.Close()
	if removeErr != nil {
		return errors.Join(
			removeErr,
			wrapDirectoryMovePostCommitError("close rejected regular-file removal handle", closeErr),
		)
	}
	return errors.Join(
		wrapDirectoryMovePostCommitError("close removed regular-file handle", closeErr),
		wrapDirectoryMovePostCommitError("sync regular-file removal parent", directory.Sync()),
	)
}

func openBoundWindowsRegularFile(
	directory *Directory,
	sourceInfo os.FileInfo,
	sourceName string,
) (*os.File, error) {
	bound, err := directory.openChild(sourceName, entryRegularFile, openIntentMoveSource)
	if err != nil {
		return nil, err
	}
	boundInfo, err := ensureRegularFile(bound)
	if err != nil {
		return nil, closeFileWithError(bound, err)
	}
	if !os.SameFile(sourceInfo, boundInfo) {
		return nil, closeFileWithError(bound, ErrMoveSourceIdentityChanged)
	}
	return bound, nil
}

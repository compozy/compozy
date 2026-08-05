//go:build darwin

package fileutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

const darwinPathBufferSize = 1024

func removeBoundEmptyDirectory(directory *Directory, transaction *Directory) (bool, error) {
	transactionPath, err := darwinFilePath(transaction.file)
	if err != nil {
		return false, err
	}
	heldPath, err := darwinFilePath(directory.file)
	if err != nil {
		return false, err
	}
	if heldPath != filepath.Join(transactionPath, boundDirectoryRemovalCandidate) {
		return false, ErrMoveSourceIdentityChanged
	}

	if err := removeNoFollowAt(transaction.file, boundDirectoryRemovalCandidate, true); err != nil {
		return false, err
	}
	afterPath, err := darwinFilePath(directory.file)
	if err != nil {
		return false, err
	}
	if afterPath != heldPath {
		return false, ErrMoveSourceIdentityChanged
	}

	current, err := transaction.OpenDirectory(boundDirectoryRemovalCandidate)
	if errors.Is(err, os.ErrNotExist) {
		return true, transaction.Sync()
	}
	if err != nil {
		return false, err
	}
	return false, errors.Join(ErrMoveSourceIdentityChanged, current.Close())
}

func darwinFilePath(file *os.File) (string, error) {
	if file == nil {
		return "", fmt.Errorf("fileutil: get path for nil file handle: %w", ErrInvalidPath)
	}

	buffer := make([]byte, darwinPathBufferSize)
	_, err := unix.FcntlInt(
		file.Fd(),
		unix.F_GETPATH,
		int(uintptr(unsafe.Pointer(&buffer[0]))),
	)
	runtime.KeepAlive(buffer)
	if err != nil {
		return "", fmt.Errorf("fileutil: get path for file handle: %w", err)
	}
	path, _, found := bytes.Cut(buffer, []byte{0})
	if !found {
		return "", fmt.Errorf("fileutil: file handle path exceeds %d bytes", len(buffer))
	}
	return string(path), nil
}

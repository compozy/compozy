// Package fileutil provides shared filesystem helpers for Compozy components.
package fileutil

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrInvalidPath reports a path that cannot be represented safely by the filesystem boundary.
var ErrInvalidPath = errors.New("fileutil: invalid path")

// AtomicWrite writes content with the default Compozy file permissions via temp-file-and-rename.
func AtomicWrite(path string, content []byte) error {
	return AtomicWriteFile(path, content, 0o644)
}

// AtomicWriteFile writes content to path via a temp file plus atomic replacement.
// It always syncs the temp file before replacement. Parent-directory metadata
// durability remains best-effort on platforms without directory fsync support.
func AtomicWriteFile(path string, content []byte, perm os.FileMode) (err error) {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: path is required", ErrInvalidPath)
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("%w: path contains NUL byte", ErrInvalidPath)
	}

	directory, name, err := OpenParentDirectory(path)
	if err != nil {
		return fmt.Errorf("fileutil: open parent directory for %q: %w", path, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close parent directory for %q: %w", path, closeErr))
		}
	}()
	return directory.AtomicWriteFile(name, content, perm, true)
}

// AtomicRemoveFile removes a file and syncs its parent directory.
func AtomicRemoveFile(path string) (err error) {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: path is required", ErrInvalidPath)
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("%w: path contains NUL byte", ErrInvalidPath)
	}
	directory, name, err := OpenParentDirectory(path)
	if err != nil {
		return fmt.Errorf("fileutil: open parent directory for %q: %w", path, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close parent directory for %q: %w", path, closeErr))
		}
	}()
	return directory.RemoveRegularFile(name)
}

// SyncDir requests a directory metadata flush using a mutation-capable handle.
func SyncDir(path string) (err error) {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: path is required", ErrInvalidPath)
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("%w: path contains NUL byte", ErrInvalidPath)
	}
	directory, err := openDirectory(path, openIntentMutate)
	if err != nil {
		return fmt.Errorf("fileutil: open directory %q for sync: %w", path, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close directory %q after sync: %w", path, closeErr))
		}
	}()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("fileutil: sync directory %q: %w", path, err)
	}
	return nil
}

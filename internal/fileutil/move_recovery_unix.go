//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"errors"
	"fmt"
	"os"
)

func locateDirectoryMoveRecoveryEntry(
	transaction *Directory,
	want directoryMoveIdentity,
) (string, error) {
	names, err := transaction.ReadDir()
	if err != nil {
		return "", err
	}
	var scanErrs []error
	for _, name := range names {
		entry, openErr := transaction.OpenDirectory(name)
		if openErr != nil {
			scanErrs = append(scanErrs, fmt.Errorf("fileutil: inspect recovery directory %q: %w", name, openErr))
			continue
		}
		matches, matchErr := directoryMoveIdentityMatches(want, entry.file)
		closeErr := entry.Close()
		if matchErr != nil {
			scanErrs = append(scanErrs, errors.Join(matchErr, closeErr))
			continue
		}
		if closeErr != nil {
			scanErrs = append(scanErrs, closeErr)
		}
		if matches {
			return name, errors.Join(scanErrs...)
		}
	}
	return "", errors.Join(ErrMoveRecoveryEntryNotFound, errors.Join(scanErrs...))
}

func locateRegularFileMoveRecoveryEntry(
	transaction *Directory,
	want os.FileInfo,
) (string, error) {
	names, err := transaction.ReadDir()
	if err != nil {
		return "", err
	}
	var scanErrs []error
	for _, name := range names {
		entry, info, openErr := transaction.openRegularFile(name)
		if openErr != nil {
			scanErrs = append(scanErrs, fmt.Errorf("fileutil: inspect recovery file %q: %w", name, openErr))
			continue
		}
		closeErr := entry.Close()
		if closeErr != nil {
			scanErrs = append(scanErrs, fmt.Errorf("fileutil: close recovery file %q: %w", name, closeErr))
		}
		if os.SameFile(want, info) {
			return name, errors.Join(scanErrs...)
		}
	}
	return "", errors.Join(ErrMoveRecoveryEntryNotFound, errors.Join(scanErrs...))
}

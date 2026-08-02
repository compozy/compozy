//go:build linux

package fileutil

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func moveDirectoryNoFollow(
	sourceParent *os.File,
	_ *os.File,
	sourceName string,
	targetParent *os.File,
	targetName string,
	replace bool,
) error {
	sourceFD, err := unixFileDescriptor(sourceParent)
	if err != nil {
		return err
	}
	targetFD, err := unixFileDescriptor(targetParent)
	if err != nil {
		return err
	}
	if replace {
		return unix.Renameat(sourceFD, sourceName, targetFD, targetName)
	}
	if err := unix.Renameat2(sourceFD, sourceName, targetFD, targetName, unix.RENAME_NOREPLACE); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return ErrTargetExists
		}
		return err
	}
	return nil
}

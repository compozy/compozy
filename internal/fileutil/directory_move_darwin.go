//go:build darwin

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
	flags := uint32(0)
	if !replace {
		flags = unix.RENAME_EXCL
	}
	if err := unix.RenameatxNp(sourceFD, sourceName, targetFD, targetName, flags); err != nil {
		if !replace && errors.Is(err, unix.EEXIST) {
			return ErrTargetExists
		}
		return err
	}
	return nil
}

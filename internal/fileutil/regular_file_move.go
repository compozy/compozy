package fileutil

import (
	"fmt"
	"os"
)

// MoveRegularFileNoReplace moves one direct regular-file child while retaining
// the supplied source handle for platform identity binding. The committed
// result stays true when only the durability sync fails.
func (d *Directory) MoveRegularFileNoReplace(
	source *os.File,
	sourceName string,
	targetName string,
) (committed bool, err error) {
	if d == nil || d.file == nil || source == nil {
		return false, fmt.Errorf("%w: directory or source file is closed", ErrInvalidPath)
	}
	if err := validateChildName(sourceName); err != nil {
		return false, err
	}
	if err := validateChildName(targetName); err != nil {
		return false, err
	}
	sourceInfo, err := ensureRegularFile(source)
	if err != nil {
		return false, fmt.Errorf("fileutil: validate regular-file move source %q: %w", sourceName, err)
	}
	return moveRegularFileNoReplace(d, source, sourceInfo, sourceName, targetName)
}

// RemoveBoundRegularFile removes the exact regular file retained by source.
// A replacement at sourceName is rejected without removing that replacement.
func (d *Directory) RemoveBoundRegularFile(source *os.File, sourceName string) error {
	if d == nil || d.file == nil || source == nil {
		return fmt.Errorf("%w: directory or source file is closed", ErrInvalidPath)
	}
	if err := validateChildName(sourceName); err != nil {
		return err
	}
	sourceInfo, err := ensureRegularFile(source)
	if err != nil {
		return fmt.Errorf("fileutil: validate regular-file removal source %q: %w", sourceName, err)
	}
	return removeBoundRegularFile(d, source, sourceInfo, sourceName)
}

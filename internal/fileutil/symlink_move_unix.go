//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// moveSymlinkNoReplace checks the entry with fstatat(AT_SYMLINK_NOFOLLOW), then renames it through
// the held parent. An entry swapped between the two calls is still only renamed, never replaced.
func moveSymlinkNoReplace(d *Directory, sourceName, targetName string) error {
	fd, err := unixFileDescriptor(d.file)
	if err != nil {
		return err
	}
	var stat unix.Stat_t
	if err := unix.Fstatat(fd, sourceName, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("fileutil: inspect symlink %q: %w", sourceName, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFLNK {
		return fmt.Errorf("%w: %q is not a symlink", ErrInvalidPath, sourceName)
	}
	if err := moveDirectoryNoFollow(d.file, nil, sourceName, d.file, targetName, false); err != nil {
		return fmt.Errorf("fileutil: move symlink %q to %q: %w", sourceName, targetName, err)
	}
	return nil
}

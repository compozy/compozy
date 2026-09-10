//go:build darwin

package skillscan

import (
	"io/fs"
	"syscall"
)

// sameFileChangeTime detects metadata-preserving edits using Darwin's inode change timestamp.
func sameFileChangeTime(previous, current fs.FileInfo) bool {
	left, leftOK := previous.Sys().(*syscall.Stat_t)
	right, rightOK := current.Sys().(*syscall.Stat_t)
	return leftOK && rightOK && left != nil && right != nil && left.Ctimespec == right.Ctimespec
}

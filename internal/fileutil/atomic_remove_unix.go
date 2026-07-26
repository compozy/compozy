//go:build !windows

package fileutil

import "syscall"

func removeFileOnly(path string) error {
	return syscall.Unlink(path)
}

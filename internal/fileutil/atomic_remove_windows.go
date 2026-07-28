//go:build windows

package fileutil

import "syscall"

func removeFileOnly(path string) error {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.DeleteFile(pathPtr)
}

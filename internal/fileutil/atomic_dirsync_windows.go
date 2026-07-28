//go:build windows

package fileutil

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFileProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func init() {
	replaceFile = replaceFileWindows
}

func replaceFileWindows(tempPath, path string) error {
	replacedPath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	replacementPath, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}

	r1, _, callErr := replaceFileProc.Call(
		uintptr(unsafe.Pointer(replacedPath)),
		uintptr(unsafe.Pointer(replacementPath)),
		0,
		0,
		0,
		0,
	)
	if r1 != 0 {
		return nil
	}
	if errors.Is(callErr, windows.ERROR_FILE_NOT_FOUND) || errors.Is(callErr, windows.ERROR_PATH_NOT_FOUND) {
		return os.Rename(tempPath, path)
	}
	if callErr != nil && callErr != windows.ERROR_SUCCESS {
		return callErr
	}

	return syscall.EINVAL
}

// Windows does not provide a portable directory fsync path through the Go stdlib.
// Existing-target replacement uses ReplaceFileW, but directory metadata durability
// still cannot be strengthened the same way we do on Unix.
func syncDir(string) error {
	return nil
}

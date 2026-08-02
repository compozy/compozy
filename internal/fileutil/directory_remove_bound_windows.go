//go:build windows

package fileutil

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func removeBoundEmptyDirectory(directory *Directory, transaction *Directory) (bool, error) {
	if err := removeBoundWindowsHandle(directory.file); err != nil {
		return false, err
	}
	return true, transaction.Sync()
}

func removeBoundWindowsHandle(file *os.File) error {
	flags := uint32(
		windows.FILE_DISPOSITION_DELETE |
			windows.FILE_DISPOSITION_POSIX_SEMANTICS |
			windows.FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE,
	)
	var status windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(
		windows.Handle(file.Fd()),
		&status,
		(*byte)(unsafe.Pointer(&flags)),
		uint32(unsafe.Sizeof(flags)),
		windows.FileDispositionInformationEx,
	); err != nil {
		return fmt.Errorf("fileutil: remove bound Windows entry: %w", windowsOpenError(err))
	}
	return nil
}

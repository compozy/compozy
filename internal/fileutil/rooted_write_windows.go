//go:build windows

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileRenameInformation struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

func openOrCreateDirectoryNoFollow(path string, perm os.FileMode) (*os.File, error) {
	root, parts, err := windowsPathParts(path)
	if err != nil {
		return nil, err
	}
	handle, err := openWindowsRoot(root, openIntentMutate)
	if err != nil {
		return nil, err
	}

	for _, part := range parts {
		next, err := openWindowsChildWithDisposition(
			handle,
			part,
			entryDirectory,
			windows.FILE_OPEN_IF,
			windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE,
		)
		if err != nil {
			if closeErr := windows.CloseHandle(handle); closeErr != nil {
				return nil, errors.Join(err, fmt.Errorf("fileutil: close parent directory: %w", closeErr))
			}
			return nil, err
		}
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			if childCloseErr := windows.CloseHandle(next); childCloseErr != nil {
				return nil, errors.Join(
					fmt.Errorf("fileutil: close parent directory: %w", closeErr),
					fmt.Errorf("fileutil: close child directory: %w", childCloseErr),
				)
			}
			return nil, fmt.Errorf("fileutil: close parent directory: %w", closeErr)
		}
		handle = next
	}

	return windowsFile(handle, path)
}

func createDirectoryNoFollowAt(parent *os.File, name string, perm os.FileMode) (*os.File, error) {
	handle, err := openWindowsChildWithDisposition(
		windows.Handle(parent.Fd()),
		name,
		entryDirectory,
		windows.FILE_CREATE,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE,
	)
	if err != nil {
		return nil, err
	}
	return windowsFile(handle, name)
}

func createFileNoFollowAt(parent *os.File, name string, perm os.FileMode) (*os.File, error) {
	handle, err := openWindowsChildWithDisposition(
		windows.Handle(parent.Fd()),
		name,
		entryRegularFile,
		windows.FILE_CREATE,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE,
	)
	if err != nil {
		return nil, err
	}
	return windowsFile(handle, name)
}

func createFileForMoveNoFollowAt(parent *os.File, name string, perm os.FileMode) (*os.File, error) {
	handle, err := openWindowsChildWithDispositionAndOptions(
		windows.Handle(parent.Fd()),
		name,
		entryRegularFile,
		windows.FILE_CREATE,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_WRITE_THROUGH,
	)
	if err != nil {
		return nil, err
	}
	return windowsFile(handle, name)
}

func publishNoFollowAt(parent *os.File, source *os.File, sourceName string, targetName string, replace bool) error {
	targetUTF16, err := windows.UTF16FromString(targetName)
	if err != nil {
		return fmt.Errorf("fileutil: encode publication target %q: %w", targetName, err)
	}
	targetByteLength := (len(targetUTF16) - 1) * 2
	var template fileRenameInformation
	bufferSize := int(unsafe.Offsetof(template.FileName)) + targetByteLength
	buffer := make([]byte, bufferSize)
	info := (*fileRenameInformation)(unsafe.Pointer(&buffer[0]))
	if replace {
		info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	}
	info.RootDirectory = windows.Handle(parent.Fd())
	info.FileNameLength = uint32(targetByteLength)
	copy((*[windows.MAX_LONG_PATH]uint16)(unsafe.Pointer(&info.FileName[0]))[:len(targetUTF16)-1], targetUTF16)

	var status windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(
		windows.Handle(source.Fd()),
		&status,
		&buffer[0],
		uint32(bufferSize),
		windows.FileRenameInformation,
	); err != nil {
		return windowsOpenError(err)
	}
	return nil
}

func removeNoFollowAt(parent *os.File, name string, directory bool) (err error) {
	expectation := entryRegularFile
	if directory {
		expectation = entryDirectory
	}
	handle, err := openWindowsChildWithDisposition(
		windows.Handle(parent.Fd()),
		name,
		expectation,
		windows.FILE_OPEN,
		windows.DELETE|windows.SYNCHRONIZE,
	)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close deleted child %q: %w", name, closeErr))
		}
	}()

	deleteOnClose := byte(1)
	var status windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(
		handle,
		&status,
		&deleteOnClose,
		uint32(unsafe.Sizeof(deleteOnClose)),
		windows.FileDispositionInformation,
	); err != nil {
		return windowsOpenError(err)
	}
	return nil
}

func syncDirectoryHandle(file *os.File) error {
	return flushWindowsDirectoryHandle(windows.Handle(file.Fd()), windows.FlushFileBuffers)
}

func flushWindowsDirectoryHandle(handle windows.Handle, flush func(windows.Handle) error) error {
	if err := flush(handle); err != nil {
		return errors.Join(
			ErrDirectoryDurabilityUnconfirmed,
			fmt.Errorf("fileutil: flush Windows directory handle: %w", err),
		)
	}
	return nil
}

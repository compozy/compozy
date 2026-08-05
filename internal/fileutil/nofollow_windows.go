//go:build windows

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openNoFollow(path string, expectation entryExpectation, intent openIntent) (*os.File, error) {
	root, parts, err := windowsPathParts(path)
	if err != nil {
		return nil, err
	}
	rootIntent := openIntentRead
	if len(parts) == 0 {
		rootIntent = intent
	}
	handle, err := openWindowsRoot(root, rootIntent)
	if err != nil {
		return nil, err
	}

	for index, part := range parts {
		nextExpectation := entryDirectory
		nextIntent := openIntentRead
		if index == len(parts)-1 {
			nextExpectation = expectation
			nextIntent = intent
		}
		next, err := openWindowsChild(handle, part, nextExpectation, nextIntent)
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
					fmt.Errorf("fileutil: close child after parent close failure: %w", childCloseErr),
				)
			}
			return nil, fmt.Errorf("fileutil: close parent directory: %w", closeErr)
		}
		handle = next
	}

	return windowsFile(handle, path)
}

func openNoFollowAt(parent *os.File, name string, expectation entryExpectation, intent openIntent) (*os.File, error) {
	handle, err := openWindowsChild(windows.Handle(parent.Fd()), name, expectation, intent)
	if err != nil {
		return nil, err
	}
	return windowsFile(handle, name)
}

func windowsPathParts(path string) (string, []string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("fileutil: resolve absolute path: %w", err)
	}
	cleaned := filepath.Clean(absPath)
	volume := filepath.VolumeName(cleaned)
	if volume == "" {
		return "", nil, fmt.Errorf("%w: path has no volume", ErrInvalidPath)
	}

	relative := strings.TrimPrefix(cleaned, volume)
	relative = strings.TrimLeft(relative, `\\/`)
	parts := strings.FieldsFunc(relative, func(r rune) bool {
		return r == '\\' || r == '/'
	})
	return volume + `\`, parts, nil
}

func openWindowsRoot(path string, intent openIntent) (windows.Handle, error) {
	name, err := windows.NewNTUnicodeString(windowsNativePath(path))
	if err != nil {
		return 0, fmt.Errorf("fileutil: encode root directory: %w", err)
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:     uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		ObjectName: name,
		Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	return openWindowsObject(
		attributes,
		entryDirectory,
		windows.FILE_OPEN,
		windowsAccessForIntent(intent),
		windowsCreateOptionsForIntent(intent),
	)
}

func openWindowsChild(
	parent windows.Handle,
	child string,
	expectation entryExpectation,
	intent openIntent,
) (windows.Handle, error) {
	name, err := windows.NewNTUnicodeString(child)
	if err != nil {
		return 0, fmt.Errorf("fileutil: encode path component: %w", err)
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	return openWindowsObject(
		attributes,
		expectation,
		windows.FILE_OPEN,
		windowsAccessForIntent(intent),
		windowsCreateOptionsForIntent(intent),
	)
}

func openWindowsChildWithDisposition(
	parent windows.Handle,
	child string,
	expectation entryExpectation,
	disposition uint32,
	access uint32,
) (windows.Handle, error) {
	return openWindowsChildWithDispositionAndOptions(parent, child, expectation, disposition, access, 0)
}

func openWindowsChildWithDispositionAndOptions(
	parent windows.Handle,
	child string,
	expectation entryExpectation,
	disposition uint32,
	access uint32,
	extraOptions uint32,
) (windows.Handle, error) {
	name, err := windows.NewNTUnicodeString(child)
	if err != nil {
		return 0, fmt.Errorf("fileutil: encode path component: %w", err)
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	return openWindowsObject(attributes, expectation, disposition, access, extraOptions)
}

// A reparse point is opened rather than rejected here because removal never traverses one.
func openWindowsChildForRemoval(parent windows.Handle, child string, directory bool) (windows.Handle, error) {
	expectation := entryAny
	if directory {
		expectation = entryDirectory
	}
	name, err := windows.NewNTUnicodeString(child)
	if err != nil {
		return 0, fmt.Errorf("fileutil: encode path component: %w", err)
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	handle, err := createWindowsObject(
		attributes,
		expectation,
		windows.FILE_OPEN,
		windows.DELETE|windows.SYNCHRONIZE,
		0,
	)
	if err != nil {
		return 0, err
	}
	if directory {
		return handle, nil
	}
	if err := rejectWindowsPlainDirectory(handle); err != nil {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			return 0, errors.Join(err, fmt.Errorf("fileutil: close directory handle: %w", closeErr))
		}
		return 0, err
	}
	return handle, nil
}

func openWindowsObject(
	attributes *windows.OBJECT_ATTRIBUTES,
	expectation entryExpectation,
	disposition uint32,
	access uint32,
	extraOptions uint32,
) (windows.Handle, error) {
	handle, err := createWindowsObject(attributes, expectation, disposition, access, extraOptions)
	if err != nil {
		return 0, err
	}
	if err := rejectWindowsReparsePoint(handle); err != nil {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			return 0, errors.Join(err, fmt.Errorf("fileutil: close reparse point handle: %w", closeErr))
		}
		return 0, err
	}
	return handle, nil
}

func createWindowsObject(
	attributes *windows.OBJECT_ATTRIBUTES,
	expectation entryExpectation,
	disposition uint32,
	access uint32,
	extraOptions uint32,
) (windows.Handle, error) {
	options := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT) | extraOptions
	switch expectation {
	case entryDirectory:
		options |= windows.FILE_DIRECTORY_FILE
	case entryRegularFile:
		options |= windows.FILE_NON_DIRECTORY_FILE
	}

	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	var allocationSize int64
	err := windows.NtCreateFile(
		&handle,
		access,
		attributes,
		&status,
		&allocationSize,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		disposition,
		options,
		0,
		0,
	)
	if err != nil {
		return 0, windowsOpenErrorForExpectation(err, expectation)
	}

	if err := rejectWindowsReparsePoint(handle); err != nil {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			return 0, errors.Join(err, fmt.Errorf("fileutil: close reparse point handle: %w", closeErr))
		}
		return 0, err
	}
	return handle, nil
}

func windowsCreateOptionsForIntent(intent openIntent) uint32 {
	if intent == openIntentMoveSource {
		return windows.FILE_WRITE_THROUGH
	}
	return 0
}

func windowsAccessForIntent(intent openIntent) uint32 {
	switch intent {
	case openIntentRead:
		return windows.FILE_GENERIC_READ
	case openIntentMutate:
		return windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE
	case openIntentMoveSource:
		return windows.FILE_GENERIC_READ | windows.DELETE | windows.SYNCHRONIZE
	default:
		return windows.FILE_GENERIC_READ
	}
}

func windowsFile(handle windows.Handle, name string) (*os.File, error) {
	file := os.NewFile(uintptr(handle), name)
	if file != nil {
		return file, nil
	}
	if closeErr := windows.CloseHandle(handle); closeErr != nil {
		return nil, errors.Join(
			errors.New("fileutil: create file handle"),
			fmt.Errorf("fileutil: close file handle: %w", closeErr),
		)
	}
	return nil, errors.New("fileutil: create file handle")
}

func rejectWindowsReparsePoint(handle windows.Handle) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return fmt.Errorf("fileutil: inspect file handle: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return ErrSymlink
	}
	return nil
}

// Mirrors unlinkat: refuse a real directory, still detach a directory reparse point.
func rejectWindowsPlainDirectory(handle windows.Handle) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return fmt.Errorf("fileutil: inspect file handle: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 &&
		info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return ErrDirectory
	}
	return nil
}

func windowsNativePath(path string) string {
	if strings.HasPrefix(path, `\\`) {
		return `\??\UNC\` + strings.TrimPrefix(path, `\\`)
	}
	return `\??\` + path
}

func windowsOpenError(err error) error {
	status, ok := errors.AsType[windows.NTStatus](err)
	if !ok {
		return err
	}
	switch status {
	case windows.STATUS_REPARSE_POINT_ENCOUNTERED,
		windows.STATUS_REPARSE_POINT_NOT_RESOLVED,
		windows.STATUS_DIRECTORY_IS_A_REPARSE_POINT:
		return fmt.Errorf("%w: %w", ErrSymlink, status)
	default:
		return fmt.Errorf("%w: %w", status.Errno(), status)
	}
}

func windowsOpenErrorForExpectation(err error, expectation entryExpectation) error {
	if status, ok := errors.AsType[windows.NTStatus](err); ok {
		switch {
		case expectation == entryDirectory && status == windows.STATUS_NOT_A_DIRECTORY:
			return ErrNotDirectory
		case expectation == entryRegularFile && status == windows.STATUS_FILE_IS_A_DIRECTORY:
			return ErrDirectory
		}
	}
	return windowsOpenError(err)
}

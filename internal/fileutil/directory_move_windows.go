//go:build windows

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func moveDirectoryNoFollow(
	_ *os.File,
	source *os.File,
	_ string,
	targetParent *os.File,
	targetName string,
	replace bool,
) error {
	targetUTF16, err := windows.UTF16FromString(targetName)
	if err != nil {
		return fmt.Errorf("fileutil: encode publication target %q: %w", targetName, err)
	}
	if len(targetUTF16)-1 > windows.MAX_LONG_PATH {
		return fmt.Errorf("%w: publication target exceeds the Windows path limit", ErrInvalidPath)
	}
	targetByteLength := (len(targetUTF16) - 1) * 2
	var template fileRenameInformation
	bufferSize := int(unsafe.Offsetof(template.FileName)) + targetByteLength
	buffer := make([]byte, bufferSize)
	info := (*fileRenameInformation)(unsafe.Pointer(&buffer[0]))
	if replace {
		info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	}
	info.RootDirectory = windows.Handle(targetParent.Fd())
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
		if !replace && windowsDirectoryMoveTargetExists(err) {
			return ErrTargetExists
		}
		return windowsOpenError(err)
	}
	return nil
}

func moveBoundDirectory(
	source *Directory,
	target *Directory,
	targetName string,
	replace bool,
) (DirectoryMoveResult, error) {
	return moveBoundDirectoryWithSync(source, target, targetName, replace, func(directory *Directory) error {
		return directory.Sync()
	})
}

func moveBoundDirectoryWithSync(
	source *Directory,
	target *Directory,
	targetName string,
	replace bool,
	syncDirectory func(*Directory) error,
) (DirectoryMoveResult, error) {
	binding := source.move
	if err := moveDirectoryNoFollow(
		binding.parent.file,
		source.file,
		binding.name,
		target.file,
		targetName,
		replace,
	); err != nil {
		state := DirectoryMovePreCommit
		if errors.Is(err, ErrTargetExists) {
			state = DirectoryMoveCollision
		}
		return DirectoryMoveResult{
				State: state,
			}, fmt.Errorf(
				"fileutil: move directory %q to %q: %w",
				binding.name,
				targetName,
				err,
			)
	}
	return committedWindowsDirectoryMoveResult(binding.parent, target, syncDirectory), nil
}

func committedWindowsDirectoryMoveResult(
	sourceParent *Directory,
	targetParent *Directory,
	syncDirectory func(*Directory) error,
) DirectoryMoveResult {
	sourceSyncErr := syncDirectory(sourceParent)
	targetSyncErr := error(nil)
	if targetParent != sourceParent {
		targetSyncErr = syncDirectory(targetParent)
	}
	return DirectoryMoveResult{
		State: DirectoryMoveCommitted,
		PostCommitErr: errors.Join(
			wrapWindowsDirectoryMoveSyncError("source parent", sourceSyncErr),
			wrapWindowsDirectoryMoveSyncError("target parent", targetSyncErr),
		),
	}
}

func wrapWindowsDirectoryMoveSyncError(parent string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("fileutil: sync %s after committed directory move: %w", parent, err)
}

func windowsDirectoryMoveTargetExists(err error) bool {
	if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return true
	}
	status, ok := errors.AsType[windows.NTStatus](err)
	return ok && status == windows.STATUS_OBJECT_NAME_COLLISION
}

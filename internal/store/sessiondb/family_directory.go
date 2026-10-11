package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

// MoveParentDirectory preserves database identity across a handle-bound directory rename.
func (f *FamilyFile) MoveParentDirectory(
	ctx context.Context,
	owner store.SessionDBOwner,
	directory *fileutil.Directory,
	parent *fileutil.Directory,
	targetPath string,
) (fileutil.DirectoryMoveResult, error) {
	if err := f.validateActive(); err != nil {
		return fileutil.DirectoryMoveResult{}, err
	}
	if runtime.GOOS != "windows" {
		return directory.MoveTo(parent, filepath.Base(targetPath), false)
	}
	// Windows forbids directory renames while any descendant file is open.
	identity := f.pinned.info
	oldPath := f.path
	if err := f.pinned.Close(); err != nil {
		return fileutil.DirectoryMoveResult{}, err
	}
	result, moveErr := directory.MoveTo(parent, filepath.Base(targetPath), false)
	path := oldPath
	if result.Committed() {
		path = filepath.Join(targetPath, filepath.Base(oldPath))
	}
	pinned, err := f.lease.pinOwnedFile(ctx, owner, path)
	if err != nil {
		return result, errors.Join(moveErr, fmt.Errorf("store: rebind database after directory move: %w", err))
	}
	if !os.SameFile(identity, pinned.info) {
		return result, errors.Join(moveErr, pinned.closeWithError(ErrSessionDBFamilyChanged))
	}
	f.pinned = pinned
	f.path = path
	return result, moveErr
}

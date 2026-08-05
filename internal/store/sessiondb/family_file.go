package sessiondb

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

// FamilyFile retains the exact no-follow identity of one regular file in an
// active FamilyLease until its move or removal completes.
type FamilyFile struct {
	lease  *FamilyLease
	path   string
	pinned *pinnedSessionDBFile
}

// BindFile binds one regular file in the leased family directory.
func (l *FamilyLease) BindFile(path string) (*FamilyFile, error) {
	pinned, cleanPath, err := l.pinFile(path)
	if err != nil {
		return nil, err
	}
	return &FamilyFile{lease: l, path: cleanPath, pinned: pinned}, nil
}

// BindOwnedFile binds one SQLite database after verifying its exact owner.
func (l *FamilyLease) BindOwnedFile(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (*FamilyFile, error) {
	pinned, err := l.pinOwnedFile(ctx, owner, path)
	if err != nil {
		return nil, err
	}
	cleanPath, err := l.validateOwnedCandidatePath(path)
	if err != nil {
		return nil, pinned.closeWithError(err)
	}
	return &FamilyFile{lease: l, path: cleanPath, pinned: pinned}, nil
}

// SHA256 returns the digest of the held file without changing its seek offset.
func (f *FamilyFile) SHA256() ([sha256.Size]byte, error) {
	if f == nil || f.pinned == nil || f.pinned.file == nil || f.pinned.info == nil {
		return [sha256.Size]byte{}, errors.New("store: bound session database family file is required")
	}
	digest := sha256.New()
	reader := io.NewSectionReader(f.pinned.file, 0, f.pinned.info.Size())
	if _, err := io.Copy(digest, reader); err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("store: digest session database family file %q: %w", f.path, err)
	}
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

// VerifyOwnerAt proves that path still names the held file and that its SQLite
// owner remains exact, then advances the capability to that path.
func (f *FamilyFile) VerifyOwnerAt(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) error {
	if err := f.validateActive(); err != nil {
		return err
	}
	cleanPath, err := f.lease.validateOwnedCandidatePath(path)
	if err != nil {
		return err
	}
	if err := f.pinned.RequireCurrent(cleanPath); err != nil {
		return err
	}
	if err := f.lease.VerifyOwner(ctx, owner, cleanPath); err != nil {
		return err
	}
	if err := f.pinned.RequireCurrent(cleanPath); err != nil {
		return err
	}
	f.path = cleanPath
	return nil
}

// RequireSameFile proves that another no-follow handle refers to the exact
// regular file retained by this capability.
func (f *FamilyFile) RequireSameFile(file *os.File) error {
	if err := f.validateActive(); err != nil {
		return err
	}
	if file == nil {
		return errors.New("store: comparison session database family file is required")
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("store: stat comparison session database family file: %w", err)
	}
	if !os.SameFile(f.pinned.info, info) {
		return ErrSessionDBFamilyChanged
	}
	return nil
}

// MoveTo moves the exact bound file to another name in the leased directory.
// A path substitution is rolled back and reported as ErrSessionDBFamilyChanged.
func (f *FamilyFile) MoveTo(targetPath string) (committed bool, retErr error) {
	if err := f.validateActive(); err != nil {
		return false, err
	}
	cleanTarget, err := f.lease.validateCandidatePath(targetPath)
	if err != nil {
		return false, err
	}
	if filepath.Dir(cleanTarget) != filepath.Dir(f.path) {
		return false, fmt.Errorf(
			"store: session database family move crosses directories: %q to %q",
			f.path,
			cleanTarget,
		)
	}
	if err := f.pinned.RequireCurrent(f.path); err != nil {
		return false, err
	}
	directory, sourceName, err := fileutil.OpenParentDirectory(f.path)
	if err != nil {
		return false, fmt.Errorf("store: open session database family directory for move: %w", err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("store: close session database family move directory: %w", closeErr),
			)
		}
	}()
	committed, moveErr := directory.MoveRegularFileNoReplace(
		f.pinned.file,
		sourceName,
		filepath.Base(cleanTarget),
	)
	if !committed {
		if errors.Is(moveErr, fileutil.ErrMoveSourceIdentityChanged) {
			moveErr = errors.Join(ErrSessionDBFamilyChanged, moveErr)
		}
		return false, moveErr
	}
	f.path = cleanTarget
	if identityErr := f.pinned.RequireCurrent(cleanTarget); identityErr != nil {
		return true, errors.Join(identityErr, moveErr)
	}
	return true, moveErr
}

// Remove removes the exact bound file through its retained identity.
func (f *FamilyFile) Remove() error {
	if err := f.validateActive(); err != nil {
		return err
	}
	directory, sourceName, err := fileutil.OpenParentDirectory(f.path)
	if err != nil {
		return fmt.Errorf("store: open session database family directory for removal: %w", err)
	}
	removeErr := directory.RemoveBoundRegularFile(f.pinned.file, sourceName)
	if errors.Is(removeErr, fileutil.ErrMoveSourceIdentityChanged) {
		removeErr = errors.Join(ErrSessionDBFamilyChanged, removeErr)
	}
	directoryCloseErr := directory.Close()
	closeErr := f.Close()
	if removeErr != nil {
		return errors.Join(
			fmt.Errorf("store: remove bound session database family file %q: %w", f.path, removeErr),
			wrapFamilyFileDirectoryCloseError(directoryCloseErr),
			closeErr,
		)
	}
	return errors.Join(wrapFamilyFileDirectoryCloseError(directoryCloseErr), closeErr)
}

func wrapFamilyFileDirectoryCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("store: close session database family directory: %w", err)
}

// Close releases the held file identity without mutating it.
func (f *FamilyFile) Close() error {
	if f == nil || f.pinned == nil {
		return nil
	}
	pinned := f.pinned
	f.pinned = nil
	return pinned.Close()
}

func (f *FamilyFile) validateActive() error {
	if f == nil || f.lease == nil || f.pinned == nil || f.pinned.file == nil {
		return errors.New("store: bound session database family file is required")
	}
	if !f.lease.active.Load() {
		return errors.New("store: active session database family lease is required")
	}
	return nil
}

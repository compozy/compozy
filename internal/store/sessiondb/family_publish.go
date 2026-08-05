package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

// PublishFreshOwnedDatabase publishes one complete empty SessionDB while the
// caller retains exclusive ownership of the family lease.
func (l *FamilyLease) PublishFreshOwnedDatabase(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (retErr error) {
	if ctx == nil {
		return errors.New("store: publish fresh session database context is required")
	}
	if l == nil || !l.active.Load() {
		return errors.New("store: active session database family lease is required")
	}
	normalizedOwner, err := owner.Normalize()
	if err != nil {
		return err
	}
	cleanPath, err := l.validateCandidatePath(path)
	if err != nil {
		return err
	}
	if cleanPath != l.path {
		return fmt.Errorf("store: fresh session database path %q is not leased path %q", cleanPath, l.path)
	}
	parent, targetName, err := fileutil.OpenParentDirectory(cleanPath)
	if err != nil {
		return fmt.Errorf("store: open fresh session database parent: %w", err)
	}
	defer func() {
		if closeErr := parent.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("store: close fresh session database parent: %w", closeErr))
		}
	}()
	if file, err := parent.OpenRegularFile(targetName); err == nil {
		closeErr := file.Close()
		return errors.Join(
			fmt.Errorf("store: fresh session database target %q already exists", cleanPath),
			closeErr,
		)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("store: inspect fresh session database target %q: %w", cleanPath, err)
	}

	contents, err := buildSessionSQLiteCandidate(ctx, normalizedOwner, vacuumSessionSQLite)
	if err != nil {
		return err
	}
	if err := parent.AtomicWriteFile(filepath.Base(cleanPath), contents, 0o600, false); err != nil {
		return fmt.Errorf("store: publish fresh session database %q: %w", cleanPath, err)
	}
	bound, err := l.BindOwnedFile(ctx, normalizedOwner, cleanPath)
	if err != nil {
		return err
	}
	return bound.Close()
}

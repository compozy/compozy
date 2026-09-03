package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

type sessionDeleteCapabilities struct {
	familyLease     *sessiondb.FamilyLease
	database        *sessiondb.FamilyFile
	parentDirectory *fileutil.Directory
	directory       *fileutil.Directory
	release         func() error
}

func (m *Manager) acquireSessionDeleteCapabilities(
	ctx context.Context,
	owner store.SessionDBOwner,
	originalPath string,
) (*sessionDeleteCapabilities, error) {
	return m.acquireSessionDeleteCapabilitiesAt(ctx, owner, originalPath, originalPath)
}

func (m *Manager) acquireSessionDeleteCapabilitiesAt(
	ctx context.Context,
	owner store.SessionDBOwner,
	canonicalPath string,
	boundPath string,
) (*sessionDeleteCapabilities, error) {
	canonicalDBPath := store.SessionDBFile(canonicalPath)
	boundDBPath := store.SessionDBFile(boundPath)
	familyLease, resumeQueries, err := m.acquireOwnedSessionDBFamily(
		ctx,
		owner,
		canonicalDBPath,
		boundDBPath,
	)
	if err != nil {
		return nil, fmt.Errorf("session: acquire owned event store for delete %q: %w", owner.SessionID, err)
	}
	capabilities := &sessionDeleteCapabilities{familyLease: familyLease}
	capabilities.release = sync.OnceValue(func() error {
		closeErr := errors.Join(
			capabilities.database.Close(),
			capabilities.directory.Close(),
			capabilities.parentDirectory.Close(),
		)
		familyLease.Release()
		resumeQueries()
		return closeErr
	})

	capabilities.database, err = familyLease.BindOwnedFile(ctx, owner, boundDBPath)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("session: bind owned event store for delete %q: %w", owner.SessionID, err),
			capabilities.Release(),
		)
	}
	capabilities.parentDirectory, err = fileutil.OpenDirectoryForMutation(m.homePaths.SessionsDir)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("session: open sessions directory for delete %q: %w", owner.SessionID, err),
			capabilities.Release(),
		)
	}
	capabilities.directory, err = capabilities.parentDirectory.OpenDirectoryForMove(filepath.Base(boundPath))
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("session: bind session directory %q for delete: %w", boundPath, err),
			capabilities.Release(),
		)
	}
	if err := requireSessionDBInDirectory(capabilities.database, capabilities.directory); err != nil {
		return nil, errors.Join(
			fmt.Errorf("session: bind owned event store to directory %q: %w", boundPath, err),
			capabilities.Release(),
		)
	}
	return capabilities, nil
}

func stageBoundSessionDelete(
	ctx context.Context,
	capabilities *sessionDeleteCapabilities,
	owner store.SessionDBOwner,
	originalPath string,
	stagedPath string,
) error {
	result, moveErr := capabilities.directory.MoveTo(
		capabilities.parentDirectory,
		filepath.Base(stagedPath),
		false,
	)
	if !result.Committed() {
		return fmt.Errorf(
			"session: stage session directory %q for deletion: %w",
			originalPath,
			errors.Join(moveErr, result.PostCommitErr, capabilities.Release()),
		)
	}
	verifyErr := capabilities.database.VerifyOwnerAt(ctx, owner, store.SessionDBFile(stagedPath))
	if moveErr == nil && result.PostCommitErr == nil && verifyErr == nil {
		return nil
	}
	rollbackResult, rollbackErr := capabilities.directory.MoveTo(
		capabilities.parentDirectory,
		filepath.Base(originalPath),
		false,
	)
	var restoredOwnerErr error
	if rollbackResult.Committed() {
		restoredOwnerErr = capabilities.database.VerifyOwnerAt(ctx, owner, store.SessionDBFile(originalPath))
	}
	return fmt.Errorf(
		"session: durably stage session directory %q for deletion: %w",
		originalPath,
		errors.Join(
			moveErr,
			result.PostCommitErr,
			verifyErr,
			rollbackErr,
			rollbackResult.PostCommitErr,
			restoredOwnerErr,
			capabilities.Release(),
		),
	)
}

func (c *sessionDeleteCapabilities) valid() bool {
	return c != nil && c.familyLease != nil && c.database != nil &&
		c.parentDirectory != nil && c.directory != nil && c.release != nil
}

func (c *sessionDeleteCapabilities) Release() error {
	if c == nil || c.release == nil {
		return nil
	}
	return c.release()
}

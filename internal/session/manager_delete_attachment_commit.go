package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/compozy/compozy/internal/fileutil"
)

func (m *Manager) commitStagedAttachmentDelete(
	ctx context.Context,
	entry *stagedAttachmentDelete,
) error {
	if entry == nil || entry.phase == deleteCommitCleaned {
		return nil
	}
	if entry.phase != deleteCommitStaged && entry.phase != deleteCommitLogical {
		return fmt.Errorf("session: invalid attachment deletion phase %d", entry.phase)
	}
	if entry.originalPath == "" {
		entry.MarkDeleted()
		entry.phase = deleteCommitCleaned
		return releaseAttachmentDeleteCapabilities(entry)
	}

	var commitErr error
	if entry.phase == deleteCommitStaged {
		if err := m.ensureAttachmentDeleteCapabilities(ctx, entry, entry.stagedPath, true); err != nil {
			return err
		}
		result, moveErr := entry.directory.MoveTo(
			entry.parent,
			filepath.Base(entry.committedPath),
			false,
		)
		if result.Committed() {
			entry.phase = deleteCommitLogical
			entry.MarkDeleted()
		}
		commitErr = errors.Join(moveErr, result.PostCommitErr, releaseAttachmentDeleteCapabilities(entry))
		if !result.Committed() {
			return commitErr
		}
	}

	if err := m.ensureAttachmentDeleteCapabilities(ctx, entry, entry.committedPath, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			entry.phase = deleteCommitCleaned
			return commitErr
		}
		return errors.Join(commitErr, err)
	}
	removeErr := entry.directory.RemoveBoundTree()
	if removeErr == nil {
		entry.phase = deleteCommitCleaned
	}
	return errors.Join(commitErr, removeErr, releaseAttachmentDeleteCapabilities(entry))
}

func (m *Manager) ensureAttachmentDeleteCapabilities(
	ctx context.Context,
	entry *stagedAttachmentDelete,
	boundPath string,
	withLease bool,
) error {
	if entry.parent != nil && entry.directory != nil {
		return nil
	}
	if withLease && m.attachmentScopeLease != nil {
		lease, err := m.attachmentScopeLease.AcquireScopeLease(ctx, entry.workspaceID, entry.sessionID)
		if err != nil {
			return fmt.Errorf("session: reacquire attachment deletion lease: %w", err)
		}
		entry.lease = lease
	}
	parent, err := fileutil.OpenDirectoryForMutation(filepath.Dir(entry.originalPath))
	if err != nil {
		return errors.Join(err, releaseAttachmentDeleteCapabilities(entry))
	}
	directory, err := parent.OpenDirectoryForMove(filepath.Base(boundPath))
	if err != nil {
		return errors.Join(err, parent.Close(), releaseAttachmentDeleteCapabilities(entry))
	}
	entry.parent = parent
	entry.directory = directory
	entry.release = sync.OnceValue(func() error {
		return errors.Join(directory.Close(), parent.Close())
	})
	return nil
}

func releaseAttachmentDeleteCapabilities(entry *stagedAttachmentDelete) error {
	if entry == nil {
		return nil
	}
	err := entry.Release()
	entry.parent = nil
	entry.directory = nil
	entry.release = nil
	entry.lease = nil
	return err
}

package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

type deleteCommitPhase uint8

const (
	deleteCommitStaged deleteCommitPhase = iota
	deleteCommitLogical
	deleteCommitCleaned
)

func (m *Manager) commitStagedSessionDeletes(ctx context.Context, staged []stagedSessionDelete) error {
	var cleanupErr error
	for i := range staged {
		cleanupErr = errors.Join(cleanupErr, m.commitStagedSessionDelete(ctx, &staged[i]))
	}
	return cleanupErr
}

func (m *Manager) commitStagedSessionDelete(ctx context.Context, entry *stagedSessionDelete) error {
	if entry == nil || entry.phase == deleteCommitCleaned {
		return nil
	}
	if entry.phase != deleteCommitStaged && entry.phase != deleteCommitLogical {
		return fmt.Errorf("session: invalid deletion phase %d", entry.phase)
	}

	var commitErr error
	if entry.phase == deleteCommitStaged {
		if err := m.ensureSessionDeleteCapabilities(ctx, entry, entry.stagedPath); err != nil {
			return err
		}
		if err := verifyStagedSessionDelete(ctx, *entry); err != nil {
			return errors.Join(err, releaseSessionDeleteCapabilities(entry))
		}
		result, moveErr := entry.capabilities.directory.MoveTo(
			entry.capabilities.parentDirectory,
			filepath.Base(entry.committedPath),
			false,
		)
		if result.Committed() {
			entry.phase = deleteCommitLogical
		}
		commitErr = errors.Join(moveErr, result.PostCommitErr, releaseSessionDeleteCapabilities(entry))
		if !result.Committed() {
			return commitErr
		}
	}

	m.publishCommittedSessionDelete(entry)
	attachmentErr := m.commitStagedAttachmentDelete(ctx, entry.attachments)
	commitErr = errors.Join(commitErr, attachmentErr)
	if attachmentErr != nil || entry.windowReconciliationPending {
		return commitErr
	}
	if err := m.ensureSessionDeleteCapabilities(ctx, entry, entry.committedPath); err != nil {
		return errors.Join(commitErr, err)
	}
	verifyErr := entry.capabilities.database.VerifyOwnerAt(
		ctx,
		entry.owner,
		store.SessionDBFile(entry.committedPath),
	)
	if verifyErr != nil {
		return errors.Join(commitErr, verifyErr, releaseSessionDeleteCapabilities(entry))
	}
	removeErr := m.removeStagedSessionDelete(*entry, entry.committedPath)
	if removeErr == nil {
		entry.phase = deleteCommitCleaned
	}
	return errors.Join(commitErr, removeErr, releaseSessionDeleteCapabilities(entry))
}

func (m *Manager) ensureSessionDeleteCapabilities(
	ctx context.Context,
	entry *stagedSessionDelete,
	boundPath string,
) error {
	if entry.capabilities != nil && entry.capabilities.valid() {
		return nil
	}
	capabilities, err := m.acquireSessionDeleteCapabilitiesAt(
		ctx,
		entry.owner,
		entry.originalPath,
		boundPath,
	)
	if err != nil {
		return fmt.Errorf("session: reacquire deletion tombstone %q: %w", boundPath, err)
	}
	entry.capabilities = capabilities
	return nil
}

func releaseSessionDeleteCapabilities(entry *stagedSessionDelete) error {
	if entry == nil || entry.capabilities == nil {
		return nil
	}
	err := entry.capabilities.Release()
	entry.capabilities = nil
	return err
}

func (m *Manager) publishCommittedSessionDelete(entry *stagedSessionDelete) {
	if entry == nil || entry.logicalRemovalPublished || entry.info == nil {
		return
	}
	m.publishWaitSessionGone(entry.info)
	m.remove(entry.info.ID)
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventDeleted, entry.info))
	entry.logicalRemovalPublished = true
}

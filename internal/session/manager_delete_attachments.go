package session

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/compozy/compozy/internal/fileutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	workspaceAttachmentDeletePrefix          = ".compozy-workspace-delete-"
	workspaceAttachmentDeleteStagedPrefix    = workspaceAttachmentDeletePrefix + "staged-"
	workspaceAttachmentDeleteCommittedPrefix = workspaceAttachmentDeletePrefix + "committed-"
)

type stagedAttachmentDelete struct {
	workspaceID   string
	sessionID     string
	originalPath  string
	stagedPath    string
	committedPath string
	parent        *fileutil.Directory
	directory     *fileutil.Directory
	release       func() error
	lease         attachmentspkg.ScopeLeaseGuard
	phase         deleteCommitPhase
}

type attachmentDeleteStageSpec struct {
	workspaceID   string
	sessionID     string
	parentPath    string
	target        string
	stagedName    string
	committedName string
}

func (m *Manager) stageSessionAttachmentDelete(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	deletionID string,
) (*stagedAttachmentDelete, error) {
	parentPath := filepath.Join(m.homePaths.SessionAttachmentsDir, workspaceID)
	return m.stageAttachmentDirectoryDeleteWithLease(ctx, attachmentDeleteStageSpec{
		workspaceID:   workspaceID,
		sessionID:     sessionID,
		parentPath:    parentPath,
		target:        sessionID,
		stagedName:    sessionDeleteTombstoneName(sessionDeleteStagedPrefix, sessionID, deletionID),
		committedName: sessionDeleteTombstoneName(sessionDeleteCommittedPrefix, sessionID, deletionID),
	})
}

func (m *Manager) stageWorkspaceAttachmentDelete(
	ctx context.Context,
	workspaceID string,
	deletionID string,
) (*stagedAttachmentDelete, error) {
	return m.stageAttachmentDirectoryDeleteWithLease(ctx, attachmentDeleteStageSpec{
		workspaceID: workspaceID,
		parentPath:  m.homePaths.SessionAttachmentsDir,
		target:      workspaceID,
		stagedName:  attachmentWorkspaceTombstoneName(workspaceAttachmentDeleteStagedPrefix, workspaceID, deletionID),
		committedName: attachmentWorkspaceTombstoneName(
			workspaceAttachmentDeleteCommittedPrefix,
			workspaceID,
			deletionID,
		),
	})
}

func (m *Manager) stageAttachmentDirectoryDeleteWithLease(
	ctx context.Context,
	spec attachmentDeleteStageSpec,
) (*stagedAttachmentDelete, error) {
	var lease attachmentspkg.ScopeLeaseGuard
	if m.attachmentScopeLease != nil {
		var err error
		lease, err = m.attachmentScopeLease.AcquireScopeLease(
			ctx,
			spec.workspaceID,
			spec.sessionID,
		)
		if err != nil {
			return nil, fmt.Errorf("session: acquire attachment deletion lease: %w", err)
		}
	}
	entry, err := m.stageAttachmentDirectoryDelete(
		spec.parentPath,
		spec.target,
		spec.stagedName,
		spec.committedName,
	)
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		return nil, err
	}
	if entry == nil {
		entry = &stagedAttachmentDelete{}
	}
	entry.workspaceID = spec.workspaceID
	entry.sessionID = spec.sessionID
	entry.lease = lease
	return entry, nil
}

func (m *Manager) stageAttachmentDirectoryDelete(
	parentPath string,
	target string,
	stagedName string,
	committedName string,
) (*stagedAttachmentDelete, error) {
	parent, err := fileutil.OpenDirectoryForMutation(parentPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("session: open attachment deletion parent %q: %w", parentPath, err)
	}
	directory, err := parent.OpenDirectoryForMove(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, parent.Close()
	}
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf(
				"session: bind attachment directory %q for deletion: %w",
				filepath.Join(parentPath, target),
				err,
			),
			parent.Close(),
		)
	}
	entry := &stagedAttachmentDelete{
		originalPath:  filepath.Join(parentPath, target),
		stagedPath:    filepath.Join(parentPath, stagedName),
		committedPath: filepath.Join(parentPath, committedName),
		parent:        parent,
		directory:     directory,
	}
	entry.release = sync.OnceValue(func() error {
		return errors.Join(directory.Close(), parent.Close())
	})

	result, moveErr := directory.MoveTo(parent, stagedName, false)
	if !result.Committed() {
		return nil, errors.Join(
			fmt.Errorf("session: stage attachment directory %q for deletion: %w", entry.originalPath, moveErr),
			result.PostCommitErr,
			entry.Release(),
		)
	}
	if moveErr == nil && result.PostCommitErr == nil {
		return entry, nil
	}
	rollbackResult, rollbackErr := directory.MoveTo(parent, target, false)
	return nil, errors.Join(
		fmt.Errorf("session: durably stage attachment directory %q for deletion: %w", entry.originalPath, moveErr),
		result.PostCommitErr,
		rollbackErr,
		rollbackResult.PostCommitErr,
		entry.Release(),
	)
}

func rollbackStagedAttachmentDelete(entry *stagedAttachmentDelete) error {
	if entry == nil {
		return nil
	}
	if entry.parent == nil && entry.directory == nil {
		return entry.Release()
	}
	if entry.parent == nil || entry.directory == nil {
		return errors.New("session: staged attachment deletion capabilities are required")
	}
	result, moveErr := entry.directory.MoveTo(entry.parent, filepath.Base(entry.originalPath), false)
	return errors.Join(moveErr, result.PostCommitErr, entry.Release())
}

func (entry *stagedAttachmentDelete) Release() error {
	if entry == nil {
		return nil
	}
	var err error
	if entry.release != nil {
		err = entry.release()
	}
	if entry.lease != nil {
		entry.lease.Release()
	}
	return err
}

func (entry *stagedAttachmentDelete) MarkDeleted() {
	if entry != nil && entry.lease != nil {
		entry.lease.MarkDeleted()
	}
}

func (m *Manager) recoverSessionAttachmentDelete(
	workspaceID string,
	sessionID string,
	deletionID string,
	restore bool,
) error {
	parentPath := filepath.Join(m.homePaths.SessionAttachmentsDir, workspaceID)
	names := []string{
		sessionDeleteTombstoneName(sessionDeleteStagedPrefix, sessionID, deletionID),
		sessionDeleteTombstoneName(sessionDeleteCommittedPrefix, sessionID, deletionID),
	}
	return recoverAttachmentDelete(parentPath, sessionID, names, restore)
}

func recoverAttachmentDelete(
	parentPath string,
	target string,
	tombstoneNames []string,
	restore bool,
) (retErr error) {
	parent, err := fileutil.OpenDirectoryForMutation(parentPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, parent.Close())
	}()

	for _, name := range tombstoneNames {
		directory, openErr := parent.OpenDirectoryForMove(name)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return openErr
		}
		if restore {
			result, moveErr := directory.MoveTo(parent, target, false)
			return errors.Join(moveErr, result.PostCommitErr, directory.Close())
		}
		return errors.Join(directory.RemoveBoundTree(), directory.Close())
	}
	if restore {
		return nil
	}
	directory, err := parent.OpenDirectoryForMove(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.Join(directory.RemoveBoundTree(), directory.Close())
}

func attachmentWorkspaceTombstoneName(prefix string, workspaceID string, deletionID string) string {
	encodedWorkspace := base64.RawURLEncoding.EncodeToString([]byte(workspaceID))
	return prefix + encodedWorkspace + "." + deletionID
}

func parseAttachmentWorkspaceTombstone(name string) (sessionDeleteTombstoneState, string, error) {
	cleanName := strings.TrimSpace(name)
	state := sessionDeleteTombstoneState(0)
	value := ""
	switch {
	case strings.HasPrefix(cleanName, workspaceAttachmentDeleteStagedPrefix):
		state = sessionDeleteTombstoneStaged
		value = strings.TrimPrefix(cleanName, workspaceAttachmentDeleteStagedPrefix)
	case strings.HasPrefix(cleanName, workspaceAttachmentDeleteCommittedPrefix):
		state = sessionDeleteTombstoneCommitted
		value = strings.TrimPrefix(cleanName, workspaceAttachmentDeleteCommittedPrefix)
	default:
		return 0, "", fmt.Errorf("session: invalid workspace attachment tombstone name %q", name)
	}
	separator := strings.LastIndexByte(value, '.')
	if separator <= 0 || separator == len(value)-1 {
		return 0, "", fmt.Errorf("session: invalid workspace attachment tombstone name %q", name)
	}
	decodedWorkspace, err := base64.RawURLEncoding.DecodeString(value[:separator])
	if err != nil {
		return 0, "", fmt.Errorf("session: decode workspace attachment tombstone %q: %w", name, err)
	}
	workspaceID := strings.TrimSpace(string(decodedWorkspace))
	if workspaceID == "" || workspaceID == "." || workspaceID == ".." ||
		strings.ContainsAny(workspaceID, `/\\`) {
		return 0, "", fmt.Errorf("session: invalid workspace attachment tombstone target %q", workspaceID)
	}
	return state, workspaceID, nil
}

func (m *Manager) cleanupWorkspaceAttachmentDeleteTombstones() {
	entries, err := os.ReadDir(m.homePaths.SessionAttachmentsDir)
	if err != nil {
		m.logger.Warn("session: scan workspace attachment deletion tombstones", "error", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), workspaceAttachmentDeletePrefix) {
			continue
		}
		if err := m.cleanupWorkspaceAttachmentDeleteTombstone(entry.Name()); err != nil {
			m.logger.Warn(
				"session: retain workspace attachment deletion tombstone for later retry",
				"path", filepath.Join(m.homePaths.SessionAttachmentsDir, entry.Name()),
				"error", err,
			)
		}
	}
}

type workspaceCatalogReader interface {
	GetWorkspace(context.Context, string) (workspacepkg.Workspace, error)
}

func (m *Manager) cleanupWorkspaceAttachmentDeleteTombstone(name string) error {
	state, workspaceID, err := parseAttachmentWorkspaceTombstone(name)
	if err != nil {
		return err
	}
	restore := state == sessionDeleteTombstoneStaged
	if reader, ok := m.sessionCatalog.(workspaceCatalogReader); ok && restore {
		ctx, cancel := m.lifecycleCleanupContext()
		defer cancel()
		_, lookupErr := reader.GetWorkspace(ctx, workspaceID)
		switch {
		case lookupErr == nil:
		case errors.Is(lookupErr, workspacepkg.ErrWorkspaceNotFound):
			restore = false
		default:
			return lookupErr
		}
	}
	return recoverAttachmentDelete(m.homePaths.SessionAttachmentsDir, workspaceID, []string{name}, restore)
}

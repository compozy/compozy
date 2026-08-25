package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	sessionDeleteTombstonePrefix = ".compozy-delete-"
	sessionDeleteStagedPrefix    = sessionDeleteTombstonePrefix + "staged-"
	sessionDeleteCommittedPrefix = sessionDeleteTombstonePrefix + "committed-"
)

type stagedSessionDelete struct {
	info                        *Info
	owner                       store.SessionDBOwner
	originalPath                string
	stagedPath                  string
	committedPath               string
	capabilities                *sessionDeleteCapabilities
	attachments                 *stagedAttachmentDelete
	windowReconciliationPending bool
}

type workspaceUnregisterPreparation struct {
	manager                      *Manager
	staged                       []stagedSessionDelete
	attachments                  *stagedAttachmentDelete
	conversationOperationUnlocks []func()
	release                      sync.Once
}

var _ workspacepkg.UnregisterPreparation = (*workspaceUnregisterPreparation)(nil)

type workspaceRemovalStage struct {
	sessions    []stagedSessionDelete
	attachments *stagedAttachmentDelete
}

// PrepareWorkspaceRemoval atomically stages every stopped session directory so
// the workspace store can delete its database-owned rows without orphaning
// transcript state. Commit or Rollback releases the serialized deletion lease.
func workspaceRemovalTarget(ctx context.Context, m *Manager, workspaceID string) (string, error) {
	if m == nil {
		return "", errors.New("session: manager is required")
	}
	if ctx == nil {
		return "", errors.New("session: workspace removal context is required")
	}
	targetWorkspace := strings.TrimSpace(workspaceID)
	if targetWorkspace == "" {
		return "", errors.New("session: workspace id is required")
	}
	return targetWorkspace, nil
}

func (m *Manager) PrepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
) (workspacepkg.UnregisterPreparation, error) {
	targetWorkspace, err := workspaceRemovalTarget(ctx, m, workspaceID)
	if err != nil {
		return nil, err
	}

	ctx, lockedSessionIDs, operationUnlocks, err := m.lockWorkspaceConversationOperations(ctx, targetWorkspace)
	if err != nil {
		return nil, err
	}

	m.lifecycleMu.Lock()
	if sessionID, pending := m.pendingSessionForWorkspace(targetWorkspace); pending {
		m.lifecycleMu.Unlock()
		releaseConversationOperations(operationUnlocks)
		return nil, fmt.Errorf(
			"session: remove workspace %q: %w: %s",
			targetWorkspace,
			workspacepkg.ErrWorkspaceHasActiveSessions,
			sessionID,
		)
	}
	infos, err := m.workspaceRemovalInfos(ctx, targetWorkspace, lockedSessionIDs)
	if err != nil {
		m.lifecycleMu.Unlock()
		releaseConversationOperations(operationUnlocks)
		return nil, err
	}
	stage, err := m.stageWorkspaceRemoval(ctx, targetWorkspace, infos)
	if err != nil {
		m.lifecycleMu.Unlock()
		releaseConversationOperations(operationUnlocks)
		return nil, err
	}

	return &workspaceUnregisterPreparation{
		manager:                      m,
		staged:                       stage.sessions,
		attachments:                  stage.attachments,
		conversationOperationUnlocks: operationUnlocks,
	}, nil
}

func (m *Manager) stageWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
	infos []*Info,
) (workspaceRemovalStage, error) {
	deletionID, err := store.NewID("")
	if err != nil {
		return workspaceRemovalStage{}, fmt.Errorf(
			"session: reserve workspace attachment deletion identity: %w",
			err,
		)
	}
	attachments, err := m.stageWorkspaceAttachmentDelete(ctx, workspaceID, deletionID)
	if err != nil {
		return workspaceRemovalStage{}, err
	}
	stage := workspaceRemovalStage{attachments: attachments}
	rollback := func(cause error) (workspaceRemovalStage, error) {
		return workspaceRemovalStage{}, errors.Join(
			cause,
			m.rollbackStagedSessionDeletes(ctx, stage.sessions),
			rollbackStagedAttachmentDelete(stage.attachments),
		)
	}
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.WorkspaceID) != workspaceID {
			continue
		}
		if info.State == StateStarting || info.State == StateActive || info.State == StateStopping {
			return rollback(fmt.Errorf(
				"session: remove workspace %q: %w: %s",
				workspaceID,
				workspacepkg.ErrWorkspaceHasActiveSessions,
				info.ID,
			))
		}
		entry, stageErr := m.stageWorkspaceSessionDelete(ctx, info)
		if stageErr != nil {
			return rollback(stageErr)
		}
		stage.sessions = append(stage.sessions, entry)
	}
	return stage, nil
}

func (m *Manager) lockWorkspaceConversationOperations(
	ctx context.Context,
	workspaceID string,
) (context.Context, map[string]struct{}, []func(), error) {
	infos, err := m.sortedWorkspaceSessions(ctx, workspaceID)
	if err != nil {
		return nil, nil, nil, err
	}
	lockedSessionIDs := make(map[string]struct{})
	operationUnlocks := make([]func(), 0)
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.WorkspaceID) != workspaceID {
			continue
		}
		operationCtx, unlock, lockErr := m.lockConversationOperation(ctx, info.ID)
		if lockErr != nil {
			releaseConversationOperations(operationUnlocks)
			return nil, nil, nil, lockErr
		}
		ctx = operationCtx
		operationUnlocks = append(operationUnlocks, unlock)
		lockedSessionIDs[info.ID] = struct{}{}
	}
	return ctx, lockedSessionIDs, operationUnlocks, nil
}

func (m *Manager) workspaceRemovalInfos(
	ctx context.Context,
	workspaceID string,
	lockedSessionIDs map[string]struct{},
) ([]*Info, error) {
	infos, err := m.sortedWorkspaceSessions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.WorkspaceID) != workspaceID {
			continue
		}
		if _, locked := lockedSessionIDs[info.ID]; !locked {
			return nil, fmt.Errorf(
				"session: remove workspace %q: session catalog changed during preparation: %w",
				workspaceID,
				workspacepkg.ErrWorkspaceHasActiveSessions,
			)
		}
	}
	return infos, nil
}

func (m *Manager) sortedWorkspaceSessions(ctx context.Context, workspaceID string) ([]*Info, error) {
	infos, err := m.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"session: list sessions before workspace removal %q: %w",
			workspaceID,
			err,
		)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
	return infos, nil
}

func (p *workspaceUnregisterPreparation) Commit(ctx context.Context) error {
	if p == nil || p.manager == nil {
		return nil
	}
	defer p.releaseResources()
	return errors.Join(
		p.manager.commitStagedSessionDeletes(ctx, p.staged),
		commitStagedAttachmentDelete(p.attachments),
	)
}

func (*workspaceUnregisterPreparation) BeforeDelete(context.Context) error {
	return nil
}

func (p *workspaceUnregisterPreparation) Rollback(ctx context.Context) error {
	if p == nil || p.manager == nil {
		return nil
	}
	defer p.releaseResources()
	return errors.Join(
		p.manager.rollbackStagedSessionDeletes(ctx, p.staged),
		rollbackStagedAttachmentDelete(p.attachments),
	)
}

func (p *workspaceUnregisterPreparation) releaseResources() {
	p.release.Do(func() {
		p.manager.lifecycleMu.Unlock()
		releaseConversationOperations(p.conversationOperationUnlocks)
	})
}

func releaseConversationOperations(unlocks []func()) {
	for _, unlock := range slices.Backward(unlocks) {
		unlock()
	}
}

func (m *Manager) stageSessionDelete(
	ctx context.Context,
	target string,
	stopActive bool,
) (stagedSessionDelete, error) {
	info, err := m.Status(ctx, target)
	if err != nil {
		return stagedSessionDelete{}, err
	}
	if m.isPending(target) {
		return stagedSessionDelete{}, fmt.Errorf(
			"session: stage %q: %w",
			target,
			workspacepkg.ErrWorkspaceHasActiveSessions,
		)
	}
	if _, active := m.Get(target); active {
		if !stopActive {
			return stagedSessionDelete{}, fmt.Errorf(
				"session: stage %q: %w",
				target,
				workspacepkg.ErrWorkspaceHasActiveSessions,
			)
		}
		if err := stopSessionBeforeDelete(ctx, target, m.StopWithCause); err != nil {
			return stagedSessionDelete{}, fmt.Errorf("session: stop %q before delete: %w", target, err)
		}
		info, err = m.Status(ctx, target)
		if err != nil {
			return stagedSessionDelete{}, fmt.Errorf("session: read %q after stop for delete: %w", target, err)
		}
	}
	return m.stageSessionDirectoryDelete(ctx, target, info)
}

func (m *Manager) stageWorkspaceSessionDelete(
	ctx context.Context,
	info *Info,
) (stagedSessionDelete, error) {
	if info == nil {
		return stagedSessionDelete{}, errors.New("session: workspace deletion info is required")
	}
	return m.stageSessionDirectoryDeleteWithoutAttachments(ctx, info.ID, info)
}

func (m *Manager) stageSessionDirectoryDelete(
	ctx context.Context,
	target string,
	info *Info,
) (stagedSessionDelete, error) {
	return m.stageSessionDirectoryDeleteWithAttachmentStaging(ctx, target, info, true)
}

func (m *Manager) stageSessionDirectoryDeleteWithoutAttachments(
	ctx context.Context,
	target string,
	info *Info,
) (stagedSessionDelete, error) {
	return m.stageSessionDirectoryDeleteWithAttachmentStaging(ctx, target, info, false)
}

func (m *Manager) stageSessionDirectoryDeleteWithAttachmentStaging(
	ctx context.Context,
	target string,
	info *Info,
	stageAttachments bool,
) (stagedSessionDelete, error) {
	if info == nil {
		return stagedSessionDelete{}, errors.New("session: deletion info is required")
	}
	normalizedTarget, err := normalizeStoredSessionID(target)
	if err != nil {
		return stagedSessionDelete{}, fmt.Errorf("session: normalize staged deletion id %q: %w", target, err)
	}
	if strings.TrimSpace(info.ID) != normalizedTarget {
		return stagedSessionDelete{}, fmt.Errorf(
			"%w: deletion metadata identity %q does not match target %q",
			ErrSessionNotFound,
			info.ID,
			normalizedTarget,
		)
	}
	target = normalizedTarget

	originalPath := filepath.Join(m.homePaths.SessionsDir, target)
	deletionID, err := store.NewID("")
	if err != nil {
		return stagedSessionDelete{}, fmt.Errorf("session: reserve deletion identity for %q: %w", target, err)
	}
	stagedPath := filepath.Join(
		m.homePaths.SessionsDir,
		sessionDeleteTombstoneName(sessionDeleteStagedPrefix, target, deletionID),
	)
	committedPath := filepath.Join(
		m.homePaths.SessionsDir,
		sessionDeleteTombstoneName(sessionDeleteCommittedPrefix, target, deletionID),
	)
	owner, err := m.resolveStoredSessionOwner(ctx, target, info.WorkspaceID)
	if err != nil {
		return stagedSessionDelete{}, fmt.Errorf("session: resolve catalog owner for delete %q: %w", target, err)
	}
	capabilities, err := m.acquireSessionDeleteCapabilities(ctx, owner, originalPath)
	if err != nil {
		return stagedSessionDelete{}, err
	}
	if err := stageBoundSessionDelete(ctx, capabilities, owner, originalPath, stagedPath); err != nil {
		return stagedSessionDelete{}, err
	}
	entry := stagedSessionDelete{
		info: info, owner: owner, originalPath: originalPath, stagedPath: stagedPath,
		committedPath: committedPath, capabilities: capabilities,
	}
	if !stageAttachments {
		return entry, nil
	}
	attachments, err := m.stageSessionAttachmentDelete(ctx, info.WorkspaceID, target, deletionID)
	if err != nil {
		return stagedSessionDelete{}, errors.Join(
			err,
			m.rollbackStagedSessionDeletes(ctx, []stagedSessionDelete{entry}),
		)
	}
	entry.attachments = attachments
	return entry, nil
}

func (m *Manager) commitStagedSessionDeletes(ctx context.Context, staged []stagedSessionDelete) error {
	var cleanupErr error
	for _, entry := range staged {
		if err := verifyStagedSessionDelete(ctx, entry); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		} else {
			result, moveErr := entry.capabilities.directory.MoveTo(
				entry.capabilities.parentDirectory,
				filepath.Base(entry.committedPath),
				false,
			)
			cleanupErr = errors.Join(cleanupErr, moveErr, result.PostCommitErr)
			if result.Committed() {
				verifyErr := entry.capabilities.database.VerifyOwnerAt(
					ctx,
					entry.owner,
					store.SessionDBFile(entry.committedPath),
				)
				cleanupErr = errors.Join(cleanupErr, verifyErr)
				attachmentErr := commitStagedAttachmentDelete(entry.attachments)
				cleanupErr = errors.Join(cleanupErr, attachmentErr)
				if verifyErr == nil && attachmentErr == nil && !entry.windowReconciliationPending {
					cleanupErr = errors.Join(
						cleanupErr,
						m.removeStagedSessionDelete(entry, entry.committedPath),
					)
				}
			}
		}
		cleanupErr = errors.Join(cleanupErr, entry.capabilities.Release(), entry.attachments.Release())
		if entry.info != nil {
			m.publishWaitSessionGone(entry.info)
			m.remove(entry.info.ID)
			m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventDeleted, entry.info))
		}
	}
	return cleanupErr
}

func (m *Manager) rollbackStagedSessionDeletes(ctx context.Context, staged []stagedSessionDelete) error {
	var rollbackErr error
	for _, entry := range slices.Backward(staged) {
		attachmentErr := rollbackStagedAttachmentDelete(entry.attachments)
		rollbackErr = errors.Join(rollbackErr, attachmentErr)
		if attachmentErr != nil {
			rollbackErr = errors.Join(rollbackErr, entry.capabilities.Release())
			continue
		}
		if err := verifyStagedSessionDelete(ctx, entry); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		} else {
			result, moveErr := entry.capabilities.directory.MoveTo(
				entry.capabilities.parentDirectory,
				filepath.Base(entry.originalPath),
				false,
			)
			rollbackErr = errors.Join(rollbackErr, moveErr, result.PostCommitErr)
			if result.Committed() {
				rollbackErr = errors.Join(
					rollbackErr,
					entry.capabilities.database.VerifyOwnerAt(
						ctx,
						entry.owner,
						store.SessionDBFile(entry.originalPath),
					),
				)
			}
		}
		rollbackErr = errors.Join(rollbackErr, entry.capabilities.Release(), entry.attachments.Release())
	}
	return rollbackErr
}

func verifyStagedSessionDelete(ctx context.Context, entry stagedSessionDelete) error {
	if entry.capabilities == nil || !entry.capabilities.valid() {
		return errors.New("session: staged deletion capabilities are required")
	}
	if err := entry.capabilities.database.VerifyOwnerAt(
		ctx,
		entry.owner,
		store.SessionDBFile(entry.stagedPath),
	); err != nil {
		return fmt.Errorf("session: verify staged deletion tombstone %q: %w", entry.stagedPath, err)
	}
	return nil
}

package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const sessionDeleteTombstonePrefix = ".agh-delete-"

type stagedSessionDelete struct {
	info          *Info
	originalPath  string
	tombstonePath string
	resumeQueries func()
}

type workspaceUnregisterPreparation struct {
	manager *Manager
	staged  []stagedSessionDelete
	release sync.Once
}

var _ workspacepkg.UnregisterPreparation = (*workspaceUnregisterPreparation)(nil)

// PrepareWorkspaceRemoval atomically stages every stopped session directory so
// the workspace store can delete its database-owned rows without orphaning
// transcript state. Commit or Rollback releases the serialized deletion lease.
func (m *Manager) PrepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
) (workspacepkg.UnregisterPreparation, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: workspace removal context is required")
	}
	targetWorkspace := strings.TrimSpace(workspaceID)
	if targetWorkspace == "" {
		return nil, errors.New("session: workspace id is required")
	}

	m.lifecycleMu.Lock()
	if sessionID, pending := m.pendingSessionForWorkspace(targetWorkspace); pending {
		m.lifecycleMu.Unlock()
		return nil, fmt.Errorf(
			"session: remove workspace %q: %w: %s",
			targetWorkspace,
			workspacepkg.ErrWorkspaceHasActiveSessions,
			sessionID,
		)
	}
	infos, err := m.ListAll(ctx)
	if err != nil {
		m.lifecycleMu.Unlock()
		return nil, fmt.Errorf("session: list sessions before workspace removal %q: %w", targetWorkspace, err)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
	staged := make([]stagedSessionDelete, 0)
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.WorkspaceID) != targetWorkspace {
			continue
		}
		if info.State == StateStarting || info.State == StateActive || info.State == StateStopping {
			rollbackErr := m.rollbackStagedSessionDeletes(staged)
			m.lifecycleMu.Unlock()
			activeErr := fmt.Errorf(
				"session: remove workspace %q: %w: %s",
				targetWorkspace,
				workspacepkg.ErrWorkspaceHasActiveSessions,
				info.ID,
			)
			return nil, errors.Join(activeErr, rollbackErr)
		}
		entry, stageErr := m.stageSessionDelete(ctx, info.ID, false)
		if stageErr != nil {
			rollbackErr := m.rollbackStagedSessionDeletes(staged)
			m.lifecycleMu.Unlock()
			return nil, errors.Join(stageErr, rollbackErr)
		}
		staged = append(staged, entry)
	}

	return &workspaceUnregisterPreparation{manager: m, staged: staged}, nil
}

func (p *workspaceUnregisterPreparation) Commit(context.Context) error {
	if p == nil || p.manager == nil {
		return nil
	}
	defer p.release.Do(p.manager.lifecycleMu.Unlock)
	return p.manager.commitStagedSessionDeletes(p.staged)
}

func (*workspaceUnregisterPreparation) BeforeDelete(context.Context) error {
	return nil
}

func (p *workspaceUnregisterPreparation) Rollback(context.Context) error {
	if p == nil || p.manager == nil {
		return nil
	}
	defer p.release.Do(p.manager.lifecycleMu.Unlock)
	return p.manager.rollbackStagedSessionDeletes(p.staged)
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
	return m.stageSessionDirectoryDelete(ctx, info)
}

func (m *Manager) stageSessionDirectoryDelete(
	ctx context.Context,
	info *Info,
) (stagedSessionDelete, error) {
	if info == nil {
		return stagedSessionDelete{}, errors.New("session: deletion info is required")
	}
	target := strings.TrimSpace(info.ID)
	if target == "" {
		return stagedSessionDelete{}, errors.New("session: deletion id is required")
	}

	originalPath := filepath.Join(m.homePaths.SessionsDir, target)
	if _, err := os.Stat(originalPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return stagedSessionDelete{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
		}
		return stagedSessionDelete{}, fmt.Errorf("session: stat session directory %q: %w", originalPath, err)
	}
	tombstonePath := filepath.Join(
		m.homePaths.SessionsDir,
		fmt.Sprintf("%s%s-%d", sessionDeleteTombstonePrefix, target, m.now().UnixNano()),
	)
	resumeQueries, err := m.quiesceSessionQueries(ctx, target, originalPath)
	if err != nil {
		return stagedSessionDelete{}, fmt.Errorf("session: quiesce stored readers for %q: %w", target, err)
	}
	if err := m.renamePath(originalPath, tombstonePath); err != nil {
		resumeQueries()
		return stagedSessionDelete{}, fmt.Errorf(
			"session: stage session directory %q for deletion: %w",
			originalPath,
			err,
		)
	}
	return stagedSessionDelete{
		info:          info,
		originalPath:  originalPath,
		tombstonePath: tombstonePath,
		resumeQueries: resumeQueries,
	}, nil
}

func (m *Manager) commitStagedSessionDeletes(staged []stagedSessionDelete) error {
	var cleanupErr error
	for _, entry := range staged {
		if err := m.removeAllPath(entry.tombstonePath); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf("session: remove deletion tombstone %q: %w", entry.tombstonePath, err),
			)
		}
		entry.resumeQueries()
		if entry.info != nil {
			m.remove(entry.info.ID)
			m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventDeleted, entry.info))
		}
	}
	return cleanupErr
}

func (m *Manager) rollbackStagedSessionDeletes(staged []stagedSessionDelete) error {
	var rollbackErr error
	for _, entry := range slices.Backward(staged) {
		if err := m.renamePath(entry.tombstonePath, entry.originalPath); err != nil {
			rollbackErr = errors.Join(
				rollbackErr,
				fmt.Errorf("session: restore staged directory %q: %w", entry.originalPath, err),
			)
		}
		entry.resumeQueries()
	}
	return rollbackErr
}

func (m *Manager) cleanupDeleteTombstones() {
	entries, err := os.ReadDir(m.homePaths.SessionsDir)
	if err != nil {
		m.logger.Warn("session: scan deletion tombstones", "error", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) {
			continue
		}
		path := filepath.Join(m.homePaths.SessionsDir, entry.Name())
		if err := m.removeAllPath(path); err != nil {
			m.logger.Warn("session: retain deletion tombstone for later retry", "path", path, "error", err)
		}
	}
}

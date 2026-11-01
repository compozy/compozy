package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// maxDeleteTreePasses bounds the re-walks that catch children created while a
// session tree was being deleted.
const maxDeleteTreePasses = 8

// SessionTree is a session with its hierarchy descendants, shallowest first.
type SessionTree struct {
	Root        *Info
	Descendants []*Info
}

// Descendants returns the hierarchy descendants of one session: its children in
// the session list, recursively, within its workspace and profile.
func (m *Manager) Descendants(ctx context.Context, workspaceID string, sessionID string) ([]*Info, error) {
	treeStore, err := m.sessionTreeStore()
	if err != nil {
		return nil, err
	}
	rows, err := treeStore.ListSessionDescendants(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	return sessionInfosFromCatalog(rows), nil
}

// ArchiveWithChildren archives a session with its hierarchy descendants in one
// transaction. With stop, live members stop through the normal user stop path
// first and a failed stop archives nothing; without it a live member refuses
// the whole archive as Archive does for a single session.
func (m *Manager) ArchiveWithChildren(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	stop bool,
) (*SessionTree, error) {
	sessionID = strings.TrimSpace(sessionID)
	treeStore, err := m.sessionTreeStore()
	if err != nil {
		return nil, err
	}
	var archived store.SessionTreeArchive
	attempt := func() error {
		var archiveErr error
		archived, archiveErr = m.archiveTreeWithResumeGuard(ctx, treeStore, workspaceID, sessionID)
		return archiveErr
	}
	if stop {
		err = m.archiveStoppingLiveMembers(ctx, sessionID, attempt)
	} else {
		err = attempt()
	}
	if err != nil {
		return nil, err
	}
	tree := &SessionTree{
		Root:        sessionInfoFromCatalog(&archived.Root),
		Descendants: sessionInfosFromCatalog(archived.Descendants),
	}
	for _, member := range slices.Concat([]*Info{tree.Root}, tree.Descendants) {
		if err := m.releaseArchivedSessionWatches(ctx, member.ID); err != nil {
			return nil, err
		}
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, member))
	}
	return tree, nil
}

func (m *Manager) archiveTreeWithResumeGuard(
	ctx context.Context,
	treeStore store.SessionTreeStore,
	workspaceID string,
	sessionID string,
) (store.SessionTreeArchive, error) {
	descendants, err := treeStore.ListSessionDescendants(ctx, workspaceID, sessionID)
	if err != nil {
		return store.SessionTreeArchive{}, err
	}
	m.resumeLifecycle.mu.Lock()
	defer m.resumeLifecycle.mu.Unlock()
	if m.resumeLifecycle.runs[sessionID] != nil {
		return store.SessionTreeArchive{}, newArchiveRequiresStoppedError(sessionID, nil)
	}
	for i := range descendants {
		if m.resumeLifecycle.runs[descendants[i].ID] != nil {
			return store.SessionTreeArchive{}, &archiveRequiresStoppedError{
				sessionID: sessionID,
				member:    descendants[i].ID,
			}
		}
	}
	archived, err := treeStore.ArchiveSessionTree(ctx, workspaceID, sessionID)
	if err != nil {
		return store.SessionTreeArchive{}, archiveStoreError(sessionID, err)
	}
	return archived, nil
}

// DeleteWithChildren deletes a session with its hierarchy descendants, deepest
// first, so a failure leaves every remaining session reachable from the root.
// Deletion is not atomic: it returns the sessions deleted before any failure.
func (m *Manager) DeleteWithChildren(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) ([]string, error) {
	sessionID = strings.TrimSpace(sessionID)
	treeStore, err := m.sessionTreeStore()
	if err != nil {
		return nil, err
	}
	// A stopped root accepts no new spawns, and nothing it delegated wakes it
	// while the tree below it goes away.
	if _, active := m.Get(sessionID); active {
		if err := stopSessionBeforeDelete(ctx, sessionID, m.stopWithDeleteReceipt); err != nil {
			return nil, fmt.Errorf("session: stop %q before delete: %w", sessionID, err)
		}
	}
	deleted := make([]string, 0)
	for range maxDeleteTreePasses {
		descendants, err := treeStore.ListSessionDescendants(ctx, workspaceID, sessionID)
		if err != nil {
			return deleted, err
		}
		if len(descendants) == 0 {
			if err := m.Delete(ctx, sessionID); err != nil {
				return deleted, err
			}
			return append(deleted, sessionID), nil
		}
		for i := len(descendants) - 1; i >= 0; i-- {
			id := descendants[i].ID
			if err := m.Delete(ctx, id); err != nil {
				if errors.Is(err, ErrSessionNotFound) {
					continue
				}
				return deleted, fmt.Errorf("session: delete child %q of %q: %w", id, sessionID, err)
			}
			deleted = append(deleted, id)
		}
	}
	return deleted, fmt.Errorf("session: delete %q: the session tree kept growing", sessionID)
}

func (m *Manager) sessionTreeStore() (store.SessionTreeStore, error) {
	treeStore, ok := m.sessionCatalog.(store.SessionTreeStore)
	if !ok || treeStore == nil {
		return nil, ErrSessionArchiveUnavailable
	}
	return treeStore, nil
}

func sessionInfosFromCatalog(rows []store.SessionInfo) []*Info {
	infos := make([]*Info, 0, len(rows))
	for i := range rows {
		infos = append(infos, sessionInfoFromCatalog(&rows[i]))
	}
	return infos
}

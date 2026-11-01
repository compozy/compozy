package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// maxDeleteTreePasses bounds the re-walks that catch children created while a
// session tree was being deleted.
const maxDeleteTreePasses = 8

const spawnAdmissionPollInterval = 20 * time.Millisecond

// ErrSessionTreeContainsCaller refuses a tree operation that would reach the
// session requesting it.
var ErrSessionTreeContainsCaller = errors.New("session: session tree contains the calling session")

// ArchiveTreeOptions shapes one tree archive.
type ArchiveTreeOptions struct {
	// Stop stops live members first; without it a live member refuses the archive.
	Stop bool
	// RefuseMember fails the archive, before anything stops, when this session
	// is in the tree at the moment the tree is fenced against adoption.
	RefuseMember string
}

// Tree is a session with its hierarchy descendants, shallowest first.
type Tree struct {
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
	opts ArchiveTreeOptions,
) (*Tree, error) {
	sessionID = strings.TrimSpace(sessionID)
	treeStore, err := m.sessionTreeStore()
	if err != nil {
		return nil, err
	}
	var archived store.SessionTreeArchive
	attempt := func() error {
		var archiveErr error
		archived, archiveErr = m.archiveTreeWithResumeGuard(ctx, treeStore, workspaceID, sessionID, opts.RefuseMember)
		return archiveErr
	}
	if opts.Stop {
		err = m.archiveStoppingLiveMembers(ctx, sessionID, attempt)
	} else {
		err = attempt()
	}
	if err != nil {
		return nil, err
	}
	tree := &Tree{
		Root:        sessionInfoFromCatalog(&archived.Root),
		Descendants: sessionInfosFromCatalog(archived.Descendants),
	}
	// The archive is committed, so a failed watch release does not fail it.
	for _, member := range slices.Concat([]*Info{tree.Root}, tree.Descendants) {
		if err := m.releaseArchivedSessionWatches(ctx, member.ID); err != nil {
			m.logger.Warn("session: release watches of archived member", "session_id", member.ID, "error", err)
		}
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, member))
	}
	return tree, nil
}

// archiveTreeWithResumeGuard holds lifecycleMu, which adoption also holds, so
// the members it checks for resume runs are the members the store archives.
func (m *Manager) archiveTreeWithResumeGuard(
	ctx context.Context,
	treeStore store.SessionTreeStore,
	workspaceID string,
	sessionID string,
	refuseMember string,
) (store.SessionTreeArchive, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
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
		if refuseMember != "" && descendants[i].ID == refuseMember {
			return store.SessionTreeArchive{}, ErrSessionTreeContainsCaller
		}
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
	if err := m.stopDeleteTree(ctx, treeStore, workspaceID, sessionID); err != nil {
		return nil, err
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
		for _, descendant := range slices.Backward(descendants) {
			id := descendant.ID
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

// stopDeleteTree stops the tree top-down, waiting for spawns already in flight,
// until no new member appears. A stopped session accepts no new spawns, so a
// deletion below cannot leave a child behind a parent it just removed, and
// nothing a parent delegated wakes it while the tree goes away.
func (m *Manager) stopDeleteTree(
	ctx context.Context,
	treeStore store.SessionTreeStore,
	workspaceID string,
	sessionID string,
) error {
	seen := make(map[string]struct{})
	pending := []string{sessionID}
	for range maxDeleteTreePasses {
		for _, id := range pending {
			seen[id] = struct{}{}
			if _, active := m.Get(id); !active {
				continue
			}
			if err := stopSessionBeforeDelete(ctx, id, m.stopWithDeleteReceipt); err != nil {
				return fmt.Errorf("session: stop %q before delete: %w", id, err)
			}
		}
		if err := m.waitForAdmittedSpawnsContext(ctx); err != nil {
			return err
		}
		descendants, err := treeStore.ListSessionDescendants(ctx, workspaceID, sessionID)
		if err != nil {
			return err
		}
		pending = pending[:0]
		for i := range descendants {
			if _, done := seen[descendants[i].ID]; !done {
				pending = append(pending, descendants[i].ID)
			}
		}
		if len(pending) == 0 {
			return nil
		}
	}
	return fmt.Errorf("session: delete %q: the session tree kept growing", sessionID)
}

// waitForAdmittedSpawnsContext is waitForAdmittedSpawns for a request: Spawn can
// hold spawnMu through a slow child start, and a canceled delete stops waiting.
func (m *Manager) waitForAdmittedSpawnsContext(ctx context.Context) error {
	for !m.spawnMu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(spawnAdmissionPollInterval):
		}
	}
	m.spawnMu.Unlock()
	return nil
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

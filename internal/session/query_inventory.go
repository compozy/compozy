package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

var errInventoryHydration = errors.New("session: hydrate inventory")

type cachedSessionMetadata struct {
	file os.FileInfo
	meta store.SessionMeta
}

type sessionInventory map[string]store.SessionInfo

var _ store.SessionCatalogOwnerReader = sessionInventory(nil)

func (rows sessionInventory) ListSessions(
	_ context.Context, query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	if row, ok := rows[query.ID]; ok {
		return []store.SessionInfo{row}, nil
	}
	return nil, nil
}

func (m *Manager) readSessionInventory(ctx context.Context) (sessionInventory, error) {
	if m.sessionCatalog == nil {
		return nil, nil
	}
	rows, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, Archive: store.SessionArchiveInclude,
	})
	if err != nil {
		return nil, fmt.Errorf("session: read inventory catalog: %w", err)
	}
	result := make(sessionInventory, len(rows))
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func (m *Manager) readInventoryMetadata(id string) (store.SessionMeta, error) {
	target, err := normalizeStoredSessionID(id)
	if err != nil {
		return store.SessionMeta{}, err
	}
	path := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	info, err := os.Stat(path)
	if err != nil {
		m.metadataCacheMu.Lock()
		delete(m.metadataCache, target)
		m.metadataCacheMu.Unlock()
		if errors.Is(err, os.ErrNotExist) {
			return store.SessionMeta{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
		}
		return store.SessionMeta{}, err
	}
	m.metadataCacheMu.Lock()
	cached, found := m.metadataCache[target]
	if found && os.SameFile(cached.file, info) && cached.file.Size() == info.Size() &&
		cached.file.ModTime().Equal(info.ModTime()) {
		m.metadataCacheMu.Unlock()
		return cached.meta, nil
	}
	delete(m.metadataCache, target)
	m.metadataCacheMu.Unlock()
	reader := m.readSessionMeta
	if reader == nil {
		reader = store.ReadSessionMeta
	}
	meta, err := reader(path)
	if err != nil {
		return meta, err
	}
	if meta.ID != target {
		return store.SessionMeta{}, fmt.Errorf("%w: metadata identity does not match %q", ErrSessionNotFound, target)
	}
	// Cache only stable files; atomic replacements must never inherit the previous identity.
	after, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.SessionMeta{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
		}
		return store.SessionMeta{}, err
	}
	if os.SameFile(info, after) && info.Size() == after.Size() && info.ModTime().Equal(after.ModTime()) {
		m.metadataCacheMu.Lock()
		if m.metadataCache == nil {
			m.metadataCache = make(map[string]cachedSessionMetadata)
		}
		m.metadataCache[target] = cachedSessionMetadata{file: info, meta: meta}
		m.metadataCacheMu.Unlock()
	}
	return meta, nil
}

func (m *Manager) inventorySessionInfo(
	ctx context.Context, id string, rows sessionInventory, roots map[string]string,
) (*Info, error) {
	meta, err := m.readInventoryMetadata(id)
	if err != nil {
		return nil, err
	}
	if rows != nil && !m.isPending(id) {
		owner, err := store.LookupSessionOwner(ctx, rows, m.creationStore, id)
		if err != nil {
			return nil, err
		}
		if err := owner.BindMetadata(&meta); err != nil {
			return nil, err
		}
	}
	if rows == nil && !m.isPending(id) {
		if _, err := meta.DatabaseOwner(); err != nil {
			return nil, err
		}
	}
	info := sessionInfoFromMeta(&meta)
	if row, ok := rows[id]; ok {
		info.InstanceID = row.InstanceID
		info.TranscriptEpoch = row.TranscriptEpoch
		if row.Attention != nil {
			applyAttentionToInfo(info, *row.Attention)
		} else if err := m.hydrateSessionInfoAttention(ctx, info); err != nil {
			return nil, errors.Join(errInventoryHydration, err)
		}
	} else {
		m.hydrateMetadataTranscriptEpoch(ctx, info)
		if err := m.hydrateSessionInfoAttention(ctx, info); err != nil {
			return nil, errors.Join(errInventoryHydration, err)
		}
	}
	if meta.WorkspaceID != "" {
		root, found := roots[meta.WorkspaceID]
		if !found {
			var err error
			root, err = m.resolveWorkspaceRoot(ctx, meta.WorkspaceID)
			if err != nil {
				m.logger.Warn("session: resolve workspace root for metadata failed", "session_id", id, "error", err)
			}
			roots[meta.WorkspaceID] = root
		}
		info.Workspace = root
	}

	return info, nil
}

// ListLive includes retained live sessions whose runtimes have not been rebound after restart.
func (m *Manager) ListLive(ctx context.Context) ([]*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: live inventory context is required")
	}
	active := m.List()
	if m.sessionCatalog == nil {
		return active, nil
	}
	activeByID := make(map[string]*Info, len(active))
	for _, info := range active {
		activeByID[info.ID] = info
	}
	rows := make(sessionInventory)
	var ids []string
	for _, state := range []State{StateStarting, StateActive, StateStopping} {
		stored, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, State: string(state), Archive: store.SessionArchiveInclude,
		})
		if err != nil {
			return nil, fmt.Errorf("session: read live catalog: %w", err)
		}
		for _, row := range stored {
			if _, exists := rows[row.ID]; !exists {
				ids = append(ids, row.ID)
			}
			rows[row.ID] = row
		}
	}
	return m.mergePersistedSessionInfos(ctx, ids, activeByID, rows)
}

func (m *Manager) pruneMetadataCache(ids []string) {
	present := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		present[id] = struct{}{}
	}
	m.metadataCacheMu.Lock()
	defer m.metadataCacheMu.Unlock()
	for id := range m.metadataCache {
		if _, exists := present[id]; !exists {
			delete(m.metadataCache, id)
		}
	}
}

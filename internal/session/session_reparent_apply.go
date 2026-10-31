package session

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// reparentNode is one session whose lineage a reparent reads or rewrites. Active
// sessions are rewritten in memory and on disk; inactive ones through metadata.
type reparentNode struct {
	id          string
	workspaceID string
	profileID   string
	sessionType Type
	lineage     store.SessionLineage
	active      *Session
	meta        store.SessionMeta
}

type reparentChange struct {
	node    *reparentNode
	lineage store.SessionLineage
}

// loadReparentNode resolves one session inside workspaceID. The caller holds
// m.lifecycleMu, so an inactive session cannot activate underneath it.
func (m *Manager) loadReparentNode(ctx context.Context, id string, workspaceID string) (reparentNode, error) {
	if active, ok := m.Get(id); ok {
		active.mu.RLock()
		node := reparentNode{
			id:          active.ID,
			workspaceID: strings.TrimSpace(active.WorkspaceID),
			profileID:   strings.TrimSpace(active.ProfileID),
			sessionType: normalizeSessionType(active.Type),
			lineage:     *store.NormalizeSessionLineage(active.ID, active.Lineage),
			active:      active,
		}
		active.mu.RUnlock()
		if node.workspaceID != workspaceID {
			return reparentNode{}, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
		}
		return node, nil
	}
	if m.isPending(id) {
		return reparentNode{}, fmt.Errorf("%w: %s is starting", ErrSessionNotActive, id)
	}
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return reparentNode{}, err
	}
	if strings.TrimSpace(meta.WorkspaceID) != workspaceID {
		return reparentNode{}, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	return reparentNode{
		id:          meta.ID,
		workspaceID: strings.TrimSpace(meta.WorkspaceID),
		profileID:   strings.TrimSpace(meta.ProfileID),
		sessionType: normalizeSessionType(Type(meta.SessionType)),
		lineage:     *store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		meta:        meta,
	}, nil
}

// collectReparentSubtree returns every session below root, following the raw
// lineage parent pointers (spawn, provenance, continue, fork, and recovery).
func (m *Manager) collectReparentSubtree(ctx context.Context, root *reparentNode) ([]reparentNode, error) {
	nodes := make([]reparentNode, 0)
	visited := map[string]struct{}{root.id: {}}
	queue := []string{root.id}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]
		rows, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
			ReadScope:       store.ReadScope{AllProfiles: true},
			ParentSessionID: parentID,
		})
		if err != nil {
			return nil, fmt.Errorf("session: list children of %q: %w", parentID, err)
		}
		for i := range rows {
			id := strings.TrimSpace(rows[i].ID)
			if _, seen := visited[id]; seen {
				continue
			}
			if strings.TrimSpace(rows[i].WorkspaceID) != root.workspaceID {
				return nil, reparentRefusal(ReparentRefusalCrossWorkspace, root.id,
					"sessions with descendants in another workspace cannot be reparented")
			}
			visited[id] = struct{}{}
			queue = append(queue, id)
			node, err := m.loadReparentNode(ctx, id, root.workspaceID)
			if errors.Is(err, ErrSessionNotFound) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("session: load descendant %q: %w", id, err)
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// applyReparent writes the source's new lineage and re-roots its subtree: every
// descendant takes the source's new root and shifts its depth by the same delta.
// Metadata is written first and rolled back when the catalog transaction fails.
func (m *Manager) applyReparent(
	ctx context.Context,
	source *reparentNode,
	lineage store.SessionLineage,
) (*Info, error) {
	lineageStore, err := m.sessionLineageStore()
	if err != nil {
		return nil, err
	}
	descendants, err := m.collectReparentSubtree(ctx, source)
	if err != nil {
		return nil, err
	}
	changes, err := planReparentChanges(source, lineage, descendants)
	if err != nil {
		return nil, err
	}

	unlock := lockReparentActiveSessions(changes)
	defer unlock()

	now := m.now()
	rollbacks := make([]func() error, 0, len(changes))
	rollback := func(cause error) error {
		for _, undo := range slices.Backward(rollbacks) {
			if rollbackErr := undo(); rollbackErr != nil {
				cause = errors.Join(cause, fmt.Errorf("session: roll back reparent: %w", rollbackErr))
			}
		}
		return cause
	}
	updates := make([]store.SessionLineageUpdate, 0, len(changes))
	for i := range changes {
		undo, err := m.writeReparentLineage(&changes[i], now)
		if err != nil {
			return nil, rollback(err)
		}
		rollbacks = append(rollbacks, undo)
		updates = append(updates, store.SessionLineageUpdate{
			SessionID: changes[i].node.id,
			Lineage:   changes[i].lineage,
			UpdatedAt: now,
		})
	}
	if err := lineageStore.UpdateSessionLineages(ctx, source.workspaceID, updates); err != nil {
		return nil, rollback(fmt.Errorf("session: persist reparent lineage: %w", err))
	}

	var result *Info
	for i := range changes {
		info := reparentNodeInfo(changes[i].node)
		if i == 0 {
			result = m.reparentResultInfo(ctx, changes[i].node, info)
		}
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	}
	return result, nil
}

func planReparentChanges(
	source *reparentNode,
	lineage store.SessionLineage,
	descendants []reparentNode,
) ([]reparentChange, error) {
	delta := lineage.SpawnDepth - source.lineage.SpawnDepth
	changes := make([]reparentChange, 0, len(descendants)+1)
	changes = append(changes, reparentChange{node: source, lineage: lineage})
	for i := range descendants {
		next := descendants[i].lineage
		next.RootSessionID = lineage.RootSessionID
		next.SpawnDepth += delta
		changes = append(changes, reparentChange{node: &descendants[i], lineage: next})
	}
	for i := range changes {
		change := &changes[i]
		if err := store.ValidateSessionLineage(change.node.id, &change.lineage); err != nil {
			return nil, fmt.Errorf("%w: reparent lineage for %q: %w", ErrValidation, change.node.id, err)
		}
	}
	return changes, nil
}

// lockReparentActiveSessions holds persistMu of every active session in the
// change set, in session-id order, so no concurrent persist interleaves.
func lockReparentActiveSessions(changes []reparentChange) func() {
	active := make([]*Session, 0, len(changes))
	for i := range changes {
		if session := changes[i].node.active; session != nil {
			active = append(active, session)
		}
	}
	slices.SortFunc(active, func(a, b *Session) int { return cmp.Compare(a.ID, b.ID) })
	for _, session := range active {
		session.persistMu.Lock()
	}
	return func() {
		for _, session := range slices.Backward(active) {
			session.persistMu.Unlock()
		}
	}
}

func (m *Manager) writeReparentLineage(change *reparentChange, now time.Time) (func() error, error) {
	node := change.node
	if active := node.active; active != nil {
		active.mu.Lock()
		defer active.mu.Unlock()
		previousMeta := active.metaLocked()
		previousLineage := store.CloneSessionLineage(active.Lineage)
		previousUpdatedAt := active.UpdatedAt
		active.Lineage = store.CloneSessionLineage(&change.lineage)
		active.UpdatedAt = now
		meta := active.metaLocked()
		if err := store.WriteSessionMeta(active.metaPath, &meta); err != nil {
			active.Lineage = previousLineage
			active.UpdatedAt = previousUpdatedAt
			return nil, fmt.Errorf("session: persist reparent for %q: %w", node.id, err)
		}
		return func() error {
			active.mu.Lock()
			defer active.mu.Unlock()
			active.Lineage = previousLineage
			active.UpdatedAt = previousUpdatedAt
			return store.WriteSessionMeta(active.metaPath, &previousMeta)
		}, nil
	}

	previousMeta := node.meta
	node.meta.Lineage = store.CloneSessionLineage(&change.lineage)
	node.meta.UpdatedAt = now
	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, node.id))
	if err := store.WriteSessionMeta(metaPath, &node.meta); err != nil {
		node.meta = previousMeta
		return nil, fmt.Errorf("session: persist reparent for %q: %w", node.id, err)
	}
	return func() error {
		node.meta = previousMeta
		return store.WriteSessionMeta(metaPath, &previousMeta)
	}, nil
}

func reparentNodeInfo(node *reparentNode) *Info {
	if node.active != nil {
		return node.active.Info()
	}
	return sessionInfoFromMeta(&node.meta)
}

func (m *Manager) reparentResultInfo(ctx context.Context, node *reparentNode, info *Info) *Info {
	if node.active != nil {
		return info
	}
	result := m.sessionInfoFromMeta(ctx, &node.meta)
	if err := m.populateCatalogMetadata(ctx, result); err != nil {
		m.logger.WarnContext(ctx, "session: read archive metadata after reparent failed",
			"session_id", node.id, "error", err)
	}
	return result
}

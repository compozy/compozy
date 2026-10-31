package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// maxLaterFamilyPasses bounds the re-walks that catch children created while
// the tree was being stopped.
const maxLaterFamilyPasses = 4

// stopAndParkLaterFamily stops the tree, waits for spawns already in flight,
// and walks it again until no unreleased member appears. The final walk and the
// parks run under lifecycleMu, which adoption also holds, so no child can join
// the tree between the last walk and the parks. Stopped parents accept no new
// spawns, so the walk settles.
func (m *Manager) stopAndParkLaterFamily(
	ctx context.Context,
	laterStore store.SessionLaterStore,
	workspaceID string,
	rootID string,
	req LaterRequest,
) (*Info, error) {
	released := make(map[string]struct{})
	for range maxLaterFamilyPasses {
		pending, err := m.unreleasedLaterMembers(ctx, workspaceID, rootID, released)
		if err != nil {
			return nil, err
		}
		if len(pending) > 0 {
			if err := m.releaseLaterFamily(ctx, pending); err != nil {
				return nil, err
			}
			for _, member := range pending {
				released[member.ID] = struct{}{}
			}
			m.waitForAdmittedSpawns()
			continue
		}
		info, settled, err := m.parkSettledLaterFamily(ctx, laterStore, workspaceID, rootID, released, req)
		if err != nil || settled {
			return info, err
		}
	}
	return nil, fmt.Errorf("%w: %s: the session tree kept growing", ErrSessionLaterStopFailed, rootID)
}

// parkSettledLaterFamily parks the tree when the walk under lifecycleMu finds
// no unreleased member; settled is false when a new member appeared.
func (m *Manager) parkSettledLaterFamily(
	ctx context.Context,
	laterStore store.SessionLaterStore,
	workspaceID string,
	rootID string,
	released map[string]struct{},
	req LaterRequest,
) (*Info, bool, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	members, err := m.laterFamily(ctx, workspaceID, rootID, false)
	if err != nil {
		return nil, false, err
	}
	for _, member := range members {
		if _, done := released[member.ID]; !done {
			return nil, false, nil
		}
		if member.State != StateStopped {
			// A released member came back to life; release it again outside the lock.
			delete(released, member.ID)
			return nil, false, nil
		}
	}
	info, err := m.parkLaterFamily(ctx, laterStore, workspaceID, members, req)
	return info, true, err
}

func (m *Manager) unreleasedLaterMembers(
	ctx context.Context,
	workspaceID string,
	rootID string,
	released map[string]struct{},
) ([]*Info, error) {
	members, err := m.laterFamily(ctx, workspaceID, rootID, false)
	if err != nil {
		return nil, err
	}
	pending := make([]*Info, 0, len(members))
	for _, member := range members {
		if _, done := released[member.ID]; !done {
			pending = append(pending, member)
		}
	}
	return pending, nil
}

// waitForAdmittedSpawns returns once every spawn admitted so far has created its
// child: Spawn holds spawnMu from admission to child creation.
func (m *Manager) waitForAdmittedSpawns() {
	m.spawnMu.Lock()
	defer m.spawnMu.Unlock()
}

// parkLaterFamily parks descendants first, so a parked root always implies a
// parked tree. A failure unparks only the parks this call made, matched by
// their parked time, leaving the tree stopped but out of Later as a whole.
func (m *Manager) parkLaterFamily(
	ctx context.Context,
	laterStore store.SessionLaterStore,
	workspaceID string,
	members []*Info,
	req LaterRequest,
) (*Info, error) {
	owned := make(map[string]time.Time, len(members))
	fail := func(err error) (*Info, error) {
		m.undoLaterParks(context.WithoutCancel(ctx), laterStore, workspaceID, owned)
		return nil, err
	}
	for i := len(members) - 1; i >= 1; i-- {
		if members[i].Later != nil {
			continue
		}
		info, err := m.parkLaterMember(ctx, laterStore, store.SessionLaterPark{
			WorkspaceID: workspaceID, SessionID: members[i].ID,
		})
		if err != nil {
			return fail(err)
		}
		if info.Later != nil {
			owned[info.ID] = info.Later.ParkedAt
		}
	}
	info, err := m.parkLaterMember(ctx, laterStore, store.SessionLaterPark{
		WorkspaceID: workspaceID, SessionID: members[0].ID, Reason: req.Reason, ReturnAt: req.ReturnAt,
	})
	if err != nil {
		return fail(err)
	}
	return info, nil
}

func (m *Manager) undoLaterParks(
	ctx context.Context,
	laterStore store.SessionLaterStore,
	workspaceID string,
	owned map[string]time.Time,
) {
	for id, parkedAt := range owned {
		undone, err := laterStore.UnparkSessionParkedAt(ctx, workspaceID, id, parkedAt)
		if err != nil {
			m.logger.Warn("session: undo partial later park", "session_id", id, "error", err)
			continue
		}
		if !undone {
			continue
		}
		if _, err := m.publishLaterCatalogUpsert(ctx, id); err != nil {
			m.logger.Warn("session: publish undone later park", "session_id", id, "error", err)
		}
	}
}

// laterFamily returns root followed by its hierarchy descendants in breadth
// order. Forks and continues are siblings, not children, and daemon-internal
// children stay out. parkedOnly keeps only parked descendants.
func (m *Manager) laterFamily(
	ctx context.Context,
	workspaceID string,
	rootID string,
	parkedOnly bool,
) ([]*Info, error) {
	root, err := m.Status(ctx, rootID)
	if err != nil {
		return nil, err
	}
	members := []*Info{root}
	visited := map[string]struct{}{rootID: {}}
	queue := []string{rootID}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]
		rows, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
			ReadScope:       store.ReadScope{AllProfiles: true},
			WorkspaceID:     workspaceID,
			ParentSessionID: parentID,
		})
		if err != nil {
			return nil, fmt.Errorf("session: list children of %q: %w", parentID, err)
		}
		for i := range rows {
			child := &rows[i]
			id := strings.TrimSpace(child.ID)
			if _, seen := visited[id]; seen || !laterFamilyChild(child, parentID) {
				continue
			}
			visited[id] = struct{}{}
			queue = append(queue, id)
			if parkedOnly && child.LaterState() == nil {
				continue
			}
			members = append(members, m.laterMemberInfo(ctx, child))
		}
	}
	return members, nil
}

func laterFamilyChild(child *store.SessionInfo, parentID string) bool {
	if child.ArchivedAt != nil || child.Lineage == nil {
		return false
	}
	if store.HierarchyParentSessionID(child.Lineage) != parentID {
		return false
	}
	return !IsInternalSpawnRole(child.Lineage.SpawnRole)
}

func (m *Manager) laterMemberInfo(ctx context.Context, row *store.SessionInfo) *Info {
	if session, ok := m.Get(row.ID); ok {
		return m.sessionInfoForRead(session)
	}
	info := sessionInfoFromCatalog(row)
	if status, err := m.Status(ctx, row.ID); err == nil {
		info = status
	}
	return info
}

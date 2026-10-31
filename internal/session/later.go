package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
)

// MaxLaterReasonLength bounds the one-line note shown on a Later card.
const MaxLaterReasonLength = 200

var (
	// ErrSessionInLater reports a runtime action on a session parked in Later.
	ErrSessionInLater = errors.New("session: session is in later")
	// ErrSessionLaterUnavailable reports that Later persistence is not configured.
	ErrSessionLaterUnavailable = errors.New("session: later store is unavailable")
	// ErrSessionLaterInvalid reports an invalid Move to Later request.
	ErrSessionLaterInvalid = errors.New("session: later request is invalid")
	// ErrSubagentLaterFollowsParent rejects parking a subagent on its own.
	ErrSubagentLaterFollowsParent = errors.New("session: subagent sessions move to later with their parent")
	// ErrSessionLaterStopFailed reports that parking could not stop the session tree.
	ErrSessionLaterStopFailed = errors.New("session: stop before moving to later failed")
)

// LaterRequest carries the optional note and return time of a Move to Later.
type LaterRequest struct {
	Reason   string
	ReturnAt *time.Time
}

// LaterProcessReleaser terminates the processes and terminals attributed to a
// session being parked. The daemon owns process attribution, so it injects this.
type LaterProcessReleaser interface {
	// CaptureSessionProcesses snapshots process ownership before sessions stop.
	CaptureSessionProcesses(ctx context.Context) error
	// TerminateSessionProcesses terminates every process attributed to info.
	TerminateSessionProcesses(ctx context.Context, info *Info) error
}

// SetLaterProcessReleaser wires process termination for Move to Later.
func (m *Manager) SetLaterProcessReleaser(releaser LaterProcessReleaser) {
	m.laterMu.Lock()
	defer m.laterMu.Unlock()
	m.laterProcesses = releaser
}

func (m *Manager) laterProcessReleaser() LaterProcessReleaser {
	m.laterMu.Lock()
	defer m.laterMu.Unlock()
	return m.laterProcesses
}

func normalizeLaterRequest(req LaterRequest) (LaterRequest, error) {
	reason := strings.TrimSpace(req.Reason)
	if strings.ContainsAny(reason, "\r\n") {
		return LaterRequest{}, fmt.Errorf("%w: reason must be one line", ErrSessionLaterInvalid)
	}
	if utf8.RuneCountInString(reason) > MaxLaterReasonLength {
		return LaterRequest{}, fmt.Errorf(
			"%w: reason must be at most %d characters", ErrSessionLaterInvalid, MaxLaterReasonLength,
		)
	}
	normalized := LaterRequest{Reason: reason}
	if req.ReturnAt != nil {
		normalized.ReturnAt = new(req.ReturnAt.UTC())
	}
	return normalized, nil
}

func (m *Manager) laterStore() (store.SessionLaterStore, error) {
	laterStore, ok := m.sessionCatalog.(store.SessionLaterStore)
	if !ok || laterStore == nil {
		return nil, ErrSessionLaterUnavailable
	}
	return laterStore, nil
}

// MoveToLater parks a session and its hierarchy children. It stops every live
// member through the user stop path (active turn, subagents, monitors), drops
// their reply watches, terminates their attributed processes and terminals, and
// then persists the parked state. History, transcripts, and worktree bindings
// stay untouched. A return time at or before now brings the session back at once.
func (m *Manager) MoveToLater(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	req LaterRequest,
) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: later context is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session: later session id is required")
	}
	req, err := normalizeLaterRequest(req)
	if err != nil {
		return nil, err
	}
	laterStore, err := m.laterStore()
	if err != nil {
		return nil, err
	}
	root, err := m.Status(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if root.Lineage != nil && root.Lineage.SpawnRole == store.SubagentSpawnRole {
		return nil, ErrSubagentLaterFollowsParent
	}
	if root.ArchivedAt != nil {
		return nil, fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
	}
	members, err := m.laterFamily(ctx, workspaceID, sessionID, false)
	if err != nil {
		return nil, err
	}
	if err := m.releaseLaterFamily(ctx, members); err != nil {
		return nil, err
	}
	// Descendants park first so a parked root always implies a parked tree.
	for i := len(members) - 1; i >= 1; i-- {
		if members[i].Later != nil {
			continue
		}
		if _, err := m.parkLaterMember(ctx, laterStore, store.SessionLaterPark{
			WorkspaceID: workspaceID, SessionID: members[i].ID,
		}); err != nil {
			return nil, err
		}
	}
	info, err := m.parkLaterMember(ctx, laterStore, store.SessionLaterPark{
		WorkspaceID: workspaceID, SessionID: sessionID, Reason: req.Reason, ReturnAt: req.ReturnAt,
	})
	if err != nil {
		return nil, err
	}
	if req.ReturnAt != nil && !req.ReturnAt.After(m.now()) {
		if _, err := m.ReturnDueLaterSessions(ctx); err != nil {
			return nil, err
		}
		return m.Status(ctx, sessionID)
	}
	return info, nil
}

// releaseLaterFamily stops each live member and releases the resources that
// could otherwise wake or keep working on its behalf.
func (m *Manager) releaseLaterFamily(ctx context.Context, members []*Info) error {
	releaser := m.laterProcessReleaser()
	if releaser != nil {
		if err := releaser.CaptureSessionProcesses(ctx); err != nil {
			return fmt.Errorf("%w: capture session processes: %w", ErrSessionLaterStopFailed, err)
		}
	}
	for _, member := range members {
		if err := m.stopLaterMember(ctx, member.ID); err != nil {
			return err
		}
		if service := m.ReplyWatches(); service != nil {
			service.OnTargetGone(ctx, member.ID)
			if err := service.OnSenderGone(ctx, member.ID); err != nil {
				return err
			}
		}
		if releaser != nil {
			if err := releaser.TerminateSessionProcesses(ctx, member); err != nil {
				return fmt.Errorf("%w: %s: %w", ErrSessionLaterStopFailed, member.ID, err)
			}
		}
	}
	return nil
}

func (m *Manager) stopLaterMember(ctx context.Context, sessionID string) error {
	if _, ok := m.Get(sessionID); !ok {
		return nil
	}
	if err := m.Stop(ctx, sessionID); err != nil && !errors.Is(err, ErrSessionNotFound) {
		return fmt.Errorf("%w: %s: %w", ErrSessionLaterStopFailed, sessionID, err)
	}
	return nil
}

// parkLaterMember persists one parked row under the resume guard. A member
// resumed after its stop is stopped once more before the park is retried.
func (m *Manager) parkLaterMember(
	ctx context.Context,
	laterStore store.SessionLaterStore,
	park store.SessionLaterPark,
) (*Info, error) {
	for attempt := 0; ; attempt++ {
		stored, err := m.parkSessionWithResumeGuard(ctx, laterStore, park)
		if err == nil {
			info := sessionInfoFromCatalog(&stored)
			m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
			return info, nil
		}
		if errors.Is(err, store.ErrSessionArchived) {
			return nil, fmt.Errorf("%w: %s", ErrSessionArchived, park.SessionID)
		}
		if !errors.Is(err, store.ErrSessionLaterRequiresStopped) || attempt > 0 {
			return nil, fmt.Errorf("%w: %s: %w", ErrSessionLaterStopFailed, park.SessionID, err)
		}
		if err := m.stopLaterMember(ctx, park.SessionID); err != nil {
			return nil, err
		}
	}
}

func (m *Manager) parkSessionWithResumeGuard(
	ctx context.Context,
	laterStore store.SessionLaterStore,
	park store.SessionLaterPark,
) (store.SessionInfo, error) {
	m.resumeLifecycle.mu.Lock()
	defer m.resumeLifecycle.mu.Unlock()
	if m.resumeLifecycle.runs[park.SessionID] != nil {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionLaterRequiresStopped, park.SessionID)
	}
	return laterStore.ParkSession(ctx, park)
}

// ResumeFromLater returns a parked session to the normal flow without starting
// its runtime. Children stay parked unless withChildren is set.
func (m *Manager) ResumeFromLater(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	withChildren bool,
) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: later context is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session: later session id is required")
	}
	laterStore, err := m.laterStore()
	if err != nil {
		return nil, err
	}
	members := []*Info{{ID: sessionID}}
	if withChildren {
		members, err = m.laterFamily(ctx, workspaceID, sessionID, true)
		if err != nil {
			return nil, err
		}
	}
	for _, member := range members {
		if _, err := laterStore.UnparkSession(ctx, workspaceID, member.ID); err != nil {
			return nil, err
		}
		if member.ID != sessionID {
			m.publishLaterCatalogUpsert(ctx, member.ID)
		}
	}
	return m.publishLaterCatalogUpsert(ctx, sessionID)
}

func (m *Manager) publishLaterCatalogUpsert(ctx context.Context, sessionID string) (*Info, error) {
	info, err := m.Status(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return info, nil
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
			if parkedOnly && child.Later == nil {
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

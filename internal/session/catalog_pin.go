package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrSessionPinUnavailable reports that pin persistence is not configured.
	ErrSessionPinUnavailable = errors.New("session: pin store is unavailable")
	// ErrSubagentPinFollowsParent reports a pin request for a subagent session.
	ErrSubagentPinFollowsParent = store.ErrSubagentPinFollowsParent
	// ErrSessionPinInstanceMain reports a pin request for an always-on agent's main session.
	ErrSessionPinInstanceMain = store.ErrSessionPinInstanceMain
)

// normalizePinnedFilter validates the pinned population and the pinned sort,
// which orders by pin time and so reads pinned sessions only.
func normalizePinnedFilter(query *ListQuery) error {
	query.Pinned = store.SessionPinnedFilter(strings.TrimSpace(string(query.Pinned)))
	if query.Pinned == PinnedInclude {
		query.Pinned = ""
	}
	if err := query.Pinned.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrListQueryInvalid, err)
	}
	if query.Sort == ListSortPinned && query.Pinned != PinnedOnly {
		return fmt.Errorf("%w: sort %q requires pinned=only", ErrListQueryInvalid, ListSortPinned)
	}
	return nil
}

func sessionMatchesPinnedFilter(info *Info, filter store.SessionPinnedFilter) bool {
	pinned := info != nil && info.PinnedAt != nil
	switch filter {
	case PinnedExclude:
		return !pinned
	case PinnedOnly:
		return pinned
	default:
		return true
	}
}

// compareSessionCatalogPinnedPosition orders the oldest pin first, so a new pin
// lands last and rows already pinned never shift.
func compareSessionCatalogPinnedPosition(left store.SessionCatalogPosition, right store.SessionCatalogPosition) int {
	if !left.PinnedAt.Equal(*right.PinnedAt) {
		if left.PinnedAt.Before(*right.PinnedAt) {
			return -1
		}
		return 1
	}
	return strings.Compare(strings.TrimSpace(left.ID), strings.TrimSpace(right.ID))
}

func (m *Manager) pinStore() (store.SessionPinStore, error) {
	pinStore, ok := m.sessionCatalog.(store.SessionPinStore)
	if !ok || pinStore == nil {
		return nil, ErrSessionPinUnavailable
	}
	return pinStore, nil
}

// overlayPinnedAt sets each live snapshot's pin from the store, which owns it:
// a pin never passes through the runtime, so a running session's snapshot
// carries it only from here.
func (m *Manager) overlayPinnedAt(ctx context.Context, infos []*Info) error {
	pinStore, err := m.pinStore()
	if err != nil || len(infos) == 0 {
		return nil
	}
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		if info != nil && strings.TrimSpace(info.ID) != "" {
			ids = append(ids, info.ID)
		}
	}
	pins, err := pinStore.SessionPinnedAt(ctx, ids)
	if err != nil {
		return fmt.Errorf("session: read live session pins: %w", err)
	}
	for _, info := range infos {
		if info == nil {
			continue
		}
		info.PinnedAt = nil
		if pinnedAt, ok := pins[info.ID]; ok {
			info.PinnedAt = new(pinnedAt)
		}
	}
	return nil
}

// Pin pins one session above the session list. The pin is organization only:
// the session keeps its state, folder, manual position and activity order.
func (m *Manager) Pin(ctx context.Context, workspaceID string, sessionID string) (*Info, error) {
	return m.setPinned(ctx, workspaceID, sessionID, true)
}

// Unpin returns a pinned session to its place in the list; unpinning a session
// that is not pinned changes nothing.
func (m *Manager) Unpin(ctx context.Context, workspaceID string, sessionID string) (*Info, error) {
	return m.setPinned(ctx, workspaceID, sessionID, false)
}

func (m *Manager) setPinned(ctx context.Context, workspaceID string, sessionID string, pinned bool) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: pin context is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session: pin session id is required")
	}
	pinStore, err := m.pinStore()
	if err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if pinned {
		if _, err := pinStore.PinSession(ctx, workspaceID, sessionID); err != nil {
			return nil, err
		}
	} else if _, _, err := pinStore.UnpinSession(ctx, workspaceID, sessionID); err != nil {
		return nil, err
	}
	// Status reads the pin from the store, so a later change wins over this one's result.
	info, err := m.Status(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return info, nil
}

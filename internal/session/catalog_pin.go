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
// which orders by the Pinned order and so reads what pins carry only.
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

// sessionMatchesPinnedFilter applies the pinned population to a live row: a pin carries its
// subtree, so a row belongs to Pinned when any pin carries it (overlayPinScopes resolves that).
func sessionMatchesPinnedFilter(info *Info, filter store.SessionPinnedFilter) bool {
	pinned := info != nil && info.PinScope != nil
	switch filter {
	case PinnedExclude:
		return !pinned
	case PinnedOnly:
		return pinned
	default:
		return true
	}
}

// compareSessionCatalogPinRoot orders pins by the operator's Pinned order: the smallest position
// leads, ties by root id. Rows a pin carries share its key.
func compareSessionCatalogPinRoot(left store.SessionCatalogPosition, right store.SessionCatalogPosition) int {
	if *left.PinPosition != *right.PinPosition {
		if *left.PinPosition < *right.PinPosition {
			return -1
		}
		return 1
	}
	return strings.Compare(left.PinRootID, right.PinRootID)
}

// compareSessionCatalogPinnedPosition orders a pinned page: pins in Pinned order, each followed
// by what it carries by last activity, as the durable read orders them.
func compareSessionCatalogPinnedPosition(left store.SessionCatalogPosition, right store.SessionCatalogPosition) int {
	if order := compareSessionCatalogPinRoot(left, right); order != 0 {
		return order
	}
	leftRoot, rightRoot := left.ID == left.PinRootID, right.ID == right.PinRootID
	if leftRoot != rightRoot {
		if leftRoot {
			return -1
		}
		return 1
	}
	left.PinPosition, right.PinPosition = nil, nil
	return compareSessionCatalogPosition(left, right)
}

// sessionCatalogAfter reports whether a row sorts after the cursor. A pinned cursor anchors a pin,
// so a row follows it only when its own pin does: a subtree never splits across pages.
func sessionCatalogAfter(position store.SessionCatalogPosition, after store.SessionCatalogPosition) bool {
	if position.PinPosition != nil && after.PinPosition != nil {
		return compareSessionCatalogPinRoot(position, after) > 0
	}
	return compareSessionCatalogPosition(position, after) > 0
}

// cutSessionCatalogPinnedPage keeps the first limit pins of a merged pinned page with every row
// they carry, and reports whether another pin follows.
func cutSessionCatalogPinnedPage(
	candidates []store.SessionInfo,
	limit int,
	position func(*store.SessionInfo) store.SessionCatalogPosition,
) ([]store.SessionInfo, bool) {
	roots := 0
	lastRoot := ""
	for index := range candidates {
		root := position(&candidates[index]).PinRootID
		if root == lastRoot {
			continue
		}
		if roots == limit {
			return candidates[:index], true
		}
		roots++
		lastRoot = root
	}
	return candidates, false
}

// sessionCatalogPinnedTotal counts the pins of a pinned read across durable and live rows: a pin
// whose subtree has a live match and a durable match counts once.
func sessionCatalogPinnedTotal(durableRoots []string, active []store.SessionInfo) int {
	roots := make(map[string]struct{}, len(durableRoots)+len(active))
	for _, root := range durableRoots {
		roots[root] = struct{}{}
	}
	for index := range active {
		if scope := active[index].PinScope; scope != nil {
			roots[scope.RootID] = struct{}{}
		}
	}
	return len(roots)
}

func clonePinScope(scope *store.SessionPinScope) *store.SessionPinScope {
	if scope == nil {
		return nil
	}
	cloned := *scope
	return &cloned
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

// overlayPinScopes resolves which live rows a pin carries, so live and durable rows of one read
// apply the same lineage-aware pinned population.
func (m *Manager) overlayPinScopes(ctx context.Context, infos []*Info) error {
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
	scopes, err := pinStore.SessionPinScopes(ctx, ids)
	if err != nil {
		return fmt.Errorf("session: read live session pin scopes: %w", err)
	}
	for _, info := range infos {
		if info == nil {
			continue
		}
		info.PinScope = nil
		if scope, ok := scopes[info.ID]; ok {
			info.PinScope = &scope
		}
	}
	return nil
}

// Pin pins one session at the top of Pinned, carrying its subtree there. The pin is organization
// only: the session and its descendants keep their state, parent, folder, manual position and
// activity order, so an unpin returns them to their places.
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
	if err := m.refuseSideOperation(ctx, sessionID, ErrSideFixedParent); err != nil {
		return nil, err
	}
	pinStore, err := m.pinStore()
	if err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	var stored store.SessionInfo
	changed := true
	if pinned {
		stored, err = pinStore.PinSession(ctx, workspaceID, sessionID)
	} else {
		stored, changed, err = pinStore.UnpinSession(ctx, workspaceID, sessionID)
	}
	if err != nil {
		return nil, err
	}
	// Status reads the pin from the store, so a later change wins over this one's result.
	info, statusErr := m.Status(ctx, sessionID)
	if !changed {
		// Unpinning a session that is not pinned changes nothing, so nothing is published.
		return info, statusErr
	}
	// The change is committed: other windows hear of it even when this response's read fails.
	published := info
	if statusErr != nil {
		published = sessionInfoFromCatalog(&stored)
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, published))
	if statusErr != nil {
		return nil, statusErr
	}
	return info, nil
}

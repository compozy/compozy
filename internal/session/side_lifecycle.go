package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) stopSideSessions(ctx context.Context, sourceID string, wait bool) error {
	sides, ok := m.sessionCatalog.(store.SessionSideStore)
	if !ok {
		return nil
	}
	info, err := m.durableSessionInfoForLifecycleTransition(ctx, sourceID)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := sides.ListSessionSides(ctx, info.WorkspaceID, sourceID, true)
	if err != nil {
		return err
	}
	var failures error
	for i := range rows {
		row := &rows[i]
		if _, live := m.Get(row.ID); !live && !m.isPending(row.ID) {
			continue
		}
		var err error
		if wait {
			err = m.Stop(ctx, row.ID)
		} else {
			err = m.RequestStopWithCause(ctx, row.ID, CauseUserRequested, "")
		}
		if err != nil && !errors.Is(err, ErrSessionNotFound) {
			failures = errors.Join(failures, fmt.Errorf("stop side chat %s of %s: %w", row.ID, sourceID, err))
		}
	}
	return failures
}

func (m *Manager) archiveStoppingSideMembers(ctx context.Context, rootID string, archive func() error) error {
	return m.archiveStoppingLiveMembers(ctx, rootID, archive, func(member string) bool {
		if member == strings.TrimSpace(rootID) {
			return false
		}
		meta, err := m.readSessionMetaReadOnly(ctx, member)
		return err == nil && m.sideFamilyMember(ctx, meta.ID, meta.WorkspaceID, meta.ProfileID, meta.Lineage)
	})
}

func (m *Manager) deleteSideSessions(ctx context.Context, sourceID string) error {
	sides, ok := m.sessionCatalog.(store.SessionSideStore)
	if !ok {
		return nil
	}
	info, err := m.durableSessionInfoForLifecycleTransition(ctx, sourceID)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := sides.ListSessionSides(ctx, info.WorkspaceID, sourceID, true)
	if err != nil {
		return err
	}
	for i := range rows {
		row := &rows[i]
		if _, err := m.DeleteWithChildren(
			ctx,
			info.WorkspaceID,
			row.ID,
		); err != nil &&
			!errors.Is(err, ErrSessionNotFound) {
			return err
		}
	}
	return nil
}

func (m *Manager) publishSideFamily(ctx context.Context, workspaceID, sourceID string) {
	sides, ok := m.sessionCatalog.(store.SessionSideStore)
	if !ok {
		return
	}
	rows, err := sides.ListSessionSides(ctx, workspaceID, sourceID, true)
	if err != nil {
		m.logger.Warn("session.side.list_after_transition", "session_id", sourceID, "error", err)
		return
	}
	for i := range rows {
		m.publishSessionCatalogEvent(
			sessionCatalogEventFromInfo(CatalogEventUpserted, sessionInfoFromCatalog(&rows[i])),
		)
	}
}

func (m *Manager) RequireGoalSupported(ctx context.Context, sessionID string) error {
	return m.refuseSideOperation(ctx, sessionID, ErrSideGoalUnsupported)
}

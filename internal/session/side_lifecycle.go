package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) stopSideSessions(ctx context.Context, sourceID string) error {
	sides, ok := m.sessionCatalog.(store.SessionSideStore)
	if !ok {
		return nil
	}
	info, err := m.Status(ctx, sourceID)
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
	for _, row := range rows {
		if _, live := m.Get(row.ID); !live && !m.isPending(row.ID) {
			continue
		}
		if err := m.Stop(ctx, row.ID); err != nil && !errors.Is(err, ErrSessionNotFound) {
			return fmt.Errorf("stop side chat %s of %s: %w", row.ID, sourceID, err)
		}
	}
	return nil
}

func (m *Manager) archiveStoppingSideMembers(ctx context.Context, archive func() error) error {
	for {
		err := archive()
		refusal, ok := errors.AsType[*archiveRequiresStoppedError](err)
		if !ok {
			return err
		}
		meta, readErr := m.readSessionMetaReadOnly(ctx, refusal.member)
		if readErr != nil || !isSideLineage(meta.Lineage) {
			return err
		}
		if stopErr := m.Stop(ctx, refusal.member); stopErr != nil && !errors.Is(stopErr, ErrSessionNotFound) {
			return errors.Join(err, stopErr)
		}
	}
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
	for _, row := range rows {
		if _, err := m.DeleteWithChildren(ctx, info.WorkspaceID, row.ID); err != nil && !errors.Is(err, ErrSessionNotFound) {
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
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, sessionInfoFromCatalog(&rows[i])))
	}
}

func (m *Manager) RequireGoalSupported(ctx context.Context, sessionID string) error {
	return m.refuseSideOperation(ctx, sessionID, ErrSideGoalUnsupported)
}

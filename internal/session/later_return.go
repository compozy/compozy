package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
)

const (
	laterReturnBatchSize = 100
	// LaterReturnReason names a session the daemon brought back from Later.
	LaterReturnReason = "Back from Later"
)

// ReturnDueLaterSessions brings back every parked session whose return time
// passed. Each returned session lands in Needs you with a back-from-Later mark,
// and the operator is notified. The durable due scan makes the sweep safe to
// repeat and lets past deadlines fire on the first sweep after a restart.
func (m *Manager) ReturnDueLaterSessions(ctx context.Context) (int, error) {
	if ctx == nil {
		return 0, errors.New("session: later return context is required")
	}
	laterStore, err := m.laterStore()
	if err != nil {
		return 0, err
	}
	total := 0
	for {
		returned, err := laterStore.ReturnDueLaterSessions(ctx, m.now(), laterReturnBatchSize)
		if err != nil {
			return total, err
		}
		for i := range returned {
			m.announceLaterReturn(ctx, returned[i])
		}
		total += len(returned)
		if len(returned) < laterReturnBatchSize {
			return total, nil
		}
	}
}

func (m *Manager) announceLaterReturn(ctx context.Context, returned store.SessionLaterReturn) {
	if _, err := m.publishLaterCatalogUpsert(ctx, returned.SessionID); err != nil {
		m.logger.Warn("session: publish later return", "session_id", returned.SessionID, "error", err)
	}
	result, err := m.PublishOperatorNotification(ctx, NotifyRequest{
		SessionID:   returned.SessionID,
		ProfileID:   returned.ProfileID,
		WorkspaceID: returned.WorkspaceID,
		Title:       LaterReturnReason,
		Body:        laterReturnNotificationBody(returned),
	})
	if err != nil {
		m.logger.Warn("session: notify later return", "session_id", returned.SessionID, "error", err)
		return
	}
	m.logger.Info(
		"session: returned from later",
		"session_id", returned.SessionID,
		"notification", string(result.Outcome),
	)
}

func laterReturnNotificationBody(returned store.SessionLaterReturn) string {
	name := strings.TrimSpace(returned.Name)
	if name == "" || name == returned.SessionID {
		name = "A session"
	}
	body := name + " is back in Needs you."
	if reason := strings.TrimSpace(returned.Reason); reason != "" {
		body = name + " is back: " + reason
	}
	if utf8.RuneCountInString(body) <= NotificationBodyMaxChars {
		return body
	}
	runes := []rune(body)
	return string(runes[:NotificationBodyMaxChars-1]) + "…"
}

// clearLaterReturn drops the back-from-Later mark once the operator sees or
// resumes the session.
func (m *Manager) clearLaterReturn(ctx context.Context, sessionID string) {
	laterStore, err := m.laterStore()
	if err != nil {
		return
	}
	cleared, err := laterStore.ClearLaterReturn(ctx, sessionID)
	if err != nil {
		m.logger.Warn("session: clear later return", "session_id", sessionID, "error", err)
		return
	}
	if cleared {
		if _, err := m.publishLaterCatalogUpsert(ctx, sessionID); err != nil {
			m.logger.Warn("session: publish later return clear", "session_id", sessionID, "error", err)
		}
	}
}

func (m *Manager) populateLaterMetadata(ctx context.Context, info *Info) error {
	if info == nil || info.State != StateStopped {
		return nil
	}
	laterStore, err := m.laterStore()
	if err != nil {
		return nil
	}
	later, returnedAt, err := laterStore.SessionLaterState(ctx, info.WorkspaceID, info.ID)
	if err != nil {
		return fmt.Errorf("session: read later metadata for %q: %w", info.ID, err)
	}
	info.Later = later
	info.LaterReturnedAt = returnedAt
	return nil
}

// sessionInLater reports whether a stored session is parked.
func (m *Manager) sessionInLater(ctx context.Context, workspaceID string, sessionID string) (bool, error) {
	laterStore, err := m.laterStore()
	if err != nil {
		return false, nil
	}
	later, _, err := laterStore.SessionLaterState(ctx, workspaceID, sessionID)
	if err != nil {
		return false, fmt.Errorf("session: read later metadata for %q: %w", sessionID, err)
	}
	return later != nil, nil
}

func (m *Manager) requireSessionNotInLater(ctx context.Context, workspaceID string, sessionID string) error {
	parked, err := m.sessionInLater(ctx, workspaceID, sessionID)
	if err != nil {
		return err
	}
	if parked {
		return fmt.Errorf("%w: %s", ErrSessionInLater, sessionID)
	}
	return nil
}

// ResumeForOperator restarts a stopped session for an operator. An operator
// resume takes the session out of Later first; agent wakes never do.
func (m *Manager) ResumeForOperator(ctx context.Context, id string) (*Session, error) {
	if err := m.unparkForOperator(ctx, id); err != nil {
		return nil, err
	}
	return m.Resume(ctx, id)
}

func (m *Manager) unparkForOperator(ctx context.Context, sessionID string) error {
	meta, err := m.readMetaWithContext(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}
	laterStore, err := m.laterStore()
	if err != nil {
		return nil
	}
	unparked, err := laterStore.UnparkSession(ctx, meta.WorkspaceID, sessionID)
	if err != nil {
		return err
	}
	if unparked {
		if _, err := m.publishLaterCatalogUpsert(ctx, sessionID); err != nil {
			m.logger.Warn("session: publish operator unpark", "session_id", sessionID, "error", err)
		}
	}
	return nil
}

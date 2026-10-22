package profile

import (
	"context"
	"fmt"
)

type UserJournalRemover interface {
	RemoveUserJournal(context.Context, string) error
}

func WithUserJournalRemover(remover UserJournalRemover) Option {
	return func(m *Manager) error { m.userJournal = remover; return nil }
}

func (m *Manager) removeUserJournal(ctx context.Context, opID string) error {
	if m.userJournal == nil {
		return nil
	}
	var profileID string
	if err := m.store.DB().QueryRowContext(ctx,
		`SELECT profile_id FROM profile_lifecycle_ops WHERE id = ?`, opID).Scan(&profileID); err != nil {
		return fmt.Errorf("profile: read User journal owner: %w", err)
	}
	return m.userJournal.RemoveUserJournal(ctx, profileID)
}

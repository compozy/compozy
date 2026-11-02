package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func (m *Manager) removeAgentMemory(ctx context.Context, opID string) error {
	var profileID string
	if err := m.store.DB().QueryRowContext(ctx,
		`SELECT profile_id FROM profile_lifecycle_ops WHERE id = ?`, opID).Scan(&profileID); err != nil {
		return fmt.Errorf("profile: read agent memory owner: %w", err)
	}
	if profileID == "" || profileID == "." || profileID == ".." || filepath.Base(profileID) != profileID {
		return errors.New("profile: invalid agent memory owner")
	}
	root := filepath.Join(m.home.HomeDir, "agent-memory")
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("profile: invalid agent memory root")
	}
	return os.RemoveAll(filepath.Join(root, profileID))
}

package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	loggerpkg "github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/store"
)

// ReconcileInactiveSessions recovers retained metadata before runtime admission.
func (m *Manager) ReconcileInactiveSessions(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session: reconciliation context is required")
	}
	entries, err := os.ReadDir(m.homePaths.SessionsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("session: read recovery inventory: %w", err)
	}
	var failures loggerpkg.FailureSummary
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) {
			continue
		}
		if _, err := m.reconcileInactiveSession(ctx, entry.Name()); err != nil {
			if errors.Is(err, ErrRecoveryPersistence) {
				return err
			}
			if !errors.Is(err, ErrSessionNotFound) {
				failures.Add(entry.Name(), err)
			}
		}
	}
	m.metadataWarnings.Warn(m.logger, "session: skip unreadable session metadata", failures, m.now())
	return nil
}

func (m *Manager) reconcileInactiveSession(ctx context.Context, id string) (store.SessionMeta, error) {
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return meta, err
	}
	if _, active := m.Get(id); active || m.isPending(id) || m.hasPendingStopSettlement(id) {
		return meta, nil
	}
	if restored, err := m.restoreRecoveredStopReceipt(&meta); err != nil || restored {
		return meta, err
	}
	return m.repairInactiveMeta(ctx, store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, id)), &meta)
}

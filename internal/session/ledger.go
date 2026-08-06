package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) materializeSessionLedger(ctx context.Context, session *Session) error {
	if m == nil || m.ledgerMaterializer == nil || session == nil {
		return nil
	}
	stopCause, _ := session.stopCauseDetail()
	if stopCause == CauseClearConversation || stopCause == CauseConversationRewind {
		return nil
	}

	info := session.Info()
	if info == nil {
		return nil
	}

	ledgerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()

	record := sessionLedgerRecordFromInfo(info, session.DBPath())
	if err := m.ledgerMaterializer.MaterializeSessionLedger(ledgerCtx, record); err != nil {
		return fmt.Errorf("session: materialize ledger for %q: %w", info.ID, err)
	}
	return nil
}

func (m *Manager) discardOwnedMaterializedSessionLedger(
	ctx context.Context,
	owner store.SessionDBOwner,
	eventsDBPath string,
) error {
	if m == nil || m.ledgerMaterializer == nil {
		return nil
	}
	normalizedOwner, err := owner.Normalize()
	if err != nil {
		return err
	}
	ledgerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()

	record := store.SessionLedgerRecord{
		SessionID:    normalizedOwner.SessionID,
		WorkspaceID:  normalizedOwner.WorkspaceID,
		EventsDBPath: strings.TrimSpace(eventsDBPath),
	}
	if err := m.ledgerMaterializer.DiscardSessionLedger(ledgerCtx, record); err != nil {
		return fmt.Errorf("session: discard materialized ledger for %q: %w", normalizedOwner.SessionID, err)
	}
	return nil
}

func (m *Manager) discardMaterializedSessionLedgerForResume(
	ctx context.Context,
	meta store.SessionMeta,
) error {
	owner, err := m.resolveStoredSessionOwner(ctx, meta.ID, meta.WorkspaceID)
	if err != nil {
		return fmt.Errorf("session: resolve ledger owner before resume %q: %w", meta.ID, err)
	}
	dbPath := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, owner.SessionID))
	return m.discardOwnedMaterializedSessionLedger(ctx, owner, dbPath)
}

func (m *Manager) rematerializeStoppedSessionLedger(ctx context.Context, sessionID string) error {
	if m == nil || m.ledgerMaterializer == nil {
		return nil
	}
	meta, err := m.readMetaWithContext(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session: read metadata to restore ledger for %q: %w", sessionID, err)
	}
	owner, err := m.resolveStoredSessionOwner(ctx, meta.ID, meta.WorkspaceID)
	if err != nil {
		return fmt.Errorf("session: resolve ledger owner after failed resume %q: %w", sessionID, err)
	}
	dbPath := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, owner.SessionID))
	record := sessionLedgerRecordFromInfo(sessionInfoFromMeta(meta), dbPath)
	record.SessionID = owner.SessionID
	record.WorkspaceID = owner.WorkspaceID
	ledgerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	if err := m.ledgerMaterializer.MaterializeSessionLedger(ledgerCtx, record); err != nil {
		return fmt.Errorf("session: restore materialized ledger for %q: %w", sessionID, err)
	}
	return nil
}

func sessionLedgerRecordFromInfo(info *Info, eventsDBPath string) store.SessionLedgerRecord {
	if info == nil {
		return store.SessionLedgerRecord{}
	}
	return store.SessionLedgerRecord{
		SessionID:    strings.TrimSpace(info.ID),
		WorkspaceID:  strings.TrimSpace(info.WorkspaceID),
		AgentName:    strings.TrimSpace(info.AgentName),
		SessionType:  strings.TrimSpace(string(info.Type)),
		EventsDBPath: strings.TrimSpace(eventsDBPath),
		Lineage:      store.NormalizeSessionLineage(info.ID, info.Lineage),
		StartedAt:    normalizeLedgerTime(info.CreatedAt),
		EndedAt:      normalizeLedgerTime(info.UpdatedAt),
	}
}

func normalizeLedgerTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

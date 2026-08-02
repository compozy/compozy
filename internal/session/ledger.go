package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) materializeSessionLedger(ctx context.Context, session *Session) error {
	if m == nil || m.ledgerMaterializer == nil || session == nil {
		return nil
	}
	stopCause, _ := session.stopCauseDetail()
	if stopCause == CauseClearConversation {
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

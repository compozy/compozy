package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
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

func (m *Manager) discardMaterializedSessionLedger(
	ctx context.Context,
	meta store.SessionMeta,
	eventsDBPath string,
) error {
	if m == nil || m.ledgerMaterializer == nil {
		return nil
	}
	ledgerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()

	record := sessionLedgerRecordFromMeta(meta, eventsDBPath)
	if err := m.ledgerMaterializer.DiscardSessionLedger(ledgerCtx, record); err != nil {
		return fmt.Errorf("session: discard materialized ledger for %q: %w", meta.ID, err)
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

func sessionLedgerRecordFromMeta(meta store.SessionMeta, eventsDBPath string) store.SessionLedgerRecord {
	return store.SessionLedgerRecord{
		SessionID:    strings.TrimSpace(meta.ID),
		WorkspaceID:  strings.TrimSpace(meta.WorkspaceID),
		AgentName:    strings.TrimSpace(meta.AgentName),
		SessionType:  strings.TrimSpace(meta.SessionType),
		EventsDBPath: strings.TrimSpace(eventsDBPath),
		Lineage:      store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		StartedAt:    normalizeLedgerTime(meta.CreatedAt),
		EndedAt:      normalizeLedgerTime(meta.UpdatedAt),
	}
}

func normalizeLedgerTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

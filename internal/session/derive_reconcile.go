package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

// ReconcileDerivedSessionEvents re-emits session.derived for committed derived children
// whose ledger row is missing (ADR-007: a crash between the child's commit and the event
// write). The receipt's immutable outcome is the payload source; a child that already has
// the event, lost its derivation (cleared conversation), or has no matching receipt is
// skipped. It runs at boot before new work is admitted and returns the re-emitted count.
func (m *Manager) ReconcileDerivedSessionEvents(ctx context.Context, infos []*Info) (int, error) {
	if m == nil {
		return 0, errors.New("session: manager is required")
	}
	if ctx == nil {
		return 0, errors.New("session: derived event reconciliation context is required")
	}
	if m.eventLedger == nil {
		return 0, nil
	}
	derivations, ok := m.creationStore.(store.SessionDerivationStore)
	if !ok || derivations == nil {
		return 0, nil
	}
	emitted := 0
	var failures []error
	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return emitted, fmt.Errorf("session: derived event reconciliation canceled: %w", err)
		}
		reemitted, err := m.reconcileDerivedSessionEvent(ctx, derivations, info)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if reemitted {
			emitted++
		}
	}
	return emitted, errors.Join(failures...)
}

func (m *Manager) reconcileDerivedSessionEvent(
	ctx context.Context,
	derivations store.SessionDerivationStore,
	info *Info,
) (bool, error) {
	if info == nil || info.Derivation == nil || !info.Derivation.Kind.Derived() {
		return false, nil
	}
	key := strings.TrimSpace(info.Derivation.IdempotencyKey)
	if key == "" {
		return false, nil
	}
	rows, err := m.eventLedger.ListEventSummaries(ctx, store.EventSummaryQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.SessionDerived,
		SessionID: info.ID, Limit: 1,
	})
	if err != nil {
		return false, fmt.Errorf("session: read session.derived for %q: %w", info.ID, err)
	}
	if len(rows) > 0 {
		return false, nil
	}
	receipt, found, err := derivations.SessionDerivationReceipt(ctx, info.WorkspaceID, key)
	if err != nil {
		return false, fmt.Errorf("session: read derive receipt for %q: %w", info.ID, err)
	}
	if !found || receipt.ChildSessionID != info.ID {
		return false, nil
	}
	if err := m.writeSessionDerivedEvent(ctx, info, *info.Derivation, receipt.Outcome); err != nil {
		return false, err
	}
	return true, nil
}

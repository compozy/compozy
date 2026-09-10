package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ notifications.AttentionStore = (*NotificationRepo)(nil)

const attentionSnapshotLifetime = 24 * time.Hour

// CaptureAttentionSnapshot freezes the complete unread population before any display limit.
func (r *NotificationRepo) CaptureAttentionSnapshot(
	ctx context.Context, scope notifications.AttentionScope, occurrences []string,
) (snapshot string, unread []string, err error) {
	if err := r.checkReady(ctx, "capture attention snapshot"); err != nil {
		return "", nil, err
	}
	if err := scope.Validate(); err != nil {
		return "", nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, fmt.Errorf("store: begin attention snapshot: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	queries := r.queries.WithTx(tx)
	now := r.now().UTC()
	if err := queries.DeleteExpiredAttentionSnapshots(ctx, store.FormatTimestamp(now)); err != nil {
		return "", nil, fmt.Errorf("store: expire attention snapshots: %w", err)
	}
	_, ids := notifications.AttentionSnapshotIdentity(scope, occurrences)
	raw, err := json.Marshal(ids)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode attention occurrences: %w", err)
	}
	acknowledged, err := queries.ListAcknowledgedAttention(ctx, sqlcgen.ListAcknowledgedAttentionParams{
		ProfileID: scope.ProfileID, ActorKind: scope.ActorKind, ActorID: scope.ActorID, OccurrenceIds: string(raw),
	})
	if err != nil {
		return "", nil, fmt.Errorf("store: read attention receipts: %w", err)
	}
	seen := make(map[string]struct{}, len(acknowledged))
	for _, id := range acknowledged {
		seen[id] = struct{}{}
	}
	unread = slices.DeleteFunc(ids, func(id string) bool { _, ok := seen[id]; return ok })
	snapshot, unread = notifications.AttentionSnapshotIdentity(scope, unread)
	if unread == nil {
		unread = []string{}
	}
	raw, err = json.Marshal(unread)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode attention snapshot: %w", err)
	}
	if err := queries.SaveAttentionSnapshot(ctx, sqlcgen.SaveAttentionSnapshotParams{
		ID:            snapshot,
		ProfileID:     scope.ProfileID,
		ActorKind:     scope.ActorKind,
		ActorID:       scope.ActorID,
		Population:    scope.Population,
		OccurrenceIds: string(raw),
		ExpiresAt:     store.FormatTimestamp(now.Add(attentionSnapshotLifetime)),
	}); err != nil {
		return "", nil, fmt.Errorf("store: save attention snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", nil, fmt.Errorf("store: commit attention snapshot: %w", err)
	}
	return snapshot, unread, nil
}

// AcknowledgeAttentionSnapshot commits all requested receipts in one statement or none.
func (r *NotificationRepo) AcknowledgeAttentionSnapshot(
	ctx context.Context, scope notifications.AttentionScope, snapshot, occurrence string,
) error {
	if err := r.checkReady(ctx, "acknowledge attention snapshot"); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	raw, err := r.queries.GetAttentionSnapshot(ctx, sqlcgen.GetAttentionSnapshotParams{
		ID: snapshot, ProfileID: scope.ProfileID, ActorKind: scope.ActorKind, ActorID: scope.ActorID,
		Population: scope.Population, Now: store.FormatTimestamp(r.now()),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return notifications.ErrAttentionSnapshotUnavailable
	}
	if err != nil {
		return fmt.Errorf("store: read attention snapshot: %w", err)
	}
	if occurrence != "" {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			return fmt.Errorf("store: decode attention snapshot: %w", err)
		}
		if !slices.Contains(ids, occurrence) {
			return notifications.ErrAttentionSnapshotUnavailable
		}
		encoded, err := json.Marshal([]string{occurrence})
		if err != nil {
			return fmt.Errorf("store: encode attention receipt: %w", err)
		}
		raw = string(encoded)
	}
	if err := r.queries.AcknowledgeAttentionOccurrences(ctx, sqlcgen.AcknowledgeAttentionOccurrencesParams{
		ProfileID: scope.ProfileID, ActorKind: scope.ActorKind, ActorID: scope.ActorID,
		OccurrenceIds: raw, Now: store.FormatTimestamp(r.now()),
	}); err != nil {
		return fmt.Errorf("store: acknowledge attention occurrences: %w", err)
	}
	return nil
}

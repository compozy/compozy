package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func (g *BridgeRepo) PutBridgeIngestDedup(ctx context.Context, record bridges.IngestDedupRecord) error {
	if err := g.checkReady(ctx, "put bridge ingest dedup"); err != nil {
		return err
	}

	normalized := record
	if err := normalized.Validate(); err != nil {
		return err
	}

	if err := g.queries.UpsertBridgeIngestDedup(ctx, sqlcgen.UpsertBridgeIngestDedupParams{
		IdempotencyKey:   normalized.IdempotencyKey,
		BridgeInstanceID: normalized.BridgeInstanceID,
		ReceivedAt: store.FormatTimestamp(
			normalized.ReceivedAt,
		),
		ExpiresAt: store.FormatTimestamp(normalized.ExpiresAt),
	}); err != nil {
		return fmt.Errorf(
			"store: put bridge ingest dedup %q: %w",
			normalized.IdempotencyKey,
			mapBridgeChildConstraintError(err),
		)
	}

	return nil
}

// GetBridgeIngestDedup loads one active dedup record and excludes expired rows at the supplied lookup time.
func (g *BridgeRepo) GetBridgeIngestDedup(
	ctx context.Context,
	idempotencyKey string,
	lookupAt time.Time,
) (bridges.IngestDedupRecord, error) {
	if err := g.checkReady(ctx, "get bridge ingest dedup"); err != nil {
		return bridges.IngestDedupRecord{}, err
	}

	trimmedKey := strings.TrimSpace(idempotencyKey)
	if trimmedKey == "" {
		return bridges.IngestDedupRecord{}, errors.New("store: ingest dedup idempotency key is required")
	}
	if lookupAt.IsZero() {
		lookupAt = g.now()
	}

	row, err := g.queries.GetBridgeIngestDedup(ctx, sqlcgen.GetBridgeIngestDedupParams{
		IdempotencyKey: trimmedKey, LookupAt: store.FormatTimestamp(lookupAt),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bridges.IngestDedupRecord{}, bridges.ErrIngestDedupRecordNotFound
		}
		return bridges.IngestDedupRecord{}, err
	}
	return bridgeDedupFromGenerated(row)
}

// DeleteExpiredBridgeIngestDedup removes expired dedup rows and reports how many were deleted.
func (g *BridgeRepo) DeleteExpiredBridgeIngestDedup(ctx context.Context, now time.Time) (int64, error) {
	if err := g.checkReady(ctx, "delete expired bridge ingest dedup"); err != nil {
		return 0, err
	}
	if now.IsZero() {
		now = g.now()
	}

	affected, err := g.queries.DeleteExpiredBridgeIngestDedup(ctx, store.FormatTimestamp(now))
	if err != nil {
		return 0, fmt.Errorf("store: delete expired bridge ingest dedup: %w", err)
	}

	return affected, nil
}

// PutBridgeTaskSubscription inserts or refreshes a terminal task notification subscription.
func (g *BridgeRepo) PutBridgeTaskSubscription(
	ctx context.Context,
	subscription bridges.BridgeTaskSubscription,
) error {
	if err := g.checkReady(ctx, "put bridge task subscription"); err != nil {
		return err
	}
	normalized := subscription.Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	if err := g.queries.UpsertBridgeTaskSubscription(ctx, sqlcgen.UpsertBridgeTaskSubscriptionParams{
		SubscriptionID: normalized.SubscriptionID, TaskID: normalized.TaskID,
		BridgeInstanceID: normalized.BridgeInstanceID, Scope: string(normalized.Scope),
		WorkspaceID: nullableBridgeString(normalized.WorkspaceID), PeerID: nullableBridgeString(normalized.PeerID),
		ThreadID: nullableBridgeString(normalized.ThreadID), GroupID: nullableBridgeString(normalized.GroupID),
		DeliveryMode: string(normalized.DeliveryMode), CreatedByKind: string(normalized.CreatedBy.Kind),
		CreatedByRef: normalized.CreatedBy.Ref, CreatedAt: store.FormatTimestamp(normalized.CreatedAt),
		UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
	}); err != nil {
		return fmt.Errorf(
			"store: put bridge task subscription %q: %w",
			normalized.SubscriptionID,
			err,
		)
	}
	return nil
}

// GetBridgeTaskSubscription loads one persisted task notification subscription.
func (g *BridgeRepo) GetBridgeTaskSubscription(
	ctx context.Context,
	subscriptionID string,
) (bridges.BridgeTaskSubscription, error) {
	if err := g.checkReady(ctx, "get bridge task subscription"); err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	trimmedID := strings.TrimSpace(subscriptionID)
	if trimmedID == "" {
		return bridges.BridgeTaskSubscription{}, errors.New("store: bridge task subscription id is required")
	}

	row, err := g.queries.GetBridgeTaskSubscription(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bridges.BridgeTaskSubscription{}, bridges.ErrBridgeTaskSubscriptionNotFound
		}
		return bridges.BridgeTaskSubscription{}, err
	}
	return bridgeSubscriptionFromGenerated(row)
}

// ListBridgeTaskSubscriptions returns active bridge task subscriptions matching the query.
func (g *BridgeRepo) ListBridgeTaskSubscriptions(
	ctx context.Context,
	query bridges.BridgeTaskSubscriptionQuery,
) (subscriptions []bridges.BridgeTaskSubscription, err error) {
	if err := g.checkReady(ctx, "list bridge task subscriptions"); err != nil {
		return nil, err
	}
	normalized := query.Normalize()
	// dynamic-sql: optional task/bridge/scope/workspace filters and caller limit change the statement shape.
	sqlQuery := `SELECT
			subscription_id, task_id, bridge_instance_id, scope, workspace_id,
			peer_id, thread_id, group_id, delivery_mode, created_by_kind,
			created_by_ref, created_at, updated_at
		FROM bridge_task_subscriptions`
	where, args := store.BuildClauses(
		store.StringClause("task_id", normalized.TaskID),
		store.StringClause("bridge_instance_id", normalized.BridgeInstanceID),
		store.StringClause("scope", string(normalized.Scope)),
		store.StringClause("workspace_id", normalized.WorkspaceID),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY task_id ASC, updated_at DESC, subscription_id ASC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, normalized.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge task subscriptions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close bridge task subscription rows: %w", closeErr))
		}
	}()

	subscriptions = make([]bridges.BridgeTaskSubscription, 0)
	for rows.Next() {
		subscription, scanErr := scanBridgeTaskSubscription(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge task subscriptions: %w", err)
	}
	return subscriptions, nil
}

// DeleteBridgeTaskSubscription removes one terminal task notification subscription.
func (g *BridgeRepo) DeleteBridgeTaskSubscription(ctx context.Context, subscriptionID string) error {
	if err := g.checkReady(ctx, "delete bridge task subscription"); err != nil {
		return err
	}
	trimmedID := strings.TrimSpace(subscriptionID)
	if trimmedID == "" {
		return errors.New("store: bridge task subscription id is required")
	}
	affected, err := g.queries.DeleteBridgeTaskSubscription(ctx, trimmedID)
	if err != nil {
		return fmt.Errorf("store: delete bridge task subscription %q: %w", trimmedID, err)
	}
	if affected == 0 {
		return fmt.Errorf(
			"store: bridge task subscription %q: %w",
			trimmedID,
			bridges.ErrBridgeTaskSubscriptionNotFound,
		)
	}
	return nil
}

package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
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
		WorkspaceID:  nullableOpaqueBridgeString(normalized.WorkspaceID),
		PeerID:       nullableOpaqueBridgeString(normalized.PeerID),
		ThreadID:     nullableOpaqueBridgeString(normalized.ThreadID),
		GroupID:      nullableOpaqueBridgeString(normalized.GroupID),
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
	readScope store.ReadScope,
	subscriptionID string,
) (bridges.BridgeTaskSubscription, error) {
	if err := g.checkReady(ctx, "get bridge task subscription"); err != nil {
		return bridges.BridgeTaskSubscription{}, err
	}
	if subscriptionID == "" {
		return bridges.BridgeTaskSubscription{}, errors.New("store: bridge task subscription id is required")
	}
	if !utf8.ValidString(subscriptionID) {
		return bridges.BridgeTaskSubscription{}, errors.New("store: bridge task subscription id must be valid UTF-8")
	}
	if err := readScope.Validate(); err != nil {
		return bridges.BridgeTaskSubscription{}, fmt.Errorf(
			"store: invalid bridge task subscription read scope: %w",
			err,
		)
	}
	statement := `SELECT
		bs.subscription_id, bi.profile_id, p.name, p.color, COALESCE(p.icon, ''), COALESCE(p.emoji, ''),
		p.archived_at IS NOT NULL,
		bs.task_id, bs.bridge_instance_id, bs.scope, bs.workspace_id,
		bs.peer_id, bs.thread_id, bs.group_id, bs.delivery_mode, bs.created_by_kind,
		bs.created_by_ref, bs.created_at, bs.updated_at
		FROM bridge_task_subscriptions bs
		JOIN bridge_instances bi ON bi.id = bs.bridge_instance_id
		JOIN profiles p ON p.id = bi.profile_id
		WHERE bs.subscription_id = ?`
	args := []any{subscriptionID}
	if !readScope.AllProfiles {
		statement += globalDBBridgeProfileClause
		args = append(args, readScope.ProfileID)
	}
	row := g.db.QueryRowContext(ctx, statement, args...)
	subscription, err := scanBridgeTaskSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bridges.BridgeTaskSubscription{}, bridges.ErrBridgeTaskSubscriptionNotFound
		}
		return bridges.BridgeTaskSubscription{}, err
	}
	return subscription, nil
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
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	// dynamic-sql: optional task/bridge/scope/workspace filters and caller limit change the statement shape.
	sqlQuery := `SELECT
			bs.subscription_id, bi.profile_id, p.name, p.color, COALESCE(p.icon, ''), COALESCE(p.emoji, ''),
			p.archived_at IS NOT NULL,
			bs.task_id, bs.bridge_instance_id, bs.scope, bs.workspace_id,
			bs.peer_id, bs.thread_id, bs.group_id, bs.delivery_mode, bs.created_by_kind,
			bs.created_by_ref, bs.created_at, bs.updated_at
		FROM bridge_task_subscriptions bs
		JOIN bridge_instances bi ON bi.id = bs.bridge_instance_id
		JOIN profiles p ON p.id = bi.profile_id`
	where, args := store.BuildClauses(
		store.ReadScopeClause("bi.profile_id", normalized.ReadScope),
		store.OpaqueStringClause("bs.task_id", normalized.TaskID),
		store.OpaqueStringClause("bs.bridge_instance_id", normalized.BridgeInstanceID),
		store.StringClause("bs.scope", string(normalized.Scope)),
		store.OpaqueStringClause("bs.workspace_id", normalized.WorkspaceID),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY bs.task_id ASC, bs.updated_at DESC, bs.subscription_id ASC"
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
	if subscriptionID == "" {
		return errors.New("store: bridge task subscription id is required")
	}
	if !utf8.ValidString(subscriptionID) {
		return errors.New("store: bridge task subscription id must be valid UTF-8")
	}
	affected, err := g.queries.DeleteBridgeTaskSubscription(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("store: delete bridge task subscription %q: %w", subscriptionID, err)
	}
	if affected == 0 {
		return fmt.Errorf(
			"store: bridge task subscription %q: %w",
			subscriptionID,
			bridges.ErrBridgeTaskSubscriptionNotFound,
		)
	}
	return nil
}

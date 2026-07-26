package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// PutBridgeDeliveryMetrics persists one complete monotonic counter snapshot.
func (g *BridgeRepo) PutBridgeDeliveryMetrics(ctx context.Context, record bridges.DeliveryMetricRecord) error {
	if err := g.checkReady(ctx, "put bridge delivery metrics"); err != nil {
		return err
	}
	normalized, err := record.Canonicalize()
	if err != nil {
		return err
	}
	return g.withImmediateTransaction(ctx, "put bridge delivery metrics", func(exec globalSQLExecutor) error {
		return putBridgeDeliveryMetricsExec(ctx, sqlcgen.New(exec), normalized)
	})
}

// ListBridgeDeliveryMetrics returns durable counters plus backlog derived from active rows.
func (g *BridgeRepo) ListBridgeDeliveryMetrics(
	ctx context.Context,
	query bridges.DeliveryLedgerQuery,
) (metrics map[string]bridges.BridgeDeliveryMetrics, err error) {
	if err := g.checkReady(ctx, "list bridge delivery metrics"); err != nil {
		return nil, err
	}
	normalized, err := query.Canonicalize()
	if err != nil {
		return nil, err
	}
	if normalized.State != "" {
		return nil, errors.New("store: bridge delivery metrics query cannot filter by delivery state")
	}

	metricWhere, metricArgs := bridgeDeliveryMetricQueryClauses(normalized)
	deliveryWhere, deliveryArgs := bridgeDeliveryMetricQueryClauses(normalized)
	backlogWhere, backlogArgs := store.BuildClauses(
		store.StringClause("scope", string(normalized.Scope)),
		store.StringClause("workspace_id", normalized.WorkspaceID),
		store.StringClause("bridge_instance_id", normalized.BridgeInstanceID),
		store.StringClause("state", string(bridges.DeliveryLedgerStateActive)),
	)
	metricIDs := store.AppendWhere(
		`SELECT bridge_instance_id FROM bridge_delivery_metrics`,
		metricWhere,
	)
	deliveryIDs := store.AppendWhere(
		`SELECT bridge_instance_id FROM bridge_deliveries`,
		deliveryWhere,
	)
	backlog := store.AppendWhere(
		`SELECT bridge_instance_id, COUNT(*) AS delivery_backlog FROM bridge_deliveries`,
		backlogWhere,
	) + ` GROUP BY bridge_instance_id`
	// dynamic-sql: optional scope/workspace/instance filters and caller limit change all three CTE branches.
	// #nosec G202 -- predicates are fixed package SQL; all caller values remain parameterized.
	statement := `WITH scoped_instances AS (` + metricIDs + ` UNION ` + deliveryIDs + `),
		active_backlog AS (` + backlog + `)
		SELECT
			i.bridge_instance_id,
			COALESCE(m.delivery_dropped_total, 0),
			COALESCE(m.delivery_dropped_by_reason_json, '{}'),
			COALESCE(m.delivery_failures_total, 0),
			m.last_error,
			m.last_error_at,
			m.last_success_at,
			COALESCE(b.delivery_backlog, 0)
		FROM scoped_instances i
		LEFT JOIN bridge_delivery_metrics m ON m.bridge_instance_id = i.bridge_instance_id
		LEFT JOIN active_backlog b ON b.bridge_instance_id = i.bridge_instance_id
		ORDER BY i.bridge_instance_id ASC`
	args := make([]any, 0, len(metricArgs)+len(deliveryArgs)+len(backlogArgs))
	args = append(args, metricArgs...)
	args = append(args, deliveryArgs...)
	args = append(args, backlogArgs...)
	statement, args = store.AppendLimit(statement, args, normalized.Limit)

	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge delivery metrics: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("store: close bridge delivery metric rows: %w", closeErr)
		}
	}()

	metrics = make(map[string]bridges.BridgeDeliveryMetrics)
	for rows.Next() {
		metric, scanErr := scanBridgeDeliveryMetrics(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		metrics[metric.BridgeInstanceID] = metric
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge delivery metric rows: %w", err)
	}
	return metrics, nil
}

func bridgeDeliveryMetricQueryClauses(query bridges.DeliveryLedgerQuery) ([]string, []any) {
	return store.BuildClauses(
		store.StringClause("scope", string(query.Scope)),
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("bridge_instance_id", query.BridgeInstanceID),
	)
}

func putBridgeDeliveryMetricsExec(
	ctx context.Context,
	queries *sqlcgen.Queries,
	record bridges.DeliveryMetricRecord,
) error {
	if err := verifyBridgeDeliveryInstanceScope(
		ctx,
		queries,
		record.BridgeInstanceID,
		record.Scope,
		record.WorkspaceID,
	); err != nil {
		return err
	}
	current, found, err := getBridgeDeliveryMetricRecord(ctx, queries, record.BridgeInstanceID)
	if err != nil {
		return err
	}
	if found {
		record, err = mergeBridgeDeliveryMetricRecord(current, record)
		if err != nil {
			return err
		}
	}
	droppedByReasonJSON, err := json.Marshal(record.DeliveryDroppedByReason)
	if err != nil {
		return fmt.Errorf("store: encode bridge delivery drop reasons: %w", err)
	}
	if err := queries.UpsertBridgeDeliveryMetrics(ctx, sqlcgen.UpsertBridgeDeliveryMetricsParams{
		BridgeInstanceID: record.BridgeInstanceID, Scope: string(record.Scope),
		WorkspaceID:                 nullableBridgeString(record.WorkspaceID),
		DeliveryDroppedTotal:        int64(record.DeliveryDroppedTotal),
		DeliveryDroppedByReasonJson: string(droppedByReasonJSON),
		DeliveryFailuresTotal:       int64(record.DeliveryFailuresTotal),
		LastError:                   nullableBridgeString(record.LastError),
		LastErrorAt:                 nullableBridgeTimestamp(record.LastErrorAt),
		LastSuccessAt:               nullableBridgeTimestamp(record.LastSuccessAt),
		UpdatedAt:                   store.FormatTimestamp(record.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("store: put bridge delivery metrics %q: %w", record.BridgeInstanceID, err)
	}
	return nil
}

func getBridgeDeliveryMetricRecord(
	ctx context.Context,
	queries *sqlcgen.Queries,
	bridgeInstanceID string,
) (bridges.DeliveryMetricRecord, bool, error) {
	row, err := queries.GetBridgeDeliveryMetric(ctx, bridgeInstanceID)
	if errors.Is(err, sql.ErrNoRows) {
		return bridges.DeliveryMetricRecord{}, false, nil
	}
	if err != nil {
		return bridges.DeliveryMetricRecord{}, false, fmt.Errorf(
			"store: get bridge delivery metrics %q: %w",
			bridgeInstanceID,
			err,
		)
	}
	record, err := bridgeDeliveryMetricFromGenerated(row)
	return record, true, err
}

func mergeBridgeDeliveryMetricRecord(
	current bridges.DeliveryMetricRecord,
	next bridges.DeliveryMetricRecord,
) (bridges.DeliveryMetricRecord, error) {
	if current.Scope != next.Scope || current.WorkspaceID != next.WorkspaceID {
		return bridges.DeliveryMetricRecord{}, deliveryMetricConflict(next.BridgeInstanceID, "scope changed")
	}
	merged := next
	merged.UpdatedAt = maxBridgeDeliveryTime(current.UpdatedAt, next.UpdatedAt)
	merged.DeliveryDroppedByReason = make(
		map[string]int,
		len(current.DeliveryDroppedByReason)+len(next.DeliveryDroppedByReason),
	)
	maps.Copy(merged.DeliveryDroppedByReason, current.DeliveryDroppedByReason)
	for reason, count := range next.DeliveryDroppedByReason {
		if count > merged.DeliveryDroppedByReason[reason] {
			merged.DeliveryDroppedByReason[reason] = count
		}
	}
	merged.DeliveryDroppedTotal = 0
	for _, count := range merged.DeliveryDroppedByReason {
		merged.DeliveryDroppedTotal += count
	}
	if current.DeliveryFailuresTotal > merged.DeliveryFailuresTotal {
		merged.DeliveryFailuresTotal = current.DeliveryFailuresTotal
	}
	if !current.LastErrorAt.IsZero() && !next.LastErrorAt.After(current.LastErrorAt) {
		merged.LastError = current.LastError
		merged.LastErrorAt = current.LastErrorAt
	}
	merged.LastSuccessAt = maxBridgeDeliveryTime(current.LastSuccessAt, next.LastSuccessAt)
	return merged.Canonicalize()
}

func validateDeliveryMetricIdentity(
	metrics bridges.DeliveryMetricRecord,
	delivery bridges.DeliveryLedgerRecord,
) error {
	if metrics.BridgeInstanceID != delivery.BridgeInstanceID ||
		metrics.Scope != delivery.Scope ||
		metrics.WorkspaceID != delivery.WorkspaceID {
		return deliveryMetricConflict(metrics.BridgeInstanceID, "identity does not match delivery")
	}
	return nil
}

func deliveryMetricConflict(bridgeInstanceID string, reason string) error {
	return fmt.Errorf(
		"store: bridge delivery metrics %q %s: %w",
		strings.TrimSpace(bridgeInstanceID),
		reason,
		bridges.ErrDeliveryLedgerConflict,
	)
}

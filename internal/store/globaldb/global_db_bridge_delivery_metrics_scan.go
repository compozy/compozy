package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type bridgeDeliveryMetricStorageRow struct {
	bridgeInstanceID            string
	scope                       string
	workspaceID                 sql.NullString
	deliveryDroppedTotal        int64
	deliveryDroppedByReasonJSON string
	deliveryFailuresTotal       int64
	lastError                   sql.NullString
	lastErrorAt                 sql.NullString
	lastSuccessAt               sql.NullString
	updatedAt                   string
}

func bridgeDeliveryMetricFromGenerated(row sqlcgen.BridgeDeliveryMetric) (bridges.DeliveryMetricRecord, error) {
	return bridgeDeliveryMetricRecordFromStorageRow(bridgeDeliveryMetricStorageRow{
		bridgeInstanceID: row.BridgeInstanceID, scope: row.Scope, workspaceID: row.WorkspaceID,
		deliveryDroppedTotal:        row.DeliveryDroppedTotal,
		deliveryDroppedByReasonJSON: row.DeliveryDroppedByReasonJson,
		deliveryFailuresTotal:       row.DeliveryFailuresTotal, lastError: row.LastError,
		lastErrorAt: row.LastErrorAt, lastSuccessAt: row.LastSuccessAt, updatedAt: row.UpdatedAt,
	})
}

func bridgeDeliveryMetricRecordFromStorageRow(
	row bridgeDeliveryMetricStorageRow,
) (bridges.DeliveryMetricRecord, error) {
	droppedTotal, err := bridgeDeliveryMetricCountFromInt64(row.deliveryDroppedTotal, "dropped total")
	if err != nil {
		return bridges.DeliveryMetricRecord{}, err
	}
	failuresTotal, err := bridgeDeliveryMetricCountFromInt64(row.deliveryFailuresTotal, "failures total")
	if err != nil {
		return bridges.DeliveryMetricRecord{}, err
	}
	record := bridges.DeliveryMetricRecord{
		Scope:       bridges.Scope(row.scope),
		WorkspaceID: bridgeStringValue(row.workspaceID),
		BridgeDeliveryMetrics: bridges.BridgeDeliveryMetrics{
			BridgeInstanceID:      row.bridgeInstanceID,
			DeliveryDroppedTotal:  droppedTotal,
			DeliveryFailuresTotal: failuresTotal,
			LastError:             bridgeStringValue(row.lastError),
		},
	}
	if err := json.Unmarshal([]byte(row.deliveryDroppedByReasonJSON), &record.DeliveryDroppedByReason); err != nil {
		return bridges.DeliveryMetricRecord{}, fmt.Errorf("store: decode bridge delivery drop reasons: %w", err)
	}
	if record.LastErrorAt, err = parseDeliveryMetricTime(row.lastErrorAt); err != nil {
		return bridges.DeliveryMetricRecord{}, fmt.Errorf("store: parse bridge delivery last_error_at: %w", err)
	}
	if record.LastSuccessAt, err = parseDeliveryMetricTime(row.lastSuccessAt); err != nil {
		return bridges.DeliveryMetricRecord{}, fmt.Errorf("store: parse bridge delivery last_success_at: %w", err)
	}
	record.UpdatedAt, err = store.ParseTimestamp(row.updatedAt)
	if err != nil {
		return bridges.DeliveryMetricRecord{}, fmt.Errorf("store: parse bridge delivery metric updated_at: %w", err)
	}
	normalized, err := record.Canonicalize()
	if err != nil {
		return bridges.DeliveryMetricRecord{}, fmt.Errorf("store: validate bridge delivery metric row: %w", err)
	}
	return normalized, nil
}

func bridgeDeliveryMetricCountFromInt64(value int64, label string) (int, error) {
	if value < 0 || uint64(value) > uint64(^uint(0)>>1) {
		return 0, fmt.Errorf("store: bridge delivery metric %s %d is outside int range", label, value)
	}
	return int(value), nil
}

func scanBridgeDeliveryMetrics(scanner rowScanner) (bridges.BridgeDeliveryMetrics, error) {
	var (
		metrics       bridges.BridgeDeliveryMetrics
		droppedJSON   string
		lastError     sql.NullString
		lastErrorAt   sql.NullString
		lastSuccessAt sql.NullString
	)
	if err := scanner.Scan(
		&metrics.BridgeInstanceID,
		&metrics.DeliveryDroppedTotal,
		&droppedJSON,
		&metrics.DeliveryFailuresTotal,
		&lastError,
		&lastErrorAt,
		&lastSuccessAt,
		&metrics.DeliveryBacklog,
	); err != nil {
		return bridges.BridgeDeliveryMetrics{}, fmt.Errorf("store: scan bridge delivery metrics: %w", err)
	}
	if err := json.Unmarshal([]byte(droppedJSON), &metrics.DeliveryDroppedByReason); err != nil {
		return bridges.BridgeDeliveryMetrics{}, fmt.Errorf("store: decode bridge delivery metric reasons: %w", err)
	}
	metrics.LastError = bridgeStringValue(lastError)
	var err error
	if metrics.LastErrorAt, err = parseDeliveryMetricTime(lastErrorAt); err != nil {
		return bridges.BridgeDeliveryMetrics{}, fmt.Errorf("store: parse metric last_error_at: %w", err)
	}
	if metrics.LastSuccessAt, err = parseDeliveryMetricTime(lastSuccessAt); err != nil {
		return bridges.BridgeDeliveryMetrics{}, fmt.Errorf("store: parse metric last_success_at: %w", err)
	}
	droppedTotal := 0
	for reason, count := range metrics.DeliveryDroppedByReason {
		if strings.TrimSpace(reason) == "" || count <= 0 {
			return bridges.BridgeDeliveryMetrics{}, fmt.Errorf(
				"store: invalid bridge delivery drop reason %q count %d",
				reason,
				count,
			)
		}
		droppedTotal += count
	}
	if droppedTotal != metrics.DeliveryDroppedTotal {
		return bridges.BridgeDeliveryMetrics{}, fmt.Errorf(
			"store: bridge delivery dropped total %d does not match reason total %d",
			metrics.DeliveryDroppedTotal,
			droppedTotal,
		)
	}
	return metrics, nil
}

func parseDeliveryMetricTime(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, nil
	}
	return store.ParseTimestamp(value.String)
}

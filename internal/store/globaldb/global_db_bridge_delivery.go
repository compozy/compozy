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

const bridgeDeliverySelectColumns = `
	delivery_id, session_id, turn_id, routing_key, bridge_instance_id,
	scope, workspace_id, state, last_sent_seq, last_acked_seq,
	remote_message_id, terminal_error, created_at, updated_at`

var _ bridges.DeliveryLedgerStore = (*BridgeRepo)(nil)

// CreateBridgeDelivery registers one active checkpoint before delivery starts.
func (g *BridgeRepo) CreateBridgeDelivery(ctx context.Context, record bridges.DeliveryLedgerRecord) error {
	if err := g.checkReady(ctx, "create bridge delivery"); err != nil {
		return err
	}
	normalized, err := record.Canonicalize()
	if err != nil {
		return err
	}
	if normalized.State != bridges.DeliveryLedgerStateActive {
		return errors.New("store: new bridge delivery must be active")
	}
	if normalized.LastSentSeq != 0 || normalized.LastAckedSeq != 0 || normalized.RemoteMessageID != "" {
		return errors.New("store: new bridge delivery cannot start with an acknowledgement checkpoint")
	}
	routingKey, err := normalized.RoutingKey.Serialize()
	if err != nil {
		return fmt.Errorf("store: serialize bridge delivery routing key: %w", err)
	}

	return g.withImmediateTransaction(ctx, "create bridge delivery", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		exists, err := queries.BridgeDeliveryExists(ctx, normalized.DeliveryID)
		if err != nil {
			return fmt.Errorf("store: check bridge delivery %q: %w", normalized.DeliveryID, err)
		}
		if exists {
			return fmt.Errorf(
				"store: create bridge delivery %q: %w",
				normalized.DeliveryID,
				bridges.ErrDeliveryLedgerConflict,
			)
		}
		if err := verifyBridgeDeliveryInstanceScope(
			ctx,
			queries,
			normalized.BridgeInstanceID,
			normalized.Scope,
			normalized.WorkspaceID,
		); err != nil {
			return err
		}
		if err := queries.InsertBridgeDelivery(ctx, sqlcgen.InsertBridgeDeliveryParams{
			DeliveryID: normalized.DeliveryID, SessionID: normalized.SessionID,
			TurnID: normalized.TurnID, RoutingKey: routingKey,
			BridgeInstanceID: normalized.BridgeInstanceID, Scope: string(normalized.Scope),
			WorkspaceID: nullableBridgeString(normalized.WorkspaceID), State: string(normalized.State),
			LastSentSeq: normalized.LastSentSeq, LastAckedSeq: normalized.LastAckedSeq,
			RemoteMessageID: nullableBridgeString(normalized.RemoteMessageID),
			TerminalError:   nullableBridgeString(normalized.TerminalError),
			CreatedAt:       store.FormatTimestamp(normalized.CreatedAt),
			UpdatedAt:       store.FormatTimestamp(normalized.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("store: insert bridge delivery %q: %w", normalized.DeliveryID, err)
		}
		return nil
	})
}

// CheckpointBridgeDelivery advances sequences, remote identity, terminal state, and optional metrics.
func (g *BridgeRepo) CheckpointBridgeDelivery(
	ctx context.Context,
	checkpoint bridges.DeliveryLedgerCheckpoint,
) error {
	if err := g.checkReady(ctx, "checkpoint bridge delivery"); err != nil {
		return err
	}
	normalized, err := checkpoint.Canonicalize()
	if err != nil {
		return err
	}

	return g.withImmediateTransaction(ctx, "checkpoint bridge delivery", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		current, err := getBridgeDelivery(ctx, queries, normalized.DeliveryID)
		if err != nil {
			return err
		}
		resolved, err := resolveBridgeDeliveryCheckpoint(current, normalized)
		if err != nil {
			return err
		}
		if normalized.Metrics != nil {
			if err := validateDeliveryMetricIdentity(*normalized.Metrics, current); err != nil {
				return err
			}
			if err := putBridgeDeliveryMetricsExec(ctx, queries, *normalized.Metrics); err != nil {
				return err
			}
		}
		if err := queries.UpdateBridgeDeliveryCheckpoint(ctx, sqlcgen.UpdateBridgeDeliveryCheckpointParams{
			State: string(resolved.State), LastSentSeq: resolved.LastSentSeq,
			LastAckedSeq:    resolved.LastAckedSeq,
			RemoteMessageID: nullableBridgeString(resolved.RemoteMessageID),
			TerminalError:   nullableBridgeString(resolved.TerminalError),
			UpdatedAt:       store.FormatTimestamp(resolved.UpdatedAt), DeliveryID: resolved.DeliveryID,
		}); err != nil {
			return fmt.Errorf("store: checkpoint bridge delivery %q: %w", resolved.DeliveryID, err)
		}
		return nil
	})
}

// ListBridgeDeliveries lists checkpoint rows inside one exact global or workspace scope.
func (g *BridgeRepo) ListBridgeDeliveries(
	ctx context.Context,
	query bridges.DeliveryLedgerQuery,
) (records []bridges.DeliveryLedgerRecord, err error) {
	if err := g.checkReady(ctx, "list bridge deliveries"); err != nil {
		return nil, err
	}
	normalized, err := query.Canonicalize()
	if err != nil {
		return nil, err
	}
	where, args := bridgeDeliveryQueryClauses(normalized)
	// dynamic-sql: optional scope/workspace/instance/state filters and caller limit change the statement shape.
	statement := `SELECT ` + bridgeDeliverySelectColumns + ` FROM bridge_deliveries`
	statement = store.AppendWhere(statement, where)
	statement += ` ORDER BY updated_at ASC, delivery_id ASC`
	statement, args = store.AppendLimit(statement, args, normalized.Limit)

	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge deliveries: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("store: close bridge delivery rows: %w", closeErr)
		}
	}()

	records = make([]bridges.DeliveryLedgerRecord, 0)
	for rows.Next() {
		record, scanErr := scanBridgeDelivery(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge delivery rows: %w", err)
	}
	return records, nil
}

func bridgeDeliveryQueryClauses(query bridges.DeliveryLedgerQuery) ([]string, []any) {
	return store.BuildClauses(
		store.StringClause("scope", string(query.Scope)),
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("bridge_instance_id", query.BridgeInstanceID),
		store.StringClause("state", string(query.State)),
	)
}

func getBridgeDelivery(
	ctx context.Context,
	queries *sqlcgen.Queries,
	deliveryID string,
) (bridges.DeliveryLedgerRecord, error) {
	row, err := queries.GetBridgeDelivery(ctx, deliveryID)
	if errors.Is(err, sql.ErrNoRows) {
		return bridges.DeliveryLedgerRecord{}, fmt.Errorf(
			"store: bridge delivery %q: %w",
			deliveryID,
			bridges.ErrDeliveryLedgerNotFound,
		)
	}
	if err != nil {
		return bridges.DeliveryLedgerRecord{}, fmt.Errorf("store: get bridge delivery %q: %w", deliveryID, err)
	}
	return bridgeDeliveryFromGenerated(row)
}

func resolveBridgeDeliveryCheckpoint(
	current bridges.DeliveryLedgerRecord,
	checkpoint bridges.DeliveryLedgerCheckpoint,
) (bridges.DeliveryLedgerRecord, error) {
	if checkpoint.LastSentSeq < current.LastSentSeq || checkpoint.LastAckedSeq < current.LastAckedSeq {
		return bridges.DeliveryLedgerRecord{}, deliveryCheckpointConflict(current.DeliveryID, "sequence regressed")
	}
	if current.State != bridges.DeliveryLedgerStateActive {
		return resolveTerminalBridgeDeliveryCheckpoint(current, checkpoint)
	}
	if checkpoint.LastSentSeq == current.LastSentSeq &&
		checkpoint.LastAckedSeq == current.LastAckedSeq &&
		checkpoint.RemoteMessageID != "" &&
		checkpoint.RemoteMessageID != current.RemoteMessageID {
		return bridges.DeliveryLedgerRecord{}, deliveryCheckpointConflict(
			current.DeliveryID,
			"remote message changed without sequence advance",
		)
	}

	resolved := current
	resolved.State = checkpoint.State
	resolved.LastSentSeq = checkpoint.LastSentSeq
	resolved.LastAckedSeq = checkpoint.LastAckedSeq
	if checkpoint.RemoteMessageID != "" {
		resolved.RemoteMessageID = checkpoint.RemoteMessageID
	}
	resolved.TerminalError = checkpoint.TerminalError
	resolved.UpdatedAt = maxBridgeDeliveryTime(current.UpdatedAt, checkpoint.UpdatedAt)
	return resolved, nil
}

func resolveTerminalBridgeDeliveryCheckpoint(
	current bridges.DeliveryLedgerRecord,
	checkpoint bridges.DeliveryLedgerCheckpoint,
) (bridges.DeliveryLedgerRecord, error) {
	remoteMessageID := checkpoint.RemoteMessageID
	if remoteMessageID == "" {
		remoteMessageID = current.RemoteMessageID
	}
	if checkpoint.State != current.State ||
		checkpoint.LastSentSeq != current.LastSentSeq ||
		checkpoint.LastAckedSeq != current.LastAckedSeq ||
		remoteMessageID != current.RemoteMessageID ||
		checkpoint.TerminalError != current.TerminalError {
		return bridges.DeliveryLedgerRecord{}, deliveryCheckpointConflict(
			current.DeliveryID,
			"terminal delivery is immutable",
		)
	}
	current.UpdatedAt = maxBridgeDeliveryTime(current.UpdatedAt, checkpoint.UpdatedAt)
	return current, nil
}

func maxBridgeDeliveryTime(current time.Time, candidate time.Time) time.Time {
	if candidate.After(current) {
		return candidate
	}
	return current
}

func deliveryCheckpointConflict(deliveryID string, reason string) error {
	return fmt.Errorf(
		"store: bridge delivery %q %s: %w",
		strings.TrimSpace(deliveryID),
		reason,
		bridges.ErrDeliveryLedgerConflict,
	)
}

func verifyBridgeDeliveryInstanceScope(
	ctx context.Context,
	queries *sqlcgen.Queries,
	bridgeInstanceID string,
	scope bridges.Scope,
	workspaceID string,
) error {
	row, err := queries.GetBridgeDeliveryInstanceScope(ctx, bridgeInstanceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf(
				"store: bridge delivery instance %q: %w",
				bridgeInstanceID,
				bridges.ErrBridgeInstanceNotFound,
			)
		}
		return fmt.Errorf("store: read bridge delivery instance %q: %w", bridgeInstanceID, err)
	}
	storedWorkspaceID := bridgeStringValue(row.WorkspaceID)
	if bridges.Scope(row.Scope).Normalize() != scope.Normalize() ||
		storedWorkspaceID != strings.TrimSpace(workspaceID) {
		return fmt.Errorf(
			"store: bridge delivery instance %q scope mismatch: %w",
			bridgeInstanceID,
			bridges.ErrDeliveryLedgerConflict,
		)
	}
	return nil
}

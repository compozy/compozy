package globaldb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type bridgeDeliveryStorageRow struct {
	deliveryID       string
	sessionID        string
	turnID           string
	routingKey       string
	bridgeInstanceID string
	scope            string
	workspaceID      sql.NullString
	state            string
	lastSentSeq      int64
	lastAckedSeq     int64
	remoteMessageID  sql.NullString
	terminalError    sql.NullString
	createdAt        string
	updatedAt        string
}

func scanBridgeDelivery(scanner rowScanner) (bridges.DeliveryLedgerRecord, error) {
	var row bridgeDeliveryStorageRow
	if err := scanner.Scan(
		&row.deliveryID,
		&row.sessionID,
		&row.turnID,
		&row.routingKey,
		&row.bridgeInstanceID,
		&row.scope,
		&row.workspaceID,
		&row.state,
		&row.lastSentSeq,
		&row.lastAckedSeq,
		&row.remoteMessageID,
		&row.terminalError,
		&row.createdAt,
		&row.updatedAt,
	); err != nil {
		return bridges.DeliveryLedgerRecord{}, err
	}
	return bridgeDeliveryFromStorageRow(row)
}

func bridgeDeliveryFromGenerated(row sqlcgen.BridgeDelivery) (bridges.DeliveryLedgerRecord, error) {
	return bridgeDeliveryFromStorageRow(bridgeDeliveryStorageRow{
		deliveryID: row.DeliveryID, sessionID: row.SessionID, turnID: row.TurnID,
		routingKey: row.RoutingKey, bridgeInstanceID: row.BridgeInstanceID,
		scope: row.Scope, workspaceID: row.WorkspaceID, state: row.State,
		lastSentSeq: row.LastSentSeq, lastAckedSeq: row.LastAckedSeq,
		remoteMessageID: row.RemoteMessageID, terminalError: row.TerminalError,
		createdAt: row.CreatedAt, updatedAt: row.UpdatedAt,
	})
}

func bridgeDeliveryFromStorageRow(row bridgeDeliveryStorageRow) (bridges.DeliveryLedgerRecord, error) {
	record := bridges.DeliveryLedgerRecord{
		DeliveryID: row.deliveryID, SessionID: row.sessionID, TurnID: row.turnID,
		BridgeInstanceID: row.bridgeInstanceID, Scope: bridges.Scope(row.scope),
		WorkspaceID: bridgeStringValue(row.workspaceID), State: bridges.DeliveryLedgerState(row.state),
		LastSentSeq: row.lastSentSeq, LastAckedSeq: row.lastAckedSeq,
		RemoteMessageID: bridgeStringValue(row.remoteMessageID),
		TerminalError:   bridgeStringValue(row.terminalError),
	}
	if err := json.Unmarshal([]byte(row.routingKey), &record.RoutingKey); err != nil {
		return bridges.DeliveryLedgerRecord{}, fmt.Errorf("store: decode bridge delivery routing key: %w", err)
	}
	canonicalRoutingKey, err := record.RoutingKey.Serialize()
	if err != nil {
		return bridges.DeliveryLedgerRecord{}, fmt.Errorf("store: validate bridge delivery routing key: %w", err)
	}
	if canonicalRoutingKey != row.routingKey {
		return bridges.DeliveryLedgerRecord{}, errors.New("store: bridge delivery routing key is not canonical")
	}
	record.CreatedAt, err = store.ParseTimestamp(row.createdAt)
	if err != nil {
		return bridges.DeliveryLedgerRecord{}, fmt.Errorf("store: parse bridge delivery created_at: %w", err)
	}
	record.UpdatedAt, err = store.ParseTimestamp(row.updatedAt)
	if err != nil {
		return bridges.DeliveryLedgerRecord{}, fmt.Errorf("store: parse bridge delivery updated_at: %w", err)
	}
	normalized, err := record.Canonicalize()
	if err != nil {
		return bridges.DeliveryLedgerRecord{}, fmt.Errorf("store: validate bridge delivery row: %w", err)
	}
	return normalized, nil
}

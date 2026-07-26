package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/bridges"
)

// ListBridgeCatalogRecords reads only metadata required to filter, count, and page the catalog.
func (g *BridgeRepo) ListBridgeCatalogRecords(
	ctx context.Context,
	query bridges.BridgeCatalogQuery,
) (records []bridges.BridgeCatalogRecord, err error) {
	if err := g.checkReady(ctx, "list bridge catalog records"); err != nil {
		return nil, err
	}
	normalized, err := bridges.NormalizeBridgeCatalogQuery(query)
	if err != nil {
		return nil, err
	}
	statement, args := bridgeCatalogRecordQuery(normalized)

	// dynamic-sql: optional scope, workspace, and platform filters change the statement shape.
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge catalog records: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close bridge catalog record rows: %w", closeErr))
		}
	}()

	records = make([]bridges.BridgeCatalogRecord, 0)
	for rows.Next() {
		record, scanErr := scanBridgeCatalogRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge catalog records: %w", err)
	}
	return records, nil
}

func bridgeCatalogRecordQuery(query bridges.BridgeCatalogQuery) (string, []any) {
	statement := `SELECT id, scope, workspace_id, platform, extension_name, display_name, enabled, status
		FROM bridge_instances`
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 2)
	switch query.Scope {
	case string(bridges.ScopeGlobal):
		conditions = append(conditions, "scope = ?")
		args = append(args, string(bridges.ScopeGlobal))
	case string(bridges.ScopeWorkspace):
		conditions = append(conditions, "scope = ? AND workspace_id = ?")
		args = append(args, string(bridges.ScopeWorkspace), query.WorkspaceID)
	default:
		if query.WorkspaceID != "" {
			conditions = append(conditions, "(scope = ? OR (scope = ? AND workspace_id = ?))")
			args = append(args, string(bridges.ScopeGlobal), string(bridges.ScopeWorkspace), query.WorkspaceID)
		}
	}
	if query.Platform != "" {
		conditions = append(conditions, "LOWER(TRIM(platform)) = ?")
		args = append(args, query.Platform)
	}
	if len(conditions) > 0 {
		statement += " WHERE " + strings.Join(conditions, " AND ")
	}
	return statement, args
}

func scanBridgeCatalogRecord(scanner rowScanner) (bridges.BridgeCatalogRecord, error) {
	var record bridges.BridgeCatalogRecord
	var scopeRaw string
	var workspaceID sql.NullString
	var statusRaw string
	if err := scanner.Scan(
		&record.ID,
		&scopeRaw,
		&workspaceID,
		&record.Platform,
		&record.ExtensionName,
		&record.DisplayName,
		&record.Enabled,
		&statusRaw,
	); err != nil {
		return bridges.BridgeCatalogRecord{}, fmt.Errorf("store: scan bridge catalog record: %w", err)
	}
	record.Scope = bridges.Scope(scopeRaw)
	if workspaceID.Valid {
		record.WorkspaceID = workspaceID.String
	}
	record.Status = bridges.BridgeStatus(statusRaw)
	if err := record.Validate(); err != nil {
		return bridges.BridgeCatalogRecord{}, fmt.Errorf("store: validate bridge catalog record: %w", err)
	}
	return record.Normalized(), nil
}

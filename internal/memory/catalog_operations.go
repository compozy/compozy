package memory

import (
	"context"

	"database/sql"

	"errors"
	"fmt"

	"strings"

	"time"

	memcontract "github.com/compozy/compozy/internal/memory/contract"
	storepkg "github.com/compozy/compozy/internal/store"
)

func (c *catalog) logEvent(ctx context.Context, profileID string, record memcontract.OperationRecord) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	db, err := c.ensureDB(ctx)
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	operation := record.Operation.Normalize()
	if strings.TrimSpace(string(operation)) == "" {
		return errors.New("memory: operation type is required")
	}
	scope := record.Scope.Normalize()
	switch scope {
	case "", memcontract.ScopeProfile, memcontract.ScopeWorkspace, memcontract.ScopeAgent:
	default:
		return fmt.Errorf("memory: unsupported operation scope %q", record.Scope)
	}
	timestamp := record.Timestamp
	if timestamp.IsZero() {
		timestamp = c.now().UTC()
	}
	agentName := strings.TrimSpace(record.AgentName)
	if agentName == "" {
		agentName = catalogEventAgentName
	}
	record.Operation = operation
	record.Scope = scope
	record.AgentName = agentName
	record.Timestamp = timestamp.UTC()
	return insertMemoryEventDB(ctx, db, profileID, record)
}

func insertMemoryEventDB(ctx context.Context, db *sql.DB, profileID string, record memcontract.OperationRecord) error {
	return storepkg.ExecuteWrite(ctx, db, func(ctx context.Context, tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO memory_events (
				op, profile_id, scope, agent_name, agent_tier, workspace_id, session_id, actor_kind,
				decision_id, target_id, metadata, ts_ms
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			canonicalEventOp(record),
			strings.TrimSpace(profileID),
			nullStringForEmpty(record.Scope.Normalize()),
			nullStringForEmpty(record.AgentName),
			nil,
			nullStringForEmpty(record.Workspace),
			nil,
			"system",
			nil,
			nullStringForEmpty(record.Filename),
			mustEventMetadata(record),
			timeToUnixMillis(record.Timestamp),
		); err != nil {
			return fmt.Errorf("memory: write memory event: %w", err)
		}
		return nil
	})
}

func (c *catalog) listOperations(
	ctx context.Context,
	profileID string,
	query memcontract.OperationHistoryQuery,
) (records []memcontract.OperationRecord, err error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}

	operation := canonicalEventOp(memcontract.OperationRecord{Operation: query.Operation.Normalize()})
	workspace := strings.TrimSpace(query.Workspace)
	switch scope := query.Scope.Normalize(); scope {
	case "", memcontract.ScopeProfile, memcontract.ScopeWorkspace:
	default:
		return nil, fmt.Errorf("memory: unsupported history scope %q", query.Scope)
	}
	scope := string(query.Scope.Normalize())
	var since int64
	if !query.Since.IsZero() {
		since = timeToUnixMillis(query.Since.UTC())
	}
	limit := clampHistoryLimit(query.Limit)

	rows, err := db.QueryContext(
		ctx,
		`SELECT id, op, scope, workspace_id, agent_name, target_id, metadata, ts_ms
		 FROM memory_events
		 WHERE (profile_id = ? OR (profile_id = '' AND COALESCE(scope, '') <> 'profile'))
		 AND (? = '' OR op = ?)
		 AND (
			(? = '' AND (? = '' OR scope IS NULL OR scope = 'profile' OR (scope = 'workspace' AND workspace_id = ?)))
			OR (? = 'profile' AND scope = 'profile')
			OR (? = 'workspace' AND scope = 'workspace' AND (? = '' OR workspace_id = ?))
		 )
		 AND (? = 0 OR ts_ms >= ?)
		 ORDER BY ts_ms DESC, id DESC
		 LIMIT ?`,
		strings.TrimSpace(profileID),
		operation,
		operation,
		scope,
		workspace,
		workspace,
		scope,
		scope,
		workspace,
		workspace,
		since,
		since,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: list operation history: %w", err)
	}
	defer closeCatalogRows(rows, "operation history", &err)

	records = make([]memcontract.OperationRecord, 0, limit)
	for rows.Next() {
		record, scanErr := scanOperationRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate operation history: %w", err)
	}
	return records, nil
}

func (c *catalog) operationStats(
	ctx context.Context,
	filters []catalogFilter,
) (count int, lastTime *time.Time, err error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return 0, nil, err
	}
	if db == nil {
		return 0, nil, nil
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT profile_id, scope, workspace_id, ts_ms FROM memory_events`,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("memory: read operation stats: %w", err)
	}
	defer closeCatalogRows(rows, "operation stats", &err)

	for rows.Next() {
		var (
			profileID   string
			scope       sql.NullString
			workspaceID sql.NullString
			tsMillis    int64
		)
		if err := rows.Scan(&profileID, &scope, &workspaceID, &tsMillis); err != nil {
			return 0, nil, fmt.Errorf("memory: scan operation stats: %w", err)
		}
		if !catalogFiltersAllow(filters, profileID, memcontract.Scope(scope.String), workspaceID.String) {
			continue
		}
		count++
		parsed := timeFromUnixMillis(tsMillis)
		if lastTime == nil || parsed.After(*lastTime) {
			lastTime = &parsed
		}
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("memory: iterate operation stats: %w", err)
	}
	return count, lastTime, nil
}

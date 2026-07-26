package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	"github.com/compozy/agh/internal/toolruntime"
)

var _ toolruntime.Store = (*ToolRuntimeRepo)(nil)

// UpsertProcessRecord writes one durable process checkpoint.
func (g *ToolRuntimeRepo) UpsertProcessRecord(ctx context.Context, record toolruntime.ProcessRecord) error {
	if err := g.checkReady(ctx, "upsert tool process record"); err != nil {
		return err
	}
	argsJSON, err := json.Marshal(record.Args)
	if err != nil {
		return fmt.Errorf("store: encode tool process args for %q: %w", record.ID, err)
	}
	err = g.queries.UpsertToolProcessRecord(ctx, sqlcgen.UpsertToolProcessRecordParams{
		ID:             record.ID,
		Source:         string(record.Source),
		SessionID:      record.Owner.SessionID,
		TurnID:         record.Owner.TurnID,
		ToolCallID:     record.Owner.ToolCallID,
		TerminalID:     record.Owner.TerminalID,
		ExtensionName:  record.Owner.ExtensionName,
		HookName:       record.Owner.HookName,
		SandboxID:      record.Owner.SandboxID,
		Pid:            int64(record.PID),
		ProcessGroupID: int64(record.ProcessGroupID),
		Command:        record.Command,
		ArgsJson:       string(argsJSON),
		Cwd:            record.Cwd,
		StartedAt:      nullableProcessTimestamp(record.StartedAt),
		StartedByPid: int64(
			record.StartedByPID,
		),
		State:     string(record.State),
		ExitCode:  nullableProcessInt(record.ExitCode),
		Error:     record.Error,
		CreatedAt: store.FormatTimestamp(record.CreatedAt),
		UpdatedAt: store.FormatTimestamp(
			record.UpdatedAt,
		),
		CompletedAt: nullableProcessTimestampPtr(record.CompletedAt),
	})
	if err != nil {
		return fmt.Errorf("store: upsert tool process record %q: %w", record.ID, err)
	}
	return nil
}

// UpdateProcessRecordState mutates lifecycle fields for one process record.
func (g *ToolRuntimeRepo) UpdateProcessRecordState(ctx context.Context, update toolruntime.ProcessStateUpdate) error {
	if err := g.checkReady(ctx, "update tool process record state"); err != nil {
		return err
	}
	if strings.TrimSpace(update.ID) == "" {
		return errors.New("store: tool process id is required")
	}
	rowsAffected, err := g.queries.UpdateToolProcessRecordState(ctx, sqlcgen.UpdateToolProcessRecordStateParams{
		State:    string(update.State),
		ExitCode: nullableProcessInt(update.ExitCode),
		Error:    update.Error,
		UpdatedAt: store.FormatTimestamp(
			update.UpdatedAt,
		),
		CompletedAt: nullableProcessTimestampPtr(update.CompletedAt),
		ID:          update.ID,
	})
	if err != nil {
		return fmt.Errorf("store: update tool process record %q state: %w", update.ID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: tool process record %q", toolruntime.ErrProcessNotFound, update.ID)
	}
	return nil
}

// ListProcessRecords returns process records matching the query.
func (g *ToolRuntimeRepo) ListProcessRecords(
	ctx context.Context,
	query toolruntime.ProcessQuery,
) (records []toolruntime.ProcessRecord, err error) {
	if err := g.checkReady(ctx, "list tool process records"); err != nil {
		return nil, err
	}

	// dynamic-sql: optional ID/state sets, owner scope fields, source, and limit change the statement shape.
	sqlQuery, args := toolProcessListQuery(query)
	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list tool process records: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close tool process rows: %w", closeErr))
		}
	}()

	records = make([]toolruntime.ProcessRecord, 0)
	for rows.Next() {
		record, scanErr := scanToolProcessRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate tool process records: %w", err)
	}
	return records, nil
}

func toolProcessListQuery(query toolruntime.ProcessQuery) (string, []any) {
	var where []string
	var args []any
	if len(query.IDs) > 0 {
		where = append(where, "id IN ("+placeholders(len(query.IDs))+")")
		for _, id := range query.IDs {
			args = append(args, strings.TrimSpace(id))
		}
	}
	if len(query.States) > 0 {
		where = append(where, "state IN ("+placeholders(len(query.States))+")")
		for _, state := range query.States {
			args = append(args, string(state))
		}
	}
	scope := query.Scope.Normalize()
	addScopeCondition := func(column string, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		where = append(where, column+" = ?")
		args = append(args, strings.TrimSpace(value))
	}
	addScopeCondition("id", scope.ProcessID)
	addScopeCondition("session_id", scope.SessionID)
	addScopeCondition("turn_id", scope.TurnID)
	addScopeCondition("tool_call_id", scope.ToolCallID)
	addScopeCondition("terminal_id", scope.TerminalID)
	addScopeCondition("extension_name", scope.ExtensionName)
	addScopeCondition("hook_name", scope.HookName)
	if scope.Source != "" {
		where = append(where, "source = ?")
		args = append(args, string(scope.Source))
	}

	var builder strings.Builder
	builder.WriteString(`SELECT
		id, source, session_id, turn_id, tool_call_id, terminal_id, extension_name,
		hook_name, sandbox_id, pid, process_group_id, command, args_json, cwd,
		started_at, started_by_pid, state, exit_code, error, created_at, updated_at, completed_at
		FROM tool_processes`)
	if len(where) > 0 {
		builder.WriteString(" WHERE ")
		builder.WriteString(strings.Join(where, " AND "))
	}
	builder.WriteString(" ORDER BY updated_at ASC, id ASC")
	if query.Limit > 0 {
		builder.WriteString(" LIMIT ?")
		args = append(args, query.Limit)
	}
	return builder.String(), args
}

func scanToolProcessRecord(rows *sql.Rows) (toolruntime.ProcessRecord, error) {
	var record toolruntime.ProcessRecord
	var source string
	var state string
	var argsJSON string
	var startedAt sql.NullString
	var exitCode sql.NullInt64
	var createdAt string
	var updatedAt string
	var completedAt sql.NullString
	err := rows.Scan(
		&record.ID,
		&source,
		&record.Owner.SessionID,
		&record.Owner.TurnID,
		&record.Owner.ToolCallID,
		&record.Owner.TerminalID,
		&record.Owner.ExtensionName,
		&record.Owner.HookName,
		&record.Owner.SandboxID,
		&record.PID,
		&record.ProcessGroupID,
		&record.Command,
		&argsJSON,
		&record.Cwd,
		&startedAt,
		&record.StartedByPID,
		&state,
		&exitCode,
		&record.Error,
		&createdAt,
		&updatedAt,
		&completedAt,
	)
	if err != nil {
		return toolruntime.ProcessRecord{}, fmt.Errorf("store: scan tool process record: %w", err)
	}
	record.Source = toolruntime.ProcessSource(source)
	record.State = toolruntime.ProcessState(state)
	if err := json.Unmarshal([]byte(argsJSON), &record.Args); err != nil {
		return toolruntime.ProcessRecord{}, fmt.Errorf("store: decode tool process args for %q: %w", record.ID, err)
	}
	var parseErr error
	if startedAt.Valid {
		record.StartedAt, parseErr = store.ParseTimestamp(startedAt.String)
		if parseErr != nil {
			return toolruntime.ProcessRecord{}, parseErr
		}
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		record.ExitCode = &value
	}
	record.CreatedAt, parseErr = store.ParseTimestamp(createdAt)
	if parseErr != nil {
		return toolruntime.ProcessRecord{}, parseErr
	}
	record.UpdatedAt, parseErr = store.ParseTimestamp(updatedAt)
	if parseErr != nil {
		return toolruntime.ProcessRecord{}, parseErr
	}
	if completedAt.Valid {
		parsed, err := store.ParseTimestamp(completedAt.String)
		if err != nil {
			return toolruntime.ProcessRecord{}, err
		}
		record.CompletedAt = &parsed
	}
	return record, nil
}

func placeholders(count int) string {
	values := make([]string, count)
	for idx := range values {
		values[idx] = "?"
	}
	return strings.Join(values, ", ")
}

func nullableProcessTimestampPtr(value *time.Time) sql.NullString {
	if value == nil || value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(*value), Valid: true}
}

func nullableProcessTimestamp(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func nullableProcessInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
)

// DerivedResetResult reports derived-catalog reset work.
type DerivedResetResult struct {
	DeletedRows int
	IndexedRows int
	ResetAt     time.Time
}

// DreamRunRecord is a redaction-safe memory_consolidations query row.
type DreamRunRecord struct {
	ID            string
	WorkspaceID   string
	Scope         memcontract.Scope
	AgentName     string
	AgentTier     memcontract.AgentTier
	Status        string
	InputCount    int
	PromotedCount int
	Error         string
	Metadata      map[string]string
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// DreamRunListQuery filters dreaming run records.
type DreamRunListQuery struct {
	Scope       memcontract.Scope
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	Limit       int
}

// DailyLogRecord summarizes memory event activity for one date and selector.
type DailyLogRecord struct {
	Date           string
	Scope          memcontract.Scope
	WorkspaceID    string
	AgentName      string
	AgentTier      memcontract.AgentTier
	OperationCount int
}

// DailyLogListQuery filters daily memory operation summaries.
type DailyLogListQuery struct {
	Date        string
	Scope       memcontract.Scope
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	Limit       int
}

// ResetDerived clears derived catalog rows and rebuilds them from Markdown sources.
func (s *Store) ResetDerived(ctx context.Context, opts memcontract.ReindexOptions) (DerivedResetResult, error) {
	if ctx == nil {
		return DerivedResetResult{}, errors.New("memory: reset derived context is required")
	}
	if s == nil {
		return DerivedResetResult{}, errors.New("memory: store is required")
	}
	scope, workspaceRoot, workspaceID, err := s.normalizeScopeAndWorkspace(ctx, opts.Scope, opts.Workspace)
	if err != nil {
		return DerivedResetResult{}, err
	}
	if s.catalog == nil {
		return DerivedResetResult{ResetAt: time.Now().UTC()}, nil
	}
	unlock := s.lockMutations()
	defer unlock()

	deletedRows, err := s.catalog.clearDerivedScope(
		ctx,
		scope,
		workspaceID,
		s.catalogAgentName(scope),
		s.catalogAgentTier(scope),
	)
	if err != nil {
		return DerivedResetResult{}, err
	}
	indexedRows, err := s.reindexScopesLocked(ctx, scope, workspaceRoot, workspaceID)
	if err != nil {
		return DerivedResetResult{}, err
	}
	return DerivedResetResult{
		DeletedRows: deletedRows,
		IndexedRows: indexedRows,
		ResetAt:     time.Now().UTC(),
	}, nil
}

// ListDreamRunRecords returns persisted dreaming runs ordered newest first.
func (s *Store) ListDreamRunRecords(ctx context.Context, query DreamRunListQuery) ([]DreamRunRecord, error) {
	if ctx == nil {
		return nil, errors.New("memory: list dream runs context is required")
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return nil, err
	}
	return s.catalog.listDreamRuns(ctx, query)
}

// LoadDreamRunRecord returns one persisted dreaming run.
func (s *Store) LoadDreamRunRecord(ctx context.Context, id string) (DreamRunRecord, error) {
	if ctx == nil {
		return DreamRunRecord{}, errors.New("memory: load dream run context is required")
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return DreamRunRecord{}, err
	}
	return s.catalog.loadDreamRun(ctx, id)
}

// ListDailyLogRecords returns memory-event daily summaries ordered newest first.
func (s *Store) ListDailyLogRecords(ctx context.Context, query DailyLogListQuery) ([]DailyLogRecord, error) {
	if ctx == nil {
		return nil, errors.New("memory: list daily logs context is required")
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return nil, err
	}
	return s.catalog.listDailyLogs(ctx, query)
}

func (c *catalog) clearDerivedScope(
	ctx context.Context,
	scope memcontract.Scope,
	workspaceID string,
	agentName string,
	agentTier memcontract.AgentTier,
) (int, error) {
	scope = scope.Normalize()
	if err := scope.Validate(); err != nil {
		return 0, wrapValidationError("reset derived scope", string(scope), err)
	}
	returnCount := 0
	err := c.withCatalogWriteTx(ctx, "catalog derived reset", func(tx *storepkg.WriteTx) error {
		result, err := tx.ExecContext(
			ctx,
			`DELETE FROM memory_catalog_entries
			 WHERE scope = ? AND workspace_id = ? AND agent_name = ? AND agent_tier = ?`,
			string(scope),
			strings.TrimSpace(workspaceID),
			strings.TrimSpace(agentName),
			string(agentTier.Normalize()),
		)
		if err != nil {
			return fmt.Errorf("memory: reset derived catalog %s/%s: %w", scope, workspaceID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("memory: inspect derived reset rows: %w", err)
		}
		returnCount = int(affected)
		return invalidateCatalogIdentityTx(
			ctx,
			tx,
			newCatalogIdentity(scope, workspaceID, agentName, agentTier),
		)
	})
	if err != nil {
		return 0, err
	}
	return returnCount, nil
}

func (c *catalog) listDreamRuns(ctx context.Context, query DreamRunListQuery) ([]DreamRunRecord, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("memory: decision catalog is disabled")
	}
	sqlText := strings.Join([]string{
		`SELECT id, workspace_id, scope, agent_name, agent_tier, started_at, finished_at,`,
		`status, input_count, promoted_count, error, metadata`,
		`FROM memory_consolidations`,
	}, "\n")
	clauses, args, err := dreamRunWhere(query)
	if err != nil {
		return nil, err
	}
	if len(clauses) > 0 {
		sqlText += "\nWHERE " + strings.Join(clauses, " AND ")
	}
	limit := clampMemoryQueryLimit(query.Limit)
	sqlText += "\nORDER BY started_at DESC, id DESC\nLIMIT ?"
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: list dream runs: %w", err)
	}
	defer closeRows(rows, "memory: close dream run rows failed")
	records := make([]DreamRunRecord, 0)
	for rows.Next() {
		record, scanErr := scanDreamRunRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate dream runs: %w", err)
	}
	return records, nil
}

func (c *catalog) loadDreamRun(ctx context.Context, id string) (DreamRunRecord, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return DreamRunRecord{}, err
	}
	if db == nil {
		return DreamRunRecord{}, errors.New("memory: decision catalog is disabled")
	}
	row := db.QueryRowContext(
		ctx,
		`SELECT id, workspace_id, scope, agent_name, agent_tier, started_at, finished_at,
			status, input_count, promoted_count, error, metadata
		 FROM memory_consolidations
		 WHERE id = ?`,
		strings.TrimSpace(id),
	)
	record, err := scanDreamRunRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DreamRunRecord{}, fmt.Errorf("memory: dream run %q: %w", strings.TrimSpace(id), os.ErrNotExist)
		}
		return DreamRunRecord{}, err
	}
	return record, nil
}

func dreamRunWhere(query DreamRunListQuery) ([]string, []any, error) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if scope := query.Scope.Normalize(); scope != "" {
		if err := scope.Validate(); err != nil {
			return nil, nil, wrapValidationError("list dream runs scope", string(query.Scope), err)
		}
		clauses = append(clauses, "scope = ?")
		args = append(args, string(scope))
	}
	if workspaceID := strings.TrimSpace(query.WorkspaceID); workspaceID != "" {
		clauses = append(clauses, "workspace_id = ?")
		args = append(args, workspaceID)
	}
	if agentName := strings.TrimSpace(query.AgentName); agentName != "" {
		clauses = append(clauses, "agent_name = ?")
		args = append(args, agentName)
	}
	if agentTier := query.AgentTier.Normalize(); agentTier != "" {
		if err := agentTier.Validate(); err != nil {
			return nil, nil, wrapValidationError("list dream runs agent tier", string(query.AgentTier), err)
		}
		clauses = append(clauses, "agent_tier = ?")
		args = append(args, string(agentTier))
	}
	return clauses, args, nil
}

func scanDreamRunRecord(scanner interface{ Scan(dest ...any) error }) (DreamRunRecord, error) {
	var (
		record       DreamRunRecord
		workspaceID  sql.NullString
		scopeRaw     string
		agentName    sql.NullString
		agentTierRaw sql.NullString
		startedAt    int64
		finishedAt   sql.NullInt64
		metadataRaw  string
	)
	if err := scanner.Scan(
		&record.ID,
		&workspaceID,
		&scopeRaw,
		&agentName,
		&agentTierRaw,
		&startedAt,
		&finishedAt,
		&record.Status,
		&record.InputCount,
		&record.PromotedCount,
		&record.Error,
		&metadataRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DreamRunRecord{}, err
		}
		return DreamRunRecord{}, fmt.Errorf("memory: scan dream run: %w", err)
	}
	record.WorkspaceID = nullableSQLString(workspaceID)
	record.Scope = memcontract.Scope(scopeRaw).Normalize()
	record.AgentName = nullableSQLString(agentName)
	record.AgentTier = memcontract.AgentTier(nullableSQLString(agentTierRaw)).Normalize()
	record.StartedAt = timeFromUnixMillis(startedAt)
	if finishedAt.Valid {
		parsed := timeFromUnixMillis(finishedAt.Int64)
		record.FinishedAt = &parsed
	}
	record.Metadata = map[string]string{}
	if strings.TrimSpace(metadataRaw) != "" {
		if err := json.Unmarshal([]byte(metadataRaw), &record.Metadata); err != nil {
			return DreamRunRecord{}, fmt.Errorf("memory: decode dream run metadata: %w", err)
		}
	}
	return record, nil
}

func (c *catalog) listDailyLogs(ctx context.Context, query DailyLogListQuery) ([]DailyLogRecord, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("memory: decision catalog is disabled")
	}
	sqlText := strings.Join([]string{
		`SELECT strftime('%Y-%m-%d', ts_ms / 1000, 'unixepoch') AS day,`,
		`COALESCE(scope, ''), COALESCE(workspace_id, ''), COALESCE(agent_name, ''),`,
		`COALESCE(agent_tier, ''), COUNT(*)`,
		`FROM memory_events`,
	}, "\n")
	clauses, args, err := dailyLogWhere(query)
	if err != nil {
		return nil, err
	}
	if len(clauses) > 0 {
		sqlText += "\nWHERE " + strings.Join(clauses, " AND ")
	}
	sqlText += "\nGROUP BY day, scope, workspace_id, agent_name, agent_tier"
	sqlText += "\nORDER BY day DESC, scope ASC, workspace_id ASC, agent_name ASC, agent_tier ASC\nLIMIT ?"
	args = append(args, clampMemoryQueryLimit(query.Limit))
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: list daily logs: %w", err)
	}
	defer closeRows(rows, "memory: close daily log rows failed")
	records := make([]DailyLogRecord, 0)
	for rows.Next() {
		var record DailyLogRecord
		var scopeRaw string
		var agentTierRaw string
		if err := rows.Scan(
			&record.Date,
			&scopeRaw,
			&record.WorkspaceID,
			&record.AgentName,
			&agentTierRaw,
			&record.OperationCount,
		); err != nil {
			return nil, fmt.Errorf("memory: scan daily log: %w", err)
		}
		record.Scope = memcontract.Scope(scopeRaw).Normalize()
		record.AgentTier = memcontract.AgentTier(agentTierRaw).Normalize()
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate daily logs: %w", err)
	}
	return records, nil
}

func dailyLogWhere(query DailyLogListQuery) ([]string, []any, error) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if date := strings.TrimSpace(query.Date); date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, nil, wrapValidationError("list daily logs date", date, err)
		}
		clauses = append(clauses, "strftime('%Y-%m-%d', ts_ms / 1000, 'unixepoch') = ?")
		args = append(args, date)
	}
	if scope := query.Scope.Normalize(); scope != "" {
		if err := scope.Validate(); err != nil {
			return nil, nil, wrapValidationError("list daily logs scope", string(query.Scope), err)
		}
		clauses = append(clauses, "scope = ?")
		args = append(args, string(scope))
	}
	if workspaceID := strings.TrimSpace(query.WorkspaceID); workspaceID != "" {
		clauses = append(clauses, "workspace_id = ?")
		args = append(args, workspaceID)
	}
	if agentName := strings.TrimSpace(query.AgentName); agentName != "" {
		clauses = append(clauses, "agent_name = ?")
		args = append(args, agentName)
	}
	if agentTier := query.AgentTier.Normalize(); agentTier != "" {
		if err := agentTier.Validate(); err != nil {
			return nil, nil, wrapValidationError("list daily logs agent tier", string(query.AgentTier), err)
		}
		clauses = append(clauses, "agent_tier = ?")
		args = append(args, string(agentTier))
	}
	return clauses, args, nil
}

func clampMemoryQueryLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 200
	}
	return limit
}

func closeRows(rows *sql.Rows, message string) {
	if rows == nil {
		return
	}
	if err := rows.Close(); err != nil {
		slog.Default().Warn(message, "error", err)
	}
}

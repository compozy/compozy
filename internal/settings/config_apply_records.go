package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/store"
)

const (
	defaultApplyRecordLimit = 50
	maxApplyRecordLimit     = 500
	applyRecordActorRuntime = "runtime"
)

// ApplyRecordDBSource exposes the SQL connection needed by the apply-record repository.
type ApplyRecordDBSource interface {
	DB() *sql.DB
}

// NewConfigApplyRecordRepository creates the SQLite-backed config_apply_records repository.
func NewConfigApplyRecordRepository(db *sql.DB, now func() time.Time) ApplyRecordStore {
	if now == nil {
		now = time.Now
	}
	return &configApplyRecordRepository{db: db, now: now}
}

type configApplyRecordRepository struct {
	db  *sql.DB
	now func() time.Time
}

func (r *configApplyRecordRepository) CreateApplyRecord(
	ctx context.Context,
	record ApplyRecord,
) (ApplyRecord, error) {
	if r == nil || r.db == nil {
		return ApplyRecord{}, errors.New("settings: config apply record database is required")
	}
	normalized := r.normalizeForCreate(record)
	diagnosticsJSON, err := marshalApplyDiagnostics(normalized.Diagnostics)
	if err != nil {
		return ApplyRecord{}, err
	}
	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO config_apply_records (
			id,
			desired_config_hash,
			active_config_hash,
			generation,
			actor,
			diff_class,
			status,
			diagnostic_json,
			created_at,
			applied_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		normalized.ID,
		normalized.DesiredHash,
		normalized.ActiveHash,
		normalized.Generation,
		normalized.Actor,
		string(normalized.DiffClass),
		string(normalized.Status),
		diagnosticsJSON,
		store.FormatTimestamp(normalized.CreatedAt),
		nullableApplyTimestamp(normalized.AppliedAt),
		store.FormatTimestamp(normalized.UpdatedAt),
	)
	if err != nil {
		return ApplyRecord{}, fmt.Errorf("settings: create config apply record: %w", err)
	}
	return normalized, nil
}

func (r *configApplyRecordRepository) UpdateApplyRecord(
	ctx context.Context,
	record ApplyRecord,
) (ApplyRecord, error) {
	if r == nil || r.db == nil {
		return ApplyRecord{}, errors.New("settings: config apply record database is required")
	}
	normalized := r.normalizeForUpdate(record)
	diagnosticsJSON, err := marshalApplyDiagnostics(normalized.Diagnostics)
	if err != nil {
		return ApplyRecord{}, err
	}
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE config_apply_records
		 SET desired_config_hash = ?,
		     active_config_hash = ?,
		     generation = ?,
		     actor = ?,
		     diff_class = ?,
		     status = ?,
		     diagnostic_json = ?,
		     applied_at = ?,
		     updated_at = ?
		 WHERE id = ?`,
		normalized.DesiredHash,
		normalized.ActiveHash,
		normalized.Generation,
		normalized.Actor,
		string(normalized.DiffClass),
		string(normalized.Status),
		diagnosticsJSON,
		nullableApplyTimestamp(normalized.AppliedAt),
		store.FormatTimestamp(normalized.UpdatedAt),
		normalized.ID,
	)
	if err != nil {
		return ApplyRecord{}, fmt.Errorf("settings: update config apply record %q: %w", normalized.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ApplyRecord{}, fmt.Errorf("settings: inspect config apply record update %q: %w", normalized.ID, err)
	}
	if affected == 0 {
		return ApplyRecord{}, notFoundError(fmt.Errorf("settings: config apply record %q not found", normalized.ID))
	}
	return normalized, nil
}

func (r *configApplyRecordRepository) ListApplyRecords(
	ctx context.Context,
	filter ApplyRecordFilter,
) ([]ApplyRecord, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("settings: config apply record database is required")
	}
	query := `SELECT id,
		desired_config_hash,
		active_config_hash,
		generation,
		actor,
		diff_class,
		status,
		diagnostic_json,
		created_at,
		applied_at,
		updated_at
		FROM config_apply_records`
	var (
		clauses []string
		args    []any
	)
	if filter.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, string(filter.Status))
	}
	if strings.TrimSpace(filter.Actor) != "" {
		clauses = append(clauses, "actor = ?")
		args = append(args, strings.TrimSpace(filter.Actor))
	}
	if len(clauses) > 0 {
		// #nosec G202 -- clauses are static predicates selected from typed filter fields.
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC, created_at DESC"
	query, args = store.AppendLimit(query, args, normalizeApplyRecordLimit(filter.Limit))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("settings: list config apply records: %w", err)
	}
	defer rows.Close()

	records := []ApplyRecord{}
	for rows.Next() {
		record, scanErr := scanApplyRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settings: iterate config apply records: %w", err)
	}
	return records, nil
}

func (r *configApplyRecordRepository) LatestAppliedRecord(
	ctx context.Context,
) (*ApplyRecord, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("settings: config apply record database is required")
	}
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id,
			desired_config_hash,
			active_config_hash,
			generation,
			actor,
			diff_class,
			status,
			diagnostic_json,
			created_at,
			applied_at,
			updated_at
		 FROM config_apply_records
		 WHERE status = ?
		 ORDER BY generation DESC, applied_at DESC, updated_at DESC
		 LIMIT 1`,
		string(lifecycle.StatusApplied),
	)
	record, err := scanApplyRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(record.ID) == "" {
		return nil, nil
	}
	return &record, nil
}

func (r *configApplyRecordRepository) normalizeForCreate(record ApplyRecord) ApplyRecord {
	now := r.now().UTC()
	normalized := record
	if strings.TrimSpace(normalized.ID) == "" {
		normalized.ID = store.NewID("cfgapp")
	}
	if strings.TrimSpace(normalized.Actor) == "" {
		normalized.Actor = applyRecordActorRuntime
	}
	if normalized.DiffClass == "" {
		normalized.DiffClass = lifecycle.DiffClassRestartRequired
	}
	if normalized.Status == "" {
		normalized.Status = lifecycle.StatusPendingApply
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = now
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	return normalized
}

func (r *configApplyRecordRepository) normalizeForUpdate(record ApplyRecord) ApplyRecord {
	normalized := record
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = r.now().UTC()
	}
	if strings.TrimSpace(normalized.Actor) == "" {
		normalized.Actor = applyRecordActorRuntime
	}
	if normalized.DiffClass == "" {
		normalized.DiffClass = lifecycle.DiffClassRestartRequired
	}
	return normalized
}

func normalizeApplyRecordLimit(limit int) int {
	if limit <= 0 {
		return defaultApplyRecordLimit
	}
	if limit > maxApplyRecordLimit {
		return maxApplyRecordLimit
	}
	return limit
}

func marshalApplyDiagnostics(items []diagnosticcontract.DiagnosticItem) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("settings: marshal config apply diagnostics: %w", err)
	}
	return string(payload), nil
}

func unmarshalApplyDiagnostics(raw sql.NullString) ([]diagnosticcontract.DiagnosticItem, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var items []diagnosticcontract.DiagnosticItem
	if err := json.Unmarshal([]byte(raw.String), &items); err != nil {
		return nil, fmt.Errorf("settings: unmarshal config apply diagnostics: %w", err)
	}
	return items, nil
}

func nullableApplyTimestamp(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return store.FormatTimestamp(*value)
}

type applyRecordScanner interface {
	Scan(dest ...any) error
}

func scanApplyRecord(row applyRecordScanner) (ApplyRecord, error) {
	var (
		record         ApplyRecord
		diffClass      string
		status         string
		diagnosticsRaw sql.NullString
		createdRaw     string
		appliedRaw     sql.NullString
		updatedRaw     string
	)
	if err := row.Scan(
		&record.ID,
		&record.DesiredHash,
		&record.ActiveHash,
		&record.Generation,
		&record.Actor,
		&diffClass,
		&status,
		&diagnosticsRaw,
		&createdRaw,
		&appliedRaw,
		&updatedRaw,
	); err != nil {
		return ApplyRecord{}, fmt.Errorf("settings: scan config apply record: %w", err)
	}
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return ApplyRecord{}, err
	}
	appliedAt, err := store.ParseNullableTimestamp(appliedRaw.String)
	if err != nil {
		return ApplyRecord{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedRaw)
	if err != nil {
		return ApplyRecord{}, err
	}
	diagnostics, err := unmarshalApplyDiagnostics(diagnosticsRaw)
	if err != nil {
		return ApplyRecord{}, err
	}
	record.DiffClass = lifecycle.DiffClass(diffClass)
	record.Status = lifecycle.Status(status)
	record.Lifecycle = lifecycle.Lifecycle(diffClass)
	record.NextAction = lifecycle.NextActionForLifecycle(record.Lifecycle, record.Status)
	record.Diagnostics = diagnostics
	record.CreatedAt = createdAt
	record.AppliedAt = appliedAt
	record.UpdatedAt = updatedAt
	return record, nil
}

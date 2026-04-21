package rundb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var closeRunSQLiteDatabase = store.CloseSQLiteDatabase

type openOptions struct {
	now func() time.Time
}

// RunDB owns one per-run SQLite store.
type RunDB struct {
	db          *sql.DB
	path        string
	runID       string
	now         func() time.Time
	closeMu     sync.Mutex
	integrityMu sync.Mutex
}

// HookRunRecord captures one hook audit row persisted independently of the canonical event stream.
type HookRunRecord struct {
	ID          string
	HookName    string
	Source      string
	Outcome     string
	DurationNS  int64
	PayloadJSON string
	RecordedAt  time.Time
}

// JobStateRow is the latest projected state for one job.
type JobStateRow struct {
	JobID       string
	TaskID      string
	Status      string
	AgentName   string
	SummaryJSON string
	UpdatedAt   time.Time
}

// TranscriptMessageRow is the projected transcript row for one event sequence.
type TranscriptMessageRow struct {
	Sequence     uint64
	Stream       string
	Role         string
	Content      string
	MetadataJSON string
	Timestamp    time.Time
}

// TokenUsageRow is the persisted token usage projection for one turn or aggregate record.
type TokenUsageRow struct {
	TurnID       string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostAmount   *float64
	Timestamp    time.Time
}

// ArtifactSyncRow is the persisted artifact sync history row.
type ArtifactSyncRow struct {
	Sequence     uint64
	RelativePath string
	ChangeKind   string
	Checksum     string
	SyncedAt     time.Time
}

// RunIntegrityState is the durable sticky integrity state for one run.
type RunIntegrityState struct {
	Incomplete              bool
	Reasons                 []string
	JournalTerminalDrops    uint64
	JournalNonTerminalDrops uint64
	FirstDetectedAt         time.Time
	UpdatedAt               time.Time
}

// RunIntegrityUpdate captures one integrity-state update that should merge with
// any existing sticky run-integrity row.
type RunIntegrityUpdate struct {
	Incomplete              bool
	Reasons                 []string
	JournalTerminalDrops    uint64
	JournalNonTerminalDrops uint64
}

// EventListResult captures one ordered event window from the canonical log.
type EventListResult struct {
	Events  []events.Event
	HasMore bool
}

// Open opens or creates one per-run operational store and applies migrations.
func Open(ctx context.Context, path string) (*RunDB, error) {
	return openWithOptions(ctx, path, openOptions{})
}

func openWithOptions(ctx context.Context, path string, opts openOptions) (*RunDB, error) {
	if ctx == nil {
		return nil, errors.New("rundb: open context is required")
	}

	runDB := &RunDB{
		path:  strings.TrimSpace(path),
		runID: filepath.Base(filepath.Dir(strings.TrimSpace(path))),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	if opts.now != nil {
		runDB.now = opts.now
	}

	db, err := store.OpenSQLiteDatabase(ctx, runDB.path, func(ctx context.Context, db *sql.DB) error {
		return applyMigrations(ctx, db, runDB.now)
	})
	if err != nil {
		return nil, fmt.Errorf("rundb: open %q: %w", runDB.path, err)
	}
	runDB.db = db
	return runDB, nil
}

// Close releases the underlying SQLite handle.
func (r *RunDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), store.DefaultDrainTimeout)
	defer cancel()
	return r.CloseContext(ctx)
}

// CloseContext checkpoints the SQLite WAL and closes the underlying handle.
func (r *RunDB) CloseContext(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("rundb: close context is required")
	}
	r.closeMu.Lock()
	defer r.closeMu.Unlock()

	if r.db == nil {
		return nil
	}
	db := r.db
	r.db = nil
	return closeRunSQLiteDatabase(ctx, db)
}

// Path reports the on-disk database path.
func (r *RunDB) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// CurrentMaxSequence returns the latest stored event sequence.
func (r *RunDB) CurrentMaxSequence(ctx context.Context) (uint64, error) {
	if err := r.requireContext(ctx, "load max sequence"); err != nil {
		return 0, err
	}

	var maxSeq sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT MAX(sequence) FROM events`).Scan(&maxSeq); err != nil {
		return 0, fmt.Errorf("rundb: query max event sequence: %w", err)
	}
	if !maxSeq.Valid || maxSeq.Int64 < 0 {
		return 0, nil
	}
	return uint64(maxSeq.Int64), nil
}

// StoreEventBatch persists canonical events and projection rows in one transaction.
func (r *RunDB) StoreEventBatch(ctx context.Context, items []events.Event) (retErr error) {
	if len(items) == 0 {
		return nil
	}
	if err := r.requireContext(ctx, "store event batch"); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("rundb: begin event batch: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				retErr = errors.Join(retErr, fmt.Errorf("rundb: rollback event batch: %w", rollbackErr))
			}
		}
	}()

	stmts, err := prepareEventBatchStatements(ctx, tx)
	if err != nil {
		return err
	}
	defer func() {
		_ = stmts.close()
	}()

	for _, item := range items {
		if err := storeProjectedEventWithStatements(ctx, stmts, item); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("rundb: commit event batch: %w", err)
	}
	committed = true
	return nil
}

// AppendSyntheticEvent appends one synthetic canonical event with the next
// available sequence. It is intended for daemon-owned recovery flows that need
// to persist a terminal event after the original writer loop is gone.
func (r *RunDB) AppendSyntheticEvent(
	ctx context.Context,
	kind events.EventKind,
	payload any,
) (events.Event, error) {
	if err := r.requireContext(ctx, "append synthetic event"); err != nil {
		return events.Event{}, err
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return events.Event{}, fmt.Errorf("rundb: marshal %s payload: %w", kind, err)
	}

	maxSeq, err := r.CurrentMaxSequence(ctx)
	if err != nil {
		return events.Event{}, err
	}

	item := events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         r.runID,
		Seq:           maxSeq + 1,
		Timestamp:     r.now(),
		Kind:          kind,
		Payload:       rawPayload,
	}
	if err := r.StoreEventBatch(ctx, []events.Event{item}); err != nil {
		return events.Event{}, err
	}
	return item, nil
}

// RecordHookRun persists one hook audit row.
func (r *RunDB) RecordHookRun(ctx context.Context, record HookRunRecord) error {
	if err := r.requireContext(ctx, "record hook run"); err != nil {
		return err
	}

	record.ID = strings.TrimSpace(record.ID)
	if record.ID == "" {
		record.ID = store.NewID("hook")
	}
	record.HookName = strings.TrimSpace(record.HookName)
	if record.HookName == "" {
		return errors.New("rundb: hook name is required")
	}
	record.Source = strings.TrimSpace(record.Source)
	if record.Source == "" {
		return errors.New("rundb: hook source is required")
	}
	record.Outcome = strings.TrimSpace(record.Outcome)
	if record.Outcome == "" {
		return errors.New("rundb: hook outcome is required")
	}
	if record.RecordedAt.IsZero() {
		record.RecordedAt = r.now()
	}

	if _, err := r.db.ExecContext(
		ctx,
		`INSERT INTO hook_runs (
			id,
			hook_name,
			source,
			outcome,
			duration_ns,
			payload_json,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.HookName,
		record.Source,
		record.Outcome,
		record.DurationNS,
		record.PayloadJSON,
		store.FormatTimestamp(record.RecordedAt),
	); err != nil {
		return fmt.Errorf("rundb: insert hook run %q: %w", record.ID, err)
	}
	return nil
}

// ListEvents returns persisted events at or after fromSeq in sequence order.
// When limit is greater than zero, it returns at most limit+1 rows so callers
// can detect a following page without a second query.
func (r *RunDB) ListEvents(ctx context.Context, fromSeq uint64, limit int) (EventListResult, error) {
	if err := r.requireContext(ctx, "list events"); err != nil {
		return EventListResult{}, err
	}

	query := `SELECT sequence, event_kind, payload_json, timestamp
		FROM events
		WHERE sequence >= ?
		ORDER BY sequence ASC`
	args := []any{fromSeq}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit+1)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return EventListResult{}, fmt.Errorf("rundb: query events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	result := EventListResult{
		Events: make([]events.Event, 0, max(limit, 0)),
	}
	for rows.Next() {
		var (
			sequence    int64
			eventKind   string
			payloadJSON string
			timestamp   string
		)
		if err := rows.Scan(&sequence, &eventKind, &payloadJSON, &timestamp); err != nil {
			return EventListResult{}, fmt.Errorf("rundb: scan event row: %w", err)
		}
		seq, err := sequenceValue(sequence, "event sequence")
		if err != nil {
			return EventListResult{}, err
		}
		parsedTS, err := store.ParseTimestamp(timestamp)
		if err != nil {
			return EventListResult{}, err
		}
		result.Events = append(result.Events, events.Event{
			SchemaVersion: events.SchemaVersion,
			RunID:         r.runID,
			Seq:           seq,
			Kind:          events.EventKind(eventKind),
			Payload:       json.RawMessage(payloadJSON),
			Timestamp:     parsedTS,
		})
	}
	if err := rows.Err(); err != nil {
		return EventListResult{}, fmt.Errorf("rundb: iterate events: %w", err)
	}
	if limit > 0 && len(result.Events) > limit {
		result.HasMore = true
	}
	return result, nil
}

// LastEvent returns the latest persisted canonical event, if any.
func (r *RunDB) LastEvent(ctx context.Context) (*events.Event, error) {
	if err := r.requireContext(ctx, "load last event"); err != nil {
		return nil, err
	}

	row := r.db.QueryRowContext(
		ctx,
		`SELECT sequence, event_kind, payload_json, timestamp
		 FROM events ORDER BY sequence DESC LIMIT 1`,
	)

	var (
		sequence    int64
		eventKind   string
		payloadJSON string
		timestamp   string
	)
	if err := row.Scan(&sequence, &eventKind, &payloadJSON, &timestamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("rundb: query last event: %w", err)
	}

	seq, err := sequenceValue(sequence, "event sequence")
	if err != nil {
		return nil, err
	}
	parsedTS, err := store.ParseTimestamp(timestamp)
	if err != nil {
		return nil, err
	}

	event := &events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         r.runID,
		Seq:           seq,
		Kind:          events.EventKind(eventKind),
		Payload:       json.RawMessage(payloadJSON),
		Timestamp:     parsedTS,
	}
	return event, nil
}

// ListJobState returns projected job rows ordered by job id.
func (r *RunDB) ListJobState(ctx context.Context) ([]JobStateRow, error) {
	if err := r.requireContext(ctx, "list job state"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT job_id, task_id, status, agent_name, summary_json, updated_at
		 FROM job_state ORDER BY job_id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("rundb: query job state: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]JobStateRow, 0)
	for rows.Next() {
		var (
			item      JobStateRow
			updatedAt string
		)
		if err := rows.Scan(
			&item.JobID,
			&item.TaskID,
			&item.Status,
			&item.AgentName,
			&item.SummaryJSON,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("rundb: scan job state row: %w", err)
		}
		parsed, err := store.ParseTimestamp(updatedAt)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt = parsed
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rundb: iterate job state: %w", err)
	}
	return items, nil
}

// ListTranscriptMessages returns projected transcript rows in sequence order.
func (r *RunDB) ListTranscriptMessages(ctx context.Context) ([]TranscriptMessageRow, error) {
	if err := r.requireContext(ctx, "list transcript messages"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT sequence, stream, role, content, metadata_json, timestamp
		 FROM transcript_messages ORDER BY sequence ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("rundb: query transcript messages: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]TranscriptMessageRow, 0)
	for rows.Next() {
		var (
			item      TranscriptMessageRow
			sequence  int64
			timestamp string
		)
		if err := rows.Scan(
			&sequence,
			&item.Stream,
			&item.Role,
			&item.Content,
			&item.MetadataJSON,
			&timestamp,
		); err != nil {
			return nil, fmt.Errorf("rundb: scan transcript row: %w", err)
		}
		seq, err := sequenceValue(sequence, "transcript sequence")
		if err != nil {
			return nil, err
		}
		parsed, err := store.ParseTimestamp(timestamp)
		if err != nil {
			return nil, err
		}
		item.Sequence = seq
		item.Timestamp = parsed
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rundb: iterate transcript messages: %w", err)
	}
	return items, nil
}

// ListHookRuns returns persisted hook audit rows in recorded order.
func (r *RunDB) ListHookRuns(ctx context.Context) ([]HookRunRecord, error) {
	if err := r.requireContext(ctx, "list hook runs"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, hook_name, source, outcome, duration_ns, payload_json, recorded_at
		 FROM hook_runs ORDER BY recorded_at ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("rundb: query hook runs: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]HookRunRecord, 0)
	for rows.Next() {
		var (
			item       HookRunRecord
			recordedAt string
		)
		if err := rows.Scan(
			&item.ID,
			&item.HookName,
			&item.Source,
			&item.Outcome,
			&item.DurationNS,
			&item.PayloadJSON,
			&recordedAt,
		); err != nil {
			return nil, fmt.Errorf("rundb: scan hook run row: %w", err)
		}
		parsed, err := store.ParseTimestamp(recordedAt)
		if err != nil {
			return nil, err
		}
		item.RecordedAt = parsed
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rundb: iterate hook runs: %w", err)
	}
	return items, nil
}

// ListTokenUsage returns token-usage rows ordered by timestamp.
func (r *RunDB) ListTokenUsage(ctx context.Context) ([]TokenUsageRow, error) {
	if err := r.requireContext(ctx, "list token usage"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT turn_id, input_tokens, output_tokens, total_tokens, cost_amount, timestamp
		 FROM token_usage ORDER BY timestamp ASC, turn_id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("rundb: query token usage: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]TokenUsageRow, 0)
	for rows.Next() {
		var (
			item      TokenUsageRow
			cost      sql.NullFloat64
			timestamp string
		)
		if err := rows.Scan(
			&item.TurnID,
			&item.InputTokens,
			&item.OutputTokens,
			&item.TotalTokens,
			&cost,
			&timestamp,
		); err != nil {
			return nil, fmt.Errorf("rundb: scan token usage row: %w", err)
		}
		if cost.Valid {
			value := cost.Float64
			item.CostAmount = &value
		}
		parsed, err := store.ParseTimestamp(timestamp)
		if err != nil {
			return nil, err
		}
		item.Timestamp = parsed
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rundb: iterate token usage: %w", err)
	}
	return items, nil
}

// ListArtifactSyncLog returns artifact sync rows in sequence order.
func (r *RunDB) ListArtifactSyncLog(ctx context.Context) ([]ArtifactSyncRow, error) {
	if err := r.requireContext(ctx, "list artifact sync log"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT sequence, relative_path, change_kind, checksum, synced_at
		 FROM artifact_sync_log ORDER BY sequence ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("rundb: query artifact sync log: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]ArtifactSyncRow, 0)
	for rows.Next() {
		var (
			item      ArtifactSyncRow
			sequence  int64
			timestamp string
		)
		if err := rows.Scan(
			&sequence,
			&item.RelativePath,
			&item.ChangeKind,
			&item.Checksum,
			&timestamp,
		); err != nil {
			return nil, fmt.Errorf("rundb: scan artifact sync row: %w", err)
		}
		seq, err := sequenceValue(sequence, "artifact sync sequence")
		if err != nil {
			return nil, err
		}
		parsed, err := store.ParseTimestamp(timestamp)
		if err != nil {
			return nil, err
		}
		item.Sequence = seq
		item.SyncedAt = parsed
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rundb: iterate artifact sync log: %w", err)
	}
	return items, nil
}

// GetIntegrity returns the durable sticky integrity state for this run.
func (r *RunDB) GetIntegrity(ctx context.Context) (RunIntegrityState, error) {
	if err := r.requireContext(ctx, "load run integrity"); err != nil {
		return RunIntegrityState{}, err
	}
	return queryIntegrityState(ctx, r.db)
}

// UpsertIntegrity merges the supplied integrity update into the sticky durable
// run-integrity row and returns the merged result.
func (r *RunDB) UpsertIntegrity(ctx context.Context, update RunIntegrityUpdate) (RunIntegrityState, error) {
	if err := r.requireContext(ctx, "upsert run integrity"); err != nil {
		return RunIntegrityState{}, err
	}
	if isNoopIntegrityUpdate(update) {
		return r.GetIntegrity(ctx)
	}

	r.integrityMu.Lock()
	defer r.integrityMu.Unlock()

	return r.upsertIntegrityLocked(ctx, update)
}

func (r *RunDB) upsertIntegrityLocked(
	ctx context.Context,
	update RunIntegrityUpdate,
) (state RunIntegrityState, retErr error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return RunIntegrityState{}, fmt.Errorf("rundb: begin run integrity transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			retErr = errors.Join(retErr, fmt.Errorf("rundb: rollback run integrity transaction: %w", rollbackErr))
		}
	}()

	existing, err := queryIntegrityState(ctx, tx)
	if err != nil {
		return RunIntegrityState{}, err
	}
	merged := mergeRunIntegrityState(existing, update, r.now)

	reasonsJSON, err := encodeIntegrityReasons(merged.Reasons)
	if err != nil {
		return RunIntegrityState{}, err
	}

	if err := upsertIntegrityState(ctx, tx, merged, reasonsJSON); err != nil {
		return RunIntegrityState{}, fmt.Errorf("rundb: upsert run integrity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return RunIntegrityState{}, fmt.Errorf("rundb: commit run integrity transaction: %w", err)
	}
	committed = true
	return merged, nil
}

func isNoopIntegrityUpdate(update RunIntegrityUpdate) bool {
	return !update.Incomplete && len(update.Reasons) == 0 &&
		update.JournalTerminalDrops == 0 && update.JournalNonTerminalDrops == 0
}

func mergeRunIntegrityState(
	existing RunIntegrityState,
	update RunIntegrityUpdate,
	now func() time.Time,
) RunIntegrityState {
	merged := RunIntegrityState{
		Incomplete:              existing.Incomplete || update.Incomplete,
		Reasons:                 mergeIntegrityReasons(existing.Reasons, update.Reasons),
		JournalTerminalDrops:    max(existing.JournalTerminalDrops, update.JournalTerminalDrops),
		JournalNonTerminalDrops: max(existing.JournalNonTerminalDrops, update.JournalNonTerminalDrops),
		FirstDetectedAt:         existing.FirstDetectedAt,
	}
	if len(merged.Reasons) > 0 || merged.JournalTerminalDrops > 0 || merged.JournalNonTerminalDrops > 0 {
		merged.Incomplete = true
	}
	if merged.Incomplete && merged.FirstDetectedAt.IsZero() {
		merged.FirstDetectedAt = now()
	}
	merged.UpdatedAt = now()
	return merged
}

func (r *RunDB) requireContext(ctx context.Context, action string) error {
	if r == nil || r.db == nil {
		return errors.New("rundb: database is required")
	}
	if ctx == nil {
		return fmt.Errorf("rundb: %s context is required", strings.TrimSpace(action))
	}
	return nil
}

type eventBatchStatements struct {
	insertEvent           *sql.Stmt
	upsertJobState        *sql.Stmt
	insertTranscript      *sql.Stmt
	upsertTokenUsage      *sql.Stmt
	insertArtifactSyncLog *sql.Stmt
}

type integrityQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type integrityExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func queryIntegrityState(ctx context.Context, queryer integrityQuerier) (RunIntegrityState, error) {
	row := queryer.QueryRowContext(
		ctx,
		`SELECT incomplete, reasons_json, journal_terminal_drops, journal_non_terminal_drops,
		        first_detected_at, updated_at
		 FROM run_integrity
		 WHERE singleton_id = 1`,
	)

	var (
		state                   RunIntegrityState
		incomplete              bool
		reasonsJSON             string
		journalTerminalDrops    int64
		journalNonTerminalDrops int64
		firstDetectedAt         string
		updatedAt               string
	)
	if err := row.Scan(
		&incomplete,
		&reasonsJSON,
		&journalTerminalDrops,
		&journalNonTerminalDrops,
		&firstDetectedAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunIntegrityState{}, nil
		}
		return RunIntegrityState{}, fmt.Errorf("rundb: query run integrity: %w", err)
	}

	reasons, err := decodeIntegrityReasons(reasonsJSON)
	if err != nil {
		return RunIntegrityState{}, err
	}
	terminalDrops, err := sequenceValue(journalTerminalDrops, "journal terminal drops")
	if err != nil {
		return RunIntegrityState{}, err
	}
	nonTerminalDrops, err := sequenceValue(journalNonTerminalDrops, "journal non-terminal drops")
	if err != nil {
		return RunIntegrityState{}, err
	}

	state = RunIntegrityState{
		Incomplete:              incomplete,
		Reasons:                 reasons,
		JournalTerminalDrops:    terminalDrops,
		JournalNonTerminalDrops: nonTerminalDrops,
	}
	if firstDetectedAt != "" {
		parsed, parseErr := store.ParseTimestamp(firstDetectedAt)
		if parseErr != nil {
			return RunIntegrityState{}, parseErr
		}
		state.FirstDetectedAt = parsed
	}
	if updatedAt != "" {
		parsed, parseErr := store.ParseTimestamp(updatedAt)
		if parseErr != nil {
			return RunIntegrityState{}, parseErr
		}
		state.UpdatedAt = parsed
	}
	return state, nil
}

func upsertIntegrityState(
	ctx context.Context,
	execer integrityExecer,
	merged RunIntegrityState,
	reasonsJSON string,
) error {
	_, err := execer.ExecContext(
		ctx,
		`INSERT INTO run_integrity (
			singleton_id,
			incomplete,
			reasons_json,
			journal_terminal_drops,
			journal_non_terminal_drops,
			first_detected_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(singleton_id) DO UPDATE SET
			incomplete=excluded.incomplete,
			reasons_json=excluded.reasons_json,
			journal_terminal_drops=excluded.journal_terminal_drops,
			journal_non_terminal_drops=excluded.journal_non_terminal_drops,
			first_detected_at=excluded.first_detected_at,
			updated_at=excluded.updated_at`,
		1,
		merged.Incomplete,
		reasonsJSON,
		merged.JournalTerminalDrops,
		merged.JournalNonTerminalDrops,
		store.FormatTimestamp(merged.FirstDetectedAt),
		store.FormatTimestamp(merged.UpdatedAt),
	)
	return err
}

type eventBatchStatementSpec struct {
	label  string
	query  string
	assign func(*eventBatchStatements, *sql.Stmt)
}

func prepareEventBatchStatements(ctx context.Context, tx *sql.Tx) (eventBatchStatements, error) {
	var statements eventBatchStatements
	specs := []eventBatchStatementSpec{
		{
			label: "event insert",
			query: `INSERT INTO events (sequence, event_kind, payload_json, timestamp, job_id, step_key)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			assign: func(dst *eventBatchStatements, stmt *sql.Stmt) { dst.insertEvent = stmt },
		},
		{
			label: "job_state upsert",
			query: `INSERT INTO job_state (job_id, task_id, status, agent_name, summary_json, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(job_id) DO UPDATE SET
				task_id=excluded.task_id,
				status=excluded.status,
				agent_name=excluded.agent_name,
				summary_json=excluded.summary_json,
				updated_at=excluded.updated_at`,
			assign: func(dst *eventBatchStatements, stmt *sql.Stmt) { dst.upsertJobState = stmt },
		},
		{
			label: "transcript insert",
			query: `INSERT OR REPLACE INTO transcript_messages (
				sequence,
				stream,
				role,
				content,
				metadata_json,
				timestamp
			) VALUES (?, ?, ?, ?, ?, ?)`,
			assign: func(dst *eventBatchStatements, stmt *sql.Stmt) { dst.insertTranscript = stmt },
		},
		{
			label: "token_usage upsert",
			query: `INSERT INTO token_usage (turn_id, input_tokens, output_tokens, total_tokens, cost_amount, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(turn_id) DO UPDATE SET
				input_tokens=excluded.input_tokens,
				output_tokens=excluded.output_tokens,
				total_tokens=excluded.total_tokens,
				cost_amount=excluded.cost_amount,
				timestamp=excluded.timestamp`,
			assign: func(dst *eventBatchStatements, stmt *sql.Stmt) { dst.upsertTokenUsage = stmt },
		},
		{
			label: "artifact sync insert",
			query: `INSERT OR REPLACE INTO artifact_sync_log (sequence, relative_path, change_kind, checksum, synced_at)
			 VALUES (?, ?, ?, ?, ?)`,
			assign: func(dst *eventBatchStatements, stmt *sql.Stmt) { dst.insertArtifactSyncLog = stmt },
		},
	}

	for _, spec := range specs {
		stmt, err := tx.PrepareContext(ctx, spec.query)
		if err != nil {
			_ = statements.close()
			return eventBatchStatements{}, fmt.Errorf("rundb: prepare %s: %w", spec.label, err)
		}
		spec.assign(&statements, stmt)
	}

	return statements, nil
}

func (s eventBatchStatements) close() error {
	var err error
	for _, stmt := range []*sql.Stmt{
		s.insertEvent,
		s.upsertJobState,
		s.insertTranscript,
		s.upsertTokenUsage,
		s.insertArtifactSyncLog,
	} {
		if stmt == nil {
			continue
		}
		if closeErr := stmt.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return err
}

func storeEventWithStatement(ctx context.Context, stmt *sql.Stmt, item events.Event) error {
	if _, err := stmt.ExecContext(
		ctx,
		item.Seq,
		string(item.Kind),
		string(item.Payload),
		store.FormatTimestamp(item.Timestamp),
		eventJobID(item),
		eventStepKey(item),
	); err != nil {
		return fmt.Errorf("rundb: insert event %d: %w", item.Seq, err)
	}
	return nil
}

func storeProjectedEventWithStatements(
	ctx context.Context,
	stmts eventBatchStatements,
	item events.Event,
) error {
	if err := storeEventWithStatement(ctx, stmts.insertEvent, item); err != nil {
		return err
	}
	if err := applyJobStateProjectionWithStatement(ctx, stmts.upsertJobState, item); err != nil {
		return err
	}
	if err := applyTranscriptProjectionWithStatement(ctx, stmts.insertTranscript, item); err != nil {
		return err
	}
	if err := applyTokenUsageProjectionWithStatement(ctx, stmts.upsertTokenUsage, item); err != nil {
		return err
	}
	if err := applyArtifactSyncProjectionWithStatement(ctx, stmts.insertArtifactSyncLog, item); err != nil {
		return err
	}
	return nil
}

func applyJobStateProjectionWithStatement(ctx context.Context, stmt *sql.Stmt, item events.Event) error {
	jobState, ok, err := projectJobState(item)
	if err != nil || !ok {
		return err
	}
	return upsertJobStateWithStatement(ctx, stmt, jobState)
}

func applyTranscriptProjectionWithStatement(ctx context.Context, stmt *sql.Stmt, item events.Event) error {
	transcriptRow, ok, err := ProjectTranscriptMessage(item)
	if err != nil || !ok {
		return err
	}
	return insertTranscriptMessageWithStatement(ctx, stmt, transcriptRow)
}

func applyTokenUsageProjectionWithStatement(ctx context.Context, stmt *sql.Stmt, item events.Event) error {
	usageRow, ok, err := projectTokenUsage(item)
	if err != nil || !ok {
		return err
	}
	return upsertTokenUsageWithStatement(ctx, stmt, usageRow)
}

func applyArtifactSyncProjectionWithStatement(ctx context.Context, stmt *sql.Stmt, item events.Event) error {
	artifactRow, ok, err := projectArtifactSync(item)
	if err != nil || !ok {
		return err
	}
	return insertArtifactSyncWithStatement(ctx, stmt, artifactRow)
}

func upsertJobStateWithStatement(ctx context.Context, stmt *sql.Stmt, item JobStateRow) error {
	if _, err := stmt.ExecContext(
		ctx,
		item.JobID,
		item.TaskID,
		item.Status,
		item.AgentName,
		item.SummaryJSON,
		store.FormatTimestamp(item.UpdatedAt),
	); err != nil {
		return fmt.Errorf("rundb: upsert job state %q: %w", item.JobID, err)
	}
	return nil
}

func insertTranscriptMessageWithStatement(ctx context.Context, stmt *sql.Stmt, item TranscriptMessageRow) error {
	if _, err := stmt.ExecContext(
		ctx,
		item.Sequence,
		item.Stream,
		item.Role,
		item.Content,
		item.MetadataJSON,
		store.FormatTimestamp(item.Timestamp),
	); err != nil {
		return fmt.Errorf("rundb: insert transcript row %d: %w", item.Sequence, err)
	}
	return nil
}

func upsertTokenUsageWithStatement(ctx context.Context, stmt *sql.Stmt, item TokenUsageRow) error {
	if _, err := stmt.ExecContext(
		ctx,
		item.TurnID,
		item.InputTokens,
		item.OutputTokens,
		item.TotalTokens,
		item.CostAmount,
		store.FormatTimestamp(item.Timestamp),
	); err != nil {
		return fmt.Errorf("rundb: upsert token usage %q: %w", item.TurnID, err)
	}
	return nil
}

func insertArtifactSyncWithStatement(ctx context.Context, stmt *sql.Stmt, item ArtifactSyncRow) error {
	if _, err := stmt.ExecContext(
		ctx,
		item.Sequence,
		item.RelativePath,
		item.ChangeKind,
		item.Checksum,
		store.FormatTimestamp(item.SyncedAt),
	); err != nil {
		return fmt.Errorf("rundb: insert artifact sync row %d: %w", item.Sequence, err)
	}
	return nil
}

func projectJobState(item events.Event) (JobStateRow, bool, error) {
	switch item.Kind {
	case events.EventKindJobQueued:
		return projectJobQueuedState(item)
	case events.EventKindJobStarted:
		return projectJobStartedState(item)
	case events.EventKindJobAttemptStarted:
		return projectJobAttemptStartedState(item)
	case events.EventKindJobAttemptFinished:
		return projectJobAttemptFinishedState(item)
	case events.EventKindJobRetryScheduled:
		return projectJobRetryScheduledState(item)
	case events.EventKindJobCompleted:
		return projectJobCompletedState(item)
	case events.EventKindJobFailed:
		return projectJobFailedState(item)
	case events.EventKindJobCancelled:
		return projectJobCancelledState(item)
	default:
		return JobStateRow{}, false, nil
	}
}

// ProjectTranscriptMessage projects one canonical event into the persisted
// transcript read model used for cold snapshots and replay.
func ProjectTranscriptMessage(item events.Event) (TranscriptMessageRow, bool, error) {
	if item.Kind != events.EventKindSessionUpdate {
		return TranscriptMessageRow{}, false, nil
	}

	var payload kinds.SessionUpdatePayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return TranscriptMessageRow{}, false, fmt.Errorf("rundb: decode session update payload: %w", err)
	}

	var role string
	blocks := payload.Update.Blocks
	switch payload.Update.Kind {
	case kinds.UpdateKindAgentMessageChunk:
		role = "assistant"
	case kinds.UpdateKindAgentThoughtChunk:
		role = "assistant_thinking"
		blocks = payload.Update.ThoughtBlocks
	case kinds.UpdateKindToolCallStarted, kinds.UpdateKindToolCallUpdated:
		role = "tool_call"
	default:
		role = "runtime_notice"
	}

	content := strings.TrimSpace(renderProjectedContentBlocks(blocks))
	if content == "" && strings.TrimSpace(payload.Update.ToolCallID) != "" {
		content = fmt.Sprintf("tool_call:%s", strings.TrimSpace(payload.Update.ToolCallID))
	}
	if content == "" {
		content = string(payload.Update.Status)
	}
	if strings.TrimSpace(content) == "" {
		return TranscriptMessageRow{}, false, nil
	}

	return TranscriptMessageRow{
		Sequence:     item.Seq,
		Stream:       "session",
		Role:         role,
		Content:      content,
		MetadataJSON: string(item.Payload),
		Timestamp:    item.Timestamp.UTC(),
	}, true, nil
}

func projectTokenUsage(item events.Event) (TokenUsageRow, bool, error) {
	switch item.Kind {
	case events.EventKindUsageUpdated:
		var payload kinds.UsageUpdatedPayload
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return TokenUsageRow{}, false, fmt.Errorf("rundb: decode usage updated payload: %w", err)
		}
		return newTokenUsageRow(fmt.Sprintf("session-%03d", payload.Index), payload.Usage, item.Timestamp), true, nil
	case events.EventKindUsageAggregated:
		var payload kinds.UsageAggregatedPayload
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return TokenUsageRow{}, false, fmt.Errorf("rundb: decode usage aggregated payload: %w", err)
		}
		return newTokenUsageRow("run-total", payload.Usage, item.Timestamp), true, nil
	case events.EventKindSessionCompleted:
		var payload kinds.SessionCompletedPayload
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return TokenUsageRow{}, false, fmt.Errorf("rundb: decode session completed payload: %w", err)
		}
		return newTokenUsageRow(fmt.Sprintf("session-%03d", payload.Index), payload.Usage, item.Timestamp), true, nil
	case events.EventKindSessionFailed:
		var payload kinds.SessionFailedPayload
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return TokenUsageRow{}, false, fmt.Errorf("rundb: decode session failed payload: %w", err)
		}
		return newTokenUsageRow(fmt.Sprintf("session-%03d", payload.Index), payload.Usage, item.Timestamp), true, nil
	default:
		return TokenUsageRow{}, false, nil
	}
}

func projectArtifactSync(item events.Event) (ArtifactSyncRow, bool, error) {
	switch item.Kind {
	case events.EventKindTaskFileUpdated:
		var payload kinds.TaskFileUpdatedPayload
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return ArtifactSyncRow{}, false, fmt.Errorf("rundb: decode task file payload: %w", err)
		}
		return ArtifactSyncRow{
			Sequence:     item.Seq,
			RelativePath: firstNonEmpty(payload.FilePath, payload.TaskName),
			ChangeKind:   "task_file_updated",
			Checksum:     "",
			SyncedAt:     item.Timestamp.UTC(),
		}, true, nil
	case events.EventKindTaskMemoryUpdated:
		var payload kinds.TaskMemoryUpdatedPayload
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return ArtifactSyncRow{}, false, fmt.Errorf("rundb: decode task memory payload: %w", err)
		}
		return ArtifactSyncRow{
			Sequence:     item.Seq,
			RelativePath: strings.TrimSpace(payload.Path),
			ChangeKind:   firstNonEmpty(strings.TrimSpace(payload.Mode), "task_memory_updated"),
			Checksum:     "",
			SyncedAt:     item.Timestamp.UTC(),
		}, true, nil
	case events.EventKindArtifactUpdated:
		var payload kinds.ArtifactUpdatedPayload
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return ArtifactSyncRow{}, false, fmt.Errorf("rundb: decode artifact updated payload: %w", err)
		}
		return ArtifactSyncRow{
			Sequence:     item.Seq,
			RelativePath: strings.TrimSpace(payload.Path),
			ChangeKind:   firstNonEmpty(strings.TrimSpace(payload.ChangeKind), "artifact_updated"),
			Checksum:     strings.TrimSpace(payload.Checksum),
			SyncedAt:     item.Timestamp.UTC(),
		}, true, nil
	default:
		return ArtifactSyncRow{}, false, nil
	}
}

func projectJobQueuedState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobQueuedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job queued payload: %w", err)
	}
	return newJobStateRow(
		item,
		jobIDFromIndex(payload.Index, payload.SafeName),
		firstNonEmpty(payload.SafeName, payload.TaskTitle, payload.CodeFile),
		"queued",
		payload.IDE,
	), true, nil
}

func projectJobStartedState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobStartedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job started payload: %w", err)
	}
	return newJobStateRow(item, jobIDFromIndex(payload.Index, ""), "", "started", payload.IDE), true, nil
}

func projectJobAttemptStartedState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobAttemptStartedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job attempt started payload: %w", err)
	}
	return newJobStateRow(item, jobIDFromIndex(payload.Index, ""), "", "attempt_started", ""), true, nil
}

func projectJobAttemptFinishedState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobAttemptFinishedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job attempt finished payload: %w", err)
	}
	status := firstNonEmpty(strings.TrimSpace(payload.Status), "attempt_finished")
	return newJobStateRow(item, jobIDFromIndex(payload.Index, ""), "", status, ""), true, nil
}

func projectJobRetryScheduledState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobRetryScheduledPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job retry payload: %w", err)
	}
	return newJobStateRow(item, jobIDFromIndex(payload.Index, ""), "", "retry_scheduled", ""), true, nil
}

func projectJobCompletedState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobCompletedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job completed payload: %w", err)
	}
	return newJobStateRow(item, jobIDFromIndex(payload.Index, ""), "", "completed", ""), true, nil
}

func projectJobFailedState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobFailedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job failed payload: %w", err)
	}
	return newJobStateRow(
		item,
		jobIDFromIndex(payload.Index, ""),
		strings.TrimSpace(payload.CodeFile),
		"failed",
		"",
	), true, nil
}

func projectJobCancelledState(item events.Event) (JobStateRow, bool, error) {
	var payload kinds.JobCancelledPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return JobStateRow{}, false, fmt.Errorf("rundb: decode job canceled payload: %w", err)
	}
	return newJobStateRow(item, jobIDFromIndex(payload.Index, ""), "", "canceled", ""), true, nil
}

func newJobStateRow(item events.Event, jobID, taskID, status, agentName string) JobStateRow {
	return JobStateRow{
		JobID:       strings.TrimSpace(jobID),
		TaskID:      strings.TrimSpace(taskID),
		Status:      strings.TrimSpace(status),
		AgentName:   strings.TrimSpace(agentName),
		SummaryJSON: string(item.Payload),
		UpdatedAt:   item.Timestamp.UTC(),
	}
}

func newTokenUsageRow(turnID string, usage kinds.Usage, timestamp time.Time) TokenUsageRow {
	return TokenUsageRow{
		TurnID:       strings.TrimSpace(turnID),
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.Total(),
		Timestamp:    timestamp.UTC(),
	}
}

func eventJobIDFromQueuedPayload(payload json.RawMessage) string {
	var envelope struct {
		Index    int    `json:"index"`
		SafeName string `json:"safe_name"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ""
	}
	return jobIDFromIndex(envelope.Index, envelope.SafeName)
}

func payloadIndex(payload json.RawMessage) (int, bool) {
	var envelope struct {
		Index int `json:"index"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return 0, false
	}
	return envelope.Index, true
}

func eventJobID(item events.Event) string {
	switch item.Kind {
	case events.EventKindJobQueued:
		return eventJobIDFromQueuedPayload(item.Payload)
	case events.EventKindJobStarted,
		events.EventKindJobAttemptStarted,
		events.EventKindJobAttemptFinished,
		events.EventKindJobRetryScheduled,
		events.EventKindJobCompleted,
		events.EventKindJobFailed,
		events.EventKindJobCancelled,
		events.EventKindSessionStarted,
		events.EventKindSessionUpdate,
		events.EventKindSessionCompleted,
		events.EventKindSessionFailed,
		events.EventKindUsageUpdated:
		index, ok := payloadIndex(item.Payload)
		if ok {
			return jobIDFromIndex(index, "")
		}
	}
	return ""
}

func eventStepKey(item events.Event) string {
	if item.Kind == events.EventKindSessionUpdate {
		var payload kinds.SessionUpdatePayload
		if err := json.Unmarshal(item.Payload, &payload); err == nil {
			return strings.TrimSpace(payload.Update.ToolCallID)
		}
	}
	return ""
}

func jobIDFromIndex(index int, safeName string) string {
	if trimmed := strings.TrimSpace(safeName); trimmed != "" {
		return trimmed
	}
	if index > 0 {
		return fmt.Sprintf("job-%03d", index)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func sequenceValue(raw int64, field string) (uint64, error) {
	if raw < 0 {
		return 0, fmt.Errorf("rundb: %s must be non-negative", strings.TrimSpace(field))
	}
	return uint64(raw), nil
}

func encodeIntegrityReasons(reasons []string) (string, error) {
	normalized := mergeIntegrityReasons(nil, reasons)
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("rundb: encode integrity reasons: %w", err)
	}
	return string(payload), nil
}

func decodeIntegrityReasons(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var reasons []string
	if err := json.Unmarshal([]byte(raw), &reasons); err != nil {
		return nil, fmt.Errorf("rundb: decode integrity reasons: %w", err)
	}
	return mergeIntegrityReasons(nil, reasons), nil
}

func mergeIntegrityReasons(left []string, right []string) []string {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(left)+len(right))
	merged := make([]string, 0, len(left)+len(right))
	for _, candidate := range append(append([]string(nil), left...), right...) {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		merged = append(merged, trimmed)
	}
	slices.Sort(merged)
	return merged
}

func renderProjectedContentBlocks(blocks []kinds.ContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		rendered := renderProjectedContentBlock(block)
		if rendered == "" {
			continue
		}
		lines = append(lines, rendered)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderProjectedContentBlock(block kinds.ContentBlock) string {
	switch block.Type {
	case kinds.BlockText:
		return renderProjectedTextBlock(block)
	case kinds.BlockToolUse:
		return renderProjectedToolUseBlock(block)
	case kinds.BlockToolResult:
		return renderProjectedToolResultBlock(block)
	case kinds.BlockDiff:
		return renderProjectedDiffBlock(block)
	case kinds.BlockTerminalOutput:
		return renderProjectedTerminalOutputBlock(block)
	case kinds.BlockImage:
		return renderProjectedImageBlock(block)
	default:
		return strings.TrimSpace(string(block.Data))
	}
}

func renderProjectedTextBlock(block kinds.ContentBlock) string {
	text, err := block.AsText()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text.Text)
}

func renderProjectedToolUseBlock(block kinds.ContentBlock) string {
	toolUse, err := block.AsToolUse()
	if err != nil {
		return ""
	}
	header := fmt.Sprintf(
		"[TOOL] %s (%s)",
		firstNonEmpty(toolUse.Title, toolUse.ToolName, toolUse.Name, toolUse.ID),
		toolUse.ID,
	)
	payload := strings.TrimSpace(string(toolUse.Input))
	if payload == "" {
		payload = strings.TrimSpace(string(toolUse.RawInput))
	}
	return strings.TrimSpace(strings.Join(compactProjectedLines(header, payload), "\n"))
}

func renderProjectedToolResultBlock(block kinds.ContentBlock) string {
	result, err := block.AsToolResult()
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(result.Content)
	if content == "" {
		content = fmt.Sprintf("[TOOL RESULT] %s", strings.TrimSpace(result.ToolUseID))
	}
	return content
}

func renderProjectedDiffBlock(block kinds.ContentBlock) string {
	diff, err := block.AsDiff()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(firstNonEmpty(diff.Diff, diff.NewText, diff.FilePath))
}

func renderProjectedTerminalOutputBlock(block kinds.ContentBlock) string {
	output, err := block.AsTerminalOutput()
	if err != nil {
		return ""
	}
	lines := make([]string, 0, 3)
	if command := strings.TrimSpace(output.Command); command != "" {
		lines = append(lines, "$ "+command)
	}
	if rendered := strings.TrimSpace(output.Output); rendered != "" {
		lines = append(lines, rendered)
	}
	if output.ExitCode != 0 {
		lines = append(lines, fmt.Sprintf("[exit code: %d]", output.ExitCode))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderProjectedImageBlock(block kinds.ContentBlock) string {
	image, err := block.AsImage()
	if err != nil {
		return ""
	}
	location := "inline"
	if image.URI != nil && strings.TrimSpace(*image.URI) != "" {
		location = strings.TrimSpace(*image.URI)
	}
	return fmt.Sprintf("[IMAGE] %s %s", strings.TrimSpace(image.MimeType), location)
}

func compactProjectedLines(lines ...string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

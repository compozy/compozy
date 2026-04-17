package rundb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type openOptions struct {
	now func() time.Time
}

// RunDB owns one per-run SQLite store.
type RunDB struct {
	db    *sql.DB
	path  string
	runID string
	now   func() time.Time
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
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
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

	for _, item := range items {
		if err := storeProjectedEvent(ctx, tx, item); err != nil {
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

// ListEvents returns all persisted events at or after fromSeq in sequence order.
func (r *RunDB) ListEvents(ctx context.Context, fromSeq uint64) ([]events.Event, error) {
	if err := r.requireContext(ctx, "list events"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT sequence, event_kind, payload_json, timestamp FROM events WHERE sequence >= ? ORDER BY sequence ASC`,
		fromSeq,
	)
	if err != nil {
		return nil, fmt.Errorf("rundb: query events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	result := make([]events.Event, 0)
	for rows.Next() {
		var (
			sequence    int64
			eventKind   string
			payloadJSON string
			timestamp   string
		)
		if err := rows.Scan(&sequence, &eventKind, &payloadJSON, &timestamp); err != nil {
			return nil, fmt.Errorf("rundb: scan event row: %w", err)
		}
		seq, err := sequenceValue(sequence, "event sequence")
		if err != nil {
			return nil, err
		}
		parsedTS, err := store.ParseTimestamp(timestamp)
		if err != nil {
			return nil, err
		}
		result = append(result, events.Event{
			SchemaVersion: events.SchemaVersion,
			RunID:         r.runID,
			Seq:           seq,
			Kind:          events.EventKind(eventKind),
			Payload:       json.RawMessage(payloadJSON),
			Timestamp:     parsedTS,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rundb: iterate events: %w", err)
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

func (r *RunDB) requireContext(ctx context.Context, action string) error {
	if r == nil || r.db == nil {
		return errors.New("rundb: database is required")
	}
	if ctx == nil {
		return fmt.Errorf("rundb: %s context is required", strings.TrimSpace(action))
	}
	return nil
}

func storeEvent(ctx context.Context, tx *sql.Tx, item events.Event) error {
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO events (sequence, event_kind, payload_json, timestamp, job_id, step_key)
		 VALUES (?, ?, ?, ?, ?, ?)`,
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

func storeProjectedEvent(ctx context.Context, tx *sql.Tx, item events.Event) error {
	if err := storeEvent(ctx, tx, item); err != nil {
		return err
	}
	if err := applyJobStateProjection(ctx, tx, item); err != nil {
		return err
	}
	if err := applyTranscriptProjection(ctx, tx, item); err != nil {
		return err
	}
	if err := applyTokenUsageProjection(ctx, tx, item); err != nil {
		return err
	}
	if err := applyArtifactSyncProjection(ctx, tx, item); err != nil {
		return err
	}
	return nil
}

func applyJobStateProjection(ctx context.Context, tx *sql.Tx, item events.Event) error {
	jobState, ok, err := projectJobState(item)
	if err != nil || !ok {
		return err
	}
	return upsertJobState(ctx, tx, jobState)
}

func applyTranscriptProjection(ctx context.Context, tx *sql.Tx, item events.Event) error {
	transcriptRow, ok, err := projectTranscriptMessage(item)
	if err != nil || !ok {
		return err
	}
	return insertTranscriptMessage(ctx, tx, transcriptRow)
}

func applyTokenUsageProjection(ctx context.Context, tx *sql.Tx, item events.Event) error {
	usageRow, ok, err := projectTokenUsage(item)
	if err != nil || !ok {
		return err
	}
	return upsertTokenUsage(ctx, tx, usageRow)
}

func applyArtifactSyncProjection(ctx context.Context, tx *sql.Tx, item events.Event) error {
	artifactRow, ok, err := projectArtifactSync(item)
	if err != nil || !ok {
		return err
	}
	return insertArtifactSync(ctx, tx, artifactRow)
}

func upsertJobState(ctx context.Context, tx *sql.Tx, item JobStateRow) error {
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO job_state (job_id, task_id, status, agent_name, summary_json, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(job_id) DO UPDATE SET
			task_id=excluded.task_id,
			status=excluded.status,
			agent_name=excluded.agent_name,
			summary_json=excluded.summary_json,
			updated_at=excluded.updated_at`,
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

func insertTranscriptMessage(ctx context.Context, tx *sql.Tx, item TranscriptMessageRow) error {
	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO transcript_messages (
			sequence,
			stream,
			role,
			content,
			metadata_json,
			timestamp
		) VALUES (?, ?, ?, ?, ?, ?)`,
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

func upsertTokenUsage(ctx context.Context, tx *sql.Tx, item TokenUsageRow) error {
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO token_usage (turn_id, input_tokens, output_tokens, total_tokens, cost_amount, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(turn_id) DO UPDATE SET
			input_tokens=excluded.input_tokens,
			output_tokens=excluded.output_tokens,
			total_tokens=excluded.total_tokens,
			cost_amount=excluded.cost_amount,
			timestamp=excluded.timestamp`,
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

func insertArtifactSync(ctx context.Context, tx *sql.Tx, item ArtifactSyncRow) error {
	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO artifact_sync_log (sequence, relative_path, change_kind, checksum, synced_at)
		 VALUES (?, ?, ?, ?, ?)`,
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

func projectTranscriptMessage(item events.Event) (TranscriptMessageRow, bool, error) {
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

	content := strings.TrimSpace(renderContentBlocks(blocks))
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
			ChangeKind:   "artifact_updated",
			Checksum:     "",
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

func renderContentBlocks(blocks []kinds.ContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if rendered := renderContentBlock(block); rendered != "" {
			parts = append(parts, rendered)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func renderContentBlock(block kinds.ContentBlock) string {
	switch block.Type {
	case kinds.BlockText:
		text, err := block.AsText()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(text.Text)
	case kinds.BlockToolUse:
		toolUse, err := block.AsToolUse()
		if err != nil {
			return ""
		}
		return firstNonEmpty(toolUse.Title, toolUse.ToolName, toolUse.Name, toolUse.ID)
	case kinds.BlockToolResult:
		result, err := block.AsToolResult()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(result.Content)
	case kinds.BlockDiff:
		diff, err := block.AsDiff()
		if err != nil {
			return ""
		}
		return firstNonEmpty(diff.NewText, diff.Diff, diff.FilePath)
	case kinds.BlockTerminalOutput:
		output, err := block.AsTerminalOutput()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(output.Output)
	default:
		return ""
	}
}

func sequenceValue(raw int64, field string) (uint64, error) {
	if raw < 0 {
		return 0, fmt.Errorf("rundb: %s must be non-negative", strings.TrimSpace(field))
	}
	return uint64(raw), nil
}

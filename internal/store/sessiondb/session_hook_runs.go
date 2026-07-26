package sessiondb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
)

// RecordHookRun stores one hook execution audit record in the per-session store.
func (s *SessionDB) RecordHookRun(ctx context.Context, record hookspkg.HookRunRecord) error {
	if s == nil {
		return errors.New("store: session database is required")
	}
	if ctx == nil {
		return errors.New("store: record hook run context is required")
	}

	return s.enqueueWrite(ctx, sessionWriteRequest{
		ctx:    ctx,
		kind:   sessionWriteHookRun,
		hook:   cloneHookRunRecord(record),
		result: make(chan sessionWriteResult, 1),
	})
}

// QueryHookRuns returns persisted hook execution records filtered by the supplied options.
func (s *SessionDB) QueryHookRuns(ctx context.Context, query store.HookRunQuery) ([]hookspkg.HookRunRecord, error) {
	if s == nil {
		return nil, errors.New("store: session database is required")
	}
	if ctx == nil {
		return nil, errors.New("store: query hook runs context is required")
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.SessionID) != "" && strings.TrimSpace(query.SessionID) != s.sessionID {
		return nil, fmt.Errorf(
			"store: hook run query session id %q does not match session database %q",
			query.SessionID,
			s.sessionID,
		)
	}
	if event := strings.TrimSpace(query.Event); event != "" {
		if err := hookspkg.HookEvent(event).Validate(); err != nil {
			return nil, err
		}
	}
	if query.Outcome != "" {
		if err := query.Outcome.Validate(); err != nil {
			return nil, err
		}
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return nil, store.ErrClosed
	}

	baseQuery := `SELECT
		rowid, hook_name, event, source, mode, duration_ns, outcome,
		dispatch_depth, patch_applied, error, required, recorded_at
		FROM hook_runs`
	where, args := store.BuildClauses(
		store.StringClause("event", query.Event),
		store.StringClause("outcome", string(query.Outcome)),
		store.TimeClause("recorded_at", ">=", query.Since),
	)
	baseQuery = store.AppendWhere(baseQuery, where)

	sqlQuery := baseQuery
	if query.Limit > 0 {
		sqlQuery = `SELECT
				rowid, hook_name, event, source, mode, duration_ns, outcome,
				dispatch_depth, patch_applied, error, required, recorded_at
				FROM (` + baseQuery + ` ORDER BY recorded_at DESC, rowid DESC LIMIT ?) AS recent_hook_runs
				ORDER BY recorded_at ASC, rowid ASC`
		args = append(args, query.Limit)
	} else {
		sqlQuery += " ORDER BY recorded_at ASC, rowid ASC"
	}

	// dynamic-sql: optional hook event/outcome/time predicates and the bounded recent-window shape vary together.
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query hook runs: %w", err)
	}
	return s.scanHookRunRecords(rows)
}

func (s *SessionDB) writeHookRun(ctx context.Context, record hookspkg.HookRunRecord) error {
	if strings.TrimSpace(record.HookName) == "" {
		return errors.New("store: hook run hook name is required")
	}
	if err := record.Event.Validate(); err != nil {
		return err
	}
	if err := record.Source.Validate(); err != nil {
		return err
	}
	if err := record.Mode.Validate(); err != nil {
		return err
	}
	if err := record.Outcome.Validate(); err != nil {
		return err
	}
	if record.RecordedAt.IsZero() {
		record.RecordedAt = s.now()
	}

	id := store.NewID("hook")
	if err := sqlcgen.New(s.db).InsertHookRun(ctx, sqlcgen.InsertHookRunParams{
		ID:            id,
		HookName:      record.HookName,
		Event:         record.Event.String(),
		Source:        record.Source.String(),
		Mode:          string(record.Mode),
		DurationNs:    record.Duration.Nanoseconds(),
		Outcome:       string(record.Outcome),
		DispatchDepth: int64(record.DispatchDepth),
		PatchApplied:  sessionNullableString(rawJSONText(record.PatchApplied)),
		Error:         sessionNullableString(record.Error),
		Required:      boolToSQLite(record.Required),
		RecordedAt:    store.FormatTimestamp(record.RecordedAt),
	}); err != nil {
		return fmt.Errorf("store: insert hook run: %w", err)
	}

	return nil
}

func (s *SessionDB) scanHookRunRecord(scanner rowScanner) (hookspkg.HookRunRecord, error) {
	var (
		record        hookspkg.HookRunRecord
		rowID         int64
		event         string
		source        string
		mode          string
		durationNS    int64
		outcome       string
		patchApplied  sql.NullString
		recordError   sql.NullString
		required      int64
		recordedAtRaw string
	)

	if err := scanner.Scan(
		&rowID,
		&record.HookName,
		&event,
		&source,
		&mode,
		&durationNS,
		&outcome,
		&record.DispatchDepth,
		&patchApplied,
		&recordError,
		&required,
		&recordedAtRaw,
	); err != nil {
		return hookspkg.HookRunRecord{}, fmt.Errorf("store: scan hook run: %w", err)
	}

	record.Event = hookspkg.HookEvent(strings.TrimSpace(event))
	if err := record.Event.Validate(); err != nil {
		return hookspkg.HookRunRecord{}, err
	}
	if err := record.Source.UnmarshalText([]byte(strings.TrimSpace(source))); err != nil {
		return hookspkg.HookRunRecord{}, err
	}
	record.Mode = hookspkg.HookMode(strings.TrimSpace(mode))
	if err := record.Mode.Validate(); err != nil {
		return hookspkg.HookRunRecord{}, err
	}
	record.Duration = time.Duration(durationNS)
	record.Outcome = hookspkg.HookRunOutcome(strings.TrimSpace(outcome))
	if err := record.Outcome.Validate(); err != nil {
		return hookspkg.HookRunRecord{}, err
	}
	record.Required = required != 0
	record.Error = strings.TrimSpace(recordError.String)
	if patchApplied.Valid && strings.TrimSpace(patchApplied.String) != "" {
		record.PatchApplied = json.RawMessage(patchApplied.String)
	}

	recordedAt, err := store.ParseTimestamp(recordedAtRaw)
	if err != nil {
		return hookspkg.HookRunRecord{}, err
	}
	record.RecordedAt = recordedAt
	_ = rowID
	return cloneHookRunRecord(record), nil
}

func rawJSONText(raw json.RawMessage) string {
	return strings.TrimSpace(string(raw))
}

func cloneHookRunRecord(src hookspkg.HookRunRecord) hookspkg.HookRunRecord {
	cloned := src
	cloned.PatchApplied = cloneRawJSON(src.PatchApplied)
	return cloned
}

func cloneRawJSON(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), src...)
}

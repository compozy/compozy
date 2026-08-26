package journal

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type pageCursor struct {
	StartedAt   int64  `json:"started_at"`
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
}

// Query returns one opaque-cursor page in descending start/id order.
func (s *Service) Query(
	ctx context.Context,
	workspaceID string,
	scope store.ReadScope,
	query terminalpkg.Query,
) (*terminalpkg.Page, error) {
	if err := scope.Validate(); err != nil || strings.TrimSpace(workspaceID) == "" {
		return &terminalpkg.Page{Entries: []terminalpkg.CommandRow{}}, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	scope.ProfileID = strings.TrimSpace(scope.ProfileID)
	query = normalizeQuery(query)
	limit := query.Limit
	if limit == 0 {
		limit = defaultPageLimit
	}
	if limit < 1 || limit > maximumPageLimit {
		return nil, fmt.Errorf("terminal journal: limit must be between 1 and %d", maximumPageLimit)
	}
	fingerprint := queryFingerprint(workspaceID, scope, query)
	cursor, err := decodeCursor(query.Cursor, fingerprint)
	if err != nil {
		return nil, err
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("terminal journal: open workspace store: %w", err)
	}
	statement, args, err := s.queryStatement(scope, query, cursor, limit+1)
	if err != nil {
		return nil, err
	}
	rows, err := db.DB().QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("terminal journal: query commands: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("terminal journal: close query rows", "error", closeErr)
		}
	}()
	entries := make([]terminalpkg.CommandRow, 0, limit+1)
	for rows.Next() {
		row, scanErr := scanCommandRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("terminal journal: iterate commands: %w", err)
	}
	next := ""
	if len(entries) > limit {
		entries = entries[:limit]
		last := entries[len(entries)-1]
		next, err = encodeCursor(pageCursor{
			StartedAt: last.StartedAt.UnixMilli(), ID: last.ID, Fingerprint: fingerprint,
		})
		if err != nil {
			return nil, err
		}
	}
	return &terminalpkg.Page{Entries: entries, Next: next}, nil
}

func normalizeQuery(query terminalpkg.Query) terminalpkg.Query {
	query.Actor = strings.TrimSpace(query.Actor)
	query.Since = strings.TrimSpace(query.Since)
	query.Terminal = strings.TrimSpace(query.Terminal)
	query.Cursor = strings.TrimSpace(query.Cursor)
	return query
}

func (s *Service) queryStatement(
	scope store.ReadScope,
	query terminalpkg.Query,
	cursor pageCursor,
	limit int,
) (string, []any, error) {
	const selectColumns = `SELECT id, terminal_id, profile_id, actor_kind, actor_id, session_id, run_id,
command, argv_digest, cwd, started_at, duration_ms, exit_code, exit_signal, exit_cause,
detected_by, approval, output_bytes, truncated, recording_id FROM terminal_commands`
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 10)
	if !scope.AllProfiles {
		conditions = append(conditions, "profile_id = ?")
		args = append(args, strings.TrimSpace(scope.ProfileID))
	}
	if actor := strings.TrimSpace(query.Actor); actor != "" {
		conditions = append(conditions, "actor_kind = ?")
		args = append(args, actor)
	}
	if terminalID := strings.TrimSpace(query.Terminal); terminalID != "" {
		conditions = append(conditions, "terminal_id = ?")
		args = append(args, terminalID)
	}
	if query.Failed {
		conditions = append(conditions, "(exit_cause <> 'exited' OR COALESCE(exit_code, 1) <> 0)")
	}
	if since := strings.TrimSpace(query.Since); since != "" {
		duration, err := time.ParseDuration(since)
		if err != nil || duration < 0 {
			return "", nil, fmt.Errorf("terminal journal: invalid since duration %q", since)
		}
		conditions = append(conditions, "started_at >= ?")
		args = append(args, s.now().Add(-duration).UnixMilli())
	}
	if cursor.ID != "" {
		conditions = append(conditions, "(started_at < ? OR (started_at = ? AND id < ?))")
		args = append(args, cursor.StartedAt, cursor.StartedAt, cursor.ID)
	}
	statement := selectColumns + " WHERE " + strings.Join(conditions, " AND ") +
		" ORDER BY started_at DESC, id DESC LIMIT ?"
	args = append(args, limit)
	return statement, args, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCommandRow(scanner rowScanner) (terminalpkg.CommandRow, error) {
	var row terminalpkg.CommandRow
	var terminalID, sessionID, runID, argvDigest, exitSignal, recordingID sql.NullString
	var startedAt int64
	var duration, exitCode sql.NullInt64
	var actorKind string
	var truncated int64
	err := scanner.Scan(
		&row.ID, &terminalID, &row.ProfileID, &actorKind, &row.Actor.ID, &sessionID, &runID,
		&row.Command, &argvDigest, &row.Cwd, &startedAt, &duration, &exitCode, &exitSignal,
		&row.ExitCause, &row.DetectedBy, &row.Approval, &row.OutputBytes, &truncated, &recordingID,
	)
	if err != nil {
		return terminalpkg.CommandRow{}, fmt.Errorf("terminal journal: scan command: %w", err)
	}
	row.Actor.Kind = terminalpkg.ActorKind(actorKind)
	row.Actor.ProfileID = row.ProfileID
	row.Actor.SessionID = sessionID.String
	row.Actor.RunID = runID.String
	row.StartedAt = time.UnixMilli(startedAt).UTC()
	row.Truncated = truncated != 0
	if terminalID.Valid {
		id := terminalpkg.ID(terminalID.String)
		row.TerminalID = &id
	}
	row.ArgvDigest = stringPointer(argvDigest)
	row.DurationMs = int64Pointer(duration)
	row.ExitCode = intPointer(exitCode)
	row.ExitSignal = stringPointer(exitSignal)
	row.RecordingID = stringPointer(recordingID)
	return row, nil
}

func queryFingerprint(workspaceID string, scope store.ReadScope, query terminalpkg.Query) string {
	value := fmt.Sprintf("%s\x00%s\x00%t\x00%s\x00%s\x00%s\x00%t",
		workspaceID, scope.ProfileID, scope.AllProfiles, query.Actor, query.Since, query.Terminal, query.Failed)
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func decodeCursor(value, fingerprint string) (pageCursor, error) {
	if strings.TrimSpace(value) == "" {
		return pageCursor{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return pageCursor{}, errors.New("terminal journal: invalid cursor")
	}
	var cursor pageCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.ID == "" || cursor.Fingerprint != fingerprint {
		return pageCursor{}, errors.New("terminal journal: invalid cursor")
	}
	return cursor, nil
}

func encodeCursor(cursor pageCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("terminal journal: encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func int64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func intPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

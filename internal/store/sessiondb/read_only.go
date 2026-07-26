package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
)

const (
	defaultReadOnlyOpenMaxAttempts   = 15
	defaultReadOnlyOpenMinRetryDelay = 20 * time.Millisecond
	defaultReadOnlyOpenMaxRetryDelay = 150 * time.Millisecond
)

// ReadOnlyOpenOption customizes read-only session database opening.
type ReadOnlyOpenOption func(*readOnlyOpenConfig)

type readOnlyOpenConfig struct {
	maxAttempts   int
	minRetryDelay time.Duration
	maxRetryDelay time.Duration
}

// WithReadOnlyOpenRetry configures retry behavior for read-only session
// database opens.
func WithReadOnlyOpenRetry(
	maxAttempts int,
	minRetryDelay time.Duration,
	maxRetryDelay time.Duration,
) ReadOnlyOpenOption {
	return func(config *readOnlyOpenConfig) {
		config.maxAttempts = maxAttempts
		config.minRetryDelay = minRetryDelay
		config.maxRetryDelay = maxRetryDelay
	}
}

func newReadOnlyOpenConfig(options []ReadOnlyOpenOption) readOnlyOpenConfig {
	config := readOnlyOpenConfig{
		maxAttempts:   defaultReadOnlyOpenMaxAttempts,
		minRetryDelay: defaultReadOnlyOpenMinRetryDelay,
		maxRetryDelay: defaultReadOnlyOpenMaxRetryDelay,
	}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return config.normalize()
}

func (c readOnlyOpenConfig) normalize() readOnlyOpenConfig {
	if c.maxAttempts <= 0 {
		c.maxAttempts = defaultReadOnlyOpenMaxAttempts
	}
	if c.minRetryDelay <= 0 {
		c.minRetryDelay = defaultReadOnlyOpenMinRetryDelay
	}
	if c.maxRetryDelay <= 0 {
		c.maxRetryDelay = defaultReadOnlyOpenMaxRetryDelay
	}
	if c.maxRetryDelay < c.minRetryDelay {
		c.maxRetryDelay = c.minRetryDelay
	}
	return c
}

// ReadOnlySessionDB opens an existing per-session events database for queries
// without creating, migrating, checkpointing, or otherwise mutating it.
type ReadOnlySessionDB struct {
	db        *sql.DB
	sessionID string
}

var _ store.EventReadCloser = (*ReadOnlySessionDB)(nil)
var _ store.EventMetadataReadCloser = (*ReadOnlySessionDB)(nil)

// OpenSessionDBReadOnly opens an existing per-session events database in
// SQLite read-only mode. It intentionally fails for missing paths instead of
// creating a fresh database during stale transcript/event reads.
func OpenSessionDBReadOnly(
	ctx context.Context,
	sessionID string,
	path string,
	options ...ReadOnlyOpenOption,
) (*ReadOnlySessionDB, error) {
	return openSessionDBReadOnlyWithRetry(
		ctx,
		sessionID,
		path,
		openSessionDBReadOnlyOnce,
		store.IsSQLiteBusy,
		newReadOnlyOpenConfig(options),
	)
}

type readOnlySessionDBOpener func(context.Context, string, string) (*ReadOnlySessionDB, error)

func openSessionDBReadOnlyWithRetry(
	ctx context.Context,
	sessionID string,
	path string,
	opener readOnlySessionDBOpener,
	retryable func(error) bool,
	config readOnlyOpenConfig,
) (*ReadOnlySessionDB, error) {
	if opener == nil {
		return nil, errors.New("store: read-only session database opener is required")
	}
	if retryable == nil {
		retryable = func(error) bool { return false }
	}
	config = config.normalize()

	var lastErr error
	for attempt := 1; attempt <= config.maxAttempts; attempt++ {
		reader, err := opener(ctx, sessionID, path)
		if err == nil {
			return reader, nil
		}
		lastErr = err
		if !retryable(err) || attempt == config.maxAttempts {
			return nil, err
		}
		if waitErr := waitForReadOnlyOpenRetry(ctx, readOnlyOpenRetryDelay(config, attempt)); waitErr != nil {
			return nil, errors.Join(err, waitErr)
		}
	}
	return nil, lastErr
}

func openSessionDBReadOnlyOnce(ctx context.Context, sessionID string, path string) (*ReadOnlySessionDB, error) {
	if ctx == nil {
		return nil, errors.New("store: open read-only session database context is required")
	}
	cleanSessionID := strings.TrimSpace(sessionID)
	if cleanSessionID == "" {
		return nil, errors.New("store: read-only session database session id is required")
	}
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("store: read-only session database path is required")
	}

	db, err := sql.Open("sqlite", readOnlySessionSQLiteDSN(cleanPath))
	if err != nil {
		return nil, fmt.Errorf("store: open read-only session database %q: %w", cleanPath, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		return nil, closeReadOnlySessionDBAfterOpenError(
			db,
			fmt.Errorf("store: ping read-only session database %q: %w", cleanPath, err),
		)
	}
	if err := store.RequireCurrent(ctx, db, MigrationStream()); err != nil {
		return nil, closeReadOnlySessionDBAfterOpenError(
			db,
			fmt.Errorf("store: validate read-only session database %q: %w", cleanPath, err),
		)
	}
	// dynamic-sql: connection-scoped SQLite PRAGMA is lifecycle configuration outside sqlc's schema query model.
	if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		return nil, closeReadOnlySessionDBAfterOpenError(
			db,
			fmt.Errorf("store: guard read-only session database %q: %w", cleanPath, err),
		)
	}

	return &ReadOnlySessionDB{db: db, sessionID: cleanSessionID}, nil
}

func readOnlySessionSQLiteDSN(path string) string {
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}
	query := u.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", store.DefaultSQLiteBusyTimeoutMS))
	u.RawQuery = query.Encode()
	return u.String()
}

func readOnlyOpenRetryDelay(config readOnlyOpenConfig, attempt int) time.Duration {
	config = config.normalize()
	if attempt <= 0 {
		return config.minRetryDelay
	}
	delay := time.Duration(attempt) * config.minRetryDelay
	if delay > config.maxRetryDelay {
		return config.maxRetryDelay
	}
	return delay
}

func waitForReadOnlyOpenRetry(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		return errors.New("store: read-only session database retry context is required")
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("store: wait for read-only session database retry: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func closeReadOnlySessionDBAfterOpenError(db *sql.DB, openErr error) error {
	if db == nil {
		return openErr
	}
	if closeErr := db.Close(); closeErr != nil {
		return errors.Join(
			openErr,
			fmt.Errorf("store: close read-only session database after open failure: %w", closeErr),
		)
	}
	return openErr
}

// Query returns events filtered by the supplied options.
func (s *ReadOnlySessionDB) Query(
	ctx context.Context,
	query store.EventQuery,
) (events []store.SessionEvent, err error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: read-only session database is required")
	}
	if ctx == nil {
		return nil, errors.New("store: query read-only session database context is required")
	}
	sqlQuery, args, err := buildEventQuerySQL(sessionEventColumns, query)
	if err != nil {
		return nil, err
	}

	// dynamic-sql: buildEventQuerySQL composes the selected projection, optional filters, cursors, and limiting.
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query read-only session events: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinSessionCleanupError(&err, fmt.Errorf("store: close read-only session event rows: %w", closeErr))
		}
	}()

	events = make([]store.SessionEvent, 0)
	for rows.Next() {
		event, scanErr := s.scanSessionEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate read-only session events: %w", err)
	}

	return events, nil
}

// MaxEventSequence returns the current highest per-session event sequence.
func (s *ReadOnlySessionDB) MaxEventSequence(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store: read-only session database is required")
	}
	if ctx == nil {
		return 0, errors.New("store: query read-only session database context is required")
	}
	sequence, err := sqlcgen.New(s.db).MaxEventSequence(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: query read-only max event sequence: %w", err)
	}
	return sequence, nil
}

// QueryEventMetadata returns ordered session event metadata without reading content.
func (s *ReadOnlySessionDB) QueryEventMetadata(
	ctx context.Context,
	query store.EventQuery,
) (events []store.EventMetadata, err error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: read-only session database is required")
	}
	if ctx == nil {
		return nil, errors.New("store: query read-only session database context is required")
	}
	sqlQuery, args, err := buildEventQuerySQL(sessionEventMetadataColumns, query)
	if err != nil {
		return nil, err
	}

	// dynamic-sql: buildEventQuerySQL composes the selected projection, optional filters, cursors, and limiting.
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query read-only session event metadata: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinSessionCleanupError(
				&err,
				fmt.Errorf("store: close read-only session event metadata rows: %w", closeErr),
			)
		}
	}()

	events = make([]store.EventMetadata, 0)
	for rows.Next() {
		event, scanErr := s.scanEventMetadata(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate read-only session event metadata: %w", err)
	}
	return events, nil
}

func (s *ReadOnlySessionDB) scanEventMetadata(row rowScanner) (store.EventMetadata, error) {
	var event store.EventMetadata
	var timestampRaw string
	if err := row.Scan(
		&event.ID,
		&event.Sequence,
		&event.TurnID,
		&event.Type,
		&event.AgentName,
		&timestampRaw,
	); err != nil {
		return store.EventMetadata{}, fmt.Errorf("store: scan session event metadata: %w", err)
	}
	timestamp, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		return store.EventMetadata{}, err
	}
	event.SessionID = s.sessionID
	event.Timestamp = timestamp
	return event, nil
}

// History returns ordered session events grouped by turn id.
func (s *ReadOnlySessionDB) History(ctx context.Context, query store.EventQuery) ([]store.TurnHistory, error) {
	return queryTurnHistory(ctx, query, s.Query)
}

// Close closes the read-only database handle without checkpointing.
func (s *ReadOnlySessionDB) Close(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close read-only session database context is required")
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("store: close read-only session database: %w", err)
	}
	s.db = nil
	return nil
}

func (s *ReadOnlySessionDB) scanSessionEvent(scanner rowScanner) (store.SessionEvent, error) {
	var (
		event     store.SessionEvent
		timestamp string
	)
	if err := scanner.Scan(
		&event.ID,
		&event.Sequence,
		&event.TurnID,
		&event.Type,
		&event.AgentName,
		&event.Content,
		&event.Archived,
		&timestamp,
	); err != nil {
		return store.SessionEvent{}, fmt.Errorf("store: scan read-only session event: %w", err)
	}

	parsed, err := store.ParseTimestamp(timestamp)
	if err != nil {
		return store.SessionEvent{}, err
	}
	event.Timestamp = parsed
	event.SessionID = s.sessionID
	return event, nil
}

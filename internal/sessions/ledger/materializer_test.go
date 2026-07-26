package ledger

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
)

func TestMaterializer(t *testing.T) {
	t.Parallel()

	t.Run("Should materialize workspace ledger from durable event store", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		record := createLedgerRecord(ctx, t, "sess-child", "ws-primary")
		materializer := newTestMaterializer(t, root)

		result, err := materializer.Materialize(ctx, record)
		if err != nil {
			t.Fatalf("Materialize() error = %v", err)
		}
		if !result.Written {
			t.Fatal("Materialize().Written = false, want true")
		}
		if result.Events != 2 {
			t.Fatalf("Materialize().Events = %d, want 2", result.Events)
		}
		wantPath := filepath.Join(root, "ws-primary", "sess-child", "ledger.jsonl")
		if result.Path != wantPath {
			t.Fatalf("Materialize().Path = %q, want %q", result.Path, wantPath)
		}

		lines := readLedgerLines(t, result.Path)
		if len(lines) != 3 {
			t.Fatalf("ledger line count = %d, want 3", len(lines))
		}
		meta := decodeLedgerLine(t, lines[0])
		if got := meta["type"]; got != "ledger_meta" {
			t.Fatalf("meta type = %v, want ledger_meta", got)
		}
		if got := meta["workspace_id"]; got != "ws-primary" {
			t.Fatalf("meta workspace_id = %v, want ws-primary", got)
		}
		if got := meta["spawn_parent_id"]; got != "sess-parent" {
			t.Fatalf("meta spawn_parent_id = %v, want sess-parent", got)
		}

		first := decodeLedgerLine(t, lines[1])
		if got := first["event_type"]; got != "agent_message" {
			t.Fatalf("first event_type = %v, want agent_message", got)
		}
		if got := first["sequence"]; got != float64(1) {
			t.Fatalf("first sequence = %v, want 1", got)
		}

		events := readEvents(ctx, t, record)
		if len(events) != 2 {
			t.Fatalf("live event rows after materialization = %d, want 2", len(events))
		}
	})

	t.Run("Should skip idempotent rematerialization", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		record := createLedgerRecord(ctx, t, "sess-idempotent", "ws-primary")
		materializer := newTestMaterializer(t, root)

		first, err := materializer.Materialize(ctx, record)
		if err != nil {
			t.Fatalf("first Materialize() error = %v", err)
		}
		second, err := materializer.Materialize(ctx, record)
		if err != nil {
			t.Fatalf("second Materialize() error = %v", err)
		}
		if !first.Written {
			t.Fatal("first Materialize().Written = false, want true")
		}
		if second.Written {
			t.Fatal("second Materialize().Written = true, want false")
		}
		if second.Checksum != first.Checksum {
			t.Fatalf("second checksum = %q, want %q", second.Checksum, first.Checksum)
		}
	})

	t.Run("Should protect existing ledger with different checksum", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		record := createLedgerRecord(ctx, t, "sess-existing", "ws-primary")
		materializer := newTestMaterializer(t, root)
		path, err := materializer.Path(record)
		if err != nil {
			t.Fatalf("Path() error = %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte("tampered\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}

		if _, err := materializer.Materialize(ctx, record); !errors.Is(err, ErrLedgerExists) {
			t.Fatalf("Materialize() error = %v, want ErrLedgerExists", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		if string(content) != "tampered\n" {
			t.Fatalf("ledger content after failed materialization = %q, want tampered", string(content))
		}
	})

	t.Run("Should place unbound session in unbound partition", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		record := createLedgerRecord(ctx, t, "sess-unbound", "")
		materializer := newTestMaterializer(t, root)

		result, err := materializer.Materialize(ctx, record)
		if err != nil {
			t.Fatalf("Materialize() error = %v", err)
		}
		wantPath := filepath.Join(root, DefaultUnboundPartition, "sess-unbound", "ledger.jsonl")
		if result.Path != wantPath {
			t.Fatalf("Materialize().Path = %q, want %q", result.Path, wantPath)
		}
		meta := decodeLedgerLine(t, readLedgerLines(t, result.Path)[0])
		if got := meta["workspace_id"]; got != DefaultUnboundPartition {
			t.Fatalf("meta workspace_id = %v, want %s", got, DefaultUnboundPartition)
		}
	})

	t.Run("Should implement session lifecycle materializer seam", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		record := createLedgerRecord(ctx, t, "sess-seam", "ws-primary")
		materializer := newTestMaterializer(t, root)

		if err := materializer.MaterializeSessionLedger(ctx, record); err != nil {
			t.Fatalf("MaterializeSessionLedger() error = %v", err)
		}
		path, err := materializer.Path(record)
		if err != nil {
			t.Fatalf("Path() error = %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
	})

	t.Run("Should derive path without requiring an events database path", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		materializer := newTestMaterializer(t, root)
		record := store.SessionLedgerRecord{
			SessionID:   "sess-path-only",
			WorkspaceID: "ws-primary",
		}

		path, err := materializer.Path(record)
		if err != nil {
			t.Fatalf("Path() error = %v", err)
		}
		wantPath := filepath.Join(root, "ws-primary", "sess-path-only", "ledger.jsonl")
		if path != wantPath {
			t.Fatalf("Path() = %q, want %q", path, wantPath)
		}
		if _, err := materializer.Materialize(ctx, record); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("Materialize(missing events db path) error = %v, want ErrInvalidRecord", err)
		}
	})

	t.Run("Should fail corrupt event stores without replacing source evidence", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		eventsDBPath := filepath.Join(t.TempDir(), "events.db")
		original := []byte("not a sqlite database\n")
		if err := os.WriteFile(eventsDBPath, original, 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", eventsDBPath, err)
		}
		materializer := newTestMaterializer(t, root)
		record := store.SessionLedgerRecord{
			SessionID:    "sess-corrupt",
			WorkspaceID:  "ws-primary",
			EventsDBPath: eventsDBPath,
		}
		ledgerPath, err := materializer.Path(record)
		if err != nil {
			t.Fatalf("Path() error = %v", err)
		}

		if _, err := materializer.Materialize(ctx, record); err == nil {
			t.Fatal("Materialize(corrupt events db) error = nil, want failure")
		}
		after, err := os.ReadFile(eventsDBPath)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", eventsDBPath, err)
		}
		if !bytes.Equal(after, original) {
			t.Fatalf("events.db content after failed materialization = %q, want original corrupt bytes", string(after))
		}
		if matches, err := filepath.Glob(eventsDBPath + ".corrupt.*"); err != nil || len(matches) != 0 {
			t.Fatalf("corrupt recovery files = %v, err = %v; want none", matches, err)
		}
		if _, err := os.Stat(ledgerPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", ledgerPath, err)
		}
	})

	t.Run("Should enforce session stream validation before materialization", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			statements []string
			want       error
		}{
			{
				name: "Should refuse a legacy event store",
				statements: []string{
					`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`,
				},
				want: store.ErrLegacyDatabase,
			},
			{
				name: "Should refuse an ahead event store",
				statements: []string{
					`CREATE TABLE goose_db_version_session (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						version_id INTEGER NOT NULL,
						is_applied INTEGER NOT NULL,
						tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
					)`,
					`INSERT INTO goose_db_version_session (version_id, is_applied) VALUES (99, 1)`,
				},
				want: store.ErrSchemaAhead,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				eventsDBPath := filepath.Join(t.TempDir(), "events.db")
				seedLedgerMigrationState(t, eventsDBPath, tt.statements...)
				before, err := os.ReadFile(eventsDBPath)
				if err != nil {
					t.Fatalf("ReadFile(before %q) error = %v", eventsDBPath, err)
				}
				materializer := newTestMaterializer(t, t.TempDir())
				record := store.SessionLedgerRecord{
					SessionID:    "sess-stream-validation",
					WorkspaceID:  "ws-primary",
					EventsDBPath: eventsDBPath,
				}
				ledgerPath, err := materializer.Path(record)
				if err != nil {
					t.Fatalf("Path() error = %v", err)
				}

				_, err = materializer.Materialize(ctx, record)
				if !errors.Is(err, tt.want) {
					t.Fatalf("Materialize() error = %v, want %v", err, tt.want)
				}
				after, err := os.ReadFile(eventsDBPath)
				if err != nil {
					t.Fatalf("ReadFile(after %q) error = %v", eventsDBPath, err)
				}
				if !bytes.Equal(after, before) {
					t.Fatal("events.db bytes changed during refused materialization")
				}
				if _, err := os.Stat(ledgerPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", ledgerPath, err)
				}
			})
		}
	})

	t.Run("Should preserve legacy raw event payloads while materializing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		eventsDBPath := createLegacyRawEventDB(ctx, t)
		before, err := os.ReadFile(eventsDBPath)
		if err != nil {
			t.Fatalf("ReadFile(before %q) error = %v", eventsDBPath, err)
		}
		materializer := newTestMaterializer(t, root)
		record := store.SessionLedgerRecord{
			SessionID:    "sess-legacy-raw",
			WorkspaceID:  "ws-primary",
			EventsDBPath: eventsDBPath,
		}

		result, err := materializer.Materialize(ctx, record)
		if err != nil {
			t.Fatalf("Materialize(legacy raw events db) error = %v", err)
		}
		if result.Events != 1 {
			t.Fatalf("Materialize().Events = %d, want 1", result.Events)
		}
		after, err := os.ReadFile(eventsDBPath)
		if err != nil {
			t.Fatalf("ReadFile(after %q) error = %v", eventsDBPath, err)
		}
		if !bytes.Equal(after, before) {
			t.Fatal("events.db bytes changed during materialization, want read-only projection")
		}
		content := readEventContentDirect(ctx, t, eventsDBPath, "ev-raw")
		if !strings.Contains(content, `"raw"`) {
			t.Fatalf("legacy event content = %q, want raw payload preserved", content)
		}
	})

	t.Run("Should reject unsafe inputs before reading event store", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		if _, err := NewMaterializer(Config{}); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("NewMaterializer(empty) error = %v, want ErrInvalidRecord", err)
		}
		materializer := newTestMaterializer(t, t.TempDir())
		record := store.SessionLedgerRecord{
			SessionID:    "sess-invalid",
			WorkspaceID:  "ws-primary",
			EventsDBPath: filepath.Join(t.TempDir(), "events.db"),
		}
		var nilMaterializer *Materializer
		if _, err := nilMaterializer.Materialize(ctx, record); err == nil {
			t.Fatal("nil Materializer.Materialize() error = nil, want error")
		}
		unsafeSession := record
		unsafeSession.SessionID = "../escape"
		if _, err := materializer.Path(unsafeSession); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("Path(unsafe session) error = %v, want ErrInvalidRecord", err)
		}
		unsafeWorkspace := record
		unsafeWorkspace.WorkspaceID = "workspace/escape"
		if _, err := materializer.Path(unsafeWorkspace); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("Path(unsafe workspace) error = %v, want ErrInvalidRecord", err)
		}
		missingDBPath := record
		missingDBPath.EventsDBPath = ""
		if _, err := materializer.Materialize(ctx, missingDBPath); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("Materialize(missing db path) error = %v, want ErrInvalidRecord", err)
		}
	})

	t.Run("Should preserve both query and event-store close failures", func(t *testing.T) {
		t.Parallel()

		queryErr := errors.New("query failed")
		closeErr := errors.New("close failed")
		materializer, err := NewMaterializer(Config{
			RootDir: t.TempDir(),
			OpenEventStore: func(context.Context, string, string) (store.EventReadCloser, error) {
				return &ledgerFailureRecorder{queryErr: queryErr, closeErr: closeErr}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		_, err = materializer.Materialize(testutil.Context(t), store.SessionLedgerRecord{
			SessionID:    "sess-cleanup-failure",
			WorkspaceID:  "ws-cleanup-failure",
			EventsDBPath: filepath.Join(t.TempDir(), "events.db"),
		})
		if !errors.Is(err, queryErr) || !errors.Is(err, closeErr) {
			t.Fatalf("Materialize() error = %v, want query and close failures", err)
		}
	})
}

type ledgerFailureRecorder struct {
	queryErr error
	closeErr error
}

var _ store.EventReadCloser = (*ledgerFailureRecorder)(nil)

func (r *ledgerFailureRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	return nil, r.queryErr
}

func (r *ledgerFailureRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *ledgerFailureRecorder) Close(context.Context) error {
	return r.closeErr
}

func newTestMaterializer(t *testing.T, root string) *Materializer {
	t.Helper()

	materializer, err := NewMaterializer(Config{RootDir: root})
	if err != nil {
		t.Fatalf("NewMaterializer() error = %v", err)
	}
	return materializer
}

func seedLedgerMigrationState(t *testing.T, path string, statements ...string) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(%q) error = %v", path, err)
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(testutil.Context(t), statement); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				t.Fatalf("seed migration state error = %v; close error = %v", err, closeErr)
			}
			t.Fatalf("seed migration state error = %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(seed migration state) error = %v", err)
	}
}

func createLedgerRecord(
	ctx context.Context,
	t *testing.T,
	sessionID string,
	workspaceID string,
) store.SessionLedgerRecord {
	t.Helper()

	eventsDBPath := filepath.Join(t.TempDir(), "events.db")
	recorder, err := sessiondb.OpenSessionDB(ctx, sessionID, eventsDBPath)
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}
	started := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	ended := started.Add(2 * time.Minute)
	if _, err := recorder.RecordPersisted(ctx, store.SessionEvent{
		ID:        "ev-one",
		TurnID:    "turn-one",
		Type:      "agent_message",
		AgentName: "coder",
		Content:   `{"text":"hello"}`,
		Timestamp: started.Add(time.Second),
	}); err != nil {
		t.Fatalf("RecordPersisted(first) error = %v", err)
	}
	if _, err := recorder.RecordPersisted(ctx, store.SessionEvent{
		ID:        "ev-two",
		TurnID:    "turn-two",
		Type:      "tool_result",
		AgentName: "coder",
		Content:   "plain text fallback",
		Timestamp: started.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("RecordPersisted(second) error = %v", err)
	}
	if err := recorder.Close(ctx); err != nil {
		t.Fatalf("Close(recorder) error = %v", err)
	}

	return store.SessionLedgerRecord{
		SessionID:    sessionID,
		WorkspaceID:  workspaceID,
		AgentName:    "coder",
		SessionType:  "user",
		EventsDBPath: eventsDBPath,
		Lineage: &store.SessionLineage{
			ParentSessionID: "sess-parent",
			RootSessionID:   "sess-parent",
			SpawnDepth:      1,
		},
		StartedAt: started,
		EndedAt:   ended,
	}
}

func createLegacyRawEventDB(ctx context.Context, t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "events.db")
	legacyContent := `{"schema":"agh.session.event.v1","type":"tool_call","raw":{"content":"preserve"}}`
	recorder, err := sessiondb.OpenSessionDB(ctx, "sess-legacy-raw", path)
	if err != nil {
		t.Fatalf("OpenSessionDB(legacy raw) error = %v", err)
	}
	if _, err := recorder.RecordPersisted(ctx, store.SessionEvent{
		ID:        "ev-raw",
		TurnID:    "turn-raw",
		Type:      "tool_call",
		AgentName: "coder",
		Content:   legacyContent,
		Timestamp: time.Date(2026, 5, 5, 12, 0, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordPersisted(legacy raw) error = %v", err)
	}
	if err := recorder.Close(ctx); err != nil {
		t.Fatalf("Close(legacy raw recorder) error = %v", err)
	}
	return path
}

func readEventContentDirect(ctx context.Context, t *testing.T, path string, eventID string) string {
	t.Helper()

	db, err := sql.Open("sqlite", readOnlyTestSQLiteDSN(path))
	if err != nil {
		t.Fatalf("sql.Open(read-only %q) error = %v", path, err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("read-only db Close() error = %v", err)
		}
	}()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("PingContext(read-only %q) error = %v", path, err)
	}
	var content string
	if err := db.QueryRowContext(ctx, `SELECT content FROM events WHERE id = ?`, eventID).Scan(&content); err != nil {
		t.Fatalf("QueryRowContext(%q) error = %v", eventID, err)
	}
	return content
}

func readOnlyTestSQLiteDSN(path string) string {
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}
	query := u.Query()
	query.Set("mode", "ro")
	u.RawQuery = query.Encode()
	return u.String()
}

func readLedgerLines(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("ledger %q is empty", path)
	}
	return lines
}

func decodeLedgerLine(t *testing.T, line string) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", line, err)
	}
	return payload
}

func readEvents(
	ctx context.Context,
	t *testing.T,
	record store.SessionLedgerRecord,
) []store.SessionEvent {
	t.Helper()

	recorder, err := sessiondb.OpenSessionDB(ctx, record.SessionID, record.EventsDBPath)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if err := recorder.Close(ctx); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()
	events, err := recorder.Query(ctx, store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	return events
}

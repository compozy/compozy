package sessiondb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
	sessionschema "github.com/compozy/compozy/internal/store/sessiondb/schema"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
	"modernc.org/sqlite"
)

type SessionEvent = store.SessionEvent
type TokenUsage = store.TokenUsage
type EventQuery = store.EventQuery

const SessionDatabaseName = store.SessionDatabaseName

const testSessionDBWorkspaceID = "ws-sessiondb-test"

const testSessionDBOwnerTableDDL = `CREATE TABLE session_db_owner (
	singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
	session_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL
)`

func testSessionDBOwner(sessionID string) store.SessionDBOwner {
	return store.SessionDBOwner{SessionID: sessionID, WorkspaceID: testSessionDBWorkspaceID}
}

func TestOpenSessionDBCreatesSchemaAndEnablesWAL(t *testing.T) {
	t.Parallel()

	t.Run("Should create schema and enable WAL", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-open")

		assertTablesPresent(
			t,
			sessionDB.db,
			sessionMigrationVersionTable,
			"session_db_owner",
			"session_db_identity",
			"events",
			"token_usage",
			"transcript_projection_state",
			"transcript_entries",
			"transcript_tool_routes",
			"conversation_rewind_state",
			"conversation_rewind_receipts",
		)
		assertUniqueIndex(t, sessionDB.db, "events", "idx_events_sequence")
		assertJournalModeWAL(t, sessionDB.db)
		assertSynchronousNormal(t, sessionDB.db)
		if got, err := currentMaxSequence(testutil.Context(t), sessionDB.db); err != nil || got != 0 {
			t.Fatalf("currentMaxSequence() = (%d, %v), want (0, nil)", got, err)
		}
	})

	t.Run("Should close the opened database when the parent directory close fails", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("injected parent close failure")
		databaseCloseErr := errors.New("injected database close failure")
		databaseCloseCount := 0
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		db, err := openOwnedSessionSQLiteWithDirectory(
			testutil.Context(t),
			testSessionDBOwner("sess-parent-close-failure"),
			path,
			vacuumSessionSQLite,
			func(path string, perm os.FileMode) (sessionDBParentDirectory, string, error) {
				directory, name, openErr := fileutil.OpenOrCreateParentDirectory(path, perm)
				if openErr != nil {
					return nil, "", openErr
				}
				return &closeErrorSessionDBParentDirectory{
					Directory: directory,
					err:       closeErr,
				}, name, nil
			},
			func(db *sql.DB) error {
				databaseCloseCount++
				return errors.Join(db.Close(), databaseCloseErr)
			},
		)
		if !errors.Is(err, closeErr) {
			t.Fatalf("openOwnedSessionSQLiteWithDirectory() error = %v, want parent close failure", err)
		}
		if !errors.Is(err, databaseCloseErr) {
			t.Fatalf("openOwnedSessionSQLiteWithDirectory() error = %v, want database close failure", err)
		}
		if databaseCloseCount != 1 {
			t.Fatalf("database close count = %d, want 1", databaseCloseCount)
		}
		if db != nil {
			if dbCloseErr := db.Close(); dbCloseErr != nil {
				t.Errorf("Close(leaked database) error = %v", dbCloseErr)
			}
			t.Fatal("openOwnedSessionSQLiteWithDirectory() database != nil after parent close failure")
		}
	})

	t.Run("Should hold the family lease throughout fresh database publication", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		owner := testSessionDBOwner("sess-fresh-publication-lease")
		parentOpening := make(chan struct{})
		allowParentOpen := make(chan struct{})
		releaseParentOpen := sync.OnceFunc(func() { close(allowParentOpen) })
		t.Cleanup(releaseParentOpen)
		type openResult struct {
			db  *sql.DB
			err error
		}
		openDone := make(chan openResult, 1)
		ctx := testutil.Context(t)
		go func() {
			db, err := openOwnedSessionSQLiteWithDirectory(
				ctx,
				owner,
				path,
				vacuumSessionSQLite,
				func(path string, perm os.FileMode) (sessionDBParentDirectory, string, error) {
					close(parentOpening)
					select {
					case <-allowParentOpen:
						return openSessionDBParentDirectory(path, perm)
					case <-ctx.Done():
						return nil, "", ctx.Err()
					}
				},
				func(db *sql.DB) error { return db.Close() },
			)
			openDone <- openResult{db: db, err: err}
		}()

		select {
		case <-parentOpening:
		case <-ctx.Done():
			t.Fatalf("fresh database opener did not reach parent publication boundary: %v", ctx.Err())
		}
		contenderCtx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		contender, err := AcquireFamilyLease(contenderCtx, path)
		if contender != nil {
			contender.Release()
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("AcquireFamilyLease(during publication) error = %v, want deadline exceeded", err)
		}

		releaseParentOpen()
		var opened openResult
		select {
		case opened = <-openDone:
		case <-ctx.Done():
			t.Fatalf("fresh database opener did not complete: %v", ctx.Err())
		}
		if opened.err != nil {
			t.Fatalf("openOwnedSessionSQLiteWithDirectory() error = %v", opened.err)
		}
		if opened.db == nil {
			t.Fatal("openOwnedSessionSQLiteWithDirectory() database = nil")
		}
		if err := opened.db.Close(); err != nil {
			t.Fatalf("sql.DB.Close() error = %v", err)
		}

		lease, err := AcquireFamilyLease(ctx, path)
		if err != nil {
			t.Fatalf("AcquireFamilyLease(after publication) error = %v", err)
		}
		lease.Release()
	})
}

func TestOpenSessionDBDisablesAutomaticWALCheckpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should disable sqlite autocheckpoint for writer-owned WAL policy", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-wal-checkpoint")

		assertWALAutoCheckpoint(t, sessionDB.db, 0)
	})
}

func TestSessionDBPassiveCheckpoint(t *testing.T) {
	t.Parallel()

	t.Run("Should keep live read-only openers complete after passive checkpoints", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		sessionDB, err := OpenSessionDB(ctx, testSessionDBOwner("sess-passive-checkpoint"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := sessionDB.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		for idx := range sessionPassiveCheckpointEvery + 5 {
			if err := sessionDB.Record(ctx, SessionEvent{
				TurnID:    "turn-checkpoint",
				Type:      "agent_message",
				AgentName: "coder",
				Content:   fmt.Sprintf(`{"text":"chunk-%d"}`, idx),
			}); err != nil {
				t.Fatalf("Record(%d) error = %v", idx, err)
			}
		}

		readOnly, err := OpenSessionDBReadOnly(ctx, testSessionDBOwner("sess-passive-checkpoint"), path)
		if err != nil {
			t.Fatalf("OpenSessionDBReadOnly() error = %v", err)
		}
		t.Cleanup(func() {
			if err := readOnly.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(readOnly) error = %v", err)
			}
		})
		events, err := readOnly.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query(readOnly) error = %v", err)
		}
		if got, want := len(events), sessionPassiveCheckpointEvery+5; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}
	})
}

func TestSessionDBAppendEventIfAbsent(t *testing.T) {
	t.Parallel()

	t.Run("Should return an identical existing event and reject an ID collision", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-idempotent-event")
		event := SessionEvent{
			ID: "goal-snapshot:event-1", TurnID: "goal-snapshot:sess-idempotent-event",
			Type: "goal_snapshot_changed", AgentName: "system",
			Content:   `{"session_id":"sess-idempotent-event","revision":1}`,
			Timestamp: time.Date(2026, 7, 10, 18, 30, 0, 0, time.UTC),
		}
		first, err := sessionDB.AppendEventIfAbsent(ctx, event)
		if err != nil {
			t.Fatalf("AppendEventIfAbsent(first) error = %v", err)
		}
		second, err := sessionDB.AppendEventIfAbsent(ctx, event)
		if err != nil {
			t.Fatalf("AppendEventIfAbsent(identical) error = %v", err)
		}
		if first.ID != second.ID || first.Sequence != 1 || second.Sequence != 1 ||
			first.Content != second.Content || !first.Timestamp.Equal(second.Timestamp) {
			t.Fatalf("idempotent events = %#v / %#v", first, second)
		}
		collision := event
		collision.Content = `{"session_id":"sess-idempotent-event","revision":2}`
		if _, err := sessionDB.AppendEventIfAbsent(ctx, collision); !errors.Is(err, ErrEventIdentityCollision) {
			t.Fatalf("AppendEventIfAbsent(collision) error = %v, want %v", err, ErrEventIdentityCollision)
		}
		stored, err := sessionDB.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(stored) != 1 || stored[0].Content != event.Content {
			t.Fatalf("stored events = %#v, want one original event", stored)
		}
	})
}

func TestSessionDBArchivesEventRangesWithoutDeletingHistory(t *testing.T) {
	t.Parallel()

	t.Run("Should archive one idempotent range while preserving history", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-archive-range")
		for index, turnID := range []string{"turn-1", "turn-1", "turn-2"} {
			if err := sessionDB.Record(ctx, SessionEvent{
				TurnID:    turnID,
				Type:      acp.EventTypeDone,
				AgentName: "coder",
				Content:   fmt.Sprintf(`{"index":%d}`, index),
			}); err != nil {
				t.Fatalf("Record(%d) error = %v", index, err)
			}
		}

		result, err := sessionDB.ArchiveEvents(ctx, store.EventArchiveRequest{
			FromSequence: 1,
			ToSequence:   2,
		})
		if err != nil {
			t.Fatalf("ArchiveEvents() error = %v", err)
		}
		if result.ArchivedCount != 2 {
			t.Fatalf("ArchiveEvents().ArchivedCount = %d, want 2", result.ArchivedCount)
		}
		repeated, err := sessionDB.ArchiveEvents(ctx, store.EventArchiveRequest{
			FromSequence: 1,
			ToSequence:   2,
		})
		if err != nil {
			t.Fatalf("ArchiveEvents(repeated) error = %v", err)
		}
		if repeated.ArchivedCount != 0 {
			t.Fatalf("ArchiveEvents(repeated).ArchivedCount = %d, want 0", repeated.ArchivedCount)
		}

		all, err := sessionDB.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query(all) error = %v", err)
		}
		if len(all) != 3 || !all[0].Archived || !all[1].Archived || all[2].Archived {
			t.Fatalf("Query(all) = %#v, want two archived rows and one live row", all)
		}
		live, err := sessionDB.Query(ctx, EventQuery{Archive: store.EventArchiveUnarchived})
		if err != nil {
			t.Fatalf("Query(unarchived) error = %v", err)
		}
		if len(live) != 1 || live[0].Sequence != 3 {
			t.Fatalf("Query(unarchived) = %#v, want only sequence 3", live)
		}
		archived, err := sessionDB.Query(ctx, EventQuery{Archive: store.EventArchiveArchived})
		if err != nil {
			t.Fatalf("Query(archived) error = %v", err)
		}
		if len(archived) != 2 || archived[0].Sequence != 1 || archived[1].Sequence != 2 {
			t.Fatalf("Query(archived) = %#v, want sequences 1 and 2", archived)
		}
		history, err := sessionDB.History(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("History(all) error = %v", err)
		}
		if len(history) != 2 || len(history[0].Events) != 2 || len(history[1].Events) != 1 {
			t.Fatalf("History(all) = %#v, want every archived and live row", history)
		}
	})
}

func TestSessionDBRecordPersistedBatchCoalescesPromptChunks(t *testing.T) {
	t.Parallel()

	t.Run("Should persist contiguous same-turn text chunks as one event row", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-coalesce-contiguous")
		now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

		persisted, err := sessionDB.RecordPersistedBatch(ctx, []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "acp-coalesce",
				TurnID:    "turn-coalesce",
				Timestamp: now,
				Text:      "hello ",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "acp-coalesce",
				TurnID:    "turn-coalesce",
				Timestamp: now.Add(time.Millisecond),
				Text:      "world",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeDone,
				SessionID: "acp-coalesce",
				TurnID:    "turn-coalesce",
				Timestamp: now.Add(2 * time.Millisecond),
			}, "coder"),
		})
		if err != nil {
			t.Fatalf("RecordPersistedBatch() error = %v", err)
		}
		if got, want := len(persisted), 2; got != want {
			t.Fatalf("len(persisted) = %d, want %d", got, want)
		}
		assertEventSequences(t, persisted, []int64{1, 2})
		if got, want := storedAgentText(t, persisted[0]), "hello world"; got != want {
			t.Fatalf("coalesced text = %q, want %q", got, want)
		}

		stored, err := sessionDB.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if got, want := len(stored), 2; got != want {
			t.Fatalf("len(stored) = %d, want %d", got, want)
		}
		assertEventSequences(t, stored, []int64{1, 2})
	})

	t.Run("Should flush coalesced text when a non chunk boundary appears", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-coalesce-boundary")
		now := time.Date(2026, 7, 7, 12, 1, 0, 0, time.UTC)
		persisted, err := sessionDB.RecordPersistedBatch(ctx, []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "acp-boundary",
				TurnID:    "turn-boundary",
				Timestamp: now,
				Text:      "a",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "acp-boundary",
				TurnID:    "turn-boundary",
				Timestamp: now.Add(time.Millisecond),
				Text:      "b",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "acp-boundary",
				TurnID:     "turn-boundary",
				Timestamp:  now.Add(2 * time.Millisecond),
				ToolCallID: "call-1",
				Title:      "Read",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "acp-boundary",
				TurnID:    "turn-boundary",
				Timestamp: now.Add(3 * time.Millisecond),
				Text:      "c",
			}, "coder"),
		})
		if err != nil {
			t.Fatalf("RecordPersistedBatch() error = %v", err)
		}
		if got, want := len(persisted), 3; got != want {
			t.Fatalf("len(persisted) = %d, want %d", got, want)
		}
		assertEventSequences(t, persisted, []int64{1, 2, 3})
		if got, want := storedAgentText(t, persisted[0]), "ab"; got != want {
			t.Fatalf("first coalesced text = %q, want %q", got, want)
		}
		if got, want := persisted[1].Type, acp.EventTypeToolCall; got != want {
			t.Fatalf("boundary type = %q, want %q", got, want)
		}
		if got, want := storedAgentText(t, persisted[2]), "c"; got != want {
			t.Fatalf("second text = %q, want %q", got, want)
		}
	})

	t.Run("Should keep transcript output equal to uncoalesced chunks", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-coalesce-transcript")
		now := time.Date(2026, 7, 7, 12, 2, 0, 0, time.UTC)
		uncoalesced := []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "acp-transcript",
				TurnID:    "turn-transcript",
				Timestamp: now,
				Text:      "same ",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "acp-transcript",
				TurnID:    "turn-transcript",
				Timestamp: now.Add(time.Millisecond),
				Text:      "answer",
			}, "coder"),
		}
		for idx := range uncoalesced {
			uncoalesced[idx].Sequence = int64(idx + 1)
		}

		persisted, err := sessionDB.RecordPersistedBatch(ctx, uncoalesced)
		if err != nil {
			t.Fatalf("RecordPersistedBatch() error = %v", err)
		}
		uncoalescedEntries, err := transcript.ToUIEntries(uncoalesced)
		if err != nil {
			t.Fatalf("ToUIEntries(uncoalesced) error = %v", err)
		}
		coalescedEntries, err := transcript.ToUIEntries(persisted)
		if err != nil {
			t.Fatalf("ToUIEntries(coalesced) error = %v", err)
		}
		if got, want := lastEntryText(t, coalescedEntries), lastEntryText(t, uncoalescedEntries); got != want {
			t.Fatalf("coalesced transcript text = %q, want %q", got, want)
		}
	})
}

func TestOpenSessionDBAppliesBaselineAndRepeatedBootIsIdempotent(t *testing.T) {
	t.Parallel()

	t.Run("Should validate and normalize the complete owner before creating files", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			owner     store.SessionDBOwner
			wantOwner store.SessionDBOwner
			wantErr   bool
		}{
			{
				name:    "Should reject an empty session id",
				owner:   store.SessionDBOwner{WorkspaceID: testSessionDBWorkspaceID},
				wantErr: true,
			},
			{
				name:    "Should reject a whitespace workspace id",
				owner:   store.SessionDBOwner{SessionID: "sess-invalid-workspace", WorkspaceID: " \t "},
				wantErr: true,
			},
			{
				name:      "Should persist the canonical form of a padded owner",
				owner:     store.SessionDBOwner{SessionID: "  sess-padded-owner  ", WorkspaceID: "  ws-padded-owner  "},
				wantOwner: store.SessionDBOwner{SessionID: "sess-padded-owner", WorkspaceID: "ws-padded-owner"},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				directory := t.TempDir()
				path := filepath.Join(directory, SessionDatabaseName)
				db, err := OpenSessionDB(ctx, test.owner, path)
				if test.wantErr {
					if err == nil {
						if closeErr := db.Close(ctx); closeErr != nil {
							t.Errorf("Close(unexpected database) error = %v", closeErr)
						}
						t.Fatal("OpenSessionDB(invalid owner) error = nil, want validation failure")
					}
					entries, readErr := os.ReadDir(directory)
					if readErr != nil {
						t.Fatalf("os.ReadDir(database directory) error = %v", readErr)
					}
					if len(entries) != 0 {
						t.Fatalf("database directory entries = %v, want no created family", entries)
					}
					return
				}
				if err != nil {
					t.Fatalf("OpenSessionDB(padded owner) error = %v", err)
				}
				if err := db.Close(ctx); err != nil {
					t.Fatalf("Close(padded owner) error = %v", err)
				}
				reopened, err := OpenSessionDB(ctx, test.wantOwner, path)
				if err != nil {
					t.Fatalf("OpenSessionDB(canonical owner) error = %v", err)
				}
				if err := reopened.Close(ctx); err != nil {
					t.Fatalf("Close(canonical owner) error = %v", err)
				}
			})
		}
	})

	t.Run("Should round-trip an event and preserve stream status across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		first, err := OpenSessionDB(ctx, testSessionDBOwner("sess-idempotent"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB(first) error = %v", err)
		}
		firstStatus, err := store.Status(ctx, first.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(first) error = %v", err)
		}
		if firstStatus.Version != 6 || firstStatus.AppliedCount != 6 {
			t.Fatalf("Status(first) = %#v, want version/applied count 6", firstStatus)
		}
		if err := verifySessionDBOwner(ctx, first.db, testSessionDBOwner("sess-idempotent")); err != nil {
			t.Fatalf("verifySessionDBOwner() error = %v", err)
		}
		var databaseID string
		if err := first.db.QueryRowContext(
			ctx,
			`SELECT database_id FROM session_db_identity WHERE singleton = 1`,
		).Scan(&databaseID); err != nil {
			t.Fatalf("query session database identity error = %v", err)
		}
		if len(databaseID) != 32 {
			t.Fatalf("session database identity length = %d, want 32", len(databaseID))
		}
		for operation, statement := range map[string]string{
			"update": `UPDATE session_db_identity SET database_id = lower(hex(randomblob(16))) WHERE singleton = 1`,
			"delete": `DELETE FROM session_db_identity WHERE singleton = 1`,
			"replace": `INSERT OR REPLACE INTO session_db_identity (singleton, database_id) ` +
				`VALUES (1, lower(hex(randomblob(16))))`,
		} {
			if _, err := first.db.ExecContext(ctx, statement); err == nil {
				t.Fatalf("%s session database identity error = nil, want immutable-identity rejection", operation)
			}
		}
		if _, err := first.db.ExecContext(
			ctx,
			`UPDATE session_db_owner SET workspace_id = 'ws-rebound' WHERE singleton = 1`,
		); err == nil {
			t.Fatal("UPDATE session_db_owner error = nil, want immutable-owner rejection")
		}
		if _, err := first.db.ExecContext(
			ctx,
			`DELETE FROM session_db_owner WHERE singleton = 1`,
		); err == nil {
			t.Fatal("DELETE session_db_owner error = nil, want immutable-owner rejection")
		}
		if _, err := first.db.ExecContext(
			ctx,
			`INSERT OR REPLACE INTO session_db_owner (singleton, session_id, workspace_id) VALUES (1, 'sess-rebound', 'ws-rebound')`,
		); err == nil {
			t.Fatal("INSERT OR REPLACE session_db_owner error = nil, want immutable-owner rejection")
		}
		if err := verifySessionDBOwner(ctx, first.db, testSessionDBOwner("sess-idempotent")); err != nil {
			t.Fatalf("verifySessionDBOwner(after rejected relabel) error = %v", err)
		}
		assertUniqueIndex(t, first.db, "events", "idx_events_sequence")
		event := SessionEvent{
			ID:        "event-before-reopen",
			TurnID:    "turn-before-reopen",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   `{"text":"survives reopen"}`,
			Timestamp: time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC),
		}
		if _, err := first.AppendEventIfAbsent(ctx, event); err != nil {
			t.Fatalf("AppendEventIfAbsent() error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second, err := OpenSessionDB(ctx, testSessionDBOwner("sess-idempotent"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(second) error = %v", err)
			}
		})
		secondStatus, err := store.Status(ctx, second.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(second) error = %v", err)
		}
		if secondStatus != firstStatus {
			t.Fatalf("Status(second) = %#v, want unchanged %#v", secondStatus, firstStatus)
		}
		var reopenedDatabaseID string
		if err := second.db.QueryRowContext(
			ctx,
			`SELECT database_id FROM session_db_identity WHERE singleton = 1`,
		).Scan(&reopenedDatabaseID); err != nil {
			t.Fatalf("query reopened session database identity error = %v", err)
		}
		if reopenedDatabaseID != databaseID {
			t.Fatalf("reopened session database identity = %q, want %q", reopenedDatabaseID, databaseID)
		}
		assertUniqueIndex(t, second.db, "events", "idx_events_sequence")
		events, err := second.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query(after reopen) error = %v", err)
		}
		if len(events) != 1 || events[0].ID != event.ID || events[0].Content != event.Content {
			t.Fatalf("events after reopen = %#v, want preserved event %#v", events, event)
		}
	})

	t.Run("Should refuse an ownerless previous stream before engine migration preserves its rows", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open() error = %v", err)
		}
		if err := store.Apply(ctx, db, previousSessionMigrationStream(t)); err != nil {
			t.Fatalf("Apply(previous session stream) error = %v", err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO events (
				id, sequence, turn_id, type, agent_name, content, timestamp, transcript_entry_key
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"event-before-archive-column",
			1,
			"turn-before-archive-column",
			acp.EventTypeDone,
			"coder",
			`{"type":"done"}`,
			store.FormatTimestamp(time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)),
			"",
		); err != nil {
			t.Fatalf("insert baseline event error = %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close(previous stream) error = %v", err)
		}
		before := readOnlySessionDatabaseFamilyDigest(t, path)

		_, err = OpenSessionDB(ctx, testSessionDBOwner("sess-prefix-upgrade"), path)
		if !errors.Is(err, ErrSessionDBOwnerMissing) {
			t.Fatalf("OpenSessionDB(ownerless previous stream) error = %v, want ErrSessionDBOwnerMissing", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before)

		migrationDB, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open(engine migration) error = %v", err)
		}
		registerTestSQLDBCleanup(t, "engine migration", migrationDB)
		if err := store.Apply(ctx, migrationDB, MigrationStream()); err != nil {
			t.Fatalf("Apply(current session stream) error = %v", err)
		}
		status, err := store.Status(ctx, migrationDB, MigrationStream())
		if err != nil {
			t.Fatalf("Status(engine-migrated) error = %v", err)
		}
		if status.Version != 6 || status.AppliedCount != 6 {
			t.Fatalf("Status(engine-migrated) = %#v, want version/applied count 6", status)
		}
		migratedOwner := testSessionDBOwner("sess-prefix-upgrade")
		if _, err := migrationDB.ExecContext(
			ctx,
			`INSERT INTO session_db_owner (singleton, session_id, workspace_id) VALUES (1, ?, ?)`,
			migratedOwner.SessionID,
			migratedOwner.WorkspaceID,
		); err != nil {
			t.Fatalf("insert engine-migrated owner error = %v", err)
		}
		if _, err := migrationDB.ExecContext(
			ctx,
			`INSERT OR REPLACE INTO session_db_owner (singleton, session_id, workspace_id) VALUES (1, 'foreign', 'foreign')`,
		); err == nil {
			t.Fatal("replace engine-migrated owner error = nil, want immutable-owner rejection")
		}
		if err := verifySessionDBOwner(ctx, migrationDB, migratedOwner); err != nil {
			t.Fatalf("verifySessionDBOwner(engine-migrated) error = %v", err)
		}
		var eventID string
		var archived bool
		if err := migrationDB.QueryRowContext(
			ctx,
			`SELECT id, archived FROM events WHERE id = ?`,
			"event-before-archive-column",
		).Scan(&eventID, &archived); err != nil {
			t.Fatalf("query engine-migrated event error = %v", err)
		}
		if eventID != "event-before-archive-column" || archived {
			t.Fatalf("engine-migrated event = {%q %t}, want preserved unarchived row", eventID, archived)
		}
	})

	// Invariant: migration 00005 assigns one immutable physical identity to a
	// v4 SessionDB while preserving its owner and event rows across production reopen.
	// Owning layer: SessionDB migration stream. Canonical suite: session_db_test.go.
	t.Run("Should assign an immutable physical identity to the previous SessionDB head", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		owner := testSessionDBOwner("sess-identity-migration")
		prefixDB, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open(v4 prefix) error = %v", err)
		}
		if err := store.Apply(ctx, prefixDB, sessionMigrationPrefixBefore(t, "00005_schema.sql")); err != nil {
			t.Fatalf("Apply(v4 prefix) error = %v", err)
		}
		if _, err := prefixDB.ExecContext(
			ctx,
			`INSERT INTO session_db_owner (singleton, session_id, workspace_id) VALUES (1, ?, ?)`,
			owner.SessionID,
			owner.WorkspaceID,
		); err != nil {
			t.Fatalf("insert v4 owner error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO events (
			id, sequence, turn_id, type, agent_name, content, timestamp, transcript_entry_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"event-before-identity", 1, "turn-before-identity", acp.EventTypeDone, "coder",
			`{"type":"done"}`, store.FormatTimestamp(time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)), "",
		); err != nil {
			t.Fatalf("insert v4 event error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("Close(v4 prefix) error = %v", err)
		}

		upgraded, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatalf("OpenSessionDB(v4 upgrade) error = %v", err)
		}
		var databaseID string
		if err := upgraded.db.QueryRowContext(
			ctx,
			`SELECT database_id FROM session_db_identity WHERE singleton = 1`,
		).Scan(&databaseID); err != nil {
			t.Fatalf("query migrated database identity error = %v", err)
		}
		if len(databaseID) != 32 {
			t.Fatalf("migrated database identity length = %d, want 32", len(databaseID))
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("Close(upgraded) error = %v", err)
		}

		reopened, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatalf("OpenSessionDB(reopen upgraded) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened upgraded) error = %v", err)
			}
		})
		events, err := reopened.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query(reopened upgraded) error = %v", err)
		}
		if len(events) != 1 || events[0].ID != "event-before-identity" {
			t.Fatalf("reopened upgraded events = %#v, want preserved v4 event", events)
		}
		var reopenedID string
		if err := reopened.db.QueryRowContext(
			ctx,
			`SELECT database_id FROM session_db_identity WHERE singleton = 1`,
		).Scan(&reopenedID); err != nil {
			t.Fatalf("query reopened database identity error = %v", err)
		}
		if reopenedID != databaseID {
			t.Fatalf("reopened database identity = %q, want %q", reopenedID, databaseID)
		}
	})

	t.Run("Should publish exactly one identity to competing fresh owners", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		owners := []store.SessionDBOwner{
			testSessionDBOwner("sess-concurrent-fresh-owner-a"),
			{SessionID: "sess-concurrent-fresh-owner-b", WorkspaceID: "ws-concurrent-fresh-owner-b"},
		}
		type openResult struct {
			owner store.SessionDBOwner
			db    *SessionDB
			err   error
		}
		start := make(chan struct{})
		resultCh := make(chan openResult, 8)
		var openers sync.WaitGroup
		for idx := range 8 {
			owner := owners[idx%len(owners)]
			openers.Go(func() {
				<-start
				db, err := OpenSessionDB(ctx, owner, path)
				resultCh <- openResult{owner: owner, db: db, err: err}
			})
		}
		close(start)
		openers.Wait()
		close(resultCh)

		guardDB, err := sql.Open("sqlite", sessionDBOwnerGuardDSN(path))
		if err != nil {
			t.Fatalf("sql.Open(owner guard) error = %v", err)
		}
		var persistedOwner store.SessionDBOwner
		if err := guardDB.QueryRowContext(
			ctx,
			`SELECT session_id, workspace_id FROM session_db_owner WHERE singleton = 1`,
		).Scan(&persistedOwner.SessionID, &persistedOwner.WorkspaceID); err != nil {
			if closeErr := guardDB.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
			t.Fatalf("query persisted concurrent owner error = %v", err)
		}
		if err := guardDB.Close(); err != nil {
			t.Fatalf("Close(owner guard) error = %v", err)
		}

		for result := range resultCh {
			if result.owner == persistedOwner {
				if result.err != nil || result.db == nil {
					t.Errorf(
						"winning owner OpenSessionDB() = (%v, %v), want non-nil database and nil error",
						result.db,
						result.err,
					)
					continue
				}
				if err := result.db.Close(ctx); err != nil {
					t.Errorf("Close(winning owner) error = %v", err)
				}
				continue
			}
			if result.db != nil {
				if closeErr := result.db.Close(ctx); closeErr != nil {
					t.Errorf("Close(losing owner) error = %v", closeErr)
				}
				t.Errorf("losing owner OpenSessionDB() database = non-nil, want nil")
			}
			if !errors.Is(result.err, ErrSessionDBOwnerMismatch) {
				t.Errorf("losing owner OpenSessionDB() error = %v, want ErrSessionDBOwnerMismatch", result.err)
			}
		}

		reader, err := OpenSessionDBReadOnly(ctx, persistedOwner, path)
		if err != nil {
			t.Fatalf("OpenSessionDBReadOnly(concurrent result) error = %v", err)
		}
		if err := reader.Close(ctx); err != nil {
			t.Fatalf("Close(concurrent result) error = %v", err)
		}
	})
}

func TestOpenSessionDBRejectsDifferentOwnerIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		foreign store.SessionDBOwner
	}{
		{
			name:    "Should reject another session identity without changing files",
			foreign: testSessionDBOwner("sess-foreign"),
		},
		{
			name: "Should reject another workspace identity without changing files",
			foreign: store.SessionDBOwner{
				SessionID: "sess-owner", WorkspaceID: "ws-foreign",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			path := filepath.Join(t.TempDir(), SessionDatabaseName)
			owner, err := OpenSessionDB(ctx, testSessionDBOwner("sess-owner"), path)
			if err != nil {
				t.Fatalf("OpenSessionDB(owner) error = %v", err)
			}
			if err := owner.Close(ctx); err != nil {
				t.Fatalf("Close(owner) error = %v", err)
			}
			before := readOnlySessionDatabaseFamilyDigest(t, path)

			foreign, err := OpenSessionDB(ctx, test.foreign, path)
			if err == nil {
				if closeErr := foreign.Close(ctx); closeErr != nil {
					t.Fatalf("Close(foreign) error = %v", closeErr)
				}
				t.Fatal("OpenSessionDB(foreign) error = nil, want owner mismatch")
			}
			if !errors.Is(err, ErrSessionDBOwnerMismatch) {
				t.Fatalf("OpenSessionDB(foreign) error = %v, want ErrSessionDBOwnerMismatch", err)
			}
			assertReadOnlySessionDatabaseUnchanged(t, path, before)
		})
	}

	t.Run("Should not recreate an existing database removed after its owner probe", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		owner := testSessionDBOwner("sess-removed-after-owner-probe")
		db, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if err := verifySessionDBOwnerReadOnly(ctx, owner, path); err != nil {
			t.Fatalf("verifySessionDBOwnerReadOnly() error = %v", err)
		}
		for _, suffix := range []string{"-wal", "-shm", "-journal", ""} {
			if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Remove(%q) error = %v", path+suffix, err)
			}
		}

		reopened, err := openSessionSQLiteForExistingOwnerWithVacuum(ctx, owner, path, nil)
		if reopened != nil {
			if closeErr := reopened.Close(); closeErr != nil {
				t.Errorf("Close(reopened) error = %v", closeErr)
			}
			t.Fatal("openSessionSQLiteForExistingOwnerWithVacuum() database = non-nil, want nil")
		}
		if err == nil {
			t.Fatal("openSessionSQLiteForExistingOwnerWithVacuum() error = nil, want missing database failure")
		}
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(path after refused reopen) error = %v, want not exist", statErr)
		}
	})

	t.Run("Should guard a replacement before every writable physical connection", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		expectedDir := filepath.Join(root, "expected")
		foreignDir := filepath.Join(root, "foreign")
		expectedPath := filepath.Join(expectedDir, SessionDatabaseName)
		foreignPath := filepath.Join(foreignDir, SessionDatabaseName)
		expectedOwner := testSessionDBOwner("sess-writable-connection-owner")
		foreignOwner := store.SessionDBOwner{
			SessionID: "sess-writable-connection-foreign", WorkspaceID: "ws-writable-connection-foreign",
		}
		expectedDB, err := OpenSessionDB(ctx, expectedOwner, expectedPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(expected) error = %v", err)
		}
		t.Cleanup(func() {
			if err := expectedDB.Close(testutil.Context(t)); err != nil &&
				!errors.Is(err, ErrSessionDBOwnerMismatch) {
				t.Errorf("Close(expected) error = %v", err)
			}
		})
		foreignDB, err := OpenSessionDB(ctx, foreignOwner, foreignPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(foreign) error = %v", err)
		}
		if err := foreignDB.Record(ctx, SessionEvent{
			ID: "foreign-event", TurnID: "foreign-turn", Type: acp.EventTypeDone, AgentName: "coder",
		}); err != nil {
			t.Fatalf("Record(foreign) error = %v", err)
		}
		if err := foreignDB.Close(ctx); err != nil {
			t.Fatalf("Close(foreign) error = %v", err)
		}
		before := readOnlySessionDatabaseFamilyDigest(t, foreignPath)

		expectedDB.db.SetMaxIdleConns(0)
		if err := os.Rename(expectedDir, filepath.Join(root, "expected-before-substitution")); err != nil {
			t.Fatalf("os.Rename(expected backup) error = %v", err)
		}
		if err := os.Rename(foreignDir, expectedDir); err != nil {
			t.Fatalf("os.Rename(foreign into expected path) error = %v", err)
		}

		events, err := expectedDB.Query(ctx, EventQuery{})
		if !errors.Is(err, ErrSessionDBOwnerMismatch) {
			t.Fatalf("Query(substituted writable connection) error = %v, want ErrSessionDBOwnerMismatch", err)
		}
		if len(events) != 0 {
			t.Fatalf("Query(substituted writable connection) events = %#v, want empty", events)
		}
		assertReadOnlySessionDatabaseUnchanged(t, expectedPath, before)
	})
}

func previousSessionMigrationStream(t *testing.T) store.MigrationStream {
	t.Helper()

	const baselineAtlasSum = "h1:qmViNU8wfHEfvn27+89baAJQgC90AMtERpblLCkX/1I=\n" +
		"00001_baseline.sql h1:bXOy48LeBzy7PLWzCXQW/1CdBi+3LQtUySjgT8uUN0E=\n" +
		"00002_schema.sql h1:ROugExdfsKSTgcLoFHvFdIALygvM7xEEUpS8llYFXpw=\n"
	baseline, err := fs.ReadFile(sessionschema.Files, "migrations/00001_baseline.sql")
	if err != nil {
		t.Fatalf("read recorded session baseline: %v", err)
	}
	archiveMigration, err := fs.ReadFile(sessionschema.Files, "migrations/00002_schema.sql")
	if err != nil {
		t.Fatalf("read recorded session archive migration: %v", err)
	}
	stream := MigrationStream()
	stream.FS = fstest.MapFS{
		"00001_baseline.sql": &fstest.MapFile{Data: baseline},
		"00002_schema.sql":   &fstest.MapFile{Data: archiveMigration},
		"atlas.sum":          &fstest.MapFile{Data: []byte(baselineAtlasSum)},
	}
	stream.Dir = "."
	return stream
}

func sessionMigrationPrefixBefore(t *testing.T, excludedMigration string) store.MigrationStream {
	t.Helper()

	stream := MigrationStream()
	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatalf("read session migration directory: %v", err)
	}
	memoryDirectory := &atlasmigrate.MemDir{}
	files := fstest.MapFS{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") || name >= excludedMigration {
			continue
		}
		contents, err := fs.ReadFile(stream.FS, stream.Dir+"/"+name)
		if err != nil {
			t.Fatalf("read session migration %q: %v", name, err)
		}
		if err := memoryDirectory.WriteFile(name, contents); err != nil {
			t.Fatalf("write in-memory session migration %q: %v", name, err)
		}
		files[stream.Dir+"/"+name] = &fstest.MapFile{Data: append([]byte(nil), contents...)}
	}
	checksum, err := memoryDirectory.Checksum()
	if err != nil {
		t.Fatalf("checksum session migration prefix: %v", err)
	}
	checksumBytes, err := checksum.MarshalText()
	if err != nil {
		t.Fatalf("marshal session migration prefix checksum: %v", err)
	}
	files[stream.Dir+"/"+atlasmigrate.HashFileName] = &fstest.MapFile{Data: checksumBytes}
	stream.FS = files
	stream.Bootstrap = nil
	return stream
}

func TestSessionDBTranscriptProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should update one stable entry across streamed event writes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-stream")
		for index, text := range []string{"hel", "lo"} {
			if err := sessionDB.Record(ctx, SessionEvent{
				TurnID:    "turn-stream",
				Type:      acp.EventTypeAgentMessage,
				AgentName: "coder",
				Content:   fmt.Sprintf(`{"type":"agent_message","turn_id":"turn-stream","text":%q}`, text),
			}); err != nil {
				t.Fatalf("Record(%d) error = %v", index, err)
			}
		}

		page, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage() error = %v", err)
		}
		if got, want := len(page.Entries), 1; got != want {
			t.Fatalf("len(page.Entries) = %d, want %d", got, want)
		}
		entry := page.Entries[0]
		if got, want := entry.StartSequence, int64(1); got != want {
			t.Fatalf("StartSequence = %d, want %d", got, want)
		}
		if got, want := entry.Sequence, int64(2); got != want {
			t.Fatalf("Sequence = %d, want %d", got, want)
		}
		if got, want := transcript.UIMessageText(entry.Message), "hello"; got != want {
			t.Fatalf("streamed text = %q, want %q", got, want)
		}
	})

	t.Run("Should publish a completed assistant upsert with its boundary entry", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-boundary-change")
		persisted, err := sessionDB.RecordPersistedBatch(ctx, []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-transcript-boundary-change",
				TurnID:    "turn-first",
				Text:      "streamed answer",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeUserMessage,
				SessionID: "sess-transcript-boundary-change",
				TurnID:    "turn-next",
				Text:      "next question",
			}, "coder"),
		})
		if err != nil {
			t.Fatalf("RecordPersistedBatch() error = %v", err)
		}
		if got, want := len(persisted), 2; got != want {
			t.Fatalf("len(persisted) = %d, want %d", got, want)
		}

		changes, err := sessionDB.TranscriptChanges(ctx, transcript.ChangeQuery{
			AfterSequence: persisted[0].Sequence,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("TranscriptChanges() error = %v", err)
		}
		if got, want := len(changes.Entries), 2; got != want {
			t.Fatalf("len(changes.Entries) = %d, want completed upsert plus boundary", got)
		}
		assistant := changes.Entries[0]
		boundary := changes.Entries[1]
		if got, want := assistant.StartSequence, persisted[0].Sequence; got != want {
			t.Fatalf("assistant.StartSequence = %d, want stable %d", got, want)
		}
		if got, want := assistant.Sequence, persisted[1].Sequence; got != want {
			t.Fatalf("assistant.Sequence = %d, want boundary %d", got, want)
		}
		for index, part := range assistant.Message.Parts {
			if part.State != "done" {
				t.Fatalf("assistant part %d state = %q, want done", index, part.State)
			}
		}
		if got, want := boundary.StartSequence, persisted[1].Sequence; got != want {
			t.Fatalf("boundary.StartSequence = %d, want %d", got, want)
		}
		if boundary.Sequence != persisted[1].Sequence || changes.HasMore || changes.NextAfter != persisted[1].Sequence {
			t.Fatalf("boundary change page = %#v, want one complete cursor group", changes)
		}

		after, err := sessionDB.TranscriptChanges(ctx, transcript.ChangeQuery{
			AfterSequence: changes.NextAfter,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("TranscriptChanges(after boundary) error = %v", err)
		}
		if len(after.Entries) != 0 || after.NextAfter != changes.NextAfter {
			t.Fatalf("TranscriptChanges(after boundary) = %#v, want no duplicate or gap", after)
		}
	})

	t.Run("Should walk canonical entries without gaps across stable start cursors", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-pages")
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		marker, err := transcript.NewMarker(
			transcript.MarkerSessionUnhealthy,
			"Provider stopped responding",
			now.Add(3*time.Second),
			nil,
		)
		if err != nil {
			t.Fatalf("NewMarker() error = %v", err)
		}
		markerEvent, err := marker.AgentEvent("sess-transcript-pages", "turn-shared")
		if err != nil {
			t.Fatalf("AgentEvent(marker) error = %v", err)
		}

		input := []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeUserMessage,
				SessionID: "sess-transcript-pages",
				TurnID:    "turn-shared",
				Timestamp: now,
				Text:      "start",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-transcript-pages",
				TurnID:    "turn-shared",
				Timestamp: now.Add(time.Second),
				Text:      "before marker",
			}, "coder"),
			canonicalStoreEvent(t, markerEvent, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-transcript-pages",
				TurnID:    "turn-shared",
				Timestamp: now.Add(4 * time.Second),
				Text:      "after marker",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeUserMessage,
				SessionID: "sess-transcript-pages",
				TurnID:    "turn-next",
				Timestamp: now.Add(5 * time.Second),
				Text:      "next",
			}, "coder"),
		}
		if _, err := sessionDB.RecordPersistedBatch(ctx, input); err != nil {
			t.Fatalf("RecordPersistedBatch() error = %v", err)
		}

		storedEvents, err := sessionDB.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		want, err := transcript.ToUIEntries(storedEvents)
		if err != nil {
			t.Fatalf("ToUIEntries() error = %v", err)
		}

		got := make([]transcript.Entry, 0, len(want))
		before := int64(0)
		for {
			page, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{
				Limit:          2,
				BeforeSequence: before,
			})
			if err != nil {
				t.Fatalf("TranscriptPage(before=%d) error = %v", before, err)
			}
			got = append(page.Entries, got...)
			if !page.HasOlder {
				break
			}
			if page.NextBeforeSequence <= 0 || page.NextBeforeSequence == before {
				t.Fatalf("NextBeforeSequence = %d after %d, want progress", page.NextBeforeSequence, before)
			}
			before = page.NextBeforeSequence
		}

		assertTranscriptEntriesEqual(t, got, want)
		if got, want := transcript.UIMessageText(got[1].Message), "before marker"; got != want {
			t.Fatalf("first assistant segment text = %q, want %q", got, want)
		}
		if got, want := transcript.UIMessageText(got[3].Message), "after marker"; got != want {
			t.Fatalf("second assistant segment text = %q, want %q", got, want)
		}
		if got[1].Message.ID == got[3].Message.ID {
			t.Fatalf("assistant segment ids = %q, want distinct ids", got[1].Message.ID)
		}
	})

	t.Run("Should surface a repaired tool lifecycle through historical entry changes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-repair")
		now := time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC)
		initial, err := sessionDB.RecordPersistedBatch(ctx, []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-transcript-repair",
				TurnID:     "turn-old",
				Timestamp:  now,
				Title:      "Bash",
				ToolCallID: "call-old",
				Raw:        json.RawMessage(`{"rawInput":{"command":"pwd"}}`),
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeSyntheticReentry,
				SessionID: "sess-transcript-repair",
				TurnID:    "turn-new",
				Timestamp: now.Add(time.Second),
				Text:      "resume",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-transcript-repair",
				TurnID:    "turn-new",
				Timestamp: now.Add(2 * time.Second),
				Text:      "new work",
			}, "coder"),
		})
		if err != nil {
			t.Fatalf("RecordPersistedBatch(initial) error = %v", err)
		}
		beforeRepair, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage(before repair) error = %v", err)
		}
		if got, want := len(beforeRepair.Entries), 3; got != want {
			t.Fatalf("len(beforeRepair.Entries) = %d, want %d", got, want)
		}
		toolEntry := beforeRepair.Entries[0]

		repair, err := sessionDB.RecordPersistedBatch(ctx, []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type:       acp.EventTypeToolResult,
				SessionID:  "sess-transcript-repair",
				TurnID:     "turn-old",
				Timestamp:  now.Add(3 * time.Second),
				Title:      "Bash",
				ToolCallID: "call-old",
				Raw: json.RawMessage(
					`{"sessionUpdate":"tool_call_update","status":"completed",` +
						`"rawOutput":{"stdout":"workspace"},` +
						`"_meta":{"claudeCode":{"toolName":"Bash"}}}`,
				),
			}, "coder"),
		})
		if err != nil {
			t.Fatalf("RecordPersistedBatch(repair) error = %v", err)
		}
		if got, want := len(repair), 1; got != want {
			t.Fatalf("len(repair) = %d, want %d", got, want)
		}

		changes, err := sessionDB.TranscriptChanges(ctx, transcript.ChangeQuery{
			AfterSequence: initial[len(initial)-1].Sequence,
			Limit:         10,
		})
		if err != nil {
			t.Fatalf("TranscriptChanges() error = %v", err)
		}
		if got, want := len(changes.Entries), 1; got != want {
			t.Fatalf("len(changes.Entries) = %d, want %d", got, want)
		}
		changed := changes.Entries[0]
		if got, want := changed.Message.ID, toolEntry.Message.ID; got != want {
			t.Fatalf("changed message id = %q, want historical %q", got, want)
		}
		if got, want := changed.StartSequence, toolEntry.StartSequence; got != want {
			t.Fatalf("changed start sequence = %d, want immutable %d", got, want)
		}
		if got, want := changed.Sequence, repair[0].Sequence; got != want {
			t.Fatalf("changed updated sequence = %d, want %d", got, want)
		}
		assertTranscriptToolPart(t, changed, "call-old", "pwd", "output-available")
	})

	t.Run("Should avoid decoding an unselected corrupt history prefix", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-bounded")
		for index := range 6 {
			if err := sessionDB.Record(ctx, SessionEvent{
				TurnID:    fmt.Sprintf("turn-%d", index),
				Type:      acp.EventTypeUserMessage,
				AgentName: "coder",
				Content:   fmt.Sprintf(`{"type":"user_message","text":"message-%d"}`, index),
			}); err != nil {
				t.Fatalf("Record(%d) error = %v", index, err)
			}
		}
		if _, err := sessionDB.db.ExecContext(
			ctx,
			`UPDATE transcript_entries SET message_json = ? WHERE start_sequence = 1`,
			`{"broken"`,
		); err != nil {
			t.Fatalf("corrupt oldest projection error = %v", err)
		}

		tail, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 1})
		if err != nil {
			t.Fatalf("TranscriptPage(tail) error = %v", err)
		}
		if got, want := transcript.UIMessageText(tail.Entries[0].Message), "message-5"; got != want {
			t.Fatalf("tail text = %q, want %q", got, want)
		}
		_, err = sessionDB.TranscriptPage(ctx, transcript.PageQuery{
			Limit:          1,
			BeforeSequence: 2,
		})
		if !errors.Is(err, transcript.ErrProjectionCorrupt) {
			t.Fatalf("TranscriptPage(corrupt prefix) error = %v, want ErrProjectionCorrupt", err)
		}
	})

	t.Run("Should roll back the raw event when projection state is incompatible", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-rollback")
		if _, err := sessionDB.db.ExecContext(
			ctx,
			`UPDATE transcript_projection_state SET projection_version = 999 WHERE singleton = 1`,
		); err != nil {
			t.Fatalf("set incompatible projection version error = %v", err)
		}

		err := sessionDB.Record(ctx, SessionEvent{
			TurnID:    "turn-rollback",
			Type:      acp.EventTypeUserMessage,
			AgentName: "coder",
			Content:   `{"type":"user_message","text":"must roll back"}`,
		})
		if !errors.Is(err, transcript.ErrProjectionIncompatible) {
			t.Fatalf("Record() error = %v, want ErrProjectionIncompatible", err)
		}
		events, err := sessionDB.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("len(events) = %d, want transaction rollback", len(events))
		}
	})

	t.Run("Should roll back the raw event when materialization fails after insert", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-atomic")
		if _, err := sessionDB.db.ExecContext(ctx, `
			CREATE TRIGGER reject_transcript_projection
			BEFORE INSERT ON transcript_entries
			BEGIN
				SELECT RAISE(ABORT, 'projection rejected');
			END`); err != nil {
			t.Fatalf("create rejection trigger error = %v", err)
		}

		err := sessionDB.Record(ctx, SessionEvent{
			TurnID:    "turn-atomic",
			Type:      acp.EventTypeUserMessage,
			AgentName: "coder",
			Content:   `{"type":"user_message","text":"must be atomic"}`,
		})
		if err == nil {
			t.Fatal("Record() error = nil, want projection failure")
		}
		events, queryErr := sessionDB.Query(ctx, EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query() error = %v", queryErr)
		}
		if len(events) != 0 {
			t.Fatalf("len(events) = %d, want raw event rollback", len(events))
		}
	})

	t.Run("Should archive the suffix and preserve the rewind receipt across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		owner := testSessionDBOwner("sess-transcript-rewind")
		sessionDB, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		var closeSessionDBErr error
		closeSessionDB := func() error {
			if sessionDB == nil {
				return closeSessionDBErr
			}
			closeSessionDBErr = sessionDB.Close(ctx)
			sessionDB = nil
			return closeSessionDBErr
		}
		t.Cleanup(func() {
			if err := closeSessionDB(); err != nil {
				t.Errorf("Close(session DB cleanup) error = %v", err)
			}
		})
		input := []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeUserMessage, SessionID: owner.SessionID,
				TurnID: "turn-1", Text: "keep",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: owner.SessionID,
				TurnID: "turn-1", Text: "kept answer",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeUserMessage, SessionID: owner.SessionID,
				TurnID: "turn-2", Text: "try again",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: owner.SessionID,
				TurnID: "turn-2", Text: "discarded answer",
			}, "coder"),
		}
		if _, err := sessionDB.RecordPersistedBatch(ctx, input); err != nil {
			t.Fatalf("RecordPersistedBatch() error = %v", err)
		}
		page, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage() error = %v", err)
		}
		if got, want := len(page.Entries), 4; got != want {
			t.Fatalf("len(page.Entries) = %d, want %d", got, want)
		}
		target := page.Entries[2]
		prefixEvents, err := sessionDB.Query(ctx, EventQuery{
			BeforeSequence: target.StartSequence,
			Archive:        store.EventArchiveUnarchived,
		})
		if err != nil {
			t.Fatalf("Query(prefix) error = %v", err)
		}
		messages, err := transcript.Assemble(prefixEvents)
		if err != nil {
			t.Fatalf("Assemble(prefix) error = %v", err)
		}
		baseline, err := json.Marshal(messages)
		if err != nil {
			t.Fatalf("json.Marshal(prefix) error = %v", err)
		}
		result, err := sessionDB.RewindConversation(ctx, store.ConversationRewindRequest{
			MessageID: target.Message.ID, IdempotencyKey: "idem-rewind", RequestHash: "hash-rewind",
			ExpectedGeneration: page.Generation, ExpectedMaxSequence: page.MaxSequence,
			TranscriptEpoch: 2, BaselineJSON: string(baseline),
		})
		if err != nil {
			t.Fatalf("RewindConversation() error = %v", err)
		}
		if result.DraftText != "try again" || result.ArchivedEventCount != 2 || result.TranscriptEpoch != 2 {
			t.Fatalf("RewindConversation() = %#v, want selected prompt and two archived events", result)
		}
		if got, want := result.MaxSequence, target.StartSequence-1; got != want {
			t.Fatalf("RewindConversation().MaxSequence = %d, want active prefix max %d", got, want)
		}
		after, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage(after rewind) error = %v", err)
		}
		if got, want := len(after.Entries), 2; got != want {
			t.Fatalf("len(after.Entries) = %d, want %d", got, want)
		}
		if got, want := after.MaxSequence, result.MaxSequence; got != want {
			t.Fatalf("TranscriptPage(after rewind).MaxSequence = %d, want %d", got, want)
		}
		if _, err := sessionDB.AppendEventIfAbsent(ctx, SessionEvent{
			ID: "rewind:audit", TurnID: "rewind:audit", Type: "session.conversation_rewound",
			AgentName: "coder", Content: `{"target_message_id":"discarded"}`,
			Archived: true, Timestamp: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("AppendEventIfAbsent(archived audit) error = %v", err)
		}
		afterAudit, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage(after archived audit) error = %v", err)
		}
		if len(afterAudit.Entries) != len(after.Entries) || afterAudit.MaxSequence != after.MaxSequence {
			t.Fatalf("TranscriptPage(after archived audit) = %#v, want unchanged active projection", afterAudit)
		}
		archived, err := sessionDB.Query(ctx, EventQuery{Archive: store.EventArchiveArchived})
		if err != nil {
			t.Fatalf("Query(archived) error = %v", err)
		}
		if got, want := len(archived), 3; got != want {
			t.Fatalf("len(archived) = %d, want %d", got, want)
		}
		if err := closeSessionDB(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		reopened, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatalf("OpenSessionDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		replayed, found, err := reopened.ConversationRewindReceipt(ctx, "idem-rewind", "hash-rewind")
		if err != nil {
			t.Fatalf("ConversationRewindReceipt() error = %v", err)
		}
		if !found || !replayed.Replayed || replayed.TargetMessageID != target.Message.ID {
			t.Fatalf("ConversationRewindReceipt() = (%#v, %t), want durable replay", replayed, found)
		}
	})

	t.Run("Should reject rewind when compaction archived a visible retained prefix", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-transcript-rewind-compacted")
		input := []SessionEvent{
			canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeUserMessage, SessionID: sessionDB.owner.SessionID,
				TurnID: "turn-1", Text: "compacted",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: sessionDB.owner.SessionID,
				TurnID: "turn-1", Text: "old answer",
			}, "coder"),
			canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeUserMessage, SessionID: sessionDB.owner.SessionID,
				TurnID: "turn-2", Text: "visible target",
			}, "coder"),
		}
		persisted, err := sessionDB.RecordPersistedBatch(ctx, input)
		if err != nil {
			t.Fatalf("RecordPersistedBatch() error = %v", err)
		}
		if _, err := sessionDB.ArchiveEvents(ctx, store.EventArchiveRequest{
			FromSequence: persisted[0].Sequence,
			ToSequence:   persisted[1].Sequence,
		}); err != nil {
			t.Fatalf("ArchiveEvents(compacted prefix) error = %v", err)
		}
		page, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage() error = %v", err)
		}
		if got, want := len(page.Entries), 3; got != want {
			t.Fatalf("len(visible entries) = %d, want %d", got, want)
		}
		_, err = sessionDB.ConversationRewindTarget(ctx, page.Entries[2].Message.ID)
		if !errors.Is(err, store.ErrConversationRewindTargetInvalid) {
			t.Fatalf("ConversationRewindTarget(compacted prefix) error = %v, want target invalid", err)
		}
	})
}

func TestOpenSessionDBReadOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should fail without creating a missing database", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		_, err := OpenSessionDBReadOnly(ctx, testSessionDBOwner("sess-read-only-missing"), path)
		if err == nil {
			t.Fatal("OpenSessionDBReadOnly(missing) error = nil, want non-nil")
		}
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(read-only missing path) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should refuse a legacy database without changing files", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		seedReadOnlySessionDatabase(
			t,
			path,
			`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`,
			testSessionDBOwnerTableDDL,
			`INSERT INTO session_db_owner VALUES (1, 'sess-read-only-legacy', 'ws-sessiondb-test')`,
		)
		before := readOnlySessionDatabaseFamilyDigest(t, path)

		_, err := OpenSessionDBReadOnly(testutil.Context(t), testSessionDBOwner("sess-read-only-legacy"), path)
		if !errors.Is(err, store.ErrLegacyDatabase) {
			t.Fatalf("OpenSessionDBReadOnly(legacy) error = %v, want ErrLegacyDatabase", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before)
	})

	t.Run("Should refuse an ahead database without changing files", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		seedReadOnlySessionDatabase(
			t,
			path,
			`CREATE TABLE goose_db_version_session (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL,
				is_applied INTEGER NOT NULL,
				tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
			`INSERT INTO goose_db_version_session (version_id, is_applied) VALUES (99, 1)`,
			testSessionDBOwnerTableDDL,
			`INSERT INTO session_db_owner VALUES (1, 'sess-read-only-ahead', 'ws-sessiondb-test')`,
		)
		before := readOnlySessionDatabaseFamilyDigest(t, path)

		_, err := OpenSessionDBReadOnly(testutil.Context(t), testSessionDBOwner("sess-read-only-ahead"), path)
		if !errors.Is(err, store.ErrSchemaAhead) {
			t.Fatalf("OpenSessionDBReadOnly(ahead) error = %v, want ErrSchemaAhead", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before)
	})

	t.Run("Should refuse a behind database without changing files", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open(previous session stream) error = %v", err)
		}
		closeDB := registerTestSQLDBCleanup(t, "previous session stream", db)
		if err := store.Apply(ctx, db, previousSessionMigrationStream(t)); err != nil {
			t.Fatalf("Apply(previous session stream) error = %v", err)
		}
		if _, err := db.ExecContext(ctx, testSessionDBOwnerTableDDL); err != nil {
			t.Fatalf("create behind session owner table error = %v", err)
		}
		if _, err := db.ExecContext(
			ctx,
			`INSERT INTO session_db_owner VALUES (1, 'sess-read-only-behind', 'ws-sessiondb-test')`,
		); err != nil {
			t.Fatalf("seed behind session owner error = %v", err)
		}
		if err := closeDB(); err != nil {
			t.Fatalf("Close(previous session stream) error = %v", err)
		}
		before := readOnlySessionDatabaseFamilyDigest(t, path)

		_, err = OpenSessionDBReadOnly(ctx, testSessionDBOwner("sess-read-only-behind"), path)
		if !errors.Is(err, store.ErrSchemaBehind) {
			t.Fatalf("OpenSessionDBReadOnly(behind) error = %v, want ErrSchemaBehind", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before)
	})

	t.Run("Should query an existing database through the read-only contract", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, testSessionDBOwner("sess-read-only-existing"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		if err := writer.Record(ctx, SessionEvent{
			TurnID:    "turn-read-only",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   "{\"text\":\"persisted\"}",
		}); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}

		reader, err := OpenSessionDBReadOnly(ctx, testSessionDBOwner("sess-read-only-existing"), path)
		if err != nil {
			t.Fatalf("OpenSessionDBReadOnly(existing) error = %v", err)
		}
		defer func() {
			if closeErr := reader.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("Close(reader) error = %v", closeErr)
			}
		}()

		events, err := reader.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query(read-only) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}
		if events[0].SessionID != "sess-read-only-existing" || events[0].TurnID != "turn-read-only" {
			t.Fatalf("events[0] = %#v, want session/turn ids set", events[0])
		}
	})

	t.Run("Should refuse a foreign workspace before read-only sidecars are created", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, testSessionDBOwner("sess-read-only-owner"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}
		before := readOnlySessionDatabaseFamilyDigest(t, path)

		_, err = OpenSessionDBReadOnly(ctx, store.SessionDBOwner{
			SessionID: "sess-read-only-owner", WorkspaceID: "ws-foreign",
		}, path)
		if !errors.Is(err, ErrSessionDBOwnerMismatch) {
			t.Fatalf("OpenSessionDBReadOnly(foreign workspace) error = %v, want ErrSessionDBOwnerMismatch", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before)
	})

	t.Run("Should recheck the owner on the effective handle after path substitution", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		expectedDir := filepath.Join(root, "expected")
		foreignDir := filepath.Join(root, "foreign")
		expectedPath := filepath.Join(expectedDir, SessionDatabaseName)
		foreignPath := filepath.Join(foreignDir, SessionDatabaseName)
		expectedOwner := testSessionDBOwner("sess-read-only-effective-owner")
		foreignOwner := store.SessionDBOwner{
			SessionID: "sess-read-only-effective-foreign", WorkspaceID: "ws-effective-foreign",
		}
		for _, fixture := range []struct {
			owner store.SessionDBOwner
			path  string
		}{
			{owner: expectedOwner, path: expectedPath},
			{owner: foreignOwner, path: foreignPath},
		} {
			db, err := OpenSessionDB(ctx, fixture.owner, fixture.path)
			if err != nil {
				t.Fatalf("OpenSessionDB(%s) error = %v", fixture.owner.SessionID, err)
			}
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close(%s) error = %v", fixture.owner.SessionID, err)
			}
		}
		if err := verifySessionDBOwnerReadOnly(ctx, expectedOwner, expectedPath); err != nil {
			t.Fatalf("verifySessionDBOwnerReadOnly(expected) error = %v", err)
		}
		if err := os.Rename(expectedDir, filepath.Join(root, "expected-before-substitution")); err != nil {
			t.Fatalf("os.Rename(expected backup) error = %v", err)
		}
		if err := os.Rename(foreignDir, expectedDir); err != nil {
			t.Fatalf("os.Rename(foreign into expected path) error = %v", err)
		}

		reader, err := openGuardedSessionDBReadOnly(ctx, expectedOwner, expectedPath)
		if err == nil {
			if closeErr := reader.Close(ctx); closeErr != nil {
				t.Errorf("Close(substituted reader) error = %v", closeErr)
			}
			t.Fatal("openGuardedSessionDBReadOnly(substituted) error = nil, want owner mismatch")
		}
		if !errors.Is(err, ErrSessionDBOwnerMismatch) {
			t.Fatalf("openGuardedSessionDBReadOnly(substituted) error = %v, want owner mismatch", err)
		}
	})

	t.Run("Should reject a same-owner database substituted during the physical open", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		expectedDir := filepath.Join(root, "expected")
		replacementDir := filepath.Join(root, "replacement")
		expectedPath := filepath.Join(expectedDir, SessionDatabaseName)
		replacementPath := filepath.Join(replacementDir, SessionDatabaseName)
		owner := testSessionDBOwner("sess-read-only-same-owner-substitution")
		for _, fixture := range []struct {
			path    string
			eventID string
		}{
			{path: expectedPath, eventID: "expected-event"},
			{path: replacementPath, eventID: "replacement-secret-event"},
		} {
			db, err := OpenSessionDB(ctx, owner, fixture.path)
			if err != nil {
				t.Fatalf("OpenSessionDB(%s) error = %v", fixture.eventID, err)
			}
			if err := db.Record(ctx, SessionEvent{
				ID: fixture.eventID, TurnID: fixture.eventID + "-turn", Type: acp.EventTypeDone, AgentName: "coder",
			}); err != nil {
				t.Fatalf("Record(%s) error = %v", fixture.eventID, err)
			}
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close(%s) error = %v", fixture.eventID, err)
			}
		}
		before := readOnlySessionDatabaseFamilyDigest(t, replacementPath)

		intercept := &interceptSessionSQLiteDriver{
			delegate: &sqlite.Driver{},
			beforeOpen: func(call int) error {
				if call != 2 {
					return nil
				}
				if err := os.Rename(expectedDir, filepath.Join(root, "expected-before-substitution")); err != nil {
					return err
				}
				return os.Rename(replacementDir, expectedDir)
			},
		}
		db := sql.OpenDB(&sessionSQLiteConnector{
			driver:   intercept,
			dsn:      sessionSQLiteOperationalDSN(expectedPath, "ro"),
			path:     expectedPath,
			owner:    owner,
			readOnly: true,
		})
		err := db.PingContext(ctx)
		if !errors.Is(err, ErrSessionDBFamilyChanged) {
			t.Fatalf("PingContext(same-owner substitution) error = %v, want ErrSessionDBFamilyChanged", err)
		}
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("Close(substituted sql.DB) error = %v", closeErr)
		}
		assertReadOnlySessionDatabaseUnchanged(t, expectedPath, before)
	})

	t.Run("Should reject a same-owner ABA substitution after the physical open", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name     string
			readOnly bool
		}{
			{name: "read-only", readOnly: true},
			{name: "writable", readOnly: false},
		} {
			t.Run("Should reject the "+testCase.name+" connection", func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				root := t.TempDir()
				expectedDir := filepath.Join(root, "expected")
				replacementDir := filepath.Join(root, "replacement")
				expectedBackupDir := filepath.Join(root, "expected-during-open")
				expectedPath := filepath.Join(expectedDir, SessionDatabaseName)
				replacementPath := filepath.Join(replacementDir, SessionDatabaseName)
				owner := testSessionDBOwner("sess-same-owner-aba-" + testCase.name)
				for _, fixture := range []struct {
					path    string
					eventID string
				}{
					{path: expectedPath, eventID: "expected-event"},
					{path: replacementPath, eventID: "replacement-secret-event"},
				} {
					db, err := OpenSessionDB(ctx, owner, fixture.path)
					if err != nil {
						t.Fatalf("OpenSessionDB(%s) error = %v", fixture.eventID, err)
					}
					if err := db.Record(ctx, SessionEvent{
						ID:        fixture.eventID,
						TurnID:    fixture.eventID + "-turn",
						Type:      acp.EventTypeDone,
						AgentName: "coder",
					}); err != nil {
						t.Fatalf("Record(%s) error = %v", fixture.eventID, err)
					}
					if err := db.Close(ctx); err != nil {
						t.Fatalf("Close(%s) error = %v", fixture.eventID, err)
					}
				}
				expectedBefore := readOnlySessionDatabaseFamilyDigest(t, expectedPath)
				replacementBefore := readOnlySessionDatabaseFamilyDigest(t, replacementPath)

				intercept := &interceptSessionSQLiteDriver{
					delegate: &sqlite.Driver{},
					beforeOpen: func(call int) error {
						if call != 2 {
							return nil
						}
						if err := os.Rename(expectedDir, expectedBackupDir); err != nil {
							return err
						}
						return os.Rename(replacementDir, expectedDir)
					},
					afterOpen: func(call int, connection driver.Conn) error {
						if call != 2 {
							return nil
						}
						querier, ok := connection.(sqlite.ExecQuerierContext)
						if !ok {
							return errors.New("intercepted sqlite connection does not support contextual queries")
						}
						if err := verifySessionDBOwnerConnection(ctx, querier, owner); err != nil {
							return err
						}
						if err := os.Rename(expectedDir, replacementDir); err != nil {
							return err
						}
						return os.Rename(expectedBackupDir, expectedDir)
					},
				}
				db := sql.OpenDB(&sessionSQLiteConnector{
					// The immutable DSN isolates the identity regression from SQLite's
					// otherwise valid WAL/SHM creation before the connector rejects B.
					driver: intercept, dsn: sessionDBOwnerGuardDSN(expectedPath),
					path: expectedPath, owner: owner, readOnly: testCase.readOnly,
				})
				err := db.PingContext(ctx)
				if !errors.Is(err, ErrSessionDBFamilyChanged) {
					t.Fatalf("PingContext(same-owner ABA) error = %v, want ErrSessionDBFamilyChanged", err)
				}
				if closeErr := db.Close(); closeErr != nil {
					t.Fatalf("Close(rejected sql.DB) error = %v", closeErr)
				}
				assertReadOnlySessionDatabaseUnchanged(t, expectedPath, expectedBefore)
				assertReadOnlySessionDatabaseUnchanged(t, replacementPath, replacementBefore)
			})
		}
	})

	t.Run("Should guard a replacement before every read-only physical connection", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		expectedDir := filepath.Join(root, "expected")
		foreignDir := filepath.Join(root, "foreign")
		expectedPath := filepath.Join(expectedDir, SessionDatabaseName)
		foreignPath := filepath.Join(foreignDir, SessionDatabaseName)
		expectedOwner := testSessionDBOwner("sess-read-only-connection-owner")
		foreignOwner := store.SessionDBOwner{
			SessionID: "sess-read-only-connection-foreign", WorkspaceID: "ws-read-only-connection-foreign",
		}
		for _, fixture := range []struct {
			owner   store.SessionDBOwner
			path    string
			eventID string
		}{
			{owner: expectedOwner, path: expectedPath, eventID: "expected-event"},
			{owner: foreignOwner, path: foreignPath, eventID: "foreign-event"},
		} {
			db, err := OpenSessionDB(ctx, fixture.owner, fixture.path)
			if err != nil {
				t.Fatalf("OpenSessionDB(%s) error = %v", fixture.owner.SessionID, err)
			}
			if err := db.Record(ctx, SessionEvent{
				ID: fixture.eventID, TurnID: fixture.eventID + "-turn", Type: acp.EventTypeDone, AgentName: "coder",
			}); err != nil {
				t.Fatalf("Record(%s) error = %v", fixture.owner.SessionID, err)
			}
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close(%s) error = %v", fixture.owner.SessionID, err)
			}
		}
		reader, err := OpenSessionDBReadOnly(ctx, expectedOwner, expectedPath)
		if err != nil {
			t.Fatalf("OpenSessionDBReadOnly(expected) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reader.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reader) error = %v", err)
			}
		})
		before := readOnlySessionDatabaseFamilyDigest(t, foreignPath)

		reader.db.SetMaxIdleConns(0)
		if err := os.Rename(expectedDir, filepath.Join(root, "expected-before-reconnect")); err != nil {
			t.Fatalf("os.Rename(expected backup) error = %v", err)
		}
		if err := os.Rename(foreignDir, expectedDir); err != nil {
			t.Fatalf("os.Rename(foreign into expected path) error = %v", err)
		}

		events, err := reader.Query(ctx, EventQuery{})
		if !errors.Is(err, ErrSessionDBOwnerMismatch) {
			t.Fatalf("Query(substituted read-only connection) error = %v, want ErrSessionDBOwnerMismatch", err)
		}
		if len(events) != 0 {
			t.Fatalf("Query(substituted read-only connection) events = %#v, want empty", events)
		}
		assertReadOnlySessionDatabaseUnchanged(t, expectedPath, before)
	})

	t.Run("Should apply identical cursor bounds to full and metadata projections", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, testSessionDBOwner("sess-read-only-projections"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		for _, turnID := range []string{"turn-1", "turn-2", "turn-3"} {
			if err := writer.Record(ctx, SessionEvent{
				TurnID:    turnID,
				Type:      "agent_message",
				AgentName: "coder",
				Content:   `{"text":"persisted"}`,
			}); err != nil {
				t.Fatalf("Record(%s) error = %v", turnID, err)
			}
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}

		reader, err := OpenSessionDBReadOnly(ctx, testSessionDBOwner("sess-read-only-projections"), path)
		if err != nil {
			t.Fatalf("OpenSessionDBReadOnly() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reader.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("Close(reader) error = %v", closeErr)
			}
		})

		query := EventQuery{BeforeSequence: 3, Limit: 1}
		fullEvents, err := reader.Query(ctx, query)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		metadata, err := reader.QueryEventMetadata(ctx, query)
		if err != nil {
			t.Fatalf("QueryEventMetadata() error = %v", err)
		}
		if got, want := eventSequences(fullEvents), []int64{2}; !equalInt64Slices(got, want) {
			t.Fatalf("Query() sequences = %#v, want %#v", got, want)
		}
		if got, want := metadataEventSequences(metadata), []int64{2}; !equalInt64Slices(got, want) {
			t.Fatalf("QueryEventMetadata() sequences = %#v, want %#v", got, want)
		}
	})

	t.Run("Should retry transient SQLite locks while opening", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		transientLock := errors.New("database is locked")
		calls := 0

		reader, err := openSessionDBReadOnlyWithRetry(
			ctx,
			testSessionDBOwner("sess-read-only-locked"),
			path,
			func(context.Context, store.SessionDBOwner, string) (*ReadOnlySessionDB, error) {
				calls++
				if calls < 3 {
					return nil, transientLock
				}
				return &ReadOnlySessionDB{owner: testSessionDBOwner("sess-read-only-locked")}, nil
			},
			func(err error) bool {
				return errors.Is(err, transientLock)
			},
			newReadOnlyOpenConfig([]ReadOnlyOpenOption{
				WithReadOnlyOpenRetry(3, time.Nanosecond, time.Nanosecond),
			}),
		)
		if err != nil {
			t.Fatalf("openSessionDBReadOnlyWithRetry() error = %v", err)
		}
		if reader == nil || reader.owner.SessionID != "sess-read-only-locked" {
			t.Fatalf("openSessionDBReadOnlyWithRetry() reader = %#v, want session id", reader)
		}
		if got, want := calls, 3; got != want {
			t.Fatalf("openSessionDBReadOnlyWithRetry() calls = %d, want %d", got, want)
		}
	})
}

func seedReadOnlySessionDatabase(t *testing.T, path string, statements ...string) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(%q) error = %v", path, err)
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(testutil.Context(t), statement); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				t.Fatalf("seed session database error = %v; close error = %v", err, closeErr)
			}
			t.Fatalf("seed session database error = %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(seed session database) error = %v", err)
	}
}

func registerTestSQLDBCleanup(t *testing.T, label string, db *sql.DB) func() error {
	t.Helper()

	closeDB := sync.OnceValue(db.Close)
	t.Cleanup(func() {
		if err := closeDB(); err != nil {
			t.Errorf("Close(%s) error = %v", label, err)
		}
	})
	return closeDB
}

type closeErrorSessionDBParentDirectory struct {
	*fileutil.Directory
	err error
}

func (d *closeErrorSessionDBParentDirectory) Close() error {
	return errors.Join(d.Directory.Close(), d.err)
}

type interceptSessionSQLiteDriver struct {
	delegate   driver.Driver
	beforeOpen func(call int) error
	afterOpen  func(call int, connection driver.Conn) error
	mu         sync.Mutex
	openCount  int
	err        error
}

func (d *interceptSessionSQLiteDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	d.openCount++
	call := d.openCount
	if d.err == nil && d.beforeOpen != nil {
		d.err = d.beforeOpen(call)
	}
	err := d.err
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	connection, err := d.delegate.Open(name)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	if d.err == nil && d.afterOpen != nil {
		d.err = d.afterOpen(call, connection)
	}
	err = d.err
	d.mu.Unlock()
	if err != nil {
		return nil, closeSessionSQLiteDriverConnection(connection, err)
	}
	return connection, nil
}

type sessionDatabaseFileDigest struct {
	exists bool
	digest [sha256.Size]byte
}

type sessionDatabaseFamilyDigest struct {
	database sessionDatabaseFileDigest
	wal      sessionDatabaseFileDigest
	shm      sessionDatabaseFileDigest
	journal  sessionDatabaseFileDigest
}

func readOnlySessionDatabaseFamilyDigest(t *testing.T, path string) sessionDatabaseFamilyDigest {
	t.Helper()

	return sessionDatabaseFamilyDigest{
		database: readOnlySessionFileDigest(t, path),
		wal:      readOnlySessionFileDigest(t, path+"-wal"),
		shm:      readOnlySessionFileDigest(t, path+"-shm"),
		journal:  readOnlySessionFileDigest(t, path+"-journal"),
	}
}

func readOnlySessionFileDigest(t *testing.T, path string) sessionDatabaseFileDigest {
	t.Helper()

	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return sessionDatabaseFileDigest{}
	}
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return sessionDatabaseFileDigest{exists: true, digest: sha256.Sum256(contents)}
}

func assertReadOnlySessionDatabaseUnchanged(
	t *testing.T,
	path string,
	want sessionDatabaseFamilyDigest,
) {
	t.Helper()

	if got := readOnlySessionDatabaseFamilyDigest(t, path); got != want {
		t.Fatalf("read-only refusal changed database family: got %#v want %#v", got, want)
	}
}

func TestSessionDBClear(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize through the writer queue", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := &SessionDB{
			writeCh: make(chan sessionWriteRequest, 1),
		}
		sessionDB.state.Store(sessionStateOpen)

		received := make(chan sessionWriteKind, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			req, ok := <-sessionDB.writeCh
			if !ok {
				return
			}
			received <- req.kind
			req.result <- sessionWriteResult{}
		}()
		t.Cleanup(func() {
			close(sessionDB.writeCh)
			<-done
		})

		if err := sessionDB.Clear(ctx); err != nil {
			t.Fatalf("Clear() error = %v", err)
		}
		select {
		case got := <-received:
			if got != sessionWriteClear {
				t.Fatalf("Clear() queued kind = %d, want %d", got, sessionWriteClear)
			}
		case <-ctx.Done():
			t.Fatalf("Clear() writer request not observed: %v", ctx.Err())
		}
	})

	t.Run("Should advance transcript generation and restart event sequences", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		sessionDB := openTestSessionDB(t, "sess-clear-transcript")
		if err := sessionDB.Record(ctx, SessionEvent{
			TurnID:    "turn-before-clear",
			Type:      acp.EventTypeUserMessage,
			AgentName: "coder",
			Content:   `{"type":"user_message","text":"before"}`,
		}); err != nil {
			t.Fatalf("Record(before clear) error = %v", err)
		}
		before, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage(before clear) error = %v", err)
		}

		if err := sessionDB.Clear(ctx); err != nil {
			t.Fatalf("Clear() error = %v", err)
		}
		if err := sessionDB.Record(ctx, SessionEvent{
			TurnID:    "turn-after-clear",
			Type:      acp.EventTypeUserMessage,
			AgentName: "coder",
			Content:   `{"type":"user_message","text":"after"}`,
		}); err != nil {
			t.Fatalf("Record(after clear) error = %v", err)
		}
		after, err := sessionDB.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage(after clear) error = %v", err)
		}
		if got, want := after.Generation, before.Generation+1; got != want {
			t.Fatalf("generation after clear = %d, want %d", got, want)
		}
		if got, want := after.Entries[0].StartSequence, int64(1); got != want {
			t.Fatalf("start sequence after clear = %d, want %d", got, want)
		}
		if got, want := transcript.UIMessageText(after.Entries[0].Message), "after"; got != want {
			t.Fatalf("transcript after clear = %q, want %q", got, want)
		}

		path := sessionDB.Path()
		if err := sessionDB.Close(ctx); err != nil {
			t.Fatalf("Close(after clear) error = %v", err)
		}
		reader, err := OpenSessionDBReadOnly(ctx, testSessionDBOwner("sess-clear-transcript"), path)
		if err != nil {
			t.Fatalf("OpenSessionDBReadOnly(after clear) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reader.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reader) error = %v", err)
			}
		})
		reopened, err := reader.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil {
			t.Fatalf("TranscriptPage(reopened) error = %v", err)
		}
		assertTranscriptEntriesEqual(t, reopened.Entries, after.Entries)
	})
}

func TestOpenSessionSQLiteDoesNotFailWhenVacuumFails(t *testing.T) {
	t.Run("Should keep opening the session database when vacuuming fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		sentinel := errors.New("vacuum unavailable")

		db, err := openOwnedSessionSQLiteWithVacuum(
			ctx,
			testSessionDBOwner("sess-vacuum-failure"),
			path,
			func(context.Context, *sql.DB) error {
				return sentinel
			},
		)
		if err != nil {
			t.Fatalf("openSessionSQLiteWithVacuum() error = %v, want nil", err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Fatalf("db.Close() error = %v", err)
			}
		})

		assertTablesPresent(t, db, sessionMigrationVersionTable, "events", "token_usage")
		assertJournalModeWAL(t, db)
	})
}

func TestSessionDBRecordAutoIncrementSequence(t *testing.T) {
	t.Parallel()

	t.Run("Should assign strict sequences for a single handle", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-seq")
		base := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
		callCount := 0
		sessionDB.now = func() time.Time {
			callCount++
			return base.Add(time.Duration(callCount) * time.Second)
		}

		ctx := testutil.Context(t)
		if err := sessionDB.Record(
			ctx,
			SessionEvent{TurnID: "turn-1", Type: "agent_message", AgentName: "coder", Content: `{"text":"one"}`},
		); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		if err := sessionDB.Record(
			ctx,
			SessionEvent{TurnID: "turn-1", Type: "tool_call", AgentName: "coder", Content: `{"tool":"ls"}`},
		); err != nil {
			t.Fatalf("Record() error = %v", err)
		}

		events, err := sessionDB.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if got, want := len(events), 2; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}
		if events[0].Sequence != 1 || events[1].Sequence != 2 {
			t.Fatalf("event sequences = [%d %d], want [1 2]", events[0].Sequence, events[1].Sequence)
		}
		if events[0].SessionID != "sess-seq" || events[1].SessionID != "sess-seq" {
			t.Fatalf("session ids = [%q %q], want sess-seq", events[0].SessionID, events[1].SessionID)
		}
	})

	t.Run("Should assign strict sequences for concurrent handles", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		first, err := OpenSessionDB(ctx, testSessionDBOwner("sess-seq-shared"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB(first) error = %v", err)
		}
		t.Cleanup(func() {
			if err := first.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(first) error = %v", err)
			}
		})
		second, err := OpenSessionDB(ctx, testSessionDBOwner("sess-seq-shared"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(second) error = %v", err)
			}
		})

		if err := first.Record(ctx, SessionEvent{
			TurnID:    "turn-1",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   `{"text":"one"}`,
		}); err != nil {
			t.Fatalf("Record(first) error = %v", err)
		}
		if err := second.Record(ctx, SessionEvent{
			TurnID:    "turn-2",
			Type:      "tool_result",
			AgentName: "coder",
			Content:   `{"text":"two"}`,
		}); err != nil {
			t.Fatalf("Record(second) error = %v", err)
		}

		events, err := first.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if gotSeqs := eventSequences(events); !equalInt64Slices(gotSeqs, []int64{1, 2}) {
			t.Fatalf("eventSequences() = %#v, want %#v", gotSeqs, []int64{1, 2})
		}
		afterFirst, err := first.Query(ctx, EventQuery{AfterSequence: 1})
		if err != nil {
			t.Fatalf("Query(AfterSequence: 1) error = %v", err)
		}
		if gotSeqs := eventSequences(afterFirst); !equalInt64Slices(gotSeqs, []int64{2}) {
			t.Fatalf("after first event sequences = %#v, want %#v", gotSeqs, []int64{2})
		}
	})
}

func TestSessionDBRecordTokenUsageStoresNullableFieldsAsNULL(t *testing.T) {
	t.Parallel()

	t.Run("Should store nullable token usage fields as NULL", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-usage")
		outputTokens := int64(12)
		usage := TokenUsage{
			TurnID:       "turn-usage",
			OutputTokens: &outputTokens,
		}

		if err := sessionDB.RecordTokenUsage(testutil.Context(t), usage); err != nil {
			t.Fatalf("RecordTokenUsage() error = %v", err)
		}

		var (
			inputTokens sql.NullInt64
			output      sql.NullInt64
			totalTokens sql.NullInt64
			currency    sql.NullString
		)
		if err := sessionDB.db.QueryRowContext(
			testutil.Context(t),
			`SELECT input_tokens, output_tokens, total_tokens, cost_currency FROM token_usage WHERE turn_id = ?`,
			"turn-usage",
		).Scan(&inputTokens, &output, &totalTokens, &currency); err != nil {
			t.Fatalf("QueryRowContext() error = %v", err)
		}

		if inputTokens.Valid {
			t.Fatalf("input_tokens.Valid = true, want false")
		}
		if !output.Valid || output.Int64 != 12 {
			t.Fatalf("output_tokens = %#v, want valid 12", output)
		}
		if totalTokens.Valid {
			t.Fatalf("total_tokens.Valid = true, want false")
		}
		if currency.Valid {
			t.Fatalf("cost_currency.Valid = true, want false")
		}
	})
}

func TestSessionDBQueryFilters(t *testing.T) {
	t.Parallel()

	sessionDB := openTestSessionDB(t, "sess-query")
	base := time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC)
	callCount := 0
	sessionDB.now = func() time.Time {
		callCount++
		return base.Add(time.Duration(callCount) * time.Minute)
	}

	events := []SessionEvent{
		{TurnID: "turn-1", Type: "agent_message", AgentName: "coder", Content: `{"text":"one"}`},
		{TurnID: "turn-1", Type: "tool_call", AgentName: "coder", Content: `{"tool":"ls"}`},
		{TurnID: "turn-2", Type: "agent_message", AgentName: "reviewer", Content: `{"text":"two"}`},
		{TurnID: "turn-3", Type: "error", AgentName: "coder", Content: `{"error":"boom"}`},
	}
	for _, event := range events {
		if err := sessionDB.Record(testutil.Context(t), event); err != nil {
			t.Fatalf("Record(%q) error = %v", event.Type, err)
		}
	}

	tests := []struct {
		name      string
		query     EventQuery
		wantSeqs  []int64
		wantTypes []string
	}{
		{
			name:      "type filter",
			query:     EventQuery{Type: "agent_message"},
			wantSeqs:  []int64{1, 3},
			wantTypes: []string{"agent_message", "agent_message"},
		},
		{
			name:      "since filter",
			query:     EventQuery{Since: base.Add(2 * time.Minute)},
			wantSeqs:  []int64{2, 3, 4},
			wantTypes: []string{"tool_call", "agent_message", "error"},
		},
		{
			name:      "limit returns most recent in ascending order",
			query:     EventQuery{Limit: 2},
			wantSeqs:  []int64{3, 4},
			wantTypes: []string{"agent_message", "error"},
		},
		{
			name:      "agent filter",
			query:     EventQuery{AgentName: "coder"},
			wantSeqs:  []int64{1, 2, 4},
			wantTypes: []string{"agent_message", "tool_call", "error"},
		},
		{
			name:      "follow compatible after sequence filter",
			query:     EventQuery{AfterSequence: 2},
			wantSeqs:  []int64{3, 4},
			wantTypes: []string{"agent_message", "error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sessionDB.Query(testutil.Context(t), tt.query)
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}
			if gotSeqs := eventSequences(got); !equalInt64Slices(gotSeqs, tt.wantSeqs) {
				t.Fatalf("eventSequences() = %#v, want %#v", gotSeqs, tt.wantSeqs)
			}
			if gotTypes := eventTypes(got); !testutil.EqualStringSlices(gotTypes, tt.wantTypes) {
				t.Fatalf("eventTypes() = %#v, want %#v", gotTypes, tt.wantTypes)
			}
		})
	}
}

func TestSessionDBQueryOrderedBySequence(t *testing.T) {
	t.Parallel()

	t.Run("Should order events by sequence", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-order")
		base := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
		customTimes := []time.Time{
			base.Add(3 * time.Minute),
			base.Add(1 * time.Minute),
			base.Add(2 * time.Minute),
		}

		for index, ts := range customTimes {
			if err := sessionDB.Record(testutil.Context(t), SessionEvent{
				TurnID:    fmt.Sprintf("turn-%d", index+1),
				Type:      "agent_message",
				AgentName: "coder",
				Content:   fmt.Sprintf(`{"index":%d}`, index+1),
				Timestamp: ts,
			}); err != nil {
				t.Fatalf("Record() error = %v", err)
			}
		}

		events, err := sessionDB.Query(testutil.Context(t), EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}

		if gotSeqs := eventSequences(events); !equalInt64Slices(gotSeqs, []int64{1, 2, 3}) {
			t.Fatalf("eventSequences() = %#v, want %#v", gotSeqs, []int64{1, 2, 3})
		}
	})
}

func TestSessionDBHistoryGroupsByTurn(t *testing.T) {
	t.Parallel()

	t.Run("Should group history by turn", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-history")
		input := []SessionEvent{
			{TurnID: "turn-a", Type: "agent_message", AgentName: "coder", Content: `{"text":"one"}`},
			{TurnID: "turn-a", Type: "tool_result", AgentName: "coder", Content: `{"tool":"ls"}`},
			{TurnID: "turn-b", Type: "agent_message", AgentName: "coder", Content: `{"text":"two"}`},
		}
		for _, event := range input {
			if err := sessionDB.Record(testutil.Context(t), event); err != nil {
				t.Fatalf("Record() error = %v", err)
			}
		}

		history, err := sessionDB.History(testutil.Context(t), EventQuery{})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if got, want := len(history), 2; got != want {
			t.Fatalf("len(history) = %d, want %d", got, want)
		}
		if history[0].TurnID != "turn-a" || history[1].TurnID != "turn-b" {
			t.Fatalf("turn ids = [%q %q], want [turn-a turn-b]", history[0].TurnID, history[1].TurnID)
		}
		if gotSeqs := eventSequences(history[0].Events); !equalInt64Slices(gotSeqs, []int64{1, 2}) {
			t.Fatalf("turn-a sequences = %#v, want %#v", gotSeqs, []int64{1, 2})
		}
		if gotSeqs := eventSequences(history[1].Events); !equalInt64Slices(gotSeqs, []int64{3}) {
			t.Fatalf("turn-b sequences = %#v, want %#v", gotSeqs, []int64{3})
		}
	})

	t.Run("Should not split a turn when after_sequence falls inside it", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-history-after")
		input := []SessionEvent{
			{TurnID: "turn-a", Type: "agent_message", AgentName: "coder", Content: `{"text":"one"}`},
			{TurnID: "turn-a", Type: "tool_result", AgentName: "coder", Content: `{"tool":"ls"}`},
			{TurnID: "turn-b", Type: "agent_message", AgentName: "coder", Content: `{"text":"two"}`},
			{TurnID: "turn-b", Type: "tool_result", AgentName: "coder", Content: `{"tool":"pwd"}`},
			{TurnID: "turn-c", Type: "agent_message", AgentName: "coder", Content: `{"text":"three"}`},
		}
		for _, event := range input {
			if err := sessionDB.Record(testutil.Context(t), event); err != nil {
				t.Fatalf("Record() error = %v", err)
			}
		}

		history, err := sessionDB.History(testutil.Context(t), EventQuery{AfterSequence: 3})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if got, want := len(history), 1; got != want {
			t.Fatalf("len(history) = %d, want %d", got, want)
		}
		if history[0].TurnID != "turn-c" {
			t.Fatalf("history[0].TurnID = %q, want turn-c", history[0].TurnID)
		}
		if gotSeqs := eventSequences(history[0].Events); !equalInt64Slices(gotSeqs, []int64{5}) {
			t.Fatalf("turn-c sequences = %#v, want %#v", gotSeqs, []int64{5})
		}
	})

	t.Run("Should read bounded history in chunks without splitting returned turns", func(t *testing.T) {
		t.Parallel()

		events := make([]store.SessionEvent, 0, 302)
		for sequence := int64(1); sequence <= 300; sequence++ {
			events = append(events, store.SessionEvent{
				ID:       fmt.Sprintf("event-%d", sequence),
				Sequence: sequence,
				TurnID:   "turn-a",
				Type:     "agent_message",
			})
		}
		events = append(events,
			store.SessionEvent{ID: "event-301", Sequence: 301, TurnID: "turn-b", Type: "agent_message"},
			store.SessionEvent{ID: "event-302", Sequence: 302, TurnID: "turn-c", Type: "agent_message"},
		)

		calls := make([]store.EventQuery, 0)
		queryEvents := func(_ context.Context, query store.EventQuery) ([]store.SessionEvent, error) {
			calls = append(calls, query)
			filtered := make([]store.SessionEvent, 0, len(events))
			for _, event := range events {
				if query.BeforeSequence > 0 && event.Sequence >= query.BeforeSequence {
					continue
				}
				filtered = append(filtered, event)
			}
			if query.Limit > 0 && len(filtered) > query.Limit {
				filtered = filtered[len(filtered)-query.Limit:]
			}
			return append([]store.SessionEvent(nil), filtered...), nil
		}

		history, err := queryTurnHistory(testutil.Context(t), store.EventQuery{Limit: 3}, queryEvents)
		if err != nil {
			t.Fatalf("queryTurnHistory() error = %v", err)
		}
		if got, want := len(calls), 2; got != want {
			t.Fatalf("query calls = %d, want %d", got, want)
		}
		if calls[0].Limit != historyEventChunkSize {
			t.Fatalf("first query limit = %d, want %d", calls[0].Limit, historyEventChunkSize)
		}
		if calls[0].AfterSequence != 0 {
			t.Fatalf("first query after_sequence = %d, want 0", calls[0].AfterSequence)
		}
		if calls[1].BeforeSequence != 47 {
			t.Fatalf("second query before_sequence = %d, want 47", calls[1].BeforeSequence)
		}
		if got, want := len(history), 3; got != want {
			t.Fatalf("len(history) = %d, want %d", got, want)
		}
		if history[0].TurnID != "turn-a" || history[1].TurnID != "turn-b" ||
			history[2].TurnID != "turn-c" {
			t.Fatalf(
				"turn ids = [%q %q %q], want [turn-a turn-b turn-c]",
				history[0].TurnID,
				history[1].TurnID,
				history[2].TurnID,
			)
		}
		if got, want := len(history[0].Events), 300; got != want {
			t.Fatalf("turn-a events = %d, want %d", got, want)
		}
		if gotSeqs := eventSequences(history[0].Events); !equalInt64Slices(gotSeqs[:2], []int64{1, 2}) {
			t.Fatalf("turn-a first sequences = %#v, want %#v", gotSeqs[:2], []int64{1, 2})
		}
	})

	t.Run("Should not split a read-only turn when after_sequence falls inside it", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, testSessionDBOwner("sess-history-read-only-after"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		input := []SessionEvent{
			{TurnID: "turn-a", Type: "agent_message", AgentName: "coder", Content: `{"text":"one"}`},
			{TurnID: "turn-a", Type: "tool_result", AgentName: "coder", Content: `{"tool":"ls"}`},
			{TurnID: "turn-b", Type: "agent_message", AgentName: "coder", Content: `{"text":"two"}`},
			{TurnID: "turn-b", Type: "tool_result", AgentName: "coder", Content: `{"tool":"pwd"}`},
			{TurnID: "turn-c", Type: "agent_message", AgentName: "coder", Content: `{"text":"three"}`},
		}
		for _, event := range input {
			if err := writer.Record(ctx, event); err != nil {
				t.Fatalf("Record() error = %v", err)
			}
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}

		reader, err := OpenSessionDBReadOnly(ctx, testSessionDBOwner("sess-history-read-only-after"), path)
		if err != nil {
			t.Fatalf("OpenSessionDBReadOnly() error = %v", err)
		}
		defer func() {
			if closeErr := reader.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("Close(reader) error = %v", closeErr)
			}
		}()

		history, err := reader.History(ctx, EventQuery{AfterSequence: 3})
		if err != nil {
			t.Fatalf("History(read-only) error = %v", err)
		}
		if got, want := len(history), 1; got != want {
			t.Fatalf("len(history) = %d, want %d", got, want)
		}
		if history[0].TurnID != "turn-c" {
			t.Fatalf("history[0].TurnID = %q, want turn-c", history[0].TurnID)
		}
		if gotSeqs := eventSequences(history[0].Events); !equalInt64Slices(gotSeqs, []int64{5}) {
			t.Fatalf("turn-c sequences = %#v, want %#v", gotSeqs, []int64{5})
		}
	})
}

func TestSessionDBWriteFailureReturnsError(t *testing.T) {
	t.Parallel()

	t.Run("Should return errors when writes fail", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-full")

		if _, err := sessionDB.db.ExecContext(
			testutil.Context(t),
			`CREATE TRIGGER reject_session_event_insert
			 BEFORE INSERT ON events
			 BEGIN
			   SELECT RAISE(FAIL, 'injected session event write failure');
			 END`,
		); err != nil {
			t.Fatalf("ExecContext(create failure trigger) error = %v", err)
		}

		err := sessionDB.Record(testutil.Context(t), SessionEvent{
			TurnID:    "turn-disk-full",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   "write must fail",
		})
		if err == nil || !strings.Contains(err.Error(), "injected session event write failure") {
			t.Fatalf("Record() error = %v, want injected write failure", err)
		}

		events, queryErr := sessionDB.Query(testutil.Context(t), EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query() error = %v", queryErr)
		}
		if got := len(events); got != 0 {
			t.Fatalf("len(events) = %d, want 0", got)
		}
	})
}

func openTestSessionDB(t *testing.T, sessionID string) *SessionDB {
	t.Helper()

	sessionDB, err := OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(sessionID),
		filepath.Join(t.TempDir(), SessionDatabaseName),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := sessionDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	return sessionDB
}

func canonicalStoreEvent(t *testing.T, event acp.AgentEvent, agentName string) SessionEvent {
	t.Helper()

	payload, err := transcript.MarshalAgentEvent(event)
	if err != nil {
		t.Fatalf("MarshalAgentEvent(%s) error = %v", event.Type, err)
	}
	return SessionEvent{
		TurnID:    event.TurnID,
		Type:      event.Type,
		AgentName: agentName,
		Content:   payload,
		Timestamp: event.Timestamp,
	}
}

func assertEventSequences(t *testing.T, events []SessionEvent, want []int64) {
	t.Helper()

	if len(events) != len(want) {
		t.Fatalf("len(events) = %d, want %d", len(events), len(want))
	}
	for idx, event := range events {
		if event.Sequence != want[idx] {
			t.Fatalf("events[%d].Sequence = %d, want %d", idx, event.Sequence, want[idx])
		}
	}
}

func storedAgentText(t *testing.T, event SessionEvent) string {
	t.Helper()

	agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
	if err != nil {
		t.Fatalf("UnmarshalAgentEvent() error = %v", err)
	}
	return agentEvent.Text
}

func lastEntryText(t *testing.T, entries []transcript.Entry) string {
	t.Helper()

	if len(entries) == 0 {
		t.Fatal("entries is empty, want transcript content")
	}
	return transcript.UIMessageText(entries[len(entries)-1].Message)
}

func assertTranscriptEntriesEqual(t *testing.T, got []transcript.Entry, want []transcript.Entry) {
	t.Helper()

	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(got transcript) error = %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(want transcript) error = %v", err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("transcript entries = %s, want %s", gotJSON, wantJSON)
	}
}

func assertTranscriptToolPart(
	t *testing.T,
	entry transcript.Entry,
	toolCallID string,
	wantCommand string,
	wantState string,
) {
	t.Helper()

	for _, part := range entry.Message.Parts {
		if part.ToolCallID != toolCallID {
			continue
		}
		if part.State != wantState {
			t.Fatalf("tool part state = %q, want %q", part.State, wantState)
		}
		var input map[string]string
		if err := json.Unmarshal(part.Input, &input); err != nil {
			t.Fatalf("json.Unmarshal(tool input) error = %v; input=%s", err, string(part.Input))
		}
		if input["command"] != wantCommand {
			t.Fatalf("tool input command = %q, want %q", input["command"], wantCommand)
		}
		return
	}
	t.Fatalf("tool part %q not found in %#v", toolCallID, entry.Message.Parts)
}

func assertTablesPresent(t *testing.T, db *sql.DB, want ...string) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatalf("QueryContext(sqlite_master) error = %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Fatalf("rows.Close() error = %v", err)
		}
	}()

	have := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		have[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}

	for _, table := range want {
		if _, ok := have[table]; !ok {
			keys := make([]string, 0, len(have))
			for key := range have {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			t.Fatalf("missing table %q, have %v", table, keys)
		}
	}
}

func assertUniqueIndex(t *testing.T, db *sql.DB, table string, indexName string) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), "PRAGMA index_list("+table+")")
	if err != nil {
		t.Fatalf("QueryContext(index_list %s) error = %v", table, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Fatalf("rows.Close() error = %v", err)
		}
	}()

	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("Scan(index_list %s) error = %v", table, err)
		}
		if name == indexName {
			if unique != 1 {
				t.Fatalf("index %s unique = %d, want 1", indexName, unique)
			}
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(index_list %s) error = %v", table, err)
	}
	t.Fatalf("index %s missing on table %s", indexName, table)
}

func assertJournalModeWAL(t *testing.T, db *sql.DB) {
	t.Helper()

	var mode string
	if err := db.QueryRowContext(testutil.Context(t), "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("QueryRowContext(journal_mode) error = %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func assertSynchronousNormal(t *testing.T, db *sql.DB) {
	t.Helper()

	var synchronous int
	if err := db.QueryRowContext(testutil.Context(t), "PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("QueryRowContext(synchronous) error = %v", err)
	}
	if synchronous != 1 {
		t.Fatalf("synchronous = %d, want 1 (NORMAL)", synchronous)
	}
}

func assertWALAutoCheckpoint(t *testing.T, db *sql.DB, want int) {
	t.Helper()

	var pages int
	if err := db.QueryRowContext(testutil.Context(t), "PRAGMA wal_autocheckpoint").Scan(&pages); err != nil {
		t.Fatalf("QueryRowContext(wal_autocheckpoint) error = %v", err)
	}
	if pages != want {
		t.Fatalf("wal_autocheckpoint = %d, want %d", pages, want)
	}
}

func eventSequences(events []SessionEvent) []int64 {
	out := make([]int64, 0, len(events))
	for _, event := range events {
		out = append(out, event.Sequence)
	}
	return out
}

func metadataEventSequences(events []store.EventMetadata) []int64 {
	out := make([]int64, 0, len(events))
	for _, event := range events {
		out = append(out, event.Sequence)
	}
	return out
}

func eventTypes(events []SessionEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event.Type)
	}
	return out
}

func equalInt64Slices(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

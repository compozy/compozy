package sessiondb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	sessionschema "github.com/compozy/agh/internal/store/sessiondb/schema"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

type SessionEvent = store.SessionEvent
type TokenUsage = store.TokenUsage
type EventQuery = store.EventQuery

const SessionDatabaseName = store.SessionDatabaseName

func TestOpenSessionDBCreatesSchemaAndEnablesWAL(t *testing.T) {
	t.Parallel()

	t.Run("Should create schema and enable WAL", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-open")

		assertTablesPresent(
			t,
			sessionDB.db,
			sessionMigrationVersionTable,
			"events",
			"token_usage",
			"transcript_projection_state",
			"transcript_entries",
			"transcript_tool_routes",
		)
		assertUniqueIndex(t, sessionDB.db, "events", "idx_events_sequence")
		assertJournalModeWAL(t, sessionDB.db)
		assertSynchronousNormal(t, sessionDB.db)
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
		sessionDB, err := OpenSessionDB(ctx, "sess-passive-checkpoint", path)
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

		readOnly, err := OpenSessionDBReadOnly(ctx, "sess-passive-checkpoint", path)
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

	t.Run("Should round-trip an event and preserve stream status across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		first, err := OpenSessionDB(ctx, "sess-idempotent", path)
		if err != nil {
			t.Fatalf("OpenSessionDB(first) error = %v", err)
		}
		firstStatus, err := store.Status(ctx, first.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(first) error = %v", err)
		}
		if firstStatus.Version != 2 || firstStatus.AppliedCount != 2 {
			t.Fatalf("Status(first) = %#v, want version/applied count 2", firstStatus)
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

		second, err := OpenSessionDB(ctx, "sess-idempotent", path)
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
		assertUniqueIndex(t, second.db, "events", "idx_events_sequence")
		events, err := second.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query(after reopen) error = %v", err)
		}
		if len(events) != 1 || events[0].ID != event.ID || events[0].Content != event.Content {
			t.Fatalf("events after reopen = %#v, want preserved event %#v", events, event)
		}
	})

	t.Run("Should upgrade the recorded baseline prefix without archiving existing rows", func(t *testing.T) {
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

		upgraded, err := OpenSessionDB(ctx, "sess-prefix-upgrade", path)
		if err != nil {
			t.Fatalf("OpenSessionDB(upgrade) error = %v", err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(upgraded) error = %v", err)
			}
		})
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(upgraded) error = %v", err)
		}
		if status.Version != 2 || status.AppliedCount != 2 {
			t.Fatalf("Status(upgraded) = %#v, want version/applied count 2", status)
		}
		events, err := upgraded.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query(upgraded) error = %v", err)
		}
		if len(events) != 1 || events[0].ID != "event-before-archive-column" || events[0].Archived {
			t.Fatalf("Query(upgraded) = %#v, want preserved unarchived baseline row", events)
		}
	})
}

func previousSessionMigrationStream(t *testing.T) store.MigrationStream {
	t.Helper()

	const baselineAtlasSum = "h1:xcXDJKI/zrIp6qX1fIBm1Ey1BRZMDZnaNAw2z8+yW9A=\n" +
		"00001_baseline.sql h1:bXOy48LeBzy7PLWzCXQW/1CdBi+3LQtUySjgT8uUN0E=\n"
	baseline, err := fs.ReadFile(sessionschema.Files, "migrations/00001_baseline.sql")
	if err != nil {
		t.Fatalf("read recorded session baseline: %v", err)
	}
	stream := MigrationStream()
	stream.FS = fstest.MapFS{
		"00001_baseline.sql": &fstest.MapFile{Data: baseline},
		"atlas.sum":          &fstest.MapFile{Data: []byte(baselineAtlasSum)},
	}
	stream.Dir = "."
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
}

func TestOpenSessionDBReadOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should fail without creating a missing database", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		_, err := OpenSessionDBReadOnly(ctx, "sess-read-only-missing", path)
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
		)
		before := readOnlySessionDatabaseDigest(t, path)
		beforeWAL := readOnlySessionFilePresence(t, path+"-wal")
		beforeSHM := readOnlySessionFilePresence(t, path+"-shm")

		_, err := OpenSessionDBReadOnly(testutil.Context(t), "sess-read-only-legacy", path)
		if !errors.Is(err, store.ErrLegacyDatabase) {
			t.Fatalf("OpenSessionDBReadOnly(legacy) error = %v, want ErrLegacyDatabase", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before, beforeWAL, beforeSHM)
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
		)
		before := readOnlySessionDatabaseDigest(t, path)
		beforeWAL := readOnlySessionFilePresence(t, path+"-wal")
		beforeSHM := readOnlySessionFilePresence(t, path+"-shm")

		_, err := OpenSessionDBReadOnly(testutil.Context(t), "sess-read-only-ahead", path)
		if !errors.Is(err, store.ErrSchemaAhead) {
			t.Fatalf("OpenSessionDBReadOnly(ahead) error = %v, want ErrSchemaAhead", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before, beforeWAL, beforeSHM)
	})

	t.Run("Should refuse a behind database without changing files", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open(previous session stream) error = %v", err)
		}
		if err := store.Apply(ctx, db, previousSessionMigrationStream(t)); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
			t.Fatalf("Apply(previous session stream) error = %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close(previous session stream) error = %v", err)
		}
		before := readOnlySessionDatabaseDigest(t, path)
		beforeWAL := readOnlySessionFilePresence(t, path+"-wal")
		beforeSHM := readOnlySessionFilePresence(t, path+"-shm")

		_, err = OpenSessionDBReadOnly(ctx, "sess-read-only-behind", path)
		if !errors.Is(err, store.ErrSchemaBehind) {
			t.Fatalf("OpenSessionDBReadOnly(behind) error = %v, want ErrSchemaBehind", err)
		}
		assertReadOnlySessionDatabaseUnchanged(t, path, before, beforeWAL, beforeSHM)
	})

	t.Run("Should query an existing database through the read-only contract", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, "sess-read-only-existing", path)
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

		reader, err := OpenSessionDBReadOnly(ctx, "sess-read-only-existing", path)
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

	t.Run("Should apply identical cursor bounds to full and metadata projections", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, "sess-read-only-projections", path)
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

		reader, err := OpenSessionDBReadOnly(ctx, "sess-read-only-projections", path)
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
			"sess-read-only-locked",
			path,
			func(context.Context, string, string) (*ReadOnlySessionDB, error) {
				calls++
				if calls < 3 {
					return nil, transientLock
				}
				return &ReadOnlySessionDB{sessionID: "sess-read-only-locked"}, nil
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
		if reader == nil || reader.sessionID != "sess-read-only-locked" {
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

func readOnlySessionDatabaseDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return sha256.Sum256(contents)
}

func readOnlySessionFilePresence(t *testing.T, path string) bool {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	t.Fatalf("Stat(%q) error = %v", path, err)
	return false
}

func assertReadOnlySessionDatabaseUnchanged(
	t *testing.T,
	path string,
	wantDigest [sha256.Size]byte,
	wantWAL bool,
	wantSHM bool,
) {
	t.Helper()

	if got := readOnlySessionDatabaseDigest(t, path); got != wantDigest {
		t.Fatalf("read-only refusal changed database digest: got %x want %x", got, wantDigest)
	}
	if got := readOnlySessionFilePresence(t, path+"-wal"); got != wantWAL {
		t.Fatalf("WAL presence after refusal = %t, want %t", got, wantWAL)
	}
	if got := readOnlySessionFilePresence(t, path+"-shm"); got != wantSHM {
		t.Fatalf("SHM presence after refusal = %t, want %t", got, wantSHM)
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
		reader, err := OpenSessionDBReadOnly(ctx, "sess-clear-transcript", path)
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

		db, err := openSessionSQLiteWithVacuum(ctx, path, func(context.Context, *sql.DB) error {
			return sentinel
		})
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
		first, err := OpenSessionDB(ctx, "sess-seq-shared", path)
		if err != nil {
			t.Fatalf("OpenSessionDB(first) error = %v", err)
		}
		t.Cleanup(func() {
			if err := first.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(first) error = %v", err)
			}
		})
		second, err := OpenSessionDB(ctx, "sess-seq-shared", path)
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
		writer, err := OpenSessionDB(ctx, "sess-history-read-only-after", path)
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

		reader, err := OpenSessionDBReadOnly(ctx, "sess-history-read-only-after", path)
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

	sessionDB, err := OpenSessionDB(testutil.Context(t), sessionID, filepath.Join(t.TempDir(), SessionDatabaseName))
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

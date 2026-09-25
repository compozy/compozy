package sessiondb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestTranscriptWhitespaceProjectionUpgrade(t *testing.T) {
	t.Parallel()
	t.Run("Should repair affected entries without changing unrelated persisted state", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		owner := testSessionDBOwner("sess-whitespace-upgrade")
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		opened, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
		var input []SessionEvent
		for index, chunk := range []string{"```mermaid", "\n", "flowchart LR", "\n", "```", "\n\n", "## Heading"} {
			input = append(input, canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: owner.SessionID,
				TurnID: "turn-markdown", Text: chunk,
				Timestamp: at.Add(time.Duration(index) * time.Millisecond),
			}, "coder"))
		}
		input = append(input, canonicalStoreEvent(t, acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, SessionID: owner.SessionID,
			TurnID: "turn-unrelated", Text: "Unaffected reply", Timestamp: at.Add(time.Second),
		}, "coder"))
		if _, err := opened.RecordPersistedBatch(ctx, input); err != nil {
			t.Fatal(err)
		}
		if err := opened.Close(ctx); err != nil {
			t.Fatal(err)
		}

		fixture, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		var entryKey, messageID string
		var start, updated int64
		if err := fixture.QueryRowContext(ctx, `SELECT entry_key,message_id,start_sequence,updated_sequence
		FROM transcript_entries WHERE kind='assistant' AND turn_id='turn-markdown'`).
			Scan(&entryKey, &messageID, &start, &updated); err != nil {
			t.Fatal(err)
		}
		const damaged = "```mermaidflowchart LR```## Heading"
		message := transcript.UIMessage{
			ID: messageID, Role: transcript.UIRoleAssistant,
			Parts: []transcript.UIMessagePart{{Type: "text", Text: damaged}},
		}
		encoded, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.ExecContext(
			ctx,
			`UPDATE transcript_entries SET message_json=? WHERE entry_key=?`,
			encoded,
			entryKey,
		); err != nil {
			t.Fatal(err)
		}
		var unrelatedBefore, newlineEventBefore string
		if err := fixture.QueryRowContext(ctx, `SELECT message_json FROM transcript_entries WHERE turn_id='turn-unrelated'`).
			Scan(&unrelatedBefore); err != nil {
			t.Fatal(err)
		}
		if err := fixture.QueryRowContext(ctx, `SELECT content FROM events WHERE turn_id='turn-markdown' AND sequence=2`).
			Scan(&newlineEventBefore); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.ExecContext(
			ctx,
			`UPDATE transcript_projection_state SET projection_version=1,generation=7,active_entry_key=?`,
			entryKey,
		); err != nil {
			t.Fatal(err)
		}
		if err := fixture.Close(); err != nil {
			t.Fatal(err)
		}
		pureReader, err := OpenSessionDBReadOnly(ctx, owner, path)
		if err != nil {
			t.Fatal(err)
		}
		_, readErr := pureReader.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if !errors.Is(readErr, transcript.ErrProjectionIncompatible) {
			t.Fatalf("legacy read-only page error = %v, want incompatible projection", readErr)
		}
		if err := pureReader.Close(ctx); err != nil {
			t.Fatal(err)
		}

		for range 2 {
			reopened, openErr := OpenSessionDBReadOnlyWithProjectionUpgrade(ctx, owner, path)
			if openErr != nil {
				t.Fatal(openErr)
			}
			page, pageErr := reopened.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
			if pageErr != nil || len(page.Entries) != 2 || page.Generation != 7 {
				t.Fatalf("upgraded page = %#v, %v", page, pageErr)
			}
			want := "```mermaid\nflowchart LR\n```\n\n## Heading"
			if got := transcript.UIMessageText(page.Entries[0].Message); got != want {
				t.Fatalf("repaired Markdown = %q, want %q", got, want)
			}
			if page.Entries[0].Message.ID != messageID || page.Entries[0].StartSequence != start ||
				page.Entries[0].Sequence != updated {
				t.Fatalf("transcript identity changed: %#v", page.Entries[0])
			}
			var version int
			var activeKey string
			var unrelatedAfter, newlineEventAfter string
			if err := reopened.db.QueryRowContext(ctx,
				`SELECT projection_version,active_entry_key FROM transcript_projection_state`).
				Scan(&version, &activeKey); err != nil {
				t.Fatal(err)
			}
			if err := reopened.db.QueryRowContext(ctx, `SELECT message_json FROM transcript_entries WHERE turn_id='turn-unrelated'`).
				Scan(&unrelatedAfter); err != nil {
				t.Fatal(err)
			}
			if err := reopened.db.QueryRowContext(ctx, `SELECT content FROM events WHERE turn_id='turn-markdown' AND sequence=2`).
				Scan(&newlineEventAfter); err != nil {
				t.Fatal(err)
			}
			if version != transcript.ProjectionVersion || activeKey != entryKey || unrelatedAfter != unrelatedBefore ||
				newlineEventAfter != newlineEventBefore {
				t.Fatalf("upgrade changed other state: version=%d, active=%q, unrelated=%q, event=%q",
					version, activeKey, unrelatedAfter, newlineEventAfter)
			}
			if err := reopened.Close(ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("Should repair padding around raw text chunks", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		owner := testSessionDBOwner("sess-raw-padding-upgrade")
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		opened, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatal(err)
		}
		if err := opened.Record(ctx, SessionEvent{
			TurnID: "turn-padding", Type: acp.EventTypeAgentMessage, AgentName: "coder",
			Content: "  indented prose  ", Timestamp: time.Date(2026, 9, 23, 11, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}
		if err := opened.Close(ctx); err != nil {
			t.Fatal(err)
		}
		fixture, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		var messageID string
		if err := fixture.QueryRowContext(ctx, `SELECT message_id FROM transcript_entries WHERE turn_id='turn-padding'`).
			Scan(&messageID); err != nil {
			t.Fatal(err)
		}
		oldMessage, err := json.Marshal(transcript.UIMessage{
			ID: messageID, Role: transcript.UIRoleAssistant,
			Parts: []transcript.UIMessagePart{{Type: "text", Text: "indented prose"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.ExecContext(ctx, `UPDATE transcript_entries SET message_json=?`, oldMessage); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.ExecContext(
			ctx,
			`UPDATE transcript_projection_state SET projection_version=1`,
		); err != nil {
			t.Fatal(err)
		}
		if err := fixture.Close(); err != nil {
			t.Fatal(err)
		}
		reader, err := OpenSessionDBReadOnlyWithProjectionUpgrade(ctx, owner, path)
		if err != nil {
			t.Fatal(err)
		}
		page, err := reader.TranscriptPage(ctx, transcript.PageQuery{Limit: 10})
		if err != nil || len(page.Entries) != 1 {
			t.Fatalf("upgraded page = %#v, %v", page, err)
		}
		if got := transcript.UIMessageText(page.Entries[0].Message); got != "  indented prose  " {
			t.Fatalf("repaired raw padding = %q", got)
		}
		if err := reader.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

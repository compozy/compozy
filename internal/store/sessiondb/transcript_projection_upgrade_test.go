package sessiondb

import (
	"context"
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

func TestTranscriptProjectionUpgrade(t *testing.T) {
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
	t.Run("Should wait for an active writer before upgrading and preserve state on cancellation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestSessionDB(t, "sess-upgrade-contention")
		if err := db.Record(ctx, SessionEvent{
			TurnID: "turn", Type: acp.EventTypeAgentMessage, AgentName: "coder", Content: " padded ",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `UPDATE transcript_projection_state SET projection_version=1`); err != nil {
			t.Fatal(err)
		}
		holder, err := db.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := holder.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
				t.Error(err)
			}
		})
		if _, err := holder.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			t.Fatal(err)
		}
		blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		upgradeErr := upgradeTranscriptProjection(blocked, db.db, db.SessionID())
		if _, err := holder.ExecContext(ctx, "ROLLBACK"); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(upgradeErr, context.DeadlineExceeded) {
			t.Fatalf("blocked upgrade error = %v, want deadline exceeded while waiting for writer", upgradeErr)
		}
		var version int
		if err := db.db.QueryRowContext(ctx, `SELECT projection_version FROM transcript_projection_state`).
			Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version != whitespaceProjectionVersion {
			t.Fatalf("canceled upgrade changed version to %d", version)
		}
		if err := upgradeTranscriptProjection(ctx, db.db, db.SessionID()); err != nil {
			t.Fatal(err)
		}
		page, err := db.TranscriptPage(ctx, transcript.PageQuery{})
		if err != nil || len(page.Entries) != 1 || transcript.UIMessageText(page.Entries[0].Message) != " padded " {
			t.Fatalf("retried upgrade page = %#v, %v", page, err)
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

	t.Run("Should project the Codex session-message cards a version 2 projection dropped", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		owner := testSessionDBOwner("sess-session-message-upgrade")
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		opened, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			t.Fatal(err)
		}
		// The recorded Codex shape (#742): an MCP call envelope, then a result that names no tool.
		for _, event := range []SessionEvent{
			{TurnID: "turn-send", Type: acp.EventTypeToolCall, AgentName: "coder", Content: `{"type":"tool_call",` +
				`"turn_id":"turn-send","tool_name":"mcp.compozy-hosted-tools.compozy__session_prompt",` +
				`"tool_call_id":"exec-send","tool_input":{"arguments":{"message":"PR opened","message_id":"msg-1",` +
				`"mode":"queue","session_id":"sess-target"},"server":"compozy-hosted-tools",` +
				`"tool":"compozy__session_prompt"}}`},
			{TurnID: "turn-send", Type: acp.EventTypeToolResult, AgentName: "coder", Content: `{"type":"tool_result",` +
				`"turn_id":"turn-send","status":"completed","tool_call_id":"exec-send","tool_result":{"raw_output":` +
				`{"error":null,"result":{"content":[{"text":"queued","type":"text"}],"structuredContent":{"prompt":` +
				`{"delivery":"after_turn","message_id":"msg-1","mode":"queue","reply_watch":{"id":"rw-1"},` +
				`"target_workspace_id":"ws-target"}}}}}}`},
			{TurnID: "turn-unrelated", Type: acp.EventTypeAgentMessage, AgentName: "coder", Content: "Unaffected reply"},
			// A shell command that only mentions the tool: its entry holds no card to repair.
			{TurnID: "turn-mention", Type: acp.EventTypeToolCall, AgentName: "coder", Content: `{"type":"tool_call",` +
				`"turn_id":"turn-mention","tool_name":"rg compozy__session_prompt internal","tool_call_id":"bash-1",` +
				`"tool_input":{"command":"rg compozy__session_prompt internal"}}`},
		} {
			if err := opened.Record(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
		if err := opened.Close(ctx); err != nil {
			t.Fatal(err)
		}

		fixture, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		var entryKey, projected, unrelatedBefore string
		if err := fixture.QueryRowContext(ctx, `SELECT entry_key,message_json FROM transcript_entries
		WHERE kind='assistant' AND turn_id='turn-send'`).Scan(&entryKey, &projected); err != nil {
			t.Fatal(err)
		}
		if err := fixture.QueryRowContext(ctx, `SELECT message_json FROM transcript_entries WHERE turn_id='turn-unrelated'`).
			Scan(&unrelatedBefore); err != nil {
			t.Fatal(err)
		}
		var message transcript.UIMessage
		if err := json.Unmarshal([]byte(projected), &message); err != nil {
			t.Fatal(err)
		}
		// What version 2 stored for this call: a card that names no target and never settles.
		message.Parts = append(message.Parts[:0:0], transcript.UIMessagePart{
			Type: "data-compozy-session-message", ID: "exec-send",
			Data: json.RawMessage(`{"tool_call_id":"exec-send","target_session_id":"","state":"running"}`),
		})
		damaged, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.ExecContext(ctx, `UPDATE transcript_entries SET message_json=? WHERE entry_key=?`,
			damaged, entryKey); err != nil {
			t.Fatal(err)
		}
		// Only entries holding a session-prompt call are reprojected; this sentinel must survive.
		const untouched = `{"id":"sentinel","role":"assistant","parts":[]}`
		if _, err := fixture.ExecContext(ctx, `UPDATE transcript_entries SET message_json=?
		WHERE kind='assistant' AND turn_id='turn-mention'`, untouched); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.ExecContext(
			ctx,
			`UPDATE transcript_projection_state SET projection_version=2`,
		); err != nil {
			t.Fatal(err)
		}
		if err := fixture.Close(); err != nil {
			t.Fatal(err)
		}

		want := transcript.UISessionMessagePayload{
			ToolCallID: "exec-send", TargetSessionID: "sess-target", TargetWorkspaceID: "ws-target",
			MessageID: "msg-1", Mode: "queue", Delivery: "after_turn", ReplyWatchID: "rw-1", State: "done",
		}
		// The second round reruns the upgrade over its own result, as a restart before the version
		// was recorded would: it reprojects from the ledger again, never adding or dropping a card.
		for round := range 2 {
			if round == 1 {
				rerun, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := rerun.ExecContext(
					ctx,
					`UPDATE transcript_projection_state SET projection_version=2`,
				); err != nil {
					t.Fatal(err)
				}
				if err := rerun.Close(); err != nil {
					t.Fatal(err)
				}
			}
			reader, err := OpenSessionDBReadOnlyWithProjectionUpgrade(ctx, owner, path)
			if err != nil {
				t.Fatal(err)
			}
			var projectedJSON, unrelatedAfter, mentionAfter string
			var version int
			if err := reader.db.QueryRowContext(ctx, `SELECT message_json FROM transcript_entries WHERE entry_key=?`,
				entryKey).Scan(&projectedJSON); err != nil {
				t.Fatal(err)
			}
			if err := reader.db.QueryRowContext(ctx, `SELECT projection_version FROM transcript_projection_state`).
				Scan(&version); err != nil {
				t.Fatal(err)
			}
			if err := reader.db.QueryRowContext(ctx, `SELECT message_json FROM transcript_entries WHERE turn_id='turn-unrelated'`).
				Scan(&unrelatedAfter); err != nil {
				t.Fatal(err)
			}
			if err := reader.db.QueryRowContext(ctx, `SELECT message_json FROM transcript_entries
			WHERE kind='assistant' AND turn_id='turn-mention'`).Scan(&mentionAfter); err != nil {
				t.Fatal(err)
			}
			if err := reader.Close(ctx); err != nil {
				t.Fatal(err)
			}
			var projectedMessage transcript.UIMessage
			if err := json.Unmarshal([]byte(projectedJSON), &projectedMessage); err != nil {
				t.Fatal(err)
			}
			var cards []transcript.UISessionMessagePayload
			for _, part := range projectedMessage.Parts {
				if part.Type != "data-compozy-session-message" {
					continue
				}
				var card transcript.UISessionMessagePayload
				if err := json.Unmarshal(part.Data, &card); err != nil {
					t.Fatal(err)
				}
				cards = append(cards, card)
			}
			if len(cards) != 1 || cards[0] != want {
				t.Fatalf("round %d upgraded cards = %+v, want [%+v]", round, cards, want)
			}
			if version != transcript.ProjectionVersion || unrelatedAfter != unrelatedBefore ||
				mentionAfter != untouched {
				t.Fatalf("round %d upgrade version=%d, unrelated changed=%v, mention reprojected=%v",
					round, version, unrelatedAfter != unrelatedBefore, mentionAfter != untouched)
			}
		}
	})
}

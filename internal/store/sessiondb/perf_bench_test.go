package sessiondb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func BenchmarkSessionDBQuery(b *testing.B) {
	b.ReportAllocs()

	sessionDB := openBenchmarkSessionDB(b, "sess-bench-query")
	ctx := b.Context()
	seedBenchmarkSessionEvents(b, sessionDB, 512, 64)

	b.ResetTimer()
	for b.Loop() {
		events, err := sessionDB.Query(ctx, store.EventQuery{Limit: 256})
		if err != nil {
			b.Fatalf("Query() error = %v", err)
		}
		if got, want := len(events), 256; got != want {
			b.Fatalf("len(Query()) = %d, want %d", got, want)
		}
	}
}

func BenchmarkSessionDBHistory(b *testing.B) {
	b.ReportAllocs()

	sessionDB := openBenchmarkSessionDB(b, "sess-bench-history")
	ctx := b.Context()
	seedBenchmarkSessionEvents(b, sessionDB, 512, 64)

	b.ResetTimer()
	for b.Loop() {
		turns, err := sessionDB.History(ctx, store.EventQuery{Limit: 256})
		if err != nil {
			b.Fatalf("History() error = %v", err)
		}
		if len(turns) == 0 {
			b.Fatal("History() returned no turns")
		}
	}
}

func openBenchmarkSessionDB(b *testing.B, sessionID string) *SessionDB {
	b.Helper()

	sessionDB, err := OpenSessionDB(
		b.Context(),
		testSessionDBOwner(sessionID),
		filepath.Join(b.TempDir(), store.SessionDatabaseName),
	)

	if err != nil {
		b.Fatalf("OpenSessionDB() error = %v", err)
	}
	b.Cleanup(func() {
		if err := sessionDB.Close(context.Background()); err != nil {
			b.Fatalf("Close() error = %v", err)
		}
	})
	return sessionDB
}

func seedBenchmarkSessionEvents(b *testing.B, sessionDB *SessionDB, eventCount int, turnCount int) {
	b.Helper()

	if turnCount <= 0 {
		turnCount = 1
	}

	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	callCount := 0
	sessionDB.now = func() time.Time {
		timestamp := base.Add(time.Duration(callCount) * time.Second)
		callCount++
		return timestamp
	}

	ctx := b.Context()
	for idx := range eventCount {
		event := store.SessionEvent{
			TurnID:    fmt.Sprintf("turn-%03d", idx%turnCount),
			Type:      benchmarkSessionEventType(idx),
			AgentName: benchmarkSessionAgentName(idx),
			Content:   fmt.Sprintf(`{"index":%d,"turn":"turn-%03d"}`, idx, idx%turnCount),
		}
		if err := sessionDB.Record(ctx, event); err != nil {
			b.Fatalf("Record(seed %d) error = %v", idx, err)
		}
	}
}

func benchmarkSessionEventType(idx int) string {
	switch idx % 3 {
	case 0:
		return "agent_message"
	case 1:
		return "tool_call"
	default:
		return "tool_result"
	}
}

func benchmarkSessionAgentName(idx int) string {
	if idx%2 == 0 {
		return "coder"
	}
	return "reviewer"
}

// BenchmarkSessionDBAppendLongAssistantTurn measures one append to an assistant
// turn of a fixed history size. Every iteration reopens a copy of the same
// seeded database and warms its writer with one untimed append.
func BenchmarkSessionDBAppendLongAssistantTurn(b *testing.B) {
	for _, history := range []int{200, 800} {
		b.Run(fmt.Sprintf("history=%d", history), func(b *testing.B) {
			benchmarkAppendLongAssistantTurn(b, history)
		})
	}
}

func benchmarkAppendLongAssistantTurn(b *testing.B, history int) {
	b.ReportAllocs()

	ctx := b.Context()
	owner := testSessionDBOwner("sess-bench-long-turn")
	seedPath := filepath.Join(b.TempDir(), store.SessionDatabaseName)
	seed, err := OpenSessionDB(ctx, owner, seedPath)
	if err != nil {
		b.Fatalf("OpenSessionDB(seed) error = %v", err)
	}
	for index := range history {
		recordLongTurnStep(b, seed, index)
	}
	if err := seed.Close(ctx); err != nil {
		b.Fatalf("Close(seed) error = %v", err)
	}
	seedBytes, err := os.ReadFile(seedPath)
	if err != nil {
		b.Fatalf("read seed database: %v", err)
	}

	for b.Loop() {
		b.StopTimer()
		path := filepath.Join(b.TempDir(), store.SessionDatabaseName)
		if err := os.WriteFile(path, seedBytes, 0o600); err != nil {
			b.Fatalf("copy seed database: %v", err)
		}
		sessionDB, err := OpenSessionDB(ctx, owner, path)
		if err != nil {
			b.Fatalf("OpenSessionDB(copy) error = %v", err)
		}
		recordLongTurnStep(b, sessionDB, history)
		b.StartTimer()
		recordLongTurnStep(b, sessionDB, history+1)
		b.StopTimer()
		if err := sessionDB.Close(ctx); err != nil {
			b.Fatalf("Close(copy) error = %v", err)
		}
		b.StartTimer()
	}
}

func recordLongTurnStep(b *testing.B, sessionDB *SessionDB, index int) {
	b.Helper()

	payload := fmt.Sprintf(
		`{"type":"tool_call","turn_id":"turn-long","tool_call_id":"call-%d","title":"Bash","tool_input":{"command":"go test ./..."}}`,
		index,
	)
	if index%2 == 1 {
		payload = fmt.Sprintf(
			`{"type":"tool_result","turn_id":"turn-long","tool_call_id":"call-%d","tool_result":{"content":%q}}`,
			index-1, strings.Repeat("compiled ok\n", 400),
		)
	}
	if err := sessionDB.Record(b.Context(), store.SessionEvent{
		TurnID: "turn-long", Type: benchmarkLongTurnEventType(index), AgentName: "coder", Content: payload,
	}); err != nil {
		b.Fatalf("Record(%d) error = %v", index, err)
	}
}

func benchmarkLongTurnEventType(index int) string {
	if index%2 == 0 {
		return "tool_call"
	}
	return "tool_result"
}

func BenchmarkSessionDBTranscriptChanges(b *testing.B) {
	for _, entryCount := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("entries_%d", entryCount), func(b *testing.B) {
			b.ReportAllocs()
			db := openBenchmarkSessionDB(b, "sess-bench-changes")
			seedBenchmarkTranscriptEntries(b, db, entryCount)
			b.ResetTimer()
			for b.Loop() {
				page, err := db.TranscriptChanges(b.Context(), transcript.ChangeQuery{Limit: 20})
				if err != nil {
					b.Fatal(err)
				}
				if len(page.Entries) != 20 || !page.HasMore || page.NextAfter != 20 {
					b.Fatalf("unexpected change page: %#v", page)
				}
			}
		})
	}
}

func BenchmarkSessionDBReadOnlyProjectionOpen(b *testing.B) {
	for _, entryCount := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("entries_%d", entryCount), func(b *testing.B) {
			b.ReportAllocs()
			db := openBenchmarkSessionDB(b, "sess-bench-projection-open")
			seedBenchmarkTranscriptEntries(b, db, entryCount)
			b.ResetTimer()
			for b.Loop() {
				reader, err := OpenSessionDBReadOnlyWithProjectionUpgrade(b.Context(), db.owner, db.path)
				if err != nil {
					b.Fatal(err)
				}
				if err := reader.Close(b.Context()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func seedBenchmarkTranscriptEntries(b *testing.B, db *SessionDB, count int) {
	b.Helper()
	events := make([]store.SessionEvent, count)
	at := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	for index := range count {
		events[index] = store.SessionEvent{
			ID: fmt.Sprintf("event-%d", index), TurnID: fmt.Sprintf("turn-%d", index),
			Type: "user_message", AgentName: "user", Content: `{"type":"user_message","text":"benchmark"}`,
			Timestamp: at.Add(time.Duration(index) * time.Second),
		}
	}
	if _, err := db.RecordPersistedBatch(b.Context(), events); err != nil {
		b.Fatal(err)
	}
}

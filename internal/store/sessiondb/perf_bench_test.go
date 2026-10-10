package sessiondb

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
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

func BenchmarkSessionDBAppendLongAssistantTurn(b *testing.B) {
	b.ReportAllocs()

	sessionDB := openBenchmarkSessionDB(b, "sess-bench-long-turn")
	ctx := b.Context()
	output := strings.Repeat("compiled ok\n", 400)
	record := func(index int, payload string) {
		if err := sessionDB.Record(ctx, store.SessionEvent{
			TurnID: "turn-long", Type: benchmarkLongTurnEventType(index), AgentName: "coder", Content: payload,
		}); err != nil {
			b.Fatalf("Record(%d) error = %v", index, err)
		}
	}
	appendStep := func(index int) {
		call := fmt.Sprintf("call-%d", index)
		if index%2 == 0 {
			record(index, fmt.Sprintf(
				`{"type":"tool_call","turn_id":"turn-long","tool_call_id":%q,"title":"Bash","tool_input":{"command":"go test ./..."}}`,
				call,
			))
			return
		}
		record(index, fmt.Sprintf(
			`{"type":"tool_result","turn_id":"turn-long","tool_call_id":%q,"tool_result":{"content":%q}}`,
			fmt.Sprintf("call-%d", index-1), output,
		))
	}
	for index := range 600 {
		appendStep(index)
	}

	b.ResetTimer()
	index := 600
	for b.Loop() {
		appendStep(index)
		index++
	}
}

func benchmarkLongTurnEventType(index int) string {
	if index%2 == 0 {
		return "tool_call"
	}
	return "tool_result"
}

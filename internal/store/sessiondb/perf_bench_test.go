package sessiondb

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
)

func BenchmarkSessionDBQuery(b *testing.B) {
	b.ReportAllocs()

	sessionDB := openBenchmarkSessionDB(b, "sess-bench-query")
	ctx := context.Background()
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
	ctx := context.Background()
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
		context.Background(),
		sessionID,
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

	ctx := context.Background()
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

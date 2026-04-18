package rundb

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func BenchmarkRunDBListEventsFromCursor(b *testing.B) {
	ctx := context.Background()
	runID := "bench-run-events"
	db := openBenchmarkRunDB(b, runID)
	defer func() {
		_ = db.Close()
	}()

	seedRunDBBenchmarkEvents(b, db, runID, 10000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := db.ListEvents(ctx, 5000, 256)
		if err != nil {
			b.Fatalf("ListEvents(): %v", err)
		}
		if len(items.Events) == 0 {
			b.Fatal("ListEvents() returned no rows")
		}
	}
}

func seedRunDBBenchmarkEvents(b testing.TB, db *RunDB, runID string, total int) {
	b.Helper()

	batch := make([]events.Event, 0, 128)
	startedAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	for seq := 1; seq <= total; seq++ {
		payload := kinds.SessionUpdatePayload{
			Index: 1,
			Update: kinds.SessionUpdate{
				Kind:   kinds.UpdateKindAgentMessageChunk,
				Status: kinds.StatusRunning,
				Blocks: []kinds.ContentBlock{mustBenchmarkTextBlock(b, fmt.Sprintf("chunk-%05d", seq))},
			},
		}
		batch = append(
			batch,
			mustBenchmarkEvent(
				b,
				runID,
				uint64(seq),
				startedAt.Add(time.Duration(seq)*time.Millisecond),
				events.EventKindSessionUpdate,
				payload,
			),
		)
		if len(batch) < cap(batch) {
			continue
		}
		if err := db.StoreEventBatch(context.Background(), batch); err != nil {
			b.Fatalf("StoreEventBatch(seed): %v", err)
		}
		batch = batch[:0]
	}
	if len(batch) > 0 {
		if err := db.StoreEventBatch(context.Background(), batch); err != nil {
			b.Fatalf("StoreEventBatch(final seed): %v", err)
		}
	}
}

func openBenchmarkRunDB(tb testing.TB, runID string) *RunDB {
	tb.Helper()
	return openBenchmarkRunDBAtPath(tb, fmt.Sprintf("%s/%s/run.db", tb.TempDir(), runID))
}

func openBenchmarkRunDBAtPath(tb testing.TB, path string) *RunDB {
	tb.Helper()

	db, err := openWithOptions(
		context.Background(),
		path,
		openOptions{
			now: func() time.Time {
				return time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)
			},
		},
	)
	if err != nil {
		tb.Fatalf("openWithOptions(): %v", err)
	}
	return db
}

func mustBenchmarkEvent(
	tb testing.TB,
	runID string,
	seq uint64,
	timestamp time.Time,
	kind events.EventKind,
	payload any,
) events.Event {
	tb.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		tb.Fatalf("marshal payload: %v", err)
	}
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     timestamp.UTC(),
		Kind:          kind,
		Payload:       raw,
	}
}

func mustBenchmarkTextBlock(tb testing.TB, text string) kinds.ContentBlock {
	tb.Helper()

	block, err := kinds.NewContentBlock(kinds.TextBlock{
		Type: kinds.BlockText,
		Text: text,
	})
	if err != nil {
		tb.Fatalf("NewContentBlock(): %v", err)
	}
	return block
}

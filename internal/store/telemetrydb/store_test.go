package telemetrydb

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestTelemetryCleanupOptions(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		options       []OpenOption
		wantBatchSize int64
	}{
		{name: "Should complete cleanup with the default batch", wantBatchSize: 256},
		{name: "Should complete every configured cleanup batch", options: []OpenOption{WithCleanupBatchSize(2)}, wantBatchSize: 2},
		{name: "Should use the default for a zero batch", options: []OpenOption{WithCleanupBatchSize(0)}, wantBatchSize: 256},
		{name: "Should use the default for a negative batch", options: []OpenOption{WithCleanupBatchSize(-1)}, wantBatchSize: 256},
		{name: "Should ignore a nil option", options: []OpenOption{nil}, wantBatchSize: 256},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db, err := Open(ctx, filepath.Join(t.TempDir(), "telemetry.db"), testCase.options...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			if db.cleanupBatchSize != testCase.wantBatchSize {
				t.Fatalf("cleanup batch size = %d, want %d", db.cleanupBatchSize, testCase.wantBatchSize)
			}
			cutoff := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
			summaries := make([]store.EventSummary, 0, 11)
			for i := range 7 {
				summaries = append(summaries, store.EventSummary{
					ID: fmt.Sprintf("expired-%d", i), ProfileID: "a", WorkspaceID: "a",
					Type: "settings.changed", Timestamp: cutoff.Add(-time.Nanosecond),
				})
			}
			for _, scope := range []struct{ profile, workspace string }{
				{profile: "a", workspace: "a"},
				{profile: "a", workspace: "b"},
				{profile: "b", workspace: "a"},
				{profile: "b", workspace: "b"},
			} {
				summaries = append(summaries, store.EventSummary{
					ID: scope.profile + "-" + scope.workspace, ProfileID: scope.profile, WorkspaceID: scope.workspace,
					Type: "settings.changed", Timestamp: cutoff,
				})
			}
			if err := db.Write(ctx, summaries, func(context.Context) error { return nil }); err != nil {
				t.Fatal(err)
			}
			if deleted, err := db.Sweep(ctx, cutoff); err != nil || deleted != 7 {
				t.Fatalf("sweep deleted = %d, %v, want 7", deleted, err)
			}
			if deleted, err := db.DeleteScope(ctx, "workspace", "a"); err != nil || deleted != 2 {
				t.Fatalf("workspace cleanup deleted = %d, %v, want 2", deleted, err)
			}
			if deleted, err := db.DeleteScope(ctx, "profile", "b"); err != nil || deleted != 1 {
				t.Fatalf("profile cleanup deleted = %d, %v, want 1", deleted, err)
			}
			var count int
			var id string
			if err := db.readDB.QueryRowContext(ctx, "SELECT COUNT(*), MIN(id) FROM event_summaries").
				Scan(&count, &id); err != nil {
				t.Fatal(err)
			}
			if count != 1 || id != "a-b" {
				t.Fatalf("remaining summaries = %d, %q, want only a-b", count, id)
			}
		})
	}
}

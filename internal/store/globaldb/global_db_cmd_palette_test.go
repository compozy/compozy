package globaldb

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBCmdPalettePersonalization(t *testing.T) {
	t.Parallel()

	t.Run("Should persist normalized usage, query history, and idempotent pins per workspace", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		usedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		for _, workspaceID := range []cmdpalette.WorkspaceID{"workspace-a", "workspace-b"} {
			if err := database.RecordCmdPaletteUsage(ctx, cmdpalette.Usage{
				WorkspaceID: workspaceID,
				CommandID:   "session.new",
				Query:       "  Séssão   NOVA ",
				UsedAt:      usedAt,
			}, cmdpalette.WeightsV1); err != nil {
				t.Fatalf("RecordCmdPaletteUsage(%q) error = %v", workspaceID, err)
			}
			if err := database.PutCmdPalettePin(ctx, workspaceID, "session.new", time.Time{}); err != nil {
				t.Fatalf("PutCmdPalettePin(%q) error = %v", workspaceID, err)
			}
			if err := database.PutCmdPalettePin(ctx, workspaceID, "session.new", usedAt.Add(time.Hour)); err != nil {
				t.Fatalf("PutCmdPalettePin(%q idempotent) error = %v", workspaceID, err)
			}
		}

		rows, err := database.CmdPalettePersonalization(ctx, "workspace-a")
		if err != nil {
			t.Fatalf("CmdPalettePersonalization() error = %v", err)
		}
		if len(rows.Usage) != 1 || rows.Usage[0].UseCount != 1 || rows.Usage[0].Weight != 1 {
			t.Fatalf("usage rows = %#v, want one first-use signal", rows.Usage)
		}
		if len(rows.QueryHits) != 1 || rows.QueryHits[0].Query != "sessao nova" {
			t.Fatalf("query rows = %#v, want normalized pre-selection query", rows.QueryHits)
		}
		if len(rows.Pins) != 1 || rows.Pins[0].PinnedAt < 0 {
			t.Fatalf("pin rows = %#v, want one valid idempotent pin", rows.Pins)
		}
		other, err := database.CmdPalettePersonalization(ctx, "workspace-b")
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(workspace-b) error = %v", err)
		}
		if len(other.Usage) != 1 || len(other.QueryHits) != 1 || len(other.Pins) != 1 {
			t.Fatalf("workspace-b rows = %#v, want independent signals", other)
		}
	})

	t.Run("Should serialize concurrent usage increments without losing writes", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		const writes = 12
		errorsByWrite := make(chan error, writes)
		var group sync.WaitGroup
		group.Add(writes)
		for index := range writes {
			go func() {
				defer group.Done()
				errorsByWrite <- database.RecordCmdPaletteUsage(
					context.Background(),
					cmdpalette.Usage{
						WorkspaceID: "workspace-race",
						CommandID:   "session.new",
						Query:       "new session",
						UsedAt:      time.Date(2026, 8, 19, 12, index, 0, 0, time.UTC),
					},
					cmdpalette.WeightsV1,
				)
			}()
		}
		group.Wait()
		close(errorsByWrite)
		for err := range errorsByWrite {
			if err != nil {
				t.Fatalf("RecordCmdPaletteUsage(concurrent) error = %v", err)
			}
		}

		rows, err := database.CmdPalettePersonalization(testutil.Context(t), "workspace-race")
		if err != nil {
			t.Fatalf("CmdPalettePersonalization() error = %v", err)
		}
		if len(rows.Usage) != 1 || rows.Usage[0].UseCount != writes {
			t.Fatalf("usage rows = %#v, want use_count %d", rows.Usage, writes)
		}
		if len(rows.QueryHits) != 1 || rows.QueryHits[0].Weight <= 1 {
			t.Fatalf("query rows = %#v, want accumulated query weight", rows.QueryHits)
		}
	})

	t.Run("Should reset only the selected workspace", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		for _, workspaceID := range []cmdpalette.WorkspaceID{"workspace-reset", "workspace-keep"} {
			if err := database.RecordCmdPaletteUsage(ctx, cmdpalette.Usage{
				WorkspaceID: workspaceID, CommandID: "session.new", Query: "new",
			}, cmdpalette.WeightsV1); err != nil {
				t.Fatalf("RecordCmdPaletteUsage(%q) error = %v", workspaceID, err)
			}
			if err := database.PutCmdPalettePin(ctx, workspaceID, "session.new", time.Time{}); err != nil {
				t.Fatalf("PutCmdPalettePin(%q) error = %v", workspaceID, err)
			}
		}
		if err := database.ResetCmdPalettePersonalization(ctx, "workspace-reset"); err != nil {
			t.Fatalf("ResetCmdPalettePersonalization() error = %v", err)
		}
		resetRows, err := database.CmdPalettePersonalization(ctx, "workspace-reset")
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(reset) error = %v", err)
		}
		if len(resetRows.Usage)+len(resetRows.QueryHits)+len(resetRows.Pins) != 0 {
			t.Fatalf("reset rows = %#v, want empty", resetRows)
		}
		keptRows, err := database.CmdPalettePersonalization(ctx, "workspace-keep")
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(kept) error = %v", err)
		}
		if len(keptRows.Usage) != 1 || len(keptRows.QueryHits) != 1 || len(keptRows.Pins) != 1 {
			t.Fatalf("kept rows = %#v, want unaffected workspace", keptRows)
		}
	})

	t.Run("Should survive reopen and cascade when the owning workspace is deleted", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		workspaceID := cmdpalette.WorkspaceID(registerWorkspaceForGlobalTests(
			t, first, "palette-cascade", filepath.Join(t.TempDir(), "palette-cascade"),
		))
		if err := first.RecordCmdPaletteUsage(ctx, cmdpalette.Usage{
			WorkspaceID: workspaceID, CommandID: "session.new", Query: "new",
		}, cmdpalette.WeightsV1); err != nil {
			t.Fatalf("RecordCmdPaletteUsage() error = %v", err)
		}
		if err := first.PutCmdPalettePin(ctx, workspaceID, "session.new", time.Time{}); err != nil {
			t.Fatalf("PutCmdPalettePin() error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		rows, err := reopened.CmdPalettePersonalization(ctx, workspaceID)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(reopen) error = %v", err)
		}
		if len(rows.Usage) != 1 || len(rows.QueryHits) != 1 || len(rows.Pins) != 1 {
			t.Fatalf("reopened rows = %#v, want persisted signals", rows)
		}
		if err := reopened.DeleteWorkspace(ctx, string(workspaceID)); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		rows, err = reopened.CmdPalettePersonalization(ctx, workspaceID)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(after delete) error = %v", err)
		}
		if len(rows.Usage)+len(rows.QueryHits)+len(rows.Pins) != 0 {
			t.Fatalf("rows after workspace delete = %#v, want cascade cleanup", rows)
		}
	})
}

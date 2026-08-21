package globaldb

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

// Invariant: command-palette ranking and pins are partitioned by an explicit real-profile or aggregate lens.
// The GlobalDB command-palette suite owns durable identity, validation, and migration-backed trigger behavior.
func TestGlobalDBCmdPalettePersonalization(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate real-profile and aggregate lenses and reject unknown owners", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		profileID := cmdpalette.ProfileLensID("01ARZ3NDEKTSV4RRFFQ69G5FAV")
		now := time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC)
		if _, err := database.db.ExecContext(
			ctx,
			`INSERT INTO profiles (id, name, color, icon, state, created_at)
			 VALUES (?, 'marketing', '#8E8EB5', 'circle', 'active', ?)`,
			profileID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert profile owner error = %v", err)
		}
		for _, lens := range []cmdpalette.ProfileLensID{
			cmdpalette.DefaultProfileLensID,
			profileID,
			cmdpalette.AggregateProfileLensID,
		} {
			if err := database.RecordCmdPaletteUsage(ctx, cmdpalette.Usage{
				ProfileLens: cmdPaletteUsageLens(lens),
				WorkspaceID: "workspace-lens",
				CommandID:   "session.new",
				UsedAt:      now,
			}, cmdpalette.WeightsV1); err != nil {
				t.Fatalf("RecordCmdPaletteUsage(%q) error = %v", lens, err)
			}
		}
		for _, lens := range []cmdpalette.ProfileLensID{
			cmdpalette.DefaultProfileLensID,
			profileID,
			cmdpalette.AggregateProfileLensID,
		} {
			rows, err := database.CmdPalettePersonalization(ctx, lens, "workspace-lens")
			if err != nil {
				t.Fatalf("CmdPalettePersonalization(%q) error = %v", lens, err)
			}
			if len(rows.Usage) != 1 || rows.Usage[0].UseCount != 1 {
				t.Fatalf("lens %q usage = %#v, want one isolated signal", lens, rows.Usage)
			}
		}
		if err := database.RecordCmdPaletteUsage(ctx, cmdpalette.Usage{
			ProfileLens: cmdPaletteUsageLens("01ARZ3NDEKTSV4RRFFQ69G5FAW"),
			WorkspaceID: "workspace-lens",
			CommandID:   "session.new",
			UsedAt:      now,
		}, cmdpalette.WeightsV1); err == nil || !strings.Contains(
			err.Error(),
			"profile_lens_not_found",
		) {
			t.Fatalf(
				"RecordCmdPaletteUsage(unknown profile) error = %v, want profile-lens validation error",
				err,
			)
		}
	})

	t.Run("Should persist normalized usage, query history, and idempotent pins per workspace", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		usedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		for _, workspaceID := range []cmdpalette.WorkspaceID{"workspace-a", "workspace-b"} {
			if err := database.RecordCmdPaletteUsage(ctx, cmdpalette.Usage{
				ProfileLens: cmdPaletteUsageLens(cmdpalette.DefaultProfileLensID),
				WorkspaceID: workspaceID,
				CommandID:   "session.new",
				Query:       "  Séssão   NOVA ",
				UsedAt:      usedAt,
			}, cmdpalette.WeightsV1); err != nil {
				t.Fatalf("RecordCmdPaletteUsage(%q) error = %v", workspaceID, err)
			}
			if err := database.PutCmdPalettePin(
				ctx, cmdpalette.DefaultProfileLensID, workspaceID, "session.new", usedAt,
			); err != nil {
				t.Fatalf("PutCmdPalettePin(%q) error = %v", workspaceID, err)
			}
			if err := database.PutCmdPalettePin(
				ctx, cmdpalette.DefaultProfileLensID, workspaceID, "session.new", usedAt.Add(time.Hour),
			); err != nil {
				t.Fatalf("PutCmdPalettePin(%q idempotent) error = %v", workspaceID, err)
			}
		}

		rows, err := database.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-a",
		)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization() error = %v", err)
		}
		if len(rows.Usage) != 1 || rows.Usage[0].UseCount != 1 || rows.Usage[0].Weight != 1 {
			t.Fatalf("usage rows = %#v, want one first-use signal", rows.Usage)
		}
		if len(rows.QueryHits) != 1 || rows.QueryHits[0].Query != "sessao nova" {
			t.Fatalf("query rows = %#v, want normalized pre-selection query", rows.QueryHits)
		}
		if len(rows.Pins) != 1 || rows.Pins[0].PinnedAt != usedAt.UnixMilli() {
			t.Fatalf("pin rows = %#v, want original timestamp %d", rows.Pins, usedAt.UnixMilli())
		}
		other, err := database.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-b",
		)
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
		ctx := testutil.Context(t)
		const writes = 12
		errorsByWrite := make(chan error, writes)
		var group sync.WaitGroup
		group.Add(writes)
		for index := range writes {
			go func() {
				defer group.Done()
				errorsByWrite <- database.RecordCmdPaletteUsage(
					ctx,
					cmdpalette.Usage{
						ProfileLens: cmdPaletteUsageLens(cmdpalette.DefaultProfileLensID),
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

		rows, err := database.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-race",
		)
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

	t.Run("Should persist trimmed pin identities", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		usedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		if err := database.PutCmdPalettePin(
			ctx, cmdpalette.DefaultProfileLensID, " workspace-trim ", " session.new ", usedAt,
		); err != nil {
			t.Fatalf("PutCmdPalettePin(padded) error = %v", err)
		}
		rows, err := database.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-trim",
		)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization() error = %v", err)
		}
		if len(rows.Pins) != 1 || rows.Pins[0].CommandID != "session.new" ||
			rows.Pins[0].PinnedAt != usedAt.UnixMilli() {
			t.Fatalf("pin rows = %#v, want trimmed identity", rows.Pins)
		}
		if err := database.DeleteCmdPalettePin(
			ctx, cmdpalette.DefaultProfileLensID, " workspace-trim ", " session.new ",
		); err != nil {
			t.Fatalf("DeleteCmdPalettePin(padded) error = %v", err)
		}
		rows, err = database.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-trim",
		)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(after delete) error = %v", err)
		}
		if len(rows.Pins) != 0 {
			t.Fatalf("pin rows after delete = %#v, want empty", rows.Pins)
		}
	})

	t.Run("Should reset only the selected workspace", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		for _, workspaceID := range []cmdpalette.WorkspaceID{"workspace-reset", "workspace-keep"} {
			if err := database.RecordCmdPaletteUsage(ctx, cmdpalette.Usage{
				ProfileLens: cmdPaletteUsageLens(cmdpalette.DefaultProfileLensID),
				WorkspaceID: workspaceID, CommandID: "session.new", Query: "new",
			}, cmdpalette.WeightsV1); err != nil {
				t.Fatalf("RecordCmdPaletteUsage(%q) error = %v", workspaceID, err)
			}
			if err := database.PutCmdPalettePin(
				ctx, cmdpalette.DefaultProfileLensID, workspaceID, "session.new", time.Time{},
			); err != nil {
				t.Fatalf("PutCmdPalettePin(%q) error = %v", workspaceID, err)
			}
		}
		if err := database.ResetCmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-reset",
		); err != nil {
			t.Fatalf("ResetCmdPalettePersonalization() error = %v", err)
		}
		resetRows, err := database.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-reset",
		)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(reset) error = %v", err)
		}
		if len(resetRows.Usage)+len(resetRows.QueryHits)+len(resetRows.Pins) != 0 {
			t.Fatalf("reset rows = %#v, want empty", resetRows)
		}
		keptRows, err := database.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, "workspace-keep",
		)
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
			ProfileLens: cmdPaletteUsageLens(cmdpalette.DefaultProfileLensID),
			WorkspaceID: workspaceID, CommandID: "session.new", Query: "new",
		}, cmdpalette.WeightsV1); err != nil {
			t.Fatalf("RecordCmdPaletteUsage() error = %v", err)
		}
		if err := first.PutCmdPalettePin(
			ctx, cmdpalette.DefaultProfileLensID, workspaceID, "session.new", time.Time{},
		); err != nil {
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
		rows, err := reopened.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, workspaceID,
		)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(reopen) error = %v", err)
		}
		if len(rows.Usage) != 1 || len(rows.QueryHits) != 1 || len(rows.Pins) != 1 {
			t.Fatalf("reopened rows = %#v, want persisted signals", rows)
		}
		if err := reopened.DeleteWorkspace(ctx, string(workspaceID)); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		rows, err = reopened.CmdPalettePersonalization(
			ctx, cmdpalette.DefaultProfileLensID, workspaceID,
		)
		if err != nil {
			t.Fatalf("CmdPalettePersonalization(after delete) error = %v", err)
		}
		if len(rows.Usage)+len(rows.QueryHits)+len(rows.Pins) != 0 {
			t.Fatalf("rows after workspace delete = %#v, want cascade cleanup", rows)
		}
	})
}

func cmdPaletteUsageLens(id cmdpalette.ProfileLensID) cmdpalette.ProfileLens {
	if id == cmdpalette.AggregateProfileLensID {
		return cmdpalette.AggregateProfileLens()
	}
	name := "marketing"
	if id == cmdpalette.DefaultProfileLensID {
		name = "default"
	}
	return cmdpalette.ScopedProfileLens(id, name)
}

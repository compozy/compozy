package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

func TestTokenUsageReceiptSurvivesPreLineageMigrationOrdinaryDelete(t *testing.T) {
	t.Run("Should retain standalone usage and admission after upgrading an old parent deletion", func(t *testing.T) {
		ctx := globalMigrationTestContext(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00216_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		prefixClosed := false
		t.Cleanup(func() {
			if !prefixClosed {
				if err := prefix.Close(); err != nil {
					t.Errorf("close migration prefix: %v", err)
				}
			}
		})
		const root, parent, child = "legacy-budget-root", "legacy-budget-parent", "legacy-budget-child"
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		for _, row := range []struct{ id, parent, lineage string }{
			{root, "", ""}, {parent, root, "spawn"}, {child, parent, "spawn"},
		} {
			if _, err := prefix.ExecContext(ctx, `INSERT INTO sessions
				(id, profile_id, scope, agent_name, workspace_id, session_type, state,
				 parent_session_id, root_session_id, lineage_kind, created_at, updated_at)
				VALUES (?, ?, 'global', 'writer', '', 'user', 'stopped', NULLIF(?,''), ?, ?, ?, ?)`,
				row.id, store.DefaultProfileID, row.parent, root, row.lineage, stamp, stamp); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := prefix.ExecContext(ctx, "DELETE FROM sessions WHERE id=?", parent); err != nil {
			t.Fatal(err)
		}
		telemetry, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		bindErr := bindTelemetryDestination(ctx, prefix, telemetry.DB())
		if err := errors.Join(bindErr, telemetry.Close(ctx)); err != nil {
			t.Fatal(err)
		}
		if err := prefix.Close(); err != nil {
			t.Fatal(err)
		}
		prefixClosed = true
		db, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close upgraded ledger: %v", err)
			}
		})
		allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, child)
		if err != nil || !allowed || trip != nil {
			t.Fatalf("standalone admission allowed=%t trip=%+v err=%v", allowed, trip, err)
		}
		stats := store.TokenStatsUpdate{SessionID: child, TurnID: "legacy-completion",
			AgentName: "writer", TotalTokens: new(int64(9)), CostStatus: "unknown", CostSource: "none", UpdatedAt: at}
		daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: store.LocalDay(at),
			AgentName: stats.AgentName, TotalTokens: stats.TotalTokens, CostStatus: stats.CostStatus,
			CostSource: stats.CostSource, Turns: 1, UpdatedAt: at}
		for _, wantAccepted := range []bool{true, false} {
			accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily)
			if err != nil || accepted != wantAccepted {
				t.Fatalf("standalone receipt accepted=%t err=%v want accepted=%t", accepted, err, wantAccepted)
			}
		}
		var usage int64
		if err := db.DB().QueryRowContext(ctx, "SELECT total_tokens FROM token_stats WHERE session_id=?", child).
			Scan(&usage); err != nil || usage != 9 {
			t.Fatalf("standalone aggregate tokens=%d err=%v", usage, err)
		}
		if owner, err := db.DailyBudgetSessionOwner(
			ctx,
			child,
		); err != nil || owner.InstanceID != "" ||
			owner.Unresolved {
			t.Fatalf("standalone owner=%+v err=%v", owner, err)
		}
	})
}

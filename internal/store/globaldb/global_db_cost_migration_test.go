package globaldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

// Suite: global token-cost schema migration
// Invariant: the v18 migration preserves cost rows and assigns truthful legacy provenance.
// Boundary IN: a v17 global database with cost-bearing and cost-free token aggregates.
// Boundary OUT: observer estimation and public session/task projections.
func TestGlobalDBTokenCostMigration(t *testing.T) {
	t.Run("Should preserve token costs and assign legacy provenance", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, tokenCostMigrationPrefix(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v17 prefix) error = %v", err)
		}
		ctx := testutil.Context(t)
		prefixGlobalDB := &GlobalDB{db: prefixDB, path: path, now: tokenCostMigrationTestTime}
		prefixGlobalDB.initializeRepositories(openConfig{})
		registerSessionForGlobalTests(t, prefixGlobalDB, "sess-cost-migration")
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO token_stats (
		id, session_id, agent_name, total_tokens, total_cost, cost_currency, turn_count, updated_at
	) VALUES
		('tok-cost', 'sess-cost-migration', 'priced', 10, 1.25, 'USD', 1, '2026-07-15T12:00:00Z'),
		('tok-none', 'sess-cost-migration', 'silent', 5, NULL, NULL, 1, '2026-07-15T12:01:00Z')`); err != nil {
			t.Fatalf("seed v17 token_stats error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v18 upgrade) error = %v", err)
		}
		ctx = testutil.Context(t)
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("Close(upgraded) error = %v", err)
		}
		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})

		stats, err := reopened.ListTokenStats(ctx, store.TokenStatsQuery{SessionID: "sess-cost-migration"})
		if err != nil {
			t.Fatalf("ListTokenStats(after migration) error = %v", err)
		}
		if got, want := len(stats), 2; got != want {
			t.Fatalf("len(stats) = %d, want %d", got, want)
		}
		byAgent := make(map[string]store.TokenStats, len(stats))
		for _, stat := range stats {
			byAgent[stat.AgentName] = stat
		}
		priced := byAgent["priced"]
		if priced.TotalCost == nil || *priced.TotalCost != 1.25 ||
			priced.CostCurrency == nil || *priced.CostCurrency != "USD" ||
			priced.CostStatus != "actual" || priced.CostSource != "agent_reported" {
			t.Fatalf("priced migration row = %#v, want preserved actual/agent_reported USD 1.25", priced)
		}
		silent := byAgent["silent"]
		if silent.TotalCost != nil || silent.CostCurrency != nil ||
			silent.CostStatus != "unknown" || silent.CostSource != "none" {
			t.Fatalf("silent migration row = %#v, want unknown/none without amount", silent)
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(global) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})
}

func tokenCostMigrationPrefix(t *testing.T) store.MigrationStream {
	t.Helper()
	return globalMigrationPrefix(t,
		"00001_baseline.sql",
		"00002_schema.sql",
		"00003_schema.sql",
		"00004_schema.sql",
		"00005_schema.sql",
		"00006_schema.sql",
		"00007_schema.sql",
		"00008_network_participation.sql",
		"00009_automation_network_participation.sql",
		"00010_network_participation_hardening.sql",
		"00011_schema.sql",
		"00012_schema.sql",
		"00013_network_subscription_hard_cut.sql",
		"00014_network_task_status_projections.sql",
		"00015_schema.sql",
		"00016_schema.sql",
		"00017_schema.sql",
	)
}

func tokenCostMigrationTestTime() time.Time {
	return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
}

package globaldb

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/store/telemetrydb"
	"github.com/compozy/compozy/internal/transcript"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

// Suite: global token-cost schema migration
// Invariant: the v18 migration preserves cost rows and assigns truthful legacy provenance.
// Boundary IN: a v17 global database with cost-bearing and cost-free token aggregates.
// Boundary OUT: observer estimation and public session/task projections.
func TestGlobalDBTokenCostMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve token costs and assign legacy provenance", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, tokenCostMigrationPrefix(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v17 prefix) error = %v", err)
		}
		ctx := testutil.Context(t)
		seedTokenCostMigrationSession(ctx, t, prefixDB, "sess-cost-migration")
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

		seedCostRepairLedger(t, path, "sess-cost-migration", "sess-cost-migration-workspace", "priced", []float64{1.25})
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

func seedTokenCostMigrationSession(ctx context.Context, t *testing.T, db *sql.DB, sessionID string) {
	t.Helper()

	if _, err := db.ExecContext(ctx, `INSERT INTO workspaces (
		id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
	) VALUES (
		'sess-cost-migration-workspace', '/tmp/sess-cost-migration', '[]',
		'sess-cost-migration', '', '', '2026-07-15T12:00:00Z', '2026-07-15T12:00:00Z'
	)`); err != nil {
		t.Fatalf("seed v17 workspace error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (
		id, agent_name, provider, workspace_id, session_type, state,
		network_spec_json, network_mode, network_source, created_at, updated_at
	) VALUES (
		?, 'coder', 'claude', 'sess-cost-migration-workspace', 'user', 'active',
		'{"version":"network-participation/v1","mode":"local","source":"built_in_local"}',
		'local', 'built_in_local', '2026-07-15T12:00:00Z', '2026-07-15T12:00:00Z'
	)`, sessionID); err != nil {
		t.Fatalf("seed v17 session error = %v", err)
	}
}

func seedCostRepairLedger(t *testing.T, globalPath, sessionID, workspaceID, agent string, amounts []float64) {
	t.Helper()
	ledgerPath := store.SessionDBFile(filepath.Join(filepath.Dir(globalPath), "sessions", sessionID))
	ledger, err := sessiondb.OpenSessionDB(
		t.Context(),
		store.SessionDBOwner{SessionID: sessionID, WorkspaceID: workspaceID},
		ledgerPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	var previous float64
	for index, amount := range amounts {
		cost := amount
		if amount >= previous {
			cost -= previous
		}
		previous = amount
		turnID := fmt.Sprintf("cost-turn-%d", index)
		timestamp := time.Date(2026, 7, 15, 12, index, 0, 0, time.UTC)
		usage := acp.TokenUsage{
			TurnID:             turnID,
			CostAmount:         new(cost),
			ReportedCostAmount: new(amount),
			CostCurrency:       new("USD"),
			Timestamp:          timestamp,
		}
		payload, err := transcript.MarshalAgentEvent(
			acp.AgentEvent{Type: acp.EventTypeDone, TurnID: turnID, Usage: &usage},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := ledger.Record(
			t.Context(),
			store.SessionEvent{
				TurnID:    turnID,
				Type:      acp.EventTypeDone,
				AgentName: agent,
				Content:   payload,
				Timestamp: timestamp,
			},
		); err != nil {
			t.Fatal(err)
		}
		if err := ledger.RecordTokenUsage(
			t.Context(),
			store.TokenUsage{
				TurnID:             turnID,
				CostAmount:         new(cost),
				ReportedCostAmount: new(amount),
				CostCurrency:       new("USD"),
				Timestamp:          timestamp,
			},
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := ledger.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// Invariant: migration 199 repairs complete monetary aggregates and leaves
// incomplete history unknown without changing tokens, counts, IDs or timestamps.
func TestGlobalDBUsageCostRepair(t *testing.T) {
	t.Parallel()
	// Invariant: cost repair cannot invalidate clear recovery's byte hashes.
	for _, suffix := range []string{".clear-manifest", ".clear-committed", ".clear-backup", "-wal.clear-backup", "-shm.clear-backup", "-journal.clear-backup"} {
		t.Run("Should leave a ledger awaiting clear recovery untouched "+suffix, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), GlobalDatabaseName)
			owner := store.SessionDBOwner{SessionID: "sess-clear", WorkspaceID: "ws-clear"}
			seedCostRepairLedger(t, path, owner.SessionID, owner.WorkspaceID, "coder", []float64{1, 3})
			ledgerPath := store.SessionDBFile(filepath.Join(filepath.Dir(path), "sessions", owner.SessionID))
			before, err := os.ReadFile(ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			marker := []byte("pending recovery evidence")
			if err := os.WriteFile(ledgerPath+suffix, marker, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := migrationSessionUsage(t.Context(), path, owner, "coder"); err == nil {
				t.Fatal("pending clear ledger was opened")
			}
			after, err := os.ReadFile(ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := os.ReadFile(ledgerPath + suffix)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) || !bytes.Equal(marker, retained) {
				t.Fatal("clear recovery evidence changed")
			}
		})
	}
	t.Run("Should repair complete history and invalidate missing history across reopen", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		old, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00199_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := old.ExecContext(
			ctx,
			`INSERT INTO workspaces(id,root_dir,name,created_at,updated_at) VALUES ('ws-cost','/tmp/cost-history','cost-history','2026-07-15T12:00:00Z','2026-07-15T12:00:00Z')`,
		); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"sess-repaired", "sess-missing"} {
			agent := "coder"
			if id == "sess-missing" {
				agent = "missing"
			}
			if _, err := old.ExecContext(
				ctx,
				`INSERT INTO sessions(id,profile_id,agent_name,provider,workspace_id,state,created_at,updated_at) VALUES (?,'00000000000000000000000000','coder','claude','ws-cost','stopped','2026-07-15T12:00:00Z','2026-07-15T12:03:00Z')`,
				id,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := old.ExecContext(
				ctx,
				`INSERT INTO token_stats(id,session_id,agent_name,total_tokens,total_cost,cost_currency,cost_status,cost_source,turn_count,updated_at) VALUES (?,?,?,20,6.5,'USD','actual','agent_reported',4,'2026-07-15T12:03:00Z')`,
				id,
				id,
				agent,
			); err != nil {
				t.Fatal(err)
			}
		}
		day := store.LocalDay(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC))
		if _, err := old.ExecContext(
			ctx,
			`INSERT INTO token_usage_daily(day,profile_id,workspace_id,agent_name,total_tokens,total_cost,cost_currency,cost_status,cost_source,turn_count,updated_at) VALUES (?,'00000000000000000000000000','ws-cost','coder',20,6.5,'USD','actual','agent_reported',4,'2026-07-15T12:03:00Z')`,
			day,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := old.ExecContext(
			ctx,
			`INSERT INTO token_usage_daily(day,profile_id,workspace_id,agent_name,total_tokens,total_cost,cost_currency,cost_status,cost_source,turn_count,updated_at) VALUES (?,'00000000000000000000000000','ws-cost','missing',20,6.5,'USD','actual','agent_reported',4,'2026-07-15T12:03:00Z')`,
			day,
		); err != nil {
			t.Fatal(err)
		}
		preserved := []costProvenanceFixture{
			{"included", "included", "none", nil, nil},
			{"estimated-config", "estimated", "catalog_config", 2.75, "USD"},
			{"estimated-models", "estimated", "models_dev", 3.25, "EUR"},
			{"estimated-builtin", "estimated", "builtin", 4.25, "USD"},
			{"unknown", "unknown", "none", nil, nil},
		}
		for _, fixture := range preserved {
			seedPreservedCostProvenance(t, old, day, fixture)
		}
		// This daily row shares an agent-reported session/turn key: restricting
		// only the session selection would still overwrite its valid estimate.
		if _, err := old.ExecContext(
			ctx,
			`INSERT INTO token_usage_daily(day,profile_id,workspace_id,agent_name,total_tokens,total_cost,cost_currency,cost_status,cost_source,turn_count,updated_at) VALUES (?,'00000000000000000000000000','ws-cost','protected-daily',20,2.75,'USD','estimated','catalog_config',4,'2026-07-15T12:03:00Z')`,
			day,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := old.ExecContext(
			ctx,
			`INSERT INTO sessions(id,profile_id,agent_name,provider,workspace_id,state,created_at,updated_at) VALUES ('sess-daily-protected','00000000000000000000000000','protected-daily','claude','ws-cost','stopped','2026-07-15T12:00:00Z','2026-07-15T12:03:00Z')`,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := old.ExecContext(
			ctx,
			`INSERT INTO token_stats(id,session_id,agent_name,total_tokens,total_cost,cost_currency,cost_status,cost_source,turn_count,updated_at) VALUES ('daily-only','sess-daily-protected','protected-daily',20,6.5,'USD','actual','agent_reported',4,'2026-07-15T12:03:00Z')`,
		); err != nil {
			t.Fatal(err)
		}
		telemetry, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		if err := bindTelemetryDestination(ctx, old, telemetry.DB()); err != nil {
			t.Fatal(err)
		}
		if err := old.Close(); err != nil {
			t.Fatal(err)
		}
		if err := telemetry.Close(ctx); err != nil {
			t.Fatal(err)
		}
		seedCostRepairLedger(t, path, "sess-repaired", "ws-cost", "coder", []float64{1, 3, 0.5, 2})
		seedCostRepairLedger(t, path, "sess-daily-protected", "ws-cost", "protected-daily", []float64{1, 3, 0.5, 2})
		for _, fixture := range preserved {
			seedCostRepairLedger(t, path, "sess-"+fixture.agent, "ws-cost", fixture.agent, []float64{1, 3, 0.5, 2})
		}
		for range 2 {
			db, err := openGlobalMigrationUpgrade(t, path)
			if err != nil {
				t.Fatal(err)
			}
			stats, err := db.ListTokenStats(ctx, store.TokenStatsQuery{})
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range stats {
				if row.TotalTokens == nil || *row.TotalTokens != 20 || row.TurnCount != 4 ||
					!row.UpdatedAt.Equal(time.Date(2026, 7, 15, 12, 3, 0, 0, time.UTC)) {
					t.Fatalf("non-cost fields changed: %+v", row)
				}
				if row.SessionID == "sess-repaired" &&
					(row.TotalCost == nil || *row.TotalCost != 5 || row.CostStatus != "actual") {
					t.Fatalf("repaired=%+v", row)
				}
				if row.SessionID == "sess-missing" &&
					(row.TotalCost != nil || row.CostStatus != "unknown" || row.CostSource != "none") {
					t.Fatalf("missing=%+v", row)
				}
			}
			for _, fixture := range preserved {
				assertPreservedCostProvenance(t, db.db, day, fixture)
			}
			assertPreservedDailyCost(
				t,
				db.db,
				day,
				costProvenanceFixture{"protected-daily", "estimated", "catalog_config", 2.75, "USD"},
			)
			var amount sql.NullFloat64
			var status string
			var turns, tokens int64
			if err := db.db.QueryRowContext(ctx, `SELECT total_cost,cost_status,turn_count,total_tokens FROM token_usage_daily WHERE day=? AND agent_name='coder'`, day).
				Scan(&amount, &status, &turns, &tokens); err != nil {
				t.Fatal(err)
			}
			if !amount.Valid || amount.Float64 != 5 || status != "actual" || turns != 4 || tokens != 20 {
				t.Fatalf("daily=%+v %s %d %d", amount, status, turns, tokens)
			}
			var missingAmount sql.NullFloat64
			var missingStatus, missingSource string
			if err := db.db.QueryRowContext(ctx, `SELECT total_cost,cost_status,cost_source FROM token_usage_daily WHERE day=? AND agent_name='missing'`, day).
				Scan(&missingAmount, &missingStatus, &missingSource); err != nil {
				t.Fatal(err)
			}
			if missingAmount.Valid || missingStatus != "unknown" || missingSource != "none" {
				t.Fatalf("unrecoverable daily=%+v %s %s", missingAmount, missingStatus, missingSource)
			}
			if err := db.Close(ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// Fixtures intentionally contain reported ledger amounts too: ownership comes
// from each aggregate's original provenance, never a coincidentally matching key.
type costProvenanceFixture struct {
	agent, status, source string
	amount, currency      any
}

func seedPreservedCostProvenance(t *testing.T, db *sql.DB, day string, fixture costProvenanceFixture) {
	t.Helper()
	ctx := t.Context()
	id := "sess-" + fixture.agent
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO sessions(id,profile_id,agent_name,provider,workspace_id,state,created_at,updated_at) VALUES (?,'00000000000000000000000000',?,'codex','ws-cost','stopped','2026-07-15T12:00:00Z','2026-07-15T12:03:00Z')`,
		id,
		fixture.agent,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO token_stats(id,session_id,agent_name,total_tokens,total_cost,cost_currency,cost_status,cost_source,turn_count,updated_at) VALUES (?,?,?,20,?,?,?,?,4,'2026-07-15T12:03:00Z')`,
		id,
		id,
		fixture.agent,
		fixture.amount,
		fixture.currency,
		fixture.status,
		fixture.source,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO token_usage_daily(day,profile_id,workspace_id,agent_name,total_tokens,total_cost,cost_currency,cost_status,cost_source,turn_count,updated_at) VALUES (?,'00000000000000000000000000','ws-cost',?,20,?,?,?,?,4,'2026-07-15T12:03:00Z')`,
		day,
		fixture.agent,
		fixture.amount,
		fixture.currency,
		fixture.status,
		fixture.source,
	); err != nil {
		t.Fatal(err)
	}
}

func assertPreservedCostProvenance(t *testing.T, db *sql.DB, day string, fixture costProvenanceFixture) {
	t.Helper()
	var same bool
	if err := db.QueryRowContext(t.Context(), `SELECT total_cost IS ? AND cost_currency IS ? AND cost_status = ? AND cost_source = ? FROM token_stats WHERE id = ?`, fixture.amount, fixture.currency, fixture.status, fixture.source, "sess-"+fixture.agent).
		Scan(&same); err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatalf("session provenance changed for %s", fixture.agent)
	}
	assertPreservedDailyCost(t, db, day, fixture)
}

func assertPreservedDailyCost(t *testing.T, db *sql.DB, day string, fixture costProvenanceFixture) {
	t.Helper()
	var same bool
	if err := db.QueryRowContext(t.Context(), `SELECT total_cost IS ? AND cost_currency IS ? AND cost_status = ? AND cost_source = ? FROM token_usage_daily WHERE day = ? AND agent_name = ?`, fixture.amount, fixture.currency, fixture.status, fixture.source, day, fixture.agent).
		Scan(&same); err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatalf("daily provenance changed for %s", fixture.agent)
	}
}

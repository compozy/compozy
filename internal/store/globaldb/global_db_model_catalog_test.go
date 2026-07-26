package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

// Suite: global model-catalog pricing migration
// Invariant: v18 cached rows retain identity and legacy prices while new rates remain absent.
// Boundary IN: a production v18 GlobalDB prefix with one cached source row.
// Boundary OUT: head repository reads after close and reopen.
func TestGlobalDBModelCatalogPricingMigration(t *testing.T) {
	t.Run("Should preserve cached model prices through the head upgrade", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, modelCatalogPricingMigrationPrefix(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v18 prefix) error = %v", err)
		}
		ctx := testutil.Context(t)
		assertTableExcludesColumns(t, prefixDB, "model_catalog_rows", []string{
			"cost_cache_read_per_million",
			"cost_cache_write_per_million",
			"cost_reasoning_per_million",
		})
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_sources (
		source_id, provider_id, source_kind, priority, refresh_state, row_count, stale
	) VALUES ('config', 'codex', 'config', 120, 'succeeded', 1, 0)`); err != nil {
			t.Fatalf("seed v18 model_catalog_sources error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_rows (
		source_id, provider_id, model_id, source_kind, priority,
		cost_input_per_million, cost_output_per_million
	) VALUES ('config', 'codex', 'gpt-5.4', 'config', 120, 1.25, 10.5)`); err != nil {
			t.Fatalf("seed v18 model_catalog_rows error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v19 upgrade) error = %v", err)
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

		rows, err := reopened.ListRows(ctx, modelcatalog.ListOptions{
			ProviderID: "codex", SourceID: "config", IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("ListRows(after migration) error = %v", err)
		}
		if len(rows) != 1 || rows[0].SourceID != "config" || rows[0].ProviderID != "codex" ||
			rows[0].ModelID != "gpt-5.4" || rows[0].CostInputPerMillion == nil ||
			*rows[0].CostInputPerMillion != 1.25 || rows[0].CostOutputPerMillion == nil ||
			*rows[0].CostOutputPerMillion != 10.5 || rows[0].CostCacheReadPerMillion != nil ||
			rows[0].CostCacheWritePerMillion != nil || rows[0].CostReasoningPerMillion != nil {
			t.Fatalf("ListRows(after migration) = %#v, want preserved identity/legacy prices and nil new rates", rows)
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(global) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})
}

func modelCatalogPricingMigrationPrefix(t *testing.T) store.MigrationStream {
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
		"00018_schema.sql",
	)
}

func TestGlobalDBModelCatalogStore(t *testing.T) {
	t.Parallel()

	t.Run("Should join reasoning-effort scan and rows-close errors", func(t *testing.T) {
		t.Parallel()

		globalDB := openQueryRowsCloseErrorGlobalDB(t)
		_, err := listModelCatalogReasoningEfforts(
			testutil.Context(t),
			globalDB.db,
			modelcatalog.ListOptions{},
		)
		if !errors.Is(err, errQueryRowsClose) ||
			!strings.Contains(err.Error(), "scan model catalog reasoning effort") {
			t.Fatalf(
				"listModelCatalogReasoningEfforts() error = %v, want joined scan and rows-close errors",
				err,
			)
		}
	})

	t.Run("Should replace source rows and reasoning efforts atomically", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		first := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
		first.ReasoningEfforts = []modelcatalog.ReasoningEffort{
			modelcatalog.ReasoningEffortLow,
			modelcatalog.ReasoningEffortHigh,
		}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{first},
		)

		second := modelCatalogRow("config", "codex", "gpt-5.5", modelcatalog.SourceKindConfig, 120)
		second.ExplicitlyCurated = true
		second.ReasoningEfforts = []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortMinimal}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{second},
		)

		rows, err := globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config", IncludeStale: true},
		)
		if err != nil {
			t.Fatalf("ListRows() error = %v", err)
		}
		if got, want := len(rows), 1; got != want {
			t.Fatalf("len(rows) = %d, want %d: %#v", got, want, rows)
		}
		if rows[0].ModelID != "gpt-5.5" || !rows[0].ExplicitlyCurated ||
			!slices.Equal(rows[0].ReasoningEfforts, second.ReasoningEfforts) {
			t.Fatalf("rows[0] = %#v, want replacement row with minimal effort", rows[0])
		}

		var oldEffortCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM model_catalog_reasoning_efforts WHERE source_id = ? AND provider_id = ? AND model_id = ?`,
			"config",
			"codex",
			"gpt-5.4",
		).Scan(&oldEffortCount); err != nil {
			t.Fatalf("QueryRowContext(old efforts) error = %v", err)
		}
		if oldEffortCount != 0 {
			t.Fatalf("old effort count = %d, want 0", oldEffortCount)
		}
		statuses, err := globalDB.ListSourceStatus(ctx, "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		if len(statuses) != 1 || statuses[0].RowCount != 1 {
			t.Fatalf("statuses = %#v, want one status with row_count 1", statuses)
		}
	})

	t.Run("Should preserve nullable five-rate pricing through source replacement", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		row := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
		cacheRead := 0.125
		cacheWrite := 2.5
		row.CostCacheReadPerMillion = &cacheRead
		row.CostCacheWritePerMillion = &cacheWrite
		row.CostReasoningPerMillion = nil
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{row},
		)

		rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
			ProviderID: "codex", SourceID: "config", IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("ListRows() error = %v", err)
		}
		if len(rows) != 1 || rows[0].CostCacheReadPerMillion == nil ||
			*rows[0].CostCacheReadPerMillion != cacheRead ||
			rows[0].CostCacheWritePerMillion == nil ||
			*rows[0].CostCacheWritePerMillion != cacheWrite ||
			rows[0].CostReasoningPerMillion != nil {
			t.Fatalf("ListRows() five-rate pricing = %#v, want distinct values and nil reasoning", rows)
		}
	})

	t.Run("Should roll back source replacement when reasoning effort insert fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		original := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
		original.ReasoningEfforts = []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortLow}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{original},
		)

		invalid := modelCatalogRow("config", "codex", "gpt-5.5", modelcatalog.SourceKindConfig, 120)
		invalid.ReasoningEfforts = []modelcatalog.ReasoningEffort{
			modelcatalog.ReasoningEffortHigh,
			modelcatalog.ReasoningEffortHigh,
		}
		err := globalDB.ReplaceSourceRows(
			ctx,
			"config",
			"codex",
			[]modelcatalog.ModelRow{invalid},
			modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
		)
		if err == nil {
			t.Fatal("ReplaceSourceRows(duplicate efforts) error = nil, want constraint error")
		}

		rows, err := globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config", IncludeStale: true},
		)
		if err != nil {
			t.Fatalf("ListRows(after failed replace) error = %v", err)
		}
		if len(rows) != 1 || rows[0].ModelID != "gpt-5.4" ||
			!slices.Equal(rows[0].ReasoningEfforts, original.ReasoningEfforts) {
			t.Fatalf("rows after failed replace = %#v, want original row preserved", rows)
		}
		statuses, err := globalDB.ListSourceStatus(ctx, "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus(after failed replace) error = %v", err)
		}
		if len(statuses) != 1 || statuses[0].RowCount != 1 {
			t.Fatalf("statuses after failed replace = %#v, want original row_count 1", statuses)
		}
	})

	t.Run("Should persist empty optional source timestamps", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		status := modelCatalogStatus("builtin", "blackbox", modelcatalog.SourceKindBuiltin, 10)
		status.NextRefresh = time.Time{}
		row := modelCatalogRow("builtin", "blackbox", "default", modelcatalog.SourceKindBuiltin, 10)
		row.RefreshedAt = time.Time{}
		row.ExpiresAt = time.Time{}
		if err := globalDB.ReplaceSourceRows(
			ctx,
			"builtin",
			"blackbox",
			[]modelcatalog.ModelRow{row},
			status,
		); err != nil {
			t.Fatalf("ReplaceSourceRows() error = %v", err)
		}

		var nextRefresh string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT next_refresh_at FROM model_catalog_sources WHERE source_id = ? AND provider_id = ?`,
			"builtin",
			"blackbox",
		).Scan(&nextRefresh); err != nil {
			t.Fatalf("QueryRowContext(next_refresh_at) error = %v", err)
		}
		if nextRefresh != "" {
			t.Fatalf("next_refresh_at = %q, want empty string", nextRefresh)
		}
		var refreshedAt string
		var expiresAt string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT refreshed_at, expires_at FROM model_catalog_rows WHERE source_id = ? AND provider_id = ?`,
			"builtin",
			"blackbox",
		).Scan(&refreshedAt, &expiresAt); err != nil {
			t.Fatalf("QueryRowContext(optional row timestamps) error = %v", err)
		}
		if refreshedAt != "" || expiresAt != "" {
			t.Fatalf("optional row timestamps = %q/%q, want empty strings", refreshedAt, expiresAt)
		}
	})

	t.Run("Should filter rows by provider source and stale flag", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		fresh := modelCatalogRow("config", "codex", "codex-fresh", modelcatalog.SourceKindConfig, 120)
		stale := modelCatalogRow("config", "codex", "codex-stale", modelcatalog.SourceKindConfig, 120)
		stale.Stale = true
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{fresh, stale},
		)
		dev := modelCatalogRow("models_dev", "codex", "codex-dev", modelcatalog.SourceKindModelsDev, 50)
		replaceModelCatalogRows(
			t,
			globalDB,
			"models_dev",
			"codex",
			modelcatalog.SourceKindModelsDev,
			50,
			[]modelcatalog.ModelRow{dev},
		)
		claude := modelCatalogRow("config", "claude", "claude-fresh", modelcatalog.SourceKindConfig, 120)
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"claude",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{claude},
		)

		rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config"})
		if err != nil {
			t.Fatalf("ListRows(fresh config) error = %v", err)
		}
		assertModelCatalogModelIDs(t, rows, []string{"codex-fresh"})

		rows, err = globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config", IncludeStale: true},
		)
		if err != nil {
			t.Fatalf("ListRows(include stale) error = %v", err)
		}
		assertModelCatalogModelIDs(t, rows, []string{"codex-fresh", "codex-stale"})

		rows, err = globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "models_dev", IncludeAll: true},
		)
		if err != nil {
			t.Fatalf("ListRows(models_dev) error = %v", err)
		}
		assertModelCatalogModelIDs(t, rows, []string{"codex-dev"})

		rows, err = globalDB.ListRows(ctx, modelcatalog.ListOptions{ProviderID: "claude", IncludeStale: true})
		if err != nil {
			t.Fatalf("ListRows(claude) error = %v", err)
		}
		assertModelCatalogModelIDs(t, rows, []string{"claude-fresh"})
	})

	t.Run("Should store models dev status per provider without empty provider sentinel", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		replaceModelCatalogRows(t, globalDB, "models_dev", "codex", modelcatalog.SourceKindModelsDev, 50, nil)
		replaceModelCatalogRows(t, globalDB, "models_dev", "claude", modelcatalog.SourceKindModelsDev, 50, nil)

		statuses, err := globalDB.ListSourceStatus(ctx, "")
		if err != nil {
			t.Fatalf("ListSourceStatus(all) error = %v", err)
		}
		if got, want := len(statuses), 2; got != want {
			t.Fatalf("len(statuses) = %d, want %d: %#v", got, want, statuses)
		}
		for _, status := range statuses {
			if status.SourceID != "models_dev" {
				t.Fatalf("status.SourceID = %q, want models_dev", status.SourceID)
			}
			if status.ProviderID == "" {
				t.Fatalf("status has empty provider sentinel: %#v", status)
			}
		}
		codexStatuses, err := globalDB.ListSourceStatus(ctx, "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus(codex) error = %v", err)
		}
		if len(codexStatuses) != 1 || codexStatuses[0].ProviderID != "codex" {
			t.Fatalf("codex statuses = %#v, want one codex status", codexStatuses)
		}
	})

	t.Run("Should reject empty provider source status sentinel writes", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		err := globalDB.ReplaceSourceRows(
			testutil.Context(t),
			"models_dev",
			"",
			nil,
			modelcatalog.SourceStatus{
				SourceID:   "models_dev",
				SourceKind: modelcatalog.SourceKindModelsDev,
				ProviderID: "",
				Priority:   50,
			},
		)
		if err == nil || !strings.Contains(err.Error(), "provider id is required") {
			t.Fatalf("ReplaceSourceRows(empty provider) error = %v, want provider id validation", err)
		}
	})

	t.Run("Should reject mismatched source status and row identities", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name   string
			rows   []modelcatalog.ModelRow
			status modelcatalog.SourceStatus
			want   string
		}{
			{
				name:   "Should reject mismatched status source",
				status: modelCatalogStatus("other", "codex", modelcatalog.SourceKindConfig, 120),
				want:   "status source id",
			},
			{
				name:   "Should reject missing status source kind",
				status: modelcatalog.SourceStatus{SourceID: "config", ProviderID: "codex", Priority: 120},
				want:   "source kind is required",
			},
			{
				name: "Should reject mismatched row provider",
				rows: []modelcatalog.ModelRow{
					modelCatalogRow("config", "claude", "gpt-5.4", modelcatalog.SourceKindConfig, 120),
				},
				status: modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
				want:   "provider id",
			},
			{
				name: "Should reject mismatched row source kind",
				rows: []modelcatalog.ModelRow{
					modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindModelsDev, 120),
				},
				status: modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
				want:   "source kind",
			},
			{
				name: "Should reject blank reasoning effort",
				rows: []modelcatalog.ModelRow{
					func() modelcatalog.ModelRow {
						row := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
						row.ReasoningEfforts = []modelcatalog.ReasoningEffort{""}
						return row
					}(),
				},
				status: modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
				want:   "reasoning effort",
			},
			{
				name: "Should reject a non-finite reasoning rate",
				rows: []modelcatalog.ModelRow{
					func() modelcatalog.ModelRow {
						row := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
						row.CostReasoningPerMillion = new(math.NaN())
						return row
					}(),
				},
				status: modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
				want:   "cost reasoning per million must be finite and non-negative",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				globalDB := openTestGlobalDB(t)
				err := globalDB.ReplaceSourceRows(testutil.Context(t), "config", "codex", tc.rows, tc.status)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("ReplaceSourceRows() error = %v, want containing %q", err, tc.want)
				}
			})
		}
	})

	t.Run("Should reject nil contexts for catalog store methods", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		nilCtx := nilModelCatalogTestContext()
		if err := globalDB.ReplaceSourceRows(
			nilCtx,
			"config",
			"codex",
			nil,
			modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
		); err == nil {
			t.Fatal("ReplaceSourceRows(nil context) error = nil, want validation error")
		}
		if _, err := globalDB.ListRows(nilCtx, modelcatalog.ListOptions{}); err == nil {
			t.Fatal("ListRows(nil context) error = nil, want validation error")
		}
		if _, err := globalDB.ListSourceStatus(nilCtx, "codex"); err == nil {
			t.Fatal("ListSourceStatus(nil context) error = nil, want validation error")
		}
	})

	t.Run("Should preserve null default reasoning effort", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		row := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
		row.ReasoningEfforts = []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortLow}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{row},
		)

		var raw sql.NullString
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT default_reasoning_effort FROM model_catalog_rows WHERE source_id = ? AND provider_id = ? AND model_id = ?`,
			"config",
			"codex",
			"gpt-5.4",
		).Scan(&raw); err != nil {
			t.Fatalf("QueryRowContext(default_reasoning_effort) error = %v", err)
		}
		if raw.Valid {
			t.Fatalf("default_reasoning_effort raw = %#v, want NULL", raw)
		}
		rows, err := globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config", IncludeStale: true},
		)
		if err != nil {
			t.Fatalf("ListRows() error = %v", err)
		}
		if len(rows) != 1 || rows[0].DefaultReasoningEffort != nil {
			t.Fatalf("rows = %#v, want nil DefaultReasoningEffort", rows)
		}
	})

	t.Run("Should round trip nullable booleans and explicit default reasoning effort", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		available := false
		supportsReasoning := false
		defaultEffort := modelcatalog.ReasoningEffortHigh
		row := modelCatalogRow("", "", "manual-model", "", 120)
		row.Available = &available
		row.SupportsTools = nil
		row.SupportsReasoning = &supportsReasoning
		row.DefaultReasoningEffort = &defaultEffort
		deprecated := true
		hidden := false
		row.Deprecated = &deprecated
		row.Hidden = &hidden
		row.Featured = nil
		releaseDate := "2026-06-26"
		row.ReleaseDate = &releaseDate
		row.ReasoningEfforts = []modelcatalog.ReasoningEffort{
			modelcatalog.ReasoningEffortLow,
			modelcatalog.ReasoningEffortHigh,
		}
		status := modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120)
		status.RefreshState = ""
		if err := globalDB.ReplaceSourceRows(ctx, "config", "codex", []modelcatalog.ModelRow{row}, status); err != nil {
			t.Fatalf("ReplaceSourceRows() error = %v", err)
		}

		statuses, err := globalDB.ListSourceStatus(ctx, "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		if len(statuses) != 1 || statuses[0].RefreshState != modelcatalog.RefreshStateIdle {
			t.Fatalf("statuses = %#v, want default idle refresh state", statuses)
		}
		rows, err := globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config", IncludeStale: true},
		)
		if err != nil {
			t.Fatalf("ListRows() error = %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %#v, want one row", rows)
		}
		got := rows[0]
		if got.SourceID != "config" || got.ProviderID != "codex" || got.SourceKind != modelcatalog.SourceKindConfig {
			t.Fatalf("row identity = %#v, want normalized source/provider/kind", got)
		}
		if got.Available == nil || *got.Available || got.SupportsTools != nil ||
			got.SupportsReasoning == nil || *got.SupportsReasoning ||
			got.DefaultReasoningEffort == nil || *got.DefaultReasoningEffort != modelcatalog.ReasoningEffortHigh {
			t.Fatalf("row nullable values = %#v, want false/nil/false/high", got)
		}
		if got.Deprecated == nil || !*got.Deprecated || got.Hidden == nil || *got.Hidden ||
			got.Featured != nil || got.ReleaseDate == nil ||
			*got.ReleaseDate != releaseDate {
			t.Fatalf("row curation values = %#v, want true/false/nil/%q", got, releaseDate)
		}
	})

	t.Run("Should update source status row count stale flag and last error", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		row := modelCatalogRow("provider_live:codex", "codex", "gpt-5.4", modelcatalog.SourceKindProviderLive, 110)
		replaceModelCatalogRows(
			t,
			globalDB,
			"provider_live:codex",
			"codex",
			modelcatalog.SourceKindProviderLive,
			110,
			[]modelcatalog.ModelRow{row},
		)

		failed := modelCatalogStatus("provider_live:codex", "codex", modelcatalog.SourceKindProviderLive, 110)
		failed.RefreshState = modelcatalog.RefreshStateFailed
		failed.LastError = "redacted refresh failed"
		failed.Stale = true
		if err := globalDB.ReplaceSourceRows(ctx, "provider_live:codex", "codex", nil, failed); err != nil {
			t.Fatalf("ReplaceSourceRows(failed) error = %v", err)
		}

		statuses, err := globalDB.ListSourceStatus(ctx, "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		if len(statuses) != 1 {
			t.Fatalf("statuses = %#v, want one status", statuses)
		}
		status := statuses[0]
		if status.RowCount != 0 || !status.Stale || status.LastError != "redacted refresh failed" ||
			status.RefreshState != modelcatalog.RefreshStateFailed {
			t.Fatalf("status = %#v, want failed stale status with row_count 0", status)
		}
		rows, err := globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "provider_live:codex", IncludeStale: true},
		)
		if err != nil {
			t.Fatalf("ListRows(after failed replace) error = %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("rows after failed replace = %#v, want empty", rows)
		}
	})

	t.Run("Should order equal freshness rows by source identity", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		refreshedAt := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
		zulu := modelCatalogRow("extension:z_source", "codex", "gpt-5.4", modelcatalog.SourceKindExtension, 100)
		zulu.RefreshedAt = refreshedAt
		alpha := modelCatalogRow("extension:a_source", "codex", "gpt-5.4", modelcatalog.SourceKindExtension, 100)
		alpha.RefreshedAt = refreshedAt
		replaceModelCatalogRows(
			t,
			globalDB,
			"extension:z_source",
			"codex",
			modelcatalog.SourceKindExtension,
			100,
			[]modelcatalog.ModelRow{zulu},
		)
		replaceModelCatalogRows(
			t,
			globalDB,
			"extension:a_source",
			"codex",
			modelcatalog.SourceKindExtension,
			100,
			[]modelcatalog.ModelRow{alpha},
		)

		rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{ProviderID: "codex", IncludeStale: true})
		if err != nil {
			t.Fatalf("ListRows() error = %v", err)
		}
		if got, want := len(rows), 2; got != want {
			t.Fatalf("len(rows) = %d, want %d: %#v", got, want, rows)
		}
		gotSources := []string{rows[0].SourceID, rows[1].SourceID}
		if !slices.Equal(gotSources, []string{"extension:a_source", "extension:z_source"}) {
			t.Fatalf("source order = %#v, want extension:a_source before extension:z_source", gotSources)
		}
	})

	t.Run("Should surface corrupt persisted catalog timestamps and booleans", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		row := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{row},
		)

		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE model_catalog_rows SET refreshed_at = ? WHERE source_id = ? AND provider_id = ? AND model_id = ?`,
			"bad-timestamp",
			"config",
			"codex",
			"gpt-5.4",
		); err != nil {
			t.Fatalf("ExecContext(corrupt row timestamp) error = %v", err)
		}
		if _, err := globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config", IncludeStale: true},
		); err == nil ||
			!strings.Contains(err.Error(), "refreshed_at") {
			t.Fatalf("ListRows(corrupt timestamp) error = %v, want refreshed_at parse error", err)
		}

		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE model_catalog_rows SET refreshed_at = ?, available = ? WHERE source_id = ? AND provider_id = ? AND model_id = ?`,
			store.FormatTimestamp(row.RefreshedAt),
			2,
			"config",
			"codex",
			"gpt-5.4",
		); err == nil {
			t.Fatal("ExecContext(corrupt boolean with checks enabled) error = nil, want constraint error")
		}
		if _, err := globalDB.db.ExecContext(ctx, `PRAGMA ignore_check_constraints = ON`); err != nil {
			t.Fatalf("enable ignore_check_constraints error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE model_catalog_rows SET refreshed_at = ?, available = ? WHERE source_id = ? AND provider_id = ? AND model_id = ?`,
			store.FormatTimestamp(row.RefreshedAt),
			2,
			"config",
			"codex",
			"gpt-5.4",
		); err != nil {
			t.Fatalf("ExecContext(corrupt boolean) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `PRAGMA ignore_check_constraints = OFF`); err != nil {
			t.Fatalf("disable ignore_check_constraints error = %v", err)
		}
		if _, err := globalDB.ListRows(
			ctx,
			modelcatalog.ListOptions{ProviderID: "codex", SourceID: "config", IncludeStale: true},
		); err == nil ||
			!strings.Contains(err.Error(), "available boolean") {
			t.Fatalf("ListRows(corrupt boolean) error = %v, want available boolean error", err)
		}

		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE model_catalog_sources SET last_refresh_at = ? WHERE source_id = ? AND provider_id = ?`,
			"bad-status-time",
			"config",
			"codex",
		); err != nil {
			t.Fatalf("ExecContext(corrupt status timestamp) error = %v", err)
		}
		if _, err := globalDB.ListSourceStatus(ctx, "codex"); err == nil ||
			!strings.Contains(err.Error(), "last_refresh_at") {
			t.Fatalf("ListSourceStatus(corrupt timestamp) error = %v, want last_refresh_at parse error", err)
		}
	})

	t.Run("Should read rows and reasoning efforts from one transaction snapshot", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)

		first := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
		first.ReasoningEfforts = []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortLow}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{first},
		)

		conn, err := globalDB.db.Conn(ctx)
		if err != nil {
			t.Fatalf("db.Conn() error = %v", err)
		}
		defer func() {
			if closeErr := conn.Close(); closeErr != nil {
				t.Errorf("conn.Close() error = %v", closeErr)
			}
		}()

		tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatalf("BeginTx() error = %v", err)
		}
		defer func() {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				t.Errorf("tx.Rollback() error = %v", rollbackErr)
			}
		}()

		var rowCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_catalog_rows`).Scan(&rowCount); err != nil {
			t.Fatalf("QueryRowContext(count) error = %v", err)
		}
		if got, want := rowCount, 1; got != want {
			t.Fatalf("rowCount = %d, want %d", got, want)
		}

		second := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
		second.ReasoningEfforts = []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortHigh}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"codex",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{second},
		)

		rows, err := listModelCatalogRows(ctx, tx, modelcatalog.ListOptions{
			ProviderID:   "codex",
			SourceID:     "config",
			IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("listModelCatalogRows(snapshot) error = %v", err)
		}
		if got, want := len(rows), 1; got != want {
			t.Fatalf("len(rows) = %d, want %d: %#v", got, want, rows)
		}
		if !slices.Equal(rows[0].ReasoningEfforts, first.ReasoningEfforts) {
			t.Fatalf("snapshot ReasoningEfforts = %#v, want %#v", rows[0].ReasoningEfforts, first.ReasoningEfforts)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("tx.Commit() error = %v", err)
		}

		freshRows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
			ProviderID:   "codex",
			SourceID:     "config",
			IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("ListRows(fresh) error = %v", err)
		}
		if !slices.Equal(freshRows[0].ReasoningEfforts, second.ReasoningEfforts) {
			t.Fatalf("fresh ReasoningEfforts = %#v, want %#v", freshRows[0].ReasoningEfforts, second.ReasoningEfforts)
		}
	})
}

func replaceModelCatalogRows(
	t *testing.T,
	globalDB *GlobalDB,
	sourceID string,
	providerID string,
	sourceKind modelcatalog.SourceKind,
	priority int,
	rows []modelcatalog.ModelRow,
) {
	t.Helper()

	if err := globalDB.ReplaceSourceRows(
		testutil.Context(t),
		sourceID,
		providerID,
		rows,
		modelCatalogStatus(sourceID, providerID, sourceKind, priority),
	); err != nil {
		t.Fatalf("ReplaceSourceRows(%s/%s) error = %v", sourceID, providerID, err)
	}
}

func modelCatalogRow(
	sourceID string,
	providerID string,
	modelID string,
	sourceKind modelcatalog.SourceKind,
	priority int,
) modelcatalog.ModelRow {
	available := true
	supportsTools := true
	supportsReasoning := true
	contextWindow := int64(256000)
	maxOutputTokens := int64(32000)
	costInput := 1.25
	costOutput := 10.5
	return modelcatalog.ModelRow{
		SourceID:             sourceID,
		ProviderID:           providerID,
		ModelID:              modelID,
		DisplayName:          strings.ToUpper(modelID),
		SourceKind:           sourceKind,
		Priority:             priority,
		Available:            &available,
		RefreshedAt:          time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC),
		ExpiresAt:            time.Date(2026, 5, 8, 11, 0, 0, 0, time.UTC),
		ContextWindow:        &contextWindow,
		MaxOutputTokens:      &maxOutputTokens,
		SupportsTools:        &supportsTools,
		SupportsReasoning:    &supportsReasoning,
		CostInputPerMillion:  &costInput,
		CostOutputPerMillion: &costOutput,
	}
}

func modelCatalogStatus(
	sourceID string,
	providerID string,
	sourceKind modelcatalog.SourceKind,
	priority int,
) modelcatalog.SourceStatus {
	now := time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC)
	return modelcatalog.SourceStatus{
		SourceID:     sourceID,
		ProviderID:   providerID,
		SourceKind:   sourceKind,
		Priority:     priority,
		LastRefresh:  now,
		NextRefresh:  now.Add(24 * time.Hour),
		LastSuccess:  now,
		RefreshState: modelcatalog.RefreshStateSucceeded,
	}
}

func assertModelCatalogModelIDs(t *testing.T, rows []modelcatalog.ModelRow, want []string) {
	t.Helper()

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.ModelID)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("model ids = %#v, want %#v", got, want)
	}
}

func nilModelCatalogTestContext() context.Context {
	return nil
}

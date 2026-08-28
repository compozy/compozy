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

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

// Suite: global model-catalog pricing migration
// Invariant: v18 cached rows retain identity and legacy prices while new rates remain absent.
// Boundary IN: a production v18 GlobalDB prefix with one cached source row.
// Boundary OUT: head repository reads after close and reopen.
func TestGlobalDBModelCatalogPricingMigration(t *testing.T) {
	t.Parallel()
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

// Suite: global model-catalog execution-context migration
// Invariant: provider-independent rows survive in global scope while scoped live observations are rediscovered.
// Boundary IN: a populated production 00095 GlobalDB prefix.
// Boundary OUT: head repository reads after close and reopen.
func TestGlobalDBModelCatalogExecutionContextMigration(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should preserve static rows and discard provider-live observations during the 00096 rebuild",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), GlobalDatabaseName)
			prefixDB, err := openGlobalMigrationPrefixDatabase(
				t,
				path,
				globalMigrationPrefixBefore(t, "00096_schema.sql"),
			)
			if err != nil {
				t.Fatalf("OpenSQLiteDatabase(00095 prefix) error = %v", err)
			}
			ctx := globalMigrationTestContext(t)
			liveSourceID := modelcatalog.SourceKindProviderLiveID("cursor")
			seedPreContextCatalogRow := func(sourceID string, sourceKind modelcatalog.SourceKind, modelID string) {
				t.Helper()
				if _, execErr := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_sources (
				source_id, provider_id, source_kind, priority, refresh_state, row_count, stale
			) VALUES (?, 'cursor', ?, 120, 'succeeded', 1, 0)`, sourceID, sourceKind); execErr != nil {
					t.Fatalf("seed 00095 model_catalog_sources(%s) error = %v", sourceID, execErr)
				}
				if _, execErr := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_rows (
				source_id, provider_id, model_id, source_kind, priority,
				display_name, supports_reasoning, default_reasoning_effort
			) VALUES (?, 'cursor', ?, ?, 120, ?, 1, 'high')`,
					sourceID,
					modelID,
					sourceKind,
					"Model "+modelID,
				); execErr != nil {
					t.Fatalf("seed 00095 model_catalog_rows(%s) error = %v", sourceID, execErr)
				}
				if _, execErr := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_reasoning_efforts (
				source_id, provider_id, model_id, effort, rank
			) VALUES (?, 'cursor', ?, 'high', 0)`, sourceID, modelID); execErr != nil {
					t.Fatalf("seed 00095 model_catalog_reasoning_efforts(%s) error = %v", sourceID, execErr)
				}
				transportModelID := modelID + "[effort=high,fast=true]"
				if _, execErr := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_transport_bindings (
				source_id, provider_id, model_id, transport_model_id,
				label, reasoning_effort, fast, thinking, rank
			) VALUES (?, 'cursor', ?, ?, ?, 'high', 1, 0, 0)`,
					sourceID,
					modelID,
					transportModelID,
					"High fast",
				); execErr != nil {
					t.Fatalf("seed 00095 model_catalog_transport_bindings(%s) error = %v", sourceID, execErr)
				}
				if _, execErr := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_options (
				source_id, provider_id, model_id, option_id,
				label, category, kind, current_value_id
			) VALUES (?, 'cursor', ?, 'context', 'Context', 'runtime', 'select', 'large')`,
					sourceID,
					modelID,
				); execErr != nil {
					t.Fatalf("seed 00095 model_catalog_options(%s) error = %v", sourceID, execErr)
				}
				if _, execErr := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_option_values (
				source_id, provider_id, model_id, option_id, value_id, label, rank
			) VALUES (?, 'cursor', ?, 'context', 'large', 'Large', 0)`, sourceID, modelID); execErr != nil {
					t.Fatalf("seed 00095 model_catalog_option_values(%s) error = %v", sourceID, execErr)
				}
				if _, execErr := prefixDB.ExecContext(ctx, `INSERT INTO model_catalog_transport_binding_selections (
				source_id, provider_id, model_id, transport_model_id, option_id, value_id
			) VALUES (?, 'cursor', ?, ?, 'context', 'large')`,
					sourceID,
					modelID,
					transportModelID,
				); execErr != nil {
					t.Fatalf("seed 00095 model_catalog_transport_binding_selections(%s) error = %v", sourceID, execErr)
				}
			}
			seedPreContextCatalogRow("config", modelcatalog.SourceKindConfig, "grok-static")
			seedPreContextCatalogRow(liveSourceID, modelcatalog.SourceKindProviderLive, "grok-live")
			if err := prefixDB.Close(); err != nil {
				t.Fatalf("prefixDB.Close() error = %v", err)
			}

			upgraded, err := openGlobalMigrationUpgrade(t, path)
			if err != nil {
				t.Fatalf("OpenGlobalDB(00096 upgrade) error = %v", err)
			}
			if err := upgraded.Close(ctx); err != nil {
				t.Fatalf("GlobalDB.Close(upgrade) error = %v", err)
			}
			reopened, err := OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
			}
			t.Cleanup(func() {
				if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
					t.Errorf("GlobalDB.Close(reopen) error = %v", closeErr)
				}
			})

			rows, err := reopened.ListRows(ctx, modelcatalog.ListOptions{
				ProviderID:       "cursor",
				SourceID:         "config",
				ExecutionContext: modelcatalog.GlobalCatalogExecutionContext(),
				SourceContexts: map[string]modelcatalog.CatalogExecutionContext{
					"config": modelcatalog.GlobalCatalogExecutionContext(),
				},
				IncludeAll:   true,
				IncludeStale: true,
			})
			if err != nil {
				t.Fatalf("ListRows(static after 00096) error = %v", err)
			}
			if len(rows) != 1 || rows[0].ModelID != "grok-static" ||
				!slices.Equal(
					rows[0].ReasoningEfforts,
					[]modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortHigh},
				) ||
				len(rows[0].ConfigOptions) != 1 || rows[0].ConfigOptions[0].CurrentValueID != "large" ||
				len(rows[0].ConfigOptions[0].Values) != 1 || rows[0].ConfigOptions[0].Values[0].ValueID != "large" ||
				len(rows[0].TransportBindings) != 1 ||
				rows[0].TransportBindings[0].TransportModelID != "grok-static[effort=high,fast=true]" ||
				len(rows[0].TransportBindings[0].OptionSelections) != 1 ||
				rows[0].TransportBindings[0].OptionSelections[0].ValueID != "large" {
				t.Fatalf("static rows after 00096 = %#v, want complete config row", rows)
			}

			var liveRecords int
			if err := reopened.db.QueryRowContext(ctx, `SELECT
			(SELECT COUNT(*) FROM model_catalog_sources WHERE source_id = ?) +
			(SELECT COUNT(*) FROM model_catalog_rows WHERE source_id = ?) +
			(SELECT COUNT(*) FROM model_catalog_reasoning_efforts WHERE source_id = ?) +
			(SELECT COUNT(*) FROM model_catalog_transport_bindings WHERE source_id = ?) +
			(SELECT COUNT(*) FROM model_catalog_options WHERE source_id = ?) +
			(SELECT COUNT(*) FROM model_catalog_option_values WHERE source_id = ?) +
			(SELECT COUNT(*) FROM model_catalog_transport_binding_selections WHERE source_id = ?)`,
				liveSourceID,
				liveSourceID,
				liveSourceID,
				liveSourceID,
				liveSourceID,
				liveSourceID,
				liveSourceID,
			).Scan(&liveRecords); err != nil {
				t.Fatalf("count provider-live records after 00096 error = %v", err)
			}
			if liveRecords != 0 {
				t.Fatalf("provider-live records after 00096 = %d, want 0 for rediscovery", liveRecords)
			}
		},
	)
}

func TestGlobalDBModelCatalogStore(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate live rows and prune superseded fingerprints within one workspace", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		copyCurrentSchemaGlobalDBSeed(t, path)
		globalDB, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		sourceID := modelcatalog.SourceKindProviderLiveID("cursor")
		contextA := modelcatalog.CatalogExecutionContext{
			Scope:              modelcatalog.ExecutionScopeWorkspace,
			ProfileID:          "profile-a",
			WorkspaceID:        "workspace-a",
			CommandFingerprint: modelcatalog.CatalogExecutionFingerprint("cursor-a"),
		}
		contextB := modelcatalog.CatalogExecutionContext{
			Scope:              modelcatalog.ExecutionScopeWorkspace,
			ProfileID:          "profile-a",
			WorkspaceID:        "workspace-a",
			CommandFingerprint: modelcatalog.CatalogExecutionFingerprint("cursor-b"),
		}
		contextOtherWorkspace := modelcatalog.CatalogExecutionContext{
			Scope:              modelcatalog.ExecutionScopeWorkspace,
			ProfileID:          "profile-a",
			WorkspaceID:        "workspace-b",
			CommandFingerprint: modelcatalog.CatalogExecutionFingerprint("cursor-a"),
		}
		persistLive := func(executionContext modelcatalog.CatalogExecutionContext, modelID string) {
			t.Helper()
			row := modelCatalogRow(sourceID, "cursor", modelID, modelcatalog.SourceKindProviderLive, 110)
			status := modelCatalogStatus(sourceID, "cursor", modelcatalog.SourceKindProviderLive, 110)
			if replaceErr := globalDB.ReplaceSourceRows(
				ctx,
				executionContext,
				sourceID,
				"cursor",
				[]modelcatalog.ModelRow{row},
				status,
			); replaceErr != nil {
				t.Fatalf("ReplaceSourceRows(%s) error = %v", modelID, replaceErr)
			}
		}
		listLive := func(db *GlobalDB, executionContext modelcatalog.CatalogExecutionContext) []modelcatalog.ModelRow {
			t.Helper()
			rows, listErr := db.ListRows(ctx, modelcatalog.ListOptions{
				ProviderID: "cursor",
				SourceID:   sourceID,
				SourceContexts: map[string]modelcatalog.CatalogExecutionContext{
					sourceID: executionContext,
				},
				IncludeAll:   true,
				IncludeStale: true,
			})
			if listErr != nil {
				t.Fatalf("ListRows() error = %v", listErr)
			}
			return rows
		}

		persistLive(contextA, "model-a")
		persistLive(contextOtherWorkspace, "model-other-workspace")
		if rows := listLive(globalDB, contextA); len(rows) != 1 || rows[0].ModelID != "model-a" {
			t.Fatalf("context A rows = %#v, want model-a", rows)
		}
		if rows := listLive(globalDB, contextOtherWorkspace); len(rows) != 1 ||
			rows[0].ModelID != "model-other-workspace" {
			t.Fatalf("other workspace rows = %#v, want isolated model", rows)
		}

		persistLive(contextB, "model-b")
		if rows := listLive(globalDB, contextA); len(rows) != 0 {
			t.Fatalf("superseded context A rows = %#v, want pruned", rows)
		}
		if rows := listLive(globalDB, contextB); len(rows) != 1 || rows[0].ModelID != "model-b" {
			t.Fatalf("context B rows = %#v, want model-b", rows)
		}
		if rows := listLive(globalDB, contextOtherWorkspace); len(rows) != 1 ||
			rows[0].ModelID != "model-other-workspace" {
			t.Fatalf("other workspace rows after prune = %#v, want preserved", rows)
		}
		contextAID, err := contextA.ID()
		if err != nil {
			t.Fatalf("contextA.ID() error = %v", err)
		}
		var contextACount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM model_catalog_execution_contexts WHERE context_id = ?`,
			contextAID,
		).Scan(&contextACount); err != nil {
			t.Fatalf("query superseded execution context error = %v", err)
		}
		if contextACount != 0 {
			t.Fatalf("superseded execution context count = %d, want 0", contextACount)
		}

		configRow := modelCatalogRow("config", "cursor", "static-model", modelcatalog.SourceKindConfig, 120)
		configStatus := modelCatalogStatus("config", "cursor", modelcatalog.SourceKindConfig, 120)
		err = globalDB.ReplaceSourceRows(
			ctx,
			modelcatalog.GlobalCatalogExecutionContext(),
			"config",
			"cursor",
			[]modelcatalog.ModelRow{configRow},
			configStatus,
		)
		if err != nil {
			t.Fatalf("ReplaceSourceRows(config) error = %v", err)
		}
		combined, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
			ProviderID: "cursor",
			SourceContexts: map[string]modelcatalog.CatalogExecutionContext{
				"config": modelcatalog.GlobalCatalogExecutionContext(),
				sourceID: contextB,
			},
			IncludeAll:   true,
			IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("ListRows(combined) error = %v", err)
		}
		if got, want := len(combined), 2; got != want {
			t.Fatalf("len(ListRows(combined)) = %d, want %d: %#v", got, want, combined)
		}

		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("CloseGlobalDB() error = %v", err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("CloseGlobalDB(reopened) error = %v", closeErr)
			}
		})
		if rows := listLive(reopened, contextB); len(rows) != 1 || rows[0].ModelID != "model-b" {
			t.Fatalf("reopened context B rows = %#v, want model-b", rows)
		}
		if rows := listLive(reopened, contextOtherWorkspace); len(rows) != 1 ||
			rows[0].ModelID != "model-other-workspace" {
			t.Fatalf("reopened other workspace rows = %#v, want isolated model", rows)
		}
	})

	t.Run("Should reject source kinds stored in the wrong execution scope", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		liveRow := modelCatalogRow(
			"provider_live:cursor",
			"cursor",
			"model",
			modelcatalog.SourceKindProviderLive,
			110,
		)
		liveStatus := modelCatalogStatus(
			"provider_live:cursor",
			"cursor",
			modelcatalog.SourceKindProviderLive,
			110,
		)
		err := globalDB.ReplaceSourceRows(
			ctx,
			modelcatalog.GlobalCatalogExecutionContext(),
			liveRow.SourceID,
			liveRow.ProviderID,
			[]modelcatalog.ModelRow{liveRow},
			liveStatus,
		)
		assertGlobalDBErrorContains(t, err, "requires a scoped execution context")

		configRow := modelCatalogRow("config", "cursor", "model", modelcatalog.SourceKindConfig, 120)
		configStatus := modelCatalogStatus("config", "cursor", modelcatalog.SourceKindConfig, 120)
		err = globalDB.ReplaceSourceRows(
			ctx,
			modelCatalogTestExecutionContext(modelcatalog.SourceKindProviderLive),
			configRow.SourceID,
			configRow.ProviderID,
			[]modelcatalog.ModelRow{configRow},
			configStatus,
		)
		assertGlobalDBErrorContains(t, err, "requires the global execution context")
	})

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
		statuses, err := globalDB.ListSourceStatus(ctx, modelcatalog.StatusOptions{ProviderID: "codex"})
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
		if len(statuses) != 1 || statuses[0].RowCount != 1 {
			t.Fatalf("statuses = %#v, want one status with row_count 1", statuses)
		}
	})

	t.Run("Should round trip transport bindings through replacement and reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		globalDB := openGlobalDBForTest(t, path)
		ctx := testutil.Context(t)
		reasoningEffort := modelcatalog.ReasoningEffortHigh
		fast := true
		thinking := false
		row := modelCatalogRow("config", "cursor", "grok-4.6", modelcatalog.SourceKindConfig, 120)
		row.TransportBindings = []modelcatalog.ModelTransportBinding{
			{
				TransportModelID: "grok-4.6[effort=high,fast=true]",
				Label:            "Grok 4.6 · High · Fast",
				ReasoningEffort:  &reasoningEffort,
				Fast:             &fast,
				Thinking:         &thinking,
			},
			{
				TransportModelID: "grok-4.6",
				Label:            "Grok 4.6",
			},
		}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"cursor",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{row},
		)

		assertTransportBindingRoundTrip(ctx, t, globalDB, row.TransportBindings)
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before reopen) error = %v", err)
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
		assertTransportBindingRoundTrip(ctx, t, reopened, row.TransportBindings)
	})

	t.Run("Should replace transport bindings without leaving old rows", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		oldRow := modelCatalogRow("config", "cursor", "grok-4.6", modelcatalog.SourceKindConfig, 120)
		oldRow.TransportBindings = []modelcatalog.ModelTransportBinding{{
			TransportModelID: "grok-4.6[effort=low,fast=false]",
			Label:            "Grok 4.6 · Low",
		}}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"cursor",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{oldRow},
		)

		newRow := modelCatalogRow("config", "cursor", "grok-4.6", modelcatalog.SourceKindConfig, 120)
		newRow.TransportBindings = []modelcatalog.ModelTransportBinding{{
			TransportModelID: "grok-4.6[effort=xhigh,fast=true]",
			Label:            "Grok 4.6 · XHigh · Fast",
		}}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"cursor",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{newRow},
		)

		rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
			ProviderID: "cursor", SourceID: "config", IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("ListRows(after binding replacement) error = %v", err)
		}
		if len(rows) != 1 || len(rows[0].TransportBindings) != 1 {
			t.Fatalf("rows(after binding replacement) = %#v, want one model with one binding", rows)
		}
		if got := rows[0].TransportBindings[0].TransportModelID; got != newRow.TransportBindings[0].TransportModelID {
			t.Fatalf("transport model id = %q, want %q", got, newRow.TransportBindings[0].TransportModelID)
		}
		var oldCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM model_catalog_transport_bindings
			 WHERE source_id = ? AND provider_id = ? AND model_id = ? AND transport_model_id = ?`,
			"config",
			"cursor",
			"grok-4.6",
			oldRow.TransportBindings[0].TransportModelID,
		).Scan(&oldCount); err != nil {
			t.Fatalf("QueryRowContext(old binding) error = %v", err)
		}
		if oldCount != 0 {
			t.Fatalf("old binding count = %d, want 0", oldCount)
		}
	})

	t.Run("Should associate transport bindings with the complete model key", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		first := modelCatalogRow("config", "cursor", "grok-4.5", modelcatalog.SourceKindConfig, 120)
		first.TransportBindings = []modelcatalog.ModelTransportBinding{{
			TransportModelID: "grok-4.5[effort=high,fast=true]",
			Label:            "Grok 4.5 · High · Fast",
		}}
		second := modelCatalogRow("config", "cursor", "grok-4.6", modelcatalog.SourceKindConfig, 120)
		second.TransportBindings = []modelcatalog.ModelTransportBinding{{
			TransportModelID: "grok-4.6[effort=xhigh,fast=true]",
			Label:            "Grok 4.6 · XHigh · Fast",
		}}
		replaceModelCatalogRows(
			t,
			globalDB,
			"config",
			"cursor",
			modelcatalog.SourceKindConfig,
			120,
			[]modelcatalog.ModelRow{first, second},
		)

		rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
			ProviderID: "cursor", SourceID: "config", IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("ListRows(composite binding key) error = %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("rows(composite binding key) = %#v, want two rows", rows)
		}
		for _, want := range []modelcatalog.ModelRow{first, second} {
			var got *modelcatalog.ModelRow
			for index := range rows {
				if rows[index].ModelID == want.ModelID {
					got = &rows[index]
					break
				}
			}
			if got == nil || len(got.TransportBindings) != 1 ||
				got.TransportBindings[0].TransportModelID != want.TransportBindings[0].TransportModelID {
				t.Fatalf("row for model %q = %#v, want binding %#v", want.ModelID, got, want.TransportBindings)
			}
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
			modelcatalog.GlobalCatalogExecutionContext(),
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
		statuses, err := globalDB.ListSourceStatus(ctx, modelcatalog.StatusOptions{ProviderID: "codex"})
		if err != nil {
			t.Fatalf("ListSourceStatus(after failed replace) error = %v", err)
		}
		if len(statuses) != 1 || statuses[0].RowCount != 1 {
			t.Fatalf("statuses after failed replace = %#v, want original row_count 1", statuses)
		}
	})

	t.Run("Should roll back every replacement when a later batch write fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		for _, providerID := range []string{"claude", "codex"} {
			replaceModelCatalogRows(
				t,
				globalDB,
				"config",
				providerID,
				modelcatalog.SourceKindConfig,
				120,
				[]modelcatalog.ModelRow{
					modelCatalogRow("config", providerID, "old-"+providerID, modelcatalog.SourceKindConfig, 120),
				},
			)
		}

		invalid := modelCatalogRow("config", "codex", "new-codex", modelcatalog.SourceKindConfig, 120)
		invalid.ReasoningEfforts = []modelcatalog.ReasoningEffort{
			modelcatalog.ReasoningEffortHigh,
			modelcatalog.ReasoningEffortHigh,
		}
		err := globalDB.ReplaceSourceRowsBatch(ctx, []modelcatalog.SourceRowsReplacement{
			{
				ExecutionContext: modelcatalog.GlobalCatalogExecutionContext(),
				SourceID:         "config",
				ProviderID:       "claude",
				Rows: []modelcatalog.ModelRow{
					modelCatalogRow("config", "claude", "new-claude", modelcatalog.SourceKindConfig, 120),
				},
				Status: modelCatalogStatus("config", "claude", modelcatalog.SourceKindConfig, 120),
			},
			{
				ExecutionContext: modelcatalog.GlobalCatalogExecutionContext(),
				SourceID:         "config",
				ProviderID:       "codex",
				Rows:             []modelcatalog.ModelRow{invalid},
				Status:           modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
			},
		})
		if err == nil {
			t.Fatal("ReplaceSourceRowsBatch() error = nil, want later replacement failure")
		}

		rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
			SourceID:     "config",
			IncludeAll:   true,
			IncludeStale: true,
		})
		if err != nil {
			t.Fatalf("ListRows(after failed batch) error = %v", err)
		}
		got := make([]string, 0, len(rows))
		for _, row := range rows {
			got = append(got, row.ProviderID+"/"+row.ModelID)
		}
		want := []string{"claude/old-claude", "codex/old-codex"}
		if !slices.Equal(got, want) {
			t.Fatalf("rows after failed batch = %#v, want %#v", got, want)
		}
		statuses, err := globalDB.ListSourceStatus(ctx, modelcatalog.StatusOptions{})
		if err != nil {
			t.Fatalf("ListSourceStatus(after failed batch) error = %v", err)
		}
		if len(statuses) != 2 || statuses[0].RowCount != 1 || statuses[1].RowCount != 1 {
			t.Fatalf("statuses after failed batch = %#v, want two original one-row statuses", statuses)
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
			modelcatalog.GlobalCatalogExecutionContext(),
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

		statuses, err := globalDB.ListSourceStatus(ctx, modelcatalog.StatusOptions{})
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
		codexStatuses, err := globalDB.ListSourceStatus(
			ctx,
			modelcatalog.StatusOptions{ProviderID: "codex"},
		)
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
			modelcatalog.GlobalCatalogExecutionContext(),
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
				name: "Should reject blank transport model id",
				rows: []modelcatalog.ModelRow{
					func() modelcatalog.ModelRow {
						row := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
						row.TransportBindings = []modelcatalog.ModelTransportBinding{{Label: "missing id"}}
						return row
					}(),
				},
				status: modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
				want:   "transport model id is required",
			},
			{
				name: "Should reject duplicate transport model id",
				rows: []modelcatalog.ModelRow{
					func() modelcatalog.ModelRow {
						row := modelCatalogRow("config", "codex", "gpt-5.4", modelcatalog.SourceKindConfig, 120)
						row.TransportBindings = []modelcatalog.ModelTransportBinding{
							{TransportModelID: "gpt-5.4-high"},
							{TransportModelID: "gpt-5.4-high"},
						}
						return row
					}(),
				},
				status: modelCatalogStatus("config", "codex", modelcatalog.SourceKindConfig, 120),
				want:   "duplicate transport model id",
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
				err := globalDB.ReplaceSourceRows(
					testutil.Context(t),
					modelcatalog.GlobalCatalogExecutionContext(),
					"config",
					"codex",
					tc.rows,
					tc.status,
				)
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
			modelcatalog.GlobalCatalogExecutionContext(),
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
		if _, err := globalDB.ListSourceStatus(
			nilCtx,
			modelcatalog.StatusOptions{ProviderID: "codex"},
		); err == nil {
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
		if err := globalDB.ReplaceSourceRows(
			ctx,
			modelcatalog.GlobalCatalogExecutionContext(),
			"config",
			"codex",
			[]modelcatalog.ModelRow{row},
			status,
		); err != nil {
			t.Fatalf("ReplaceSourceRows() error = %v", err)
		}

		statuses, err := globalDB.ListSourceStatus(ctx, modelcatalog.StatusOptions{ProviderID: "codex"})
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
		liveContext := modelCatalogTestExecutionContext(modelcatalog.SourceKindProviderLive)
		if err := globalDB.ReplaceSourceRows(
			ctx,
			liveContext,
			"provider_live:codex",
			"codex",
			nil,
			failed,
		); err != nil {
			t.Fatalf("ReplaceSourceRows(failed) error = %v", err)
		}

		statuses, err := globalDB.ListSourceStatus(ctx, modelcatalog.StatusOptions{
			ProviderID: "codex",
			SourceContexts: map[string]modelcatalog.CatalogExecutionContext{
				"provider_live:codex": liveContext,
			},
		})
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
			modelcatalog.ListOptions{
				ProviderID: "codex", SourceID: "provider_live:codex", IncludeStale: true,
				SourceContexts: map[string]modelcatalog.CatalogExecutionContext{
					"provider_live:codex": liveContext,
				},
			},
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

		extensionContext := modelCatalogTestExecutionContext(modelcatalog.SourceKindExtension)
		rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
			ProviderID: "codex", IncludeStale: true,
			SourceContexts: map[string]modelcatalog.CatalogExecutionContext{
				"extension:a_source": extensionContext,
				"extension:z_source": extensionContext,
			},
		})
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
		if _, err := globalDB.ListSourceStatus(ctx, modelcatalog.StatusOptions{ProviderID: "codex"}); err == nil ||
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

func assertTransportBindingRoundTrip(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	want []modelcatalog.ModelTransportBinding,
) {
	t.Helper()

	rows, err := globalDB.ListRows(ctx, modelcatalog.ListOptions{
		ProviderID: "cursor", SourceID: "config", IncludeStale: true,
	})
	if err != nil {
		t.Fatalf("ListRows(transport bindings) error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows(transport bindings) = %#v, want one row", rows)
	}
	got := rows[0].TransportBindings
	if len(got) != len(want) {
		t.Fatalf("transport bindings = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index].TransportModelID != want[index].TransportModelID ||
			got[index].Label != want[index].Label {
			t.Fatalf("transport binding[%d] = %#v, want %#v", index, got[index], want[index])
		}
		if (got[index].ReasoningEffort == nil) != (want[index].ReasoningEffort == nil) ||
			(got[index].ReasoningEffort != nil &&
				*got[index].ReasoningEffort != *want[index].ReasoningEffort) {
			t.Fatalf("transport binding[%d] reasoning effort = %#v, want %#v", index, got[index], want[index])
		}
		if (got[index].Fast == nil) != (want[index].Fast == nil) ||
			(got[index].Fast != nil && *got[index].Fast != *want[index].Fast) {
			t.Fatalf("transport binding[%d] fast = %#v, want %#v", index, got[index], want[index])
		}
		if (got[index].Thinking == nil) != (want[index].Thinking == nil) ||
			(got[index].Thinking != nil && *got[index].Thinking != *want[index].Thinking) {
			t.Fatalf("transport binding[%d] thinking = %#v, want %#v", index, got[index], want[index])
		}
		if !modelOptionSelectionsEqual(got[index].OptionSelections, want[index].OptionSelections) {
			t.Fatalf(
				"transport binding[%d] option selections = %#v, want %#v",
				index,
				got[index].OptionSelections,
				want[index].OptionSelections,
			)
		}
	}
}

func modelOptionSelectionsEqual(
	left []modelcatalog.ModelOptionSelection,
	right []modelcatalog.ModelOptionSelection,
) bool {
	return slices.EqualFunc(left, right, func(a, b modelcatalog.ModelOptionSelection) bool {
		if a.ID != b.ID || a.ValueID != b.ValueID || (a.BoolValue == nil) != (b.BoolValue == nil) {
			return false
		}
		return a.BoolValue == nil || *a.BoolValue == *b.BoolValue
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
		modelCatalogTestExecutionContext(sourceKind),
		sourceID,
		providerID,
		rows,
		modelCatalogStatus(sourceID, providerID, sourceKind, priority),
	); err != nil {
		t.Fatalf("ReplaceSourceRows(%s/%s) error = %v", sourceID, providerID, err)
	}
}

func modelCatalogTestExecutionContext(sourceKind modelcatalog.SourceKind) modelcatalog.CatalogExecutionContext {
	switch sourceKind {
	case modelcatalog.SourceKindProviderLive, modelcatalog.SourceKindExtension, modelcatalog.SourceKindACPSession:
		return modelcatalog.CatalogExecutionContext{
			Scope:              modelcatalog.ExecutionScopeWorkspace,
			ProfileID:          "profile-test",
			WorkspaceID:        "workspace-test",
			CommandFingerprint: modelcatalog.CatalogExecutionFingerprint("test", string(sourceKind)),
		}
	case modelcatalog.SourceKindBuiltin, modelcatalog.SourceKindConfig, modelcatalog.SourceKindModelsDev:
		return modelcatalog.GlobalCatalogExecutionContext()
	default:
		panic("unhandled model catalog test source kind: " + string(sourceKind))
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

func assertGlobalDBErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want error containing %q", err, want)
	}
}

func nilModelCatalogTestContext() context.Context {
	return nil
}

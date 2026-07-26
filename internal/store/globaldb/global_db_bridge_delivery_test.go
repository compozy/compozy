package globaldb

import (
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	globalschema "github.com/compozy/agh/internal/store/globaldb/schema"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBBridgeDeliverySchemaMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should create the bridge delivery schema on a fresh database", func(t *testing.T) {
		t.Parallel()

		globalDB := openFreshTestGlobalDB(t)

		assertBridgeDeliverySchema(t, globalDB.db)
		assertBridgeDeliveryMigrationStatus(t, globalDB.db)
	})

	t.Run("Should append the bridge delivery schema after the observed migration prefix", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		legacyDB, err := openGlobalMigrationPrefixDatabase(t, path, previousBridgeDeliveryMigrationStream(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(previous schema) error = %v", err)
		}
		ctx := testutil.Context(t)
		if _, err := legacyDB.ExecContext(
			ctx,
			`INSERT INTO bridge_instances (
				id, scope, platform, extension_name, display_name, enabled, status,
				routing_policy, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"brg-delivery-migration",
			string(bridges.ScopeGlobal),
			"telegram",
			"telegram-extension",
			"Telegram",
			true,
			string(bridges.BridgeStatusReady),
			`{"include_peer":true}`,
			store.FormatTimestamp(bridgeDeliveryTestTime()),
			store.FormatTimestamp(bridgeDeliveryTestTime()),
		); err != nil {
			t.Fatalf("seed previous bridge instance error = %v", err)
		}
		if err := legacyDB.Close(); err != nil {
			t.Fatalf("legacyDB.Close() error = %v", err)
		}

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(upgrade) error = %v", err)
		}
		ctx = testutil.Context(t)
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("GlobalDB.Close(upgrade) error = %v", err)
		}

		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("reopened.Close() error = %v", err)
			}
		})

		assertBridgeDeliverySchema(t, reopened.db)
		assertBridgeDeliveryMigrationStatus(t, reopened.db)
		var instanceCount int
		if err := reopened.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM bridge_instances WHERE id = ?`,
			"brg-delivery-migration",
		).Scan(&instanceCount); err != nil {
			t.Fatalf("query preserved bridge instance error = %v", err)
		}
		if instanceCount != 1 {
			t.Fatalf("preserved bridge instance count = %d, want 1", instanceCount)
		}
	})
}

func TestGlobalDBBridgeDeliveryStore(t *testing.T) {
	t.Parallel()

	t.Run("Should create checkpoint and terminate one delivery monotonically", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		insertBridgeDeliveryInstance(t, globalDB, bridges.ScopeGlobal, "", "brg-delivery-global")

		createdAt := bridgeDeliveryTestTime()
		record := bridgeDeliveryRecordForTest(
			"delivery-global",
			"brg-delivery-global",
			bridges.ScopeGlobal,
			"",
			createdAt,
		)
		if err := globalDB.CreateBridgeDelivery(ctx, record); err != nil {
			t.Fatalf("CreateBridgeDelivery() error = %v", err)
		}
		if err := globalDB.CreateBridgeDelivery(ctx, record); !errors.Is(err, bridges.ErrDeliveryLedgerConflict) {
			t.Fatalf("CreateBridgeDelivery(duplicate) error = %v, want ErrDeliveryLedgerConflict", err)
		}

		listed, err := globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(active) error = %v", err)
		}
		if len(listed) != 1 {
			t.Fatalf("ListBridgeDeliveries(active) length = %d, want 1", len(listed))
		}
		assertBridgeDeliveryRecordEqual(t, listed[0], record)
		assertCanonicalRoutingKeyPersisted(t, globalDB.db, record)

		ackedAt := createdAt.Add(time.Minute)
		metrics := bridgeDeliveryMetricRecordForTest(
			"brg-delivery-global",
			bridges.ScopeGlobal,
			"",
			ackedAt,
		)
		if err := globalDB.CheckpointBridgeDelivery(ctx, bridges.DeliveryLedgerCheckpoint{
			DeliveryID:      record.DeliveryID,
			LastSentSeq:     4,
			LastAckedSeq:    3,
			RemoteMessageID: "remote-4",
			State:           bridges.DeliveryLedgerStateActive,
			UpdatedAt:       ackedAt,
			Metrics:         &metrics,
		}); err != nil {
			t.Fatalf("CheckpointBridgeDelivery(ack) error = %v", err)
		}

		listed, err = globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(acked) error = %v", err)
		}
		if got := listed[0]; got.LastSentSeq != 4 || got.LastAckedSeq != 3 || got.RemoteMessageID != "remote-4" {
			t.Fatalf("acked delivery = %#v, want sent=4 acked=3 remote=remote-4", got)
		}

		metricRows, err := globalDB.ListBridgeDeliveryMetrics(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveryMetrics(active) error = %v", err)
		}
		gotMetrics := metricRows[record.BridgeInstanceID]
		if gotMetrics.DeliveryBacklog != 1 {
			t.Fatalf("DeliveryBacklog = %d, want 1 active checkpoint", gotMetrics.DeliveryBacklog)
		}
		assertBridgeDeliveryMetricsEqual(t, gotMetrics, metrics.BridgeDeliveryMetrics)

		regression := bridges.DeliveryLedgerCheckpoint{
			DeliveryID:      record.DeliveryID,
			LastSentSeq:     3,
			LastAckedSeq:    2,
			RemoteMessageID: "remote-3",
			State:           bridges.DeliveryLedgerStateActive,
			UpdatedAt:       ackedAt.Add(time.Minute),
		}
		if err := globalDB.CheckpointBridgeDelivery(ctx, regression); !errors.Is(
			err,
			bridges.ErrDeliveryLedgerConflict,
		) {
			t.Fatalf("CheckpointBridgeDelivery(regression) error = %v, want ErrDeliveryLedgerConflict", err)
		}
		regressedMetrics := metrics
		regressedMetrics.DeliveryDroppedTotal = 2
		regressedMetrics.DeliveryDroppedByReason = map[string]int{"queue_saturated": 1, "transport": 1}
		regressedMetrics.UpdatedAt = ackedAt.Add(-time.Minute)
		if err := globalDB.CheckpointBridgeDelivery(ctx, bridges.DeliveryLedgerCheckpoint{
			DeliveryID:      record.DeliveryID,
			LastSentSeq:     5,
			LastAckedSeq:    4,
			RemoteMessageID: "remote-5",
			State:           bridges.DeliveryLedgerStateActive,
			UpdatedAt:       regressedMetrics.UpdatedAt,
			Metrics:         &regressedMetrics,
		}); err != nil {
			t.Fatalf("CheckpointBridgeDelivery(stale metric snapshot) error = %v", err)
		}
		listed, err = globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(after metric regression) error = %v", err)
		}
		if got := listed[0]; got.LastSentSeq != 5 || got.LastAckedSeq != 4 || got.RemoteMessageID != "remote-5" {
			t.Fatalf("delivery after stale metric merge = %#v, want sent=5 acked=4 remote=remote-5", got)
		}
		metricRows, err = globalDB.ListBridgeDeliveryMetrics(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveryMetrics(after stale snapshot) error = %v", err)
		}
		if got := metricRows[record.BridgeInstanceID]; got.DeliveryDroppedTotal != 3 {
			t.Fatalf("merged stale metrics = %#v, want non-regressed dropped total 3", got)
		}

		terminalAt := ackedAt.Add(2 * time.Minute)
		if err := globalDB.CheckpointBridgeDelivery(ctx, bridges.DeliveryLedgerCheckpoint{
			DeliveryID:      record.DeliveryID,
			LastSentSeq:     5,
			LastAckedSeq:    5,
			RemoteMessageID: "remote-5",
			State:           bridges.DeliveryLedgerStateTerminalOK,
			UpdatedAt:       terminalAt,
		}); err != nil {
			t.Fatalf("CheckpointBridgeDelivery(terminal) error = %v", err)
		}
		if err := globalDB.CheckpointBridgeDelivery(ctx, bridges.DeliveryLedgerCheckpoint{
			DeliveryID:   record.DeliveryID,
			LastSentSeq:  5,
			LastAckedSeq: 5,
			State:        bridges.DeliveryLedgerStateActive,
			UpdatedAt:    terminalAt.Add(time.Minute),
		}); !errors.Is(err, bridges.ErrDeliveryLedgerConflict) {
			t.Fatalf("CheckpointBridgeDelivery(reopen) error = %v, want ErrDeliveryLedgerConflict", err)
		}

		terminalRows, err := globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
			State: bridges.DeliveryLedgerStateTerminalOK,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(terminal) error = %v", err)
		}
		if len(terminalRows) != 1 || terminalRows[0].State != bridges.DeliveryLedgerStateTerminalOK {
			t.Fatalf("terminal deliveries = %#v, want one terminal_ok row", terminalRows)
		}
		metricRows, err = globalDB.ListBridgeDeliveryMetrics(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveryMetrics(terminal) error = %v", err)
		}
		if got := metricRows[record.BridgeInstanceID].DeliveryBacklog; got != 0 {
			t.Fatalf("DeliveryBacklog after terminal = %d, want 0", got)
		}
	})

	t.Run("Should reject checkpoints for unknown deliveries", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		err := globalDB.CheckpointBridgeDelivery(testutil.Context(t), bridges.DeliveryLedgerCheckpoint{
			DeliveryID:   "delivery-missing",
			State:        bridges.DeliveryLedgerStateActive,
			UpdatedAt:    bridgeDeliveryTestTime(),
			LastSentSeq:  1,
			LastAckedSeq: 1,
		})
		if !errors.Is(err, bridges.ErrDeliveryLedgerNotFound) {
			t.Fatalf("CheckpointBridgeDelivery(missing) error = %v, want ErrDeliveryLedgerNotFound", err)
		}
	})
}

func TestGlobalDBBridgeDeliveryMetricsSurviveRestart(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve full counters across database reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		globalDB, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		insertBridgeDeliveryInstance(t, globalDB, bridges.ScopeGlobal, "", "brg-delivery-restart")
		record := bridgeDeliveryRecordForTest(
			"delivery-restart",
			"brg-delivery-restart",
			bridges.ScopeGlobal,
			"",
			bridgeDeliveryTestTime(),
		)
		if err := globalDB.CreateBridgeDelivery(ctx, record); err != nil {
			t.Fatalf("CreateBridgeDelivery() error = %v", err)
		}
		metrics := bridgeDeliveryMetricRecordForTest(
			record.BridgeInstanceID,
			bridges.ScopeGlobal,
			"",
			bridgeDeliveryTestTime().Add(time.Minute),
		)
		if err := globalDB.PutBridgeDeliveryMetrics(ctx, metrics); err != nil {
			t.Fatalf("PutBridgeDeliveryMetrics() error = %v", err)
		}
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("reopened.Close() error = %v", err)
			}
		})
		metricRows, err := reopened.ListBridgeDeliveryMetrics(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveryMetrics() error = %v", err)
		}
		got := metricRows[record.BridgeInstanceID]
		if got.DeliveryBacklog != 1 {
			t.Fatalf("DeliveryBacklog after restart = %d, want 1", got.DeliveryBacklog)
		}
		assertBridgeDeliveryMetricsEqual(t, got, metrics.BridgeDeliveryMetrics)
	})
}

func TestGlobalDBBridgeDeliveryProtectsActiveInstanceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject removal or identity changes until the delivery is terminal", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		const instanceID = "brg-delivery-identity-lock"
		insertBridgeDeliveryInstance(t, globalDB, bridges.ScopeGlobal, "", instanceID)
		instance, err := globalDB.GetBridgeInstance(ctx, instanceID)
		if err != nil {
			t.Fatalf("GetBridgeInstance() error = %v", err)
		}
		record := bridgeDeliveryRecordForTest(
			"delivery-identity-lock",
			instanceID,
			bridges.ScopeGlobal,
			"",
			bridgeDeliveryTestTime(),
		)
		if err := globalDB.CreateBridgeDelivery(ctx, record); err != nil {
			t.Fatalf("CreateBridgeDelivery() error = %v", err)
		}

		providerChanged := instance
		providerChanged.Platform = "slack"
		providerChanged.ExtensionName = "slack-extension"
		providerChanged.UpdatedAt = record.UpdatedAt.Add(time.Minute)
		if err := globalDB.ReplaceBridgeInstances(ctx, []bridges.BridgeInstance{providerChanged}); err == nil {
			t.Fatal("ReplaceBridgeInstances(provider change) error = nil, want active-delivery guard")
		}

		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"delivery-identity-lock",
			filepath.Join(t.TempDir(), "workspace"),
		)
		scopeChanged := instance
		scopeChanged.Scope = bridges.ScopeWorkspace
		scopeChanged.WorkspaceID = workspaceID
		scopeChanged.UpdatedAt = record.UpdatedAt.Add(2 * time.Minute)
		if err := globalDB.ReplaceBridgeInstances(ctx, []bridges.BridgeInstance{scopeChanged}); err == nil {
			t.Fatal("ReplaceBridgeInstances(scope change) error = nil, want active-delivery guard")
		}
		if err := globalDB.ReplaceBridgeInstances(ctx, nil); err == nil {
			t.Fatal("ReplaceBridgeInstances(delete active instance) error = nil, want active-delivery guard")
		}

		stored, err := globalDB.GetBridgeInstance(ctx, instanceID)
		if err != nil {
			t.Fatalf("GetBridgeInstance(after rejected mutations) error = %v", err)
		}
		if stored.Scope != bridges.ScopeGlobal || stored.Platform != instance.Platform ||
			stored.ExtensionName != instance.ExtensionName {
			t.Fatalf("stored instance after rejected mutations = %#v, want original identity", stored)
		}
		active, err := globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
			State: bridges.DeliveryLedgerStateActive,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(active) error = %v", err)
		}
		if got, want := len(active), 1; got != want {
			t.Fatalf("len(active deliveries) = %d, want %d after rejected mutations", got, want)
		}

		if err := globalDB.CheckpointBridgeDelivery(ctx, bridges.DeliveryLedgerCheckpoint{
			DeliveryID: record.DeliveryID,
			State:      bridges.DeliveryLedgerStateTerminalOK,
			UpdatedAt:  record.UpdatedAt.Add(3 * time.Minute),
		}); err != nil {
			t.Fatalf("CheckpointBridgeDelivery(terminal) error = %v", err)
		}
		if err := globalDB.ReplaceBridgeInstances(ctx, nil); err != nil {
			t.Fatalf("ReplaceBridgeInstances(delete terminal instance) error = %v", err)
		}
		remaining, err := globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope: bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(after terminal delete) error = %v", err)
		}
		if len(remaining) != 0 {
			t.Fatalf("remaining deliveries = %#v, want terminal rows cascaded with deleted instance", remaining)
		}
	})
}

func TestGlobalDBBridgeDeliveriesAreWorkspaceIsolated(t *testing.T) {
	t.Parallel()

	t.Run("Should list deliveries and metrics only inside the requested workspace", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "delivery-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "delivery-b", filepath.Join(t.TempDir(), "b"))
		insertBridgeDeliveryInstance(t, globalDB, bridges.ScopeWorkspace, workspaceA, "brg-delivery-a")
		insertBridgeDeliveryInstance(t, globalDB, bridges.ScopeWorkspace, workspaceB, "brg-delivery-b")

		for _, record := range []bridges.DeliveryLedgerRecord{
			bridgeDeliveryRecordForTest(
				"delivery-a",
				"brg-delivery-a",
				bridges.ScopeWorkspace,
				workspaceA,
				bridgeDeliveryTestTime(),
			),
			bridgeDeliveryRecordForTest(
				"delivery-b",
				"brg-delivery-b",
				bridges.ScopeWorkspace,
				workspaceB,
				bridgeDeliveryTestTime(),
			),
		} {
			if err := globalDB.CreateBridgeDelivery(ctx, record); err != nil {
				t.Fatalf("CreateBridgeDelivery(%q) error = %v", record.DeliveryID, err)
			}
			metrics := bridgeDeliveryMetricRecordForTest(
				record.BridgeInstanceID,
				record.Scope,
				record.WorkspaceID,
				bridgeDeliveryTestTime().Add(time.Minute),
			)
			if err := globalDB.PutBridgeDeliveryMetrics(ctx, metrics); err != nil {
				t.Fatalf("PutBridgeDeliveryMetrics(%q) error = %v", record.BridgeInstanceID, err)
			}
		}

		deliveriesA, err := globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope:       bridges.ScopeWorkspace,
			WorkspaceID: workspaceA,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveries(workspace A) error = %v", err)
		}
		if len(deliveriesA) != 1 || deliveriesA[0].WorkspaceID != workspaceA {
			t.Fatalf("workspace A deliveries = %#v, want only workspace A", deliveriesA)
		}

		metricsA, err := globalDB.ListBridgeDeliveryMetrics(ctx, bridges.DeliveryLedgerQuery{
			Scope:       bridges.ScopeWorkspace,
			WorkspaceID: workspaceA,
		})
		if err != nil {
			t.Fatalf("ListBridgeDeliveryMetrics(workspace A) error = %v", err)
		}
		if len(metricsA) != 1 {
			t.Fatalf("workspace A metrics length = %d, want 1", len(metricsA))
		}
		if _, exists := metricsA["brg-delivery-b"]; exists {
			t.Fatalf("workspace A metrics leaked workspace B: %#v", metricsA)
		}

		if _, err := globalDB.ListBridgeDeliveries(ctx, bridges.DeliveryLedgerQuery{
			Scope:       bridges.ScopeGlobal,
			WorkspaceID: workspaceA,
		}); err == nil {
			t.Fatal("ListBridgeDeliveries(global with workspace) error = nil, want validation error")
		}
	})

	t.Run("Should reject a row whose direct scope disagrees with its bridge instance", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"delivery-mismatch",
			filepath.Join(t.TempDir(), "mismatch"),
		)
		insertBridgeDeliveryInstance(
			t,
			globalDB,
			bridges.ScopeWorkspace,
			workspaceID,
			"brg-delivery-mismatch",
		)
		record := bridgeDeliveryRecordForTest(
			"delivery-mismatch",
			"brg-delivery-mismatch",
			bridges.ScopeWorkspace,
			workspaceID,
			bridgeDeliveryTestTime(),
		)
		record.Scope = bridges.ScopeGlobal
		record.WorkspaceID = ""
		record.RoutingKey.Scope = bridges.ScopeGlobal
		record.RoutingKey.WorkspaceID = ""
		if err := globalDB.CreateBridgeDelivery(testutil.Context(t), record); err == nil {
			t.Fatal("CreateBridgeDelivery(scope mismatch) error = nil, want validation error")
		}
	})
}

func assertBridgeDeliverySchema(t *testing.T, db *sql.DB) {
	t.Helper()

	assertBridgeDeliveryTableColumns(t, db, "bridge_deliveries", []string{
		"delivery_id",
		"session_id",
		"turn_id",
		"routing_key",
		"bridge_instance_id",
		"scope",
		"workspace_id",
		"state",
		"last_sent_seq",
		"last_acked_seq",
		"remote_message_id",
		"terminal_error",
		"created_at",
		"updated_at",
	})
	assertBridgeDeliveryTableColumns(t, db, "bridge_delivery_metrics", []string{
		"bridge_instance_id",
		"scope",
		"workspace_id",
		"delivery_dropped_total",
		"delivery_dropped_by_reason_json",
		"delivery_failures_total",
		"last_error",
		"last_error_at",
		"last_success_at",
		"updated_at",
	})
	for _, indexName := range []string{
		"idx_bridge_deliveries_scope",
		"idx_bridge_deliveries_instance",
		"idx_bridge_delivery_metrics_scope",
	} {
		var count int
		if err := db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`,
			indexName,
		).Scan(&count); err != nil {
			t.Fatalf("query index %q error = %v", indexName, err)
		}
		if count != 1 {
			t.Fatalf("index %q count = %d, want 1", indexName, count)
		}
	}
	for _, triggerName := range []string{
		"trg_bridge_instance_active_delivery_delete",
		"trg_bridge_instance_active_delivery_identity",
	} {
		var count int
		if err := db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?`,
			triggerName,
		).Scan(&count); err != nil {
			t.Fatalf("query trigger %q error = %v", triggerName, err)
		}
		if count != 1 {
			t.Fatalf("trigger %q count = %d, want 1", triggerName, count)
		}
	}
}

func assertBridgeDeliveryMigrationStatus(t *testing.T, db *sql.DB) {
	t.Helper()

	status, err := store.Status(testutil.Context(t), db, MigrationStream())
	if err != nil {
		t.Fatalf("Status(global) error = %v", err)
	}
	assertCompleteMigrationStream(t, status, MigrationStream())
	if status.SumDigest == "" {
		t.Fatalf("Status(global) = %#v, want a migration digest", status)
	}
}

func previousBridgeDeliveryMigrationStream(t *testing.T) store.MigrationStream {
	t.Helper()

	const (
		baselinePath = "migrations/00001_baseline.sql"
		baselineSum  = "h1:sflbj6iO466pZng53VW9YGxoeswxiSSuTezj50qs5Ys=\n" +
			"00001_baseline.sql h1:S/Hl9w8PyqMpsW5NP7g/7u/VnQI84pVFJZUUBi0MMy0=\n"
	)
	baseline, err := fs.ReadFile(globalschema.Files, baselinePath)
	if err != nil {
		t.Fatalf("read previous global migration prefix: %v", err)
	}
	stream := MigrationStream()
	stream.FS = fstest.MapFS{
		"00001_baseline.sql": &fstest.MapFile{Data: baseline},
		"atlas.sum":          &fstest.MapFile{Data: []byte(baselineSum)},
	}
	stream.Dir = "."
	return stream
}

func assertBridgeDeliveryTableColumns(t *testing.T, db *sql.DB, table string, want []string) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), `SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatalf("query %s columns error = %v", table, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close %s column rows error = %v", table, err)
		}
	}()
	got := make([]string, 0, len(want))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan %s column error = %v", table, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns error = %v", table, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s columns = %#v, want %#v", table, got, want)
	}
}

func insertBridgeDeliveryInstance(
	t *testing.T,
	globalDB *GlobalDB,
	scope bridges.Scope,
	workspaceID string,
	id string,
) {
	t.Helper()

	now := bridgeDeliveryTestTime()
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), bridges.BridgeInstance{
		ID:            id,
		Scope:         scope,
		WorkspaceID:   workspaceID,
		Platform:      "telegram",
		ExtensionName: "telegram-extension",
		DisplayName:   "Telegram",
		Source:        bridges.BridgeInstanceSourceDynamic,
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		DMPolicy:      bridges.BridgeDMPolicyOpen,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("InsertBridgeInstance(%q) error = %v", id, err)
	}
}

func bridgeDeliveryRecordForTest(
	deliveryID string,
	bridgeInstanceID string,
	scope bridges.Scope,
	workspaceID string,
	createdAt time.Time,
) bridges.DeliveryLedgerRecord {
	return bridges.DeliveryLedgerRecord{
		DeliveryID:       deliveryID,
		SessionID:        "session-" + deliveryID,
		TurnID:           "turn-" + deliveryID,
		BridgeInstanceID: bridgeInstanceID,
		Scope:            scope,
		WorkspaceID:      workspaceID,
		RoutingKey: bridges.RoutingKey{
			Scope:            scope,
			WorkspaceID:      workspaceID,
			BridgeInstanceID: bridgeInstanceID,
			PeerID:           "peer-1",
			ThreadID:         "thread-1",
		},
		State:     bridges.DeliveryLedgerStateActive,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func bridgeDeliveryMetricRecordForTest(
	bridgeInstanceID string,
	scope bridges.Scope,
	workspaceID string,
	updatedAt time.Time,
) bridges.DeliveryMetricRecord {
	return bridges.DeliveryMetricRecord{
		Scope:       scope,
		WorkspaceID: workspaceID,
		BridgeDeliveryMetrics: bridges.BridgeDeliveryMetrics{
			BridgeInstanceID:        bridgeInstanceID,
			DeliveryDroppedTotal:    3,
			DeliveryDroppedByReason: map[string]int{"queue_saturated": 2, "transport": 1},
			DeliveryFailuresTotal:   2,
			LastError:               "provider unavailable",
			LastErrorAt:             updatedAt.Add(-time.Second),
			LastSuccessAt:           updatedAt.Add(-2 * time.Second),
		},
		UpdatedAt: updatedAt,
	}
}

func assertBridgeDeliveryRecordEqual(
	t *testing.T,
	got bridges.DeliveryLedgerRecord,
	want bridges.DeliveryLedgerRecord,
) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delivery record = %#v, want %#v", got, want)
	}
}

func assertCanonicalRoutingKeyPersisted(
	t *testing.T,
	db *sql.DB,
	record bridges.DeliveryLedgerRecord,
) {
	t.Helper()

	want, err := record.RoutingKey.Serialize()
	if err != nil {
		t.Fatalf("RoutingKey.Serialize() error = %v", err)
	}
	var got string
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT routing_key FROM bridge_deliveries WHERE delivery_id = ?`,
		record.DeliveryID,
	).Scan(&got); err != nil {
		t.Fatalf("query persisted routing key error = %v", err)
	}
	if got != want {
		t.Fatalf("persisted routing key = %q, want canonical %q", got, want)
	}
}

func assertBridgeDeliveryMetricsEqual(
	t *testing.T,
	got bridges.BridgeDeliveryMetrics,
	want bridges.BridgeDeliveryMetrics,
) {
	t.Helper()

	want.DeliveryBacklog = got.DeliveryBacklog
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delivery metrics = %#v, want %#v", got, want)
	}
}

func bridgeDeliveryTestTime() time.Time {
	return time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)
}

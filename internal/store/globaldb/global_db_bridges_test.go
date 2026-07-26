package globaldb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestOpenGlobalDBCreatesBridgeTables(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	assertTablesPresent(
		t,
		globalDB.db,
		"bridge_instances",
		"bridge_secret_bindings",
		"bridge_routes",
		"bridge_ingest_dedup",
	)
	assertTableColumns(t, globalDB.db, "bridge_instances", []string{
		"id",
		"scope",
		"workspace_id",
		"platform",
		"extension_name",
		"display_name",
		"source",
		"enabled",
		"status",
		"dm_policy",
		"routing_policy",
		"provider_config",
		"delivery_defaults",
		"notification_suppress",
		"degradation_reason",
		"degradation_message",
		"created_at",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "bridge_secret_bindings", []string{
		"bridge_instance_id",
		"binding_name",
		"secret_ref",
		"kind",
		"created_at",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "bridge_routes", []string{
		"routing_key_hash",
		"scope",
		"workspace_id",
		"bridge_instance_id",
		"peer_id",
		"thread_id",
		"group_id",
		"session_id",
		"agent_name",
		"last_activity_at",
		"created_at",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "bridge_ingest_dedup", []string{
		"idempotency_key",
		"bridge_instance_id",
		"received_at",
		"expires_at",
	})

	now := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO bridge_instances (
			id, scope, workspace_id, platform, extension_name, display_name, enabled, status, routing_policy, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"brg-default-source",
		string(bridges.ScopeGlobal),
		nil,
		"telegram",
		"telegram-adapter",
		"Default Source Bridge",
		true,
		string(bridges.BridgeStatusReady),
		`{"include_peer":true}`,
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("ExecContext(insert legacy bridge row without source) error = %v", err)
	}

	loaded, err := globalDB.GetBridgeInstance(testutil.Context(t), "brg-default-source")
	if err != nil {
		t.Fatalf("GetBridgeInstance() error = %v", err)
	}
	if got, want := loaded.Source, bridges.BridgeInstanceSourceDynamic; got != want {
		t.Fatalf("loaded.Source = %q, want %q", got, want)
	}
	if loaded.Source == "" {
		t.Fatal("loaded.Source = empty, want default source value")
	}
}

func TestGlobalDBListBridgeInstancesByIDsIsBoundedAndOrdered(t *testing.T) {
	t.Parallel()

	t.Run("Should return requested rows in caller order and omit missing or foreign ids", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		for _, id := range []string{"brg-a", "brg-b", "brg-foreign"} {
			if err := globalDB.InsertBridgeInstance(ctx, bridges.BridgeInstance{
				ID:            id,
				Scope:         bridges.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   id,
				Enabled:       true,
				Status:        bridges.BridgeStatusReady,
				RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			}); err != nil {
				t.Fatalf("InsertBridgeInstance(%s) error = %v", id, err)
			}
		}

		instances, err := globalDB.ListBridgeInstancesByIDs(ctx, []string{"brg-b", "brg-missing", "brg-a"})
		if err != nil {
			t.Fatalf("ListBridgeInstancesByIDs() error = %v", err)
		}
		ids := make([]string, 0, len(instances))
		for _, instance := range instances {
			ids = append(ids, instance.ID)
		}
		if !slices.Equal(ids, []string{"brg-b", "brg-a"}) {
			t.Fatalf("ListBridgeInstancesByIDs() ids = %#v, want caller order without missing/foreign ids", ids)
		}
	})

	t.Run("Should reject empty and oversized batches", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		if _, err := globalDB.ListBridgeInstancesByIDs(testutil.Context(t), nil); err == nil {
			t.Fatal("ListBridgeInstancesByIDs(nil) error = nil, want non-nil")
		}
		ids := make([]string, bridges.MaxBridgeCatalogLimit+1)
		for index := range ids {
			ids[index] = fmt.Sprintf("brg-%03d", index)
		}
		if _, err := globalDB.ListBridgeInstancesByIDs(testutil.Context(t), ids); err == nil {
			t.Fatal("ListBridgeInstancesByIDs(oversized) error = nil, want non-nil")
		}
	})
}

func TestGlobalDBBridgeCatalogProjectionHydratesOnlyPageIDs(t *testing.T) {
	t.Parallel()

	t.Run("Should not read invalid oversized JSON outside the selected page", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
		if err := globalDB.InsertWorkspace(ctx, aghworkspace.Workspace{
			ID:        "ws-beta",
			RootDir:   t.TempDir(),
			Name:      "Beta",
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace(ws-beta) error = %v", err)
		}
		for _, instance := range []bridges.BridgeInstance{
			{
				ID:            "brg-alpha",
				Scope:         bridges.ScopeGlobal,
				Platform:      "slack",
				ExtensionName: "slack-adapter",
				DisplayName:   "Alpha",
				Enabled:       true,
				Status:        bridges.BridgeStatusReady,
				RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			},
			{
				ID:            "brg-zulu",
				Scope:         bridges.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   "Zulu",
				Enabled:       true,
				Status:        bridges.BridgeStatusReady,
				RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			},
			{
				ID:            "brg-foreign",
				Scope:         bridges.ScopeWorkspace,
				WorkspaceID:   "ws-beta",
				Platform:      "slack",
				ExtensionName: "slack-adapter",
				DisplayName:   "Foreign",
				Enabled:       true,
				Status:        bridges.BridgeStatusReady,
				RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			},
		} {
			if err := globalDB.InsertBridgeInstance(ctx, instance); err != nil {
				t.Fatalf("InsertBridgeInstance(%s) error = %v", instance.ID, err)
			}
		}
		invalidOversizedJSON := strings.Repeat("x", 1<<20)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE bridge_instances SET routing_policy = ? WHERE id = ?`,
			invalidOversizedJSON,
			"brg-zulu",
		); err != nil {
			t.Fatalf("ExecContext(corrupt off-page routing policy) error = %v", err)
		}
		if _, err := globalDB.ListBridgeInstances(ctx); err == nil {
			t.Fatal("ListBridgeInstances() error = nil, want corrupt off-page JSON to fail full hydration")
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE bridge_instances SET status = ? WHERE id = ?`,
			"foreign-invalid-status",
			"brg-foreign",
		); err != nil {
			t.Fatalf("ExecContext(corrupt foreign catalog metadata) error = %v", err)
		}

		query := bridges.BridgeCatalogQuery{Scope: "all", WorkspaceID: "ws-alpha", Limit: 1}
		records, err := globalDB.ListBridgeCatalogRecords(ctx, query)
		if err != nil {
			t.Fatalf("ListBridgeCatalogRecords() error = %v", err)
		}
		selection, err := bridges.BuildBridgeCatalogRecordPage(
			records,
			nil,
			query,
		)
		if err != nil {
			t.Fatalf("BuildBridgeCatalogRecordPage() error = %v", err)
		}
		if len(selection.Items) != 1 || selection.Items[0].Record.ID != "brg-alpha" ||
			selection.Total != 2 || !selection.HasMore {
			t.Fatalf("catalog selection = %#v, want alpha-only first page of two records", selection)
		}

		instances, err := globalDB.ListBridgeInstancesByIDs(ctx, []string{selection.Items[0].Record.ID})
		if err != nil {
			t.Fatalf("ListBridgeInstancesByIDs(page) error = %v", err)
		}
		page, err := bridges.HydrateBridgeCatalogPage(selection, instances)
		if err != nil {
			t.Fatalf("HydrateBridgeCatalogPage() error = %v", err)
		}
		if len(page.Items) != 1 || page.Items[0].Instance.ID != "brg-alpha" {
			t.Fatalf("hydrated page = %#v, want alpha only", page)
		}
	})
}

func TestGlobalDBBridgeListsPropagateRowsCloseErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		list func(context.Context, *GlobalDB) error
	}{
		{
			name: "Should propagate secret binding rows close errors",
			list: func(ctx context.Context, globalDB *GlobalDB) error {
				_, err := globalDB.ListBridgeSecretBindings(ctx, "brg-close-error")
				return err
			},
		},
		{
			name: "Should propagate route rows close errors",
			list: func(ctx context.Context, globalDB *GlobalDB) error {
				_, err := globalDB.ListBridgeRoutes(ctx, "brg-close-error")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openBridgeCloseErrorGlobalDB(t)
			err := test.list(testutil.Context(t), globalDB)
			if !errors.Is(err, errBridgeCatalogRowsClose) {
				t.Fatalf("bridge list error = %v, want joined rows close error", err)
			}
		})
	}
}

var errBridgeCatalogRowsClose = errors.New("bridge catalog rows close failed")

const bridgeCloseErrorDriverName = "agh-bridge-close-error"

func init() {
	sql.Register(bridgeCloseErrorDriverName, bridgeCloseErrorDriver{})
}

type bridgeCloseErrorDriver struct{}

func (bridgeCloseErrorDriver) Open(string) (driver.Conn, error) {
	return bridgeCloseErrorConn{}, nil
}

type bridgeCloseErrorConn struct{}

func (bridgeCloseErrorConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("bridge close-error driver does not prepare statements")
}

func (bridgeCloseErrorConn) Close() error {
	return nil
}

func (bridgeCloseErrorConn) Begin() (driver.Tx, error) {
	return nil, errors.New("bridge close-error driver does not begin transactions")
}

func (bridgeCloseErrorConn) QueryContext(
	ctx context.Context,
	query string,
	_ []driver.NamedValue,
) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(query, "FROM bridge_secret_bindings"):
		return &bridgeCloseErrorRows{
			columns: []string{
				"bridge_instance_id",
				"binding_name",
				"secret_ref",
				"kind",
				"created_at",
				"updated_at",
			},
			values: []driver.Value{
				"brg-close-error",
				"bot_token",
				"vault:bridges/brg-close-error/bot_token",
				"token",
				"invalid-created-at",
				"invalid-updated-at",
			},
		}, nil
	case strings.Contains(query, "FROM bridge_routes"):
		return &bridgeCloseErrorRows{
			columns: []string{
				"routing_key_hash",
				"scope",
				"workspace_id",
				"bridge_instance_id",
				"peer_id",
				"thread_id",
				"group_id",
				"session_id",
				"agent_name",
				"last_activity_at",
				"created_at",
				"updated_at",
			},
			values: []driver.Value{
				"route-close-error",
				string(bridges.ScopeGlobal),
				nil,
				"brg-close-error",
				"peer-1",
				nil,
				nil,
				"sess-1",
				"coder",
				"invalid-last-activity-at",
				"invalid-created-at",
				"invalid-updated-at",
			},
		}, nil
	default:
		return nil, fmt.Errorf("bridge close-error driver: unexpected query %q", query)
	}
}

type bridgeCloseErrorRows struct {
	columns []string
	values  []driver.Value
	yielded bool
}

func (r *bridgeCloseErrorRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*bridgeCloseErrorRows) Close() error {
	return errBridgeCatalogRowsClose
}

func (r *bridgeCloseErrorRows) Next(destination []driver.Value) error {
	if r.yielded {
		return io.EOF
	}
	copy(destination, r.values)
	r.yielded = true
	return nil
}

func openBridgeCloseErrorGlobalDB(t *testing.T) *GlobalDB {
	t.Helper()

	db, err := sql.Open(bridgeCloseErrorDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open(bridge close-error driver) error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("sql.DB.Close(bridge close-error driver) error = %v", err)
		}
	})
	globalDB := &GlobalDB{db: db, now: func() time.Time { return time.Now().UTC() }}
	globalDB.initializeRepositories(openConfig{})
	return globalDB
}

func TestGlobalDBBridgeTargetDirectoryRefresh(t *testing.T) {
	t.Run("Should preserve missing target rows while advancing bridge refresh freshness", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		instance := bridges.BridgeInstance{
			ID:            "brg-target-refresh",
			Scope:         bridges.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "slack-extension",
			DisplayName:   "Slack",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			DMPolicy:      bridges.BridgeDMPolicyOpen,
			CreatedAt:     time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		}
		if err := globalDB.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}

		firstRefresh := time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
		firstService := bridges.NewRegistry(globalDB, bridges.WithNow(func() time.Time { return firstRefresh }))
		if _, err := firstService.RefreshBridgeTargets(ctx, instance.ID, []bridges.BridgeTargetSnapshot{
			{
				CanonicalRoute: "slack:channel:C123",
				DisplayName:    "#alerts",
				TargetType:     bridges.BridgeTargetTypeChannel,
				Qualifier:      "slack",
			},
			{
				CanonicalRoute: "slack:channel:C999",
				DisplayName:    "#archive",
				TargetType:     bridges.BridgeTargetTypeChannel,
				Qualifier:      "slack",
			},
		}); err != nil {
			t.Fatalf("RefreshBridgeTargets(first) error = %v", err)
		}

		secondRefresh := firstRefresh.Add(5 * time.Minute)
		secondService := bridges.NewRegistry(globalDB, bridges.WithNow(func() time.Time { return secondRefresh }))
		if _, err := secondService.RefreshBridgeTargets(ctx, instance.ID, []bridges.BridgeTargetSnapshot{
			{
				CanonicalRoute: "slack:channel:C123",
				DisplayName:    "#alerts",
				TargetType:     bridges.BridgeTargetTypeChannel,
				Qualifier:      "slack",
			},
		}); err != nil {
			t.Fatalf("RefreshBridgeTargets(second) error = %v", err)
		}

		page, err := globalDB.ListBridgeTargets(ctx, bridges.BridgeTargetQuery{BridgeID: instance.ID, Limit: 10})
		if err != nil {
			t.Fatalf("ListBridgeTargets() error = %v", err)
		}
		if got, want := page.Total, 2; got != want {
			t.Fatalf("ListBridgeTargets().Total = %d, want %d", got, want)
		}
		if !page.LastSuccessfulRefreshAt.Equal(secondRefresh) {
			t.Fatalf(
				"ListBridgeTargets().LastSuccessfulRefreshAt = %s, want %s",
				page.LastSuccessfulRefreshAt,
				secondRefresh,
			)
		}

		var archive bridges.BridgeTarget
		for _, target := range page.Items {
			if target.CanonicalRoute == "slack:channel:C999" {
				archive = target
			}
		}
		if archive.CanonicalRoute == "" {
			t.Fatal("ListBridgeTargets() missing stale archive target")
		}
		if !archive.LastSeenAt.Equal(firstRefresh) {
			t.Fatalf("archive.LastSeenAt = %s, want %s", archive.LastSeenAt, firstRefresh)
		}
	})
}

func TestGlobalDBBridgeGuardClauses(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	if err := globalDB.InsertBridgeInstance(nilGlobalContext(), bridges.BridgeInstance{}); err == nil {
		t.Fatal("InsertBridgeInstance(nil ctx) error = nil, want non-nil")
	}
	if _, err := globalDB.GetBridgeInstance(nilGlobalContext(), "brg-1"); err == nil {
		t.Fatal("GetBridgeInstance(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.PutBridgeSecretBinding(nilGlobalContext(), bridges.BridgeSecretBinding{}); err == nil {
		t.Fatal("PutBridgeSecretBinding(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.PutBridgeRoute(nilGlobalContext(), bridges.BridgeRoute{}); err == nil {
		t.Fatal("PutBridgeRoute(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.PutBridgeIngestDedup(nilGlobalContext(), bridges.IngestDedupRecord{}); err == nil {
		t.Fatal("PutBridgeIngestDedup(nil ctx) error = nil, want non-nil")
	}
}

func TestGlobalDBBridgePersistenceHelpers(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	base := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	callCount := 0
	globalDB.now = func() time.Time {
		callCount++
		return base.Add(time.Duration(callCount) * time.Minute)
	}

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"bridge-workspace",
		filepath.Join(t.TempDir(), "bridge-workspace"),
	)
	instance := bridges.BridgeInstance{
		ID:             "brg-workspace",
		Scope:          bridges.ScopeWorkspace,
		WorkspaceID:    workspaceID,
		Platform:       "telegram",
		ExtensionName:  "telegram-adapter",
		DisplayName:    "Workspace Telegram",
		Enabled:        true,
		Status:         bridges.BridgeStatusReady,
		DMPolicy:       bridges.BridgeDMPolicyAllowlist,
		RoutingPolicy:  bridges.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		ProviderConfig: []byte(`{"mode":"bot","tenant":"workspace-alpha"}`),
	}
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), instance); err != nil {
		t.Fatalf("InsertBridgeInstance() error = %v", err)
	}

	loaded, err := globalDB.GetBridgeInstance(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("GetBridgeInstance() error = %v", err)
	}
	if loaded.WorkspaceID != workspaceID || loaded.Status != bridges.BridgeStatusReady {
		t.Fatalf("loaded bridge instance = %#v", loaded)
	}
	if got, want := loaded.DMPolicy, bridges.BridgeDMPolicyAllowlist; got != want {
		t.Fatalf("loaded.DMPolicy = %q, want %q", got, want)
	}
	if got, want := string(loaded.ProviderConfig), `{"mode":"bot","tenant":"workspace-alpha"}`; got != want {
		t.Fatalf("loaded.ProviderConfig = %s, want %s", got, want)
	}

	loaded.DisplayName = "Workspace Telegram Updated"
	loaded.Enabled = false
	loaded.Status = bridges.BridgeStatusDisabled
	loaded.DMPolicy = bridges.BridgeDMPolicyOpen
	loaded.ProviderConfig = []byte(`{"mode":"bot","tenant":"workspace-beta"}`)
	if err := globalDB.UpdateBridgeInstance(testutil.Context(t), loaded); err != nil {
		t.Fatalf("UpdateBridgeInstance() error = %v", err)
	}

	instances, err := globalDB.ListBridgeInstances(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListBridgeInstances() error = %v", err)
	}
	if got, want := len(instances), 1; got != want {
		t.Fatalf("len(instances) = %d, want %d", got, want)
	}
	if got, want := instances[0].DisplayName, "Workspace Telegram Updated"; got != want {
		t.Fatalf("instances[0].DisplayName = %q, want %q", got, want)
	}
	if got, want := string(instances[0].ProviderConfig), `{"mode":"bot","tenant":"workspace-beta"}`; got != want {
		t.Fatalf("instances[0].ProviderConfig = %s, want %s", got, want)
	}

	binding := bridges.BridgeSecretBinding{
		BridgeInstanceID: instance.ID,
		BindingName:      "bot_token",
		SecretRef:        "vault:bridges/" + instance.ID + "/bot_token",
		Kind:             "token",
	}
	if err := globalDB.PutBridgeSecretBinding(testutil.Context(t), binding); err != nil {
		t.Fatalf("PutBridgeSecretBinding() error = %v", err)
	}

	gotBinding, err := globalDB.GetBridgeSecretBinding(
		testutil.Context(t),
		binding.BridgeInstanceID,
		binding.BindingName,
	)
	if err != nil {
		t.Fatalf("GetBridgeSecretBinding() error = %v", err)
	}
	if gotBinding.SecretRef != binding.SecretRef {
		t.Fatalf("GetBridgeSecretBinding().SecretRef = %q, want %q", gotBinding.SecretRef, binding.SecretRef)
	}

	bindings, err := globalDB.ListBridgeSecretBindings(testutil.Context(t), binding.BridgeInstanceID)
	if err != nil {
		t.Fatalf("ListBridgeSecretBindings() error = %v", err)
	}
	if got, want := len(bindings), 1; got != want {
		t.Fatalf("len(bindings) = %d, want %d", got, want)
	}

	record := bridges.IngestDedupRecord{
		IdempotencyKey:   "idem-live",
		BridgeInstanceID: instance.ID,
		ReceivedAt:       base,
		ExpiresAt:        base.Add(5 * time.Minute),
	}
	if err := globalDB.PutBridgeIngestDedup(testutil.Context(t), record); err != nil {
		t.Fatalf("PutBridgeIngestDedup() error = %v", err)
	}

	expired := bridges.IngestDedupRecord{
		IdempotencyKey:   "idem-expired",
		BridgeInstanceID: instance.ID,
		ReceivedAt:       base.Add(-2 * time.Minute),
		ExpiresAt:        base.Add(-time.Minute),
	}
	if err := globalDB.PutBridgeIngestDedup(testutil.Context(t), expired); err != nil {
		t.Fatalf("PutBridgeIngestDedup(expired) error = %v", err)
	}

	liveRecord, err := globalDB.GetBridgeIngestDedup(testutil.Context(t), record.IdempotencyKey, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetBridgeIngestDedup(live) error = %v", err)
	}
	if liveRecord.BridgeInstanceID != instance.ID {
		t.Fatalf("GetBridgeIngestDedup(live) = %#v", liveRecord)
	}
	if _, err := globalDB.GetBridgeIngestDedup(
		testutil.Context(t),
		expired.IdempotencyKey,
		base.Add(time.Minute),
	); !errors.Is(
		err,
		bridges.ErrIngestDedupRecordNotFound,
	) {
		t.Fatalf("GetBridgeIngestDedup(expired) error = %v, want ErrIngestDedupRecordNotFound", err)
	}

	deleted, err := globalDB.DeleteExpiredBridgeIngestDedup(testutil.Context(t), base.Add(time.Minute))
	if err != nil {
		t.Fatalf("DeleteExpiredBridgeIngestDedup() error = %v", err)
	}
	if got, want := deleted, int64(1); got != want {
		t.Fatalf("DeleteExpiredBridgeIngestDedup() = %d, want %d", got, want)
	}

	if err := globalDB.DeleteBridgeSecretBinding(
		testutil.Context(t),
		binding.BridgeInstanceID,
		binding.BindingName,
	); err != nil {
		t.Fatalf("DeleteBridgeSecretBinding() error = %v", err)
	}
	if err := globalDB.DeleteBridgeInstance(testutil.Context(t), instance.ID); err != nil {
		t.Fatalf("DeleteBridgeInstance() error = %v", err)
	}
}

func TestGlobalDBBridgeTargetDirectoryShouldPreserveMissingSnapshots(t *testing.T) {
	t.Parallel()

	t.Run("Should upsert refreshed targets without deleting stale rows", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
		registry := bridges.NewRegistry(globalDB, bridges.WithNow(func() time.Time { return now }))
		instance := bridges.BridgeInstance{
			ID:            "brg-targets",
			Scope:         bridges.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-slack",
			DisplayName:   "Slack Targets",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true, IncludeGroup: true},
		}
		if err := globalDB.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}

		generalLastSeen := now.Add(-30 * time.Minute)
		if _, err := registry.RefreshBridgeTargets(ctx, instance.ID, []bridges.BridgeTargetSnapshot{
			{
				CanonicalRoute: "slack://T1/C-general",
				DisplayName:    "General",
				TargetType:     bridges.BridgeTargetTypeChannel,
				Qualifier:      "northstar",
				Capabilities:   []string{"thread", "send", "send"},
				LastSeenAt:     generalLastSeen,
			},
			{
				CanonicalRoute: "slack://T1/C-ops",
				DisplayName:    "Ops",
				TargetType:     bridges.BridgeTargetTypeChannel,
				Qualifier:      "northstar",
				Capabilities:   []string{"send"},
			},
		}); err != nil {
			t.Fatalf("RefreshBridgeTargets(initial) error = %v", err)
		}

		now = now.Add(10 * time.Minute)
		if _, err := registry.RefreshBridgeTargets(ctx, instance.ID, []bridges.BridgeTargetSnapshot{
			{
				CanonicalRoute: "slack://T1/C-ops",
				DisplayName:    "Ops",
				TargetType:     bridges.BridgeTargetTypeChannel,
				Qualifier:      "northstar",
				Capabilities:   []string{"send", "mention"},
			},
		}); err != nil {
			t.Fatalf("RefreshBridgeTargets(refresh) error = %v", err)
		}

		page, err := globalDB.ListBridgeTargets(ctx, bridges.BridgeTargetQuery{
			BridgeID: instance.ID,
			Limit:    10,
		})
		if err != nil {
			t.Fatalf("ListBridgeTargets() error = %v", err)
		}
		if got, want := page.Total, 2; got != want {
			t.Fatalf("ListBridgeTargets().Total = %d, want %d", got, want)
		}
		targets := make(map[string]bridges.BridgeTarget, len(page.Items))
		for _, target := range page.Items {
			targets[target.CanonicalRoute] = target
		}
		general, ok := targets["slack://T1/C-general"]
		if !ok {
			t.Fatal("missing stale general target after refresh")
		}
		if !general.LastSeenAt.Equal(generalLastSeen) {
			t.Fatalf("general.LastSeenAt = %s, want %s", general.LastSeenAt, generalLastSeen)
		}
		if got, want := general.Capabilities, []string{"send", "thread"}; !slices.Equal(got, want) {
			t.Fatalf("general.Capabilities = %#v, want %#v", got, want)
		}
		ops, ok := targets["slack://T1/C-ops"]
		if !ok {
			t.Fatal("missing refreshed ops target")
		}
		if !ops.LastSeenAt.Equal(now) {
			t.Fatalf("ops.LastSeenAt = %s, want %s", ops.LastSeenAt, now)
		}
		if got, want := ops.Capabilities, []string{"mention", "send"}; !slices.Equal(got, want) {
			t.Fatalf("ops.Capabilities = %#v, want %#v", got, want)
		}
		if !page.LastSuccessfulRefreshAt.Equal(now) {
			t.Fatalf("LastSuccessfulRefreshAt = %s, want %s", page.LastSuccessfulRefreshAt, now)
		}
	})
}

func TestGlobalDBReplaceBridgeInstancesAtomicallySwapsProjection(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	now := time.Date(2026, 4, 16, 13, 0, 0, 0, time.UTC)
	stale := bridges.BridgeInstance{
		ID:            "brg-stale",
		Scope:         bridges.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "Stale Bridge",
		Source:        bridges.BridgeInstanceSourceDynamic,
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		DMPolicy:      bridges.BridgeDMPolicyOpen,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
		CreatedAt:     now.Add(-time.Hour),
		UpdatedAt:     now.Add(-time.Hour),
	}
	keep := stale
	keep.ID = "brg-keep"
	keep.DisplayName = "Keep Bridge"
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), stale); err != nil {
		t.Fatalf("InsertBridgeInstance(stale) error = %v", err)
	}
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), keep); err != nil {
		t.Fatalf("InsertBridgeInstance(keep) error = %v", err)
	}

	keep.DisplayName = "Projected Bridge"
	keep.UpdatedAt = now
	added := bridges.BridgeInstance{
		ID:            "brg-added",
		Scope:         bridges.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: "ext-slack",
		DisplayName:   "Added Bridge",
		Source:        bridges.BridgeInstanceSourceDynamic,
		Enabled:       false,
		Status:        bridges.BridgeStatusDisabled,
		DMPolicy:      bridges.BridgeDMPolicyPairing,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := globalDB.ReplaceBridgeInstances(testutil.Context(t), []bridges.BridgeInstance{keep, added}); err != nil {
		t.Fatalf("ReplaceBridgeInstances() error = %v", err)
	}

	if _, err := globalDB.GetBridgeInstance(testutil.Context(t), stale.ID); !errors.Is(
		err,
		bridges.ErrBridgeInstanceNotFound,
	) {
		t.Fatalf("GetBridgeInstance(stale) error = %v, want ErrBridgeInstanceNotFound", err)
	}
	loaded, err := globalDB.GetBridgeInstance(testutil.Context(t), keep.ID)
	if err != nil {
		t.Fatalf("GetBridgeInstance(keep) error = %v", err)
	}
	if got, want := loaded.DisplayName, "Projected Bridge"; got != want {
		t.Fatalf("loaded.DisplayName = %q, want %q", got, want)
	}
	instances, err := globalDB.ListBridgeInstances(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListBridgeInstances() error = %v", err)
	}
	if got, want := len(instances), 2; got != want {
		t.Fatalf("len(ListBridgeInstances()) = %d, want %d", got, want)
	}
}

func TestGlobalDBBridgeRouteCRUD(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	instance := bridges.BridgeInstance{
		ID:            "brg-route-unit",
		Scope:         bridges.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Route Unit",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true, IncludeThread: true},
	}
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), instance); err != nil {
		t.Fatalf("InsertBridgeInstance() error = %v", err)
	}

	route := bridges.BridgeRoute{
		Scope:            bridges.ScopeGlobal,
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		GroupID:          "group-1",
		SessionID:        "sess-route-unit",
		AgentName:        "coder",
		LastActivityAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		CreatedAt:        time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	}
	if err := globalDB.PutBridgeRoute(testutil.Context(t), route); err != nil {
		t.Fatalf("PutBridgeRoute() error = %v", err)
	}

	canonical, err := route.Canonicalize()
	if err != nil {
		t.Fatalf("route.Canonicalize() error = %v", err)
	}
	loaded, err := globalDB.GetBridgeRoute(testutil.Context(t), canonical.RoutingKeyHash)
	if err != nil {
		t.Fatalf("GetBridgeRoute() error = %v", err)
	}
	if loaded.ThreadID != route.ThreadID || loaded.GroupID != route.GroupID {
		t.Fatalf("GetBridgeRoute() = %#v, want thread/group from %#v", loaded, route)
	}

	resolved, err := globalDB.ResolveBridgeRoute(testutil.Context(t), route.RoutingKey())
	if err != nil {
		t.Fatalf("ResolveBridgeRoute() error = %v", err)
	}
	if resolved.RoutingKeyHash != canonical.RoutingKeyHash {
		t.Fatalf("ResolveBridgeRoute().RoutingKeyHash = %q, want %q", resolved.RoutingKeyHash, canonical.RoutingKeyHash)
	}

	routes, err := globalDB.ListBridgeRoutes(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("ListBridgeRoutes() error = %v", err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}

	if err := globalDB.DeleteBridgeRoute(testutil.Context(t), canonical.RoutingKeyHash); err != nil {
		t.Fatalf("DeleteBridgeRoute() error = %v", err)
	}
	if _, err := globalDB.GetBridgeRoute(
		testutil.Context(t),
		canonical.RoutingKeyHash,
	); !errors.Is(
		err,
		bridges.ErrBridgeRouteNotFound,
	) {
		t.Fatalf("GetBridgeRoute(after delete) error = %v, want ErrBridgeRouteNotFound", err)
	}
	if err := globalDB.DeleteBridgeRoute(
		testutil.Context(t),
		canonical.RoutingKeyHash,
	); !errors.Is(
		err,
		bridges.ErrBridgeRouteNotFound,
	) {
		t.Fatalf("DeleteBridgeRoute(after delete) error = %v, want ErrBridgeRouteNotFound", err)
	}
}

func TestGlobalDBBridgeCatalogBatchReads(t *testing.T) {
	t.Parallel()

	t.Run("Should count routes and load bindings only for requested bridge ids", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		for _, id := range []string{"brg-batch-a", "brg-batch-b", "brg-batch-c"} {
			if err := globalDB.InsertBridgeInstance(ctx, bridges.BridgeInstance{
				ID:            id,
				Scope:         bridges.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-adapter",
				DisplayName:   id,
				Enabled:       true,
				Status:        bridges.BridgeStatusReady,
				RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			}); err != nil {
				t.Fatalf("InsertBridgeInstance(%s) error = %v", id, err)
			}
		}
		for _, route := range []bridges.BridgeRoute{
			{Scope: bridges.ScopeGlobal, BridgeInstanceID: "brg-batch-a", PeerID: "peer-a1", SessionID: "sess-a1", AgentName: "coder"},
			{Scope: bridges.ScopeGlobal, BridgeInstanceID: "brg-batch-a", PeerID: "peer-a2", SessionID: "sess-a2", AgentName: "coder"},
			{Scope: bridges.ScopeGlobal, BridgeInstanceID: "brg-batch-b", PeerID: "peer-b1", SessionID: "sess-b1", AgentName: "coder"},
			{Scope: bridges.ScopeGlobal, BridgeInstanceID: "brg-batch-c", PeerID: "peer-c1", SessionID: "sess-c1", AgentName: "coder"},
		} {
			if err := globalDB.PutBridgeRoute(ctx, route); err != nil {
				t.Fatalf("PutBridgeRoute(%s/%s) error = %v", route.BridgeInstanceID, route.PeerID, err)
			}
		}
		for _, binding := range []bridges.BridgeSecretBinding{
			{
				BridgeInstanceID: "brg-batch-a",
				BindingName:      "bot_token",
				SecretRef:        "vault:bridges/brg-batch-a/bot_token",
				Kind:             "token",
			},
			{
				BridgeInstanceID: "brg-batch-c",
				BindingName:      "bot_token",
				SecretRef:        "vault:bridges/brg-batch-c/bot_token",
				Kind:             "token",
			},
		} {
			if err := globalDB.PutBridgeSecretBinding(ctx, binding); err != nil {
				t.Fatalf("PutBridgeSecretBinding(%s) error = %v", binding.BridgeInstanceID, err)
			}
		}

		ids := []string{"brg-batch-a", "brg-batch-b"}
		counts, err := globalDB.CountBridgeRoutes(ctx, ids)
		if err != nil {
			t.Fatalf("CountBridgeRoutes() error = %v", err)
		}
		if counts["brg-batch-a"] != 2 || counts["brg-batch-b"] != 1 || counts["brg-batch-c"] != 0 {
			t.Fatalf("CountBridgeRoutes() = %#v, want a=2 b=1 without c", counts)
		}

		bindings, err := globalDB.ListBridgeSecretBindingsForInstances(ctx, ids)
		if err != nil {
			t.Fatalf("ListBridgeSecretBindingsForInstances() error = %v", err)
		}
		if got, want := len(bindings["brg-batch-a"]), 1; got != want {
			t.Fatalf("len(bindings[a]) = %d, want %d", got, want)
		}
		if got := len(bindings["brg-batch-b"]); got != 0 {
			t.Fatalf("len(bindings[b]) = %d, want 0", got)
		}
		if _, leaked := bindings["brg-batch-c"]; leaked {
			t.Fatalf("bindings leaked unrequested bridge: %#v", bindings)
		}
	})

	t.Run("Should return initialized empty batch results", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		counts, err := globalDB.CountBridgeRoutes(testutil.Context(t), nil)
		if err != nil {
			t.Fatalf("CountBridgeRoutes(empty) error = %v", err)
		}
		bindings, err := globalDB.ListBridgeSecretBindingsForInstances(testutil.Context(t), nil)
		if err != nil {
			t.Fatalf("ListBridgeSecretBindingsForInstances(empty) error = %v", err)
		}
		if counts == nil || bindings == nil || len(counts) != 0 || len(bindings) != 0 {
			t.Fatalf("empty batch results = counts:%#v bindings:%#v, want initialized empty maps", counts, bindings)
		}
	})
}

func TestNormalizeBridgeInstanceRecordEncodesProviderConfigAndDegradation(t *testing.T) {
	t.Parallel()

	instance := bridges.BridgeInstance{
		ID:               "brg-encode",
		Scope:            bridges.ScopeGlobal,
		Platform:         "slack",
		ExtensionName:    "slack-adapter",
		DisplayName:      "Slack",
		Enabled:          true,
		Status:           bridges.BridgeStatusDegraded,
		DMPolicy:         bridges.BridgeDMPolicyAllowlist,
		RoutingPolicy:    bridges.RoutingPolicy{IncludePeer: true},
		ProviderConfig:   json.RawMessage(`{"mode":"bot","tenant":"acme"}`),
		DeliveryDefaults: json.RawMessage(`{"mode":"reply","peer_id":"peer-1"}`),
		Degradation: &bridges.BridgeDegradation{
			Reason:  bridges.BridgeDegradationReasonRateLimited,
			Message: "provider throttled requests",
		},
	}

	normalized, routingPolicyJSON, providerConfig, deliveryDefaults, degradationReason, degradationMessage, err := normalizeBridgeInstanceRecord(
		instance,
	)
	if err != nil {
		t.Fatalf("normalizeBridgeInstanceRecord() error = %v", err)
	}
	if got, want := normalized.DMPolicy, bridges.BridgeDMPolicyAllowlist; got != want {
		t.Fatalf("normalized.DMPolicy = %q, want %q", got, want)
	}
	if got, want := providerConfig, any(`{"mode":"bot","tenant":"acme"}`); got != want {
		t.Fatalf("providerConfig = %#v, want %#v", got, want)
	}
	if got, want := deliveryDefaults, any(`{"mode":"reply","peer_id":"peer-1"}`); got != want {
		t.Fatalf("deliveryDefaults = %#v, want %#v", got, want)
	}
	if got, want := degradationReason, any("rate_limited"); got != want {
		t.Fatalf("degradationReason = %#v, want %#v", got, want)
	}
	if degradationMessage == nil {
		t.Fatal("degradationMessage = nil, want value")
	}
	if routingPolicyJSON == "" {
		t.Fatal("routingPolicyJSON = empty, want JSON")
	}
}

func TestGlobalDBBridgeDeleteAndRouteLookupNotFound(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	if err := globalDB.DeleteBridgeInstance(
		testutil.Context(t),
		"missing-bridge",
	); !errors.Is(
		err,
		bridges.ErrBridgeInstanceNotFound,
	) {
		t.Fatalf("DeleteBridgeInstance(missing) error = %v, want ErrBridgeInstanceNotFound", err)
	}
	if err := globalDB.DeleteBridgeSecretBinding(
		testutil.Context(t),
		"missing-bridge",
		"bot_token",
	); !errors.Is(
		err,
		bridges.ErrBridgeSecretBindingNotFound,
	) {
		t.Fatalf("DeleteBridgeSecretBinding(missing) error = %v, want ErrBridgeSecretBindingNotFound", err)
	}
	if _, err := globalDB.ResolveBridgeRoute(testutil.Context(t), bridges.RoutingKey{
		Scope:            bridges.ScopeGlobal,
		BridgeInstanceID: "missing-bridge",
		PeerID:           "peer-1",
	}); !errors.Is(err, bridges.ErrBridgeRouteNotFound) {
		t.Fatalf("ResolveBridgeRoute(missing) error = %v, want ErrBridgeRouteNotFound", err)
	}
	if err := globalDB.DeleteBridgeRoute(
		testutil.Context(t),
		"missing-route",
	); !errors.Is(
		err,
		bridges.ErrBridgeRouteNotFound,
	) {
		t.Fatalf("DeleteBridgeRoute(missing) error = %v, want ErrBridgeRouteNotFound", err)
	}
}

func TestGlobalDBBridgeMissingLookupsAndHelpers(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	if _, err := globalDB.GetBridgeInstance(
		testutil.Context(t),
		"missing",
	); !errors.Is(
		err,
		bridges.ErrBridgeInstanceNotFound,
	) {
		t.Fatalf("GetBridgeInstance(missing) error = %v, want ErrBridgeInstanceNotFound", err)
	}
	if err := globalDB.UpdateBridgeInstance(testutil.Context(t), bridges.BridgeInstance{
		ID:            "missing",
		Scope:         bridges.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Missing",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
	}); !errors.Is(err, bridges.ErrBridgeInstanceNotFound) {
		t.Fatalf("UpdateBridgeInstance(missing) error = %v, want ErrBridgeInstanceNotFound", err)
	}
	if err := globalDB.DeleteBridgeInstance(
		testutil.Context(t),
		"missing",
	); !errors.Is(
		err,
		bridges.ErrBridgeInstanceNotFound,
	) {
		t.Fatalf("DeleteBridgeInstance(missing) error = %v, want ErrBridgeInstanceNotFound", err)
	}
	if _, err := globalDB.GetBridgeSecretBinding(
		testutil.Context(t),
		"missing",
		"token",
	); !errors.Is(
		err,
		bridges.ErrBridgeSecretBindingNotFound,
	) {
		t.Fatalf("GetBridgeSecretBinding(missing) error = %v, want ErrBridgeSecretBindingNotFound", err)
	}
	if err := globalDB.DeleteBridgeSecretBinding(
		testutil.Context(t),
		"missing",
		"token",
	); !errors.Is(
		err,
		bridges.ErrBridgeSecretBindingNotFound,
	) {
		t.Fatalf("DeleteBridgeSecretBinding(missing) error = %v, want ErrBridgeSecretBindingNotFound", err)
	}
	if _, err := globalDB.GetBridgeRoute(
		testutil.Context(t),
		"missing",
	); !errors.Is(
		err,
		bridges.ErrBridgeRouteNotFound,
	) {
		t.Fatalf("GetBridgeRoute(missing) error = %v, want ErrBridgeRouteNotFound", err)
	}
	if _, err := globalDB.ResolveBridgeRoute(
		testutil.Context(t),
		bridges.RoutingKey{Scope: bridges.ScopeGlobal, BridgeInstanceID: "missing"},
	); !errors.Is(
		err,
		bridges.ErrBridgeRouteNotFound,
	) {
		t.Fatalf("ResolveBridgeRoute(missing) error = %v, want ErrBridgeRouteNotFound", err)
	}

	if got := (*GlobalDB)(nil).DB(); got != nil {
		t.Fatalf("nil GlobalDB.DB() = %#v, want nil", got)
	}
	if got := globalDB.DB(); got == nil {
		t.Fatal("GlobalDB.DB() = nil, want non-nil")
	}

	raw, err := normalizeOptionalRawJSON([]byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("normalizeOptionalRawJSON(valid) error = %v", err)
	}
	if raw == nil {
		t.Fatal("normalizeOptionalRawJSON(valid) = nil, want non-nil")
	}
	if got, err := normalizeOptionalRawJSON(nil); err != nil || got != nil {
		t.Fatalf("normalizeOptionalRawJSON(nil) = (%v, %v), want (nil, nil)", got, err)
	}
	if _, err := normalizeOptionalRawJSON([]byte(`{`)); err == nil {
		t.Fatal("normalizeOptionalRawJSON(invalid) error = nil, want non-nil")
	}
}

func TestGlobalDBBridgeConstraintFailuresAndDefaultDedupLookupTime(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	base := time.Date(2026, 4, 10, 15, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return base }

	if err := globalDB.InsertBridgeInstance(testutil.Context(t), bridges.BridgeInstance{
		ID:            "brg-missing-workspace",
		Scope:         bridges.ScopeWorkspace,
		WorkspaceID:   "ws-missing",
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Missing Workspace",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
	}); !errors.Is(err, aghworkspace.ErrWorkspaceNotFound) {
		t.Fatalf("InsertBridgeInstance(missing workspace) error = %v, want ErrWorkspaceNotFound", err)
	}

	if err := globalDB.PutBridgeSecretBinding(testutil.Context(t), bridges.BridgeSecretBinding{
		BridgeInstanceID: "brg-missing",
		BindingName:      "bot_token",
		SecretRef:        "vault:bridges/brg-missing/bot_token",
		Kind:             "token",
	}); !errors.Is(err, bridges.ErrBridgeInstanceNotFound) {
		t.Fatalf("PutBridgeSecretBinding(missing instance) error = %v, want ErrBridgeInstanceNotFound", err)
	}

	if err := globalDB.PutBridgeRoute(testutil.Context(t), bridges.BridgeRoute{
		Scope:            bridges.ScopeGlobal,
		BridgeInstanceID: "brg-missing",
		PeerID:           "peer-1",
		SessionID:        "sess-1",
		AgentName:        "coder",
	}); !errors.Is(err, bridges.ErrBridgeInstanceNotFound) {
		t.Fatalf("PutBridgeRoute(missing instance) error = %v, want ErrBridgeInstanceNotFound", err)
	}

	if err := globalDB.PutBridgeIngestDedup(testutil.Context(t), bridges.IngestDedupRecord{
		IdempotencyKey:   "idem-missing",
		BridgeInstanceID: "brg-missing",
		ReceivedAt:       base.Add(-time.Minute),
		ExpiresAt:        base.Add(time.Minute),
	}); !errors.Is(err, bridges.ErrBridgeInstanceNotFound) {
		t.Fatalf("PutBridgeIngestDedup(missing instance) error = %v, want ErrBridgeInstanceNotFound", err)
	}

	instance := bridges.BridgeInstance{
		ID:            "brg-live-default-lookup",
		Scope:         bridges.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Live Lookup",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
	}
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), instance); err != nil {
		t.Fatalf("InsertBridgeInstance(valid) error = %v", err)
	}
	if err := globalDB.PutBridgeIngestDedup(testutil.Context(t), bridges.IngestDedupRecord{
		IdempotencyKey:   "idem-default-lookup",
		BridgeInstanceID: instance.ID,
		ReceivedAt:       base.Add(-time.Minute),
		ExpiresAt:        base.Add(time.Minute),
	}); err != nil {
		t.Fatalf("PutBridgeIngestDedup(valid) error = %v", err)
	}
	if _, err := globalDB.GetBridgeIngestDedup(testutil.Context(t), "idem-default-lookup", time.Time{}); err != nil {
		t.Fatalf("GetBridgeIngestDedup(default lookup time) error = %v", err)
	}
	if deleted, err := globalDB.DeleteExpiredBridgeIngestDedup(
		testutil.Context(t),
		time.Time{},
	); err != nil ||
		deleted != 0 {
		t.Fatalf("DeleteExpiredBridgeIngestDedup(default now) = (%d, %v), want (0, nil)", deleted, err)
	}
}

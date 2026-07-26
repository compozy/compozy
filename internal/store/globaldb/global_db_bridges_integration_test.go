//go:build integration

package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBBridgeInstanceRoundTripAcrossReopen(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	first, err := OpenGlobalDB(testutil.Context(t), dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		first,
		"integration-bridge-instance",
		filepath.Join(t.TempDir(), "integration-bridge-instance"),
	)
	instance := bridges.BridgeInstance{
		ID:               "brg-integration",
		Scope:            bridges.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		Platform:         "telegram",
		ExtensionName:    "telegram-adapter",
		DisplayName:      "Integration Telegram",
		Enabled:          true,
		Status:           bridges.BridgeStatusDegraded,
		DMPolicy:         bridges.BridgeDMPolicyPairing,
		RoutingPolicy:    bridges.RoutingPolicy{IncludePeer: true},
		ProviderConfig:   []byte(`{"mode":"bot","webhook_url":"https://example.invalid/hook"}`),
		DeliveryDefaults: []byte(`{"peer_id":"peer-1","mode":"reply"}`),
		Degradation: &bridges.BridgeDegradation{
			Reason:  bridges.BridgeDegradationReasonRateLimited,
			Message: "provider API throttled the tenant",
		},
		CreatedAt: time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	}
	if err := first.InsertBridgeInstance(testutil.Context(t), instance); err != nil {
		t.Fatalf("InsertBridgeInstance() error = %v", err)
	}
	if err := first.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(testutil.Context(t), dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	assertTablesPresent(
		t,
		second.db,
		"bridge_instances",
		"bridge_secret_bindings",
		"bridge_routes",
		"bridge_ingest_dedup",
	)

	loaded, err := second.GetBridgeInstance(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("GetBridgeInstance() error = %v", err)
	}
	if loaded.Scope != bridges.ScopeWorkspace || loaded.WorkspaceID != workspaceID {
		t.Fatalf("loaded bridge instance = %#v", loaded)
	}
	if got, want := loaded.DMPolicy, bridges.BridgeDMPolicyPairing; got != want {
		t.Fatalf("loaded.DMPolicy = %q, want %q", got, want)
	}
	if got, want := string(
		loaded.ProviderConfig,
	), `{"mode":"bot","webhook_url":"https://example.invalid/hook"}`; got != want {
		t.Fatalf("loaded.ProviderConfig = %s, want %s", got, want)
	}
	if got, want := string(loaded.DeliveryDefaults), `{"peer_id":"peer-1","mode":"reply"}`; got != want {
		t.Fatalf("loaded.DeliveryDefaults = %s, want %s", got, want)
	}
	if loaded.Degradation == nil {
		t.Fatal("loaded.Degradation = nil, want value")
	}
	if got, want := loaded.Degradation.Reason, bridges.BridgeDegradationReasonRateLimited; got != want {
		t.Fatalf("loaded.Degradation.Reason = %q, want %q", got, want)
	}
}

func TestGlobalDBBridgeRouteSurvivesReopen(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	first, err := OpenGlobalDB(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		first,
		"integration-bridge-route",
		filepath.Join(t.TempDir(), "integration-bridge-route"),
	)
	registerSessionForGlobalTests(t, first, "sess-bridge-route")
	instance := bridges.BridgeInstance{
		ID:            "brg-route",
		Scope:         bridges.ScopeWorkspace,
		WorkspaceID:   workspaceID,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Route Telegram",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		CreatedAt:     time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	}
	if err := first.InsertBridgeInstance(testutil.Context(t), instance); err != nil {
		t.Fatalf("InsertBridgeInstance() error = %v", err)
	}

	route := bridges.BridgeRoute{
		Scope:            bridges.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-9",
		SessionID:        "sess-bridge-route",
		AgentName:        "coder",
		LastActivityAt:   time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC),
		CreatedAt:        time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC),
	}
	if err := first.PutBridgeRoute(testutil.Context(t), route); err != nil {
		t.Fatalf("PutBridgeRoute() error = %v", err)
	}
	if err := first.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(testutil.Context(t), dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	resolved, err := second.ResolveBridgeRoute(testutil.Context(t), route.RoutingKey())
	if err != nil {
		t.Fatalf("ResolveBridgeRoute() error = %v", err)
	}
	if resolved.SessionID != route.SessionID || resolved.ThreadID != route.ThreadID || resolved.PeerID != route.PeerID {
		t.Fatalf("resolved route = %#v, want session/thread/peer from %#v", resolved, route)
	}
}

func TestGlobalDBGlobalAndWorkspaceInstancesCoexist(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"integration-coexist",
		filepath.Join(t.TempDir(), "integration-coexist"),
	)

	globalInstance := bridges.BridgeInstance{
		ID:            "brg-global",
		Scope:         bridges.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Global Telegram",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
	}
	workspaceInstance := bridges.BridgeInstance{
		ID:            "brg-workspace",
		Scope:         bridges.ScopeWorkspace,
		WorkspaceID:   workspaceID,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Workspace Telegram",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
	}

	if err := globalDB.InsertBridgeInstance(testutil.Context(t), globalInstance); err != nil {
		t.Fatalf("InsertBridgeInstance(global) error = %v", err)
	}
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), workspaceInstance); err != nil {
		t.Fatalf("InsertBridgeInstance(workspace) error = %v", err)
	}

	instances, err := globalDB.ListBridgeInstances(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListBridgeInstances() error = %v", err)
	}
	if got, want := len(instances), 2; got != want {
		t.Fatalf("len(instances) = %d, want %d", got, want)
	}
}

func TestGlobalDBExpiredDedupRecordsExcluded(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	instance := bridges.BridgeInstance{
		ID:            "brg-dedup",
		Scope:         bridges.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Dedup Telegram",
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
	}
	if err := globalDB.InsertBridgeInstance(testutil.Context(t), instance); err != nil {
		t.Fatalf("InsertBridgeInstance() error = %v", err)
	}

	base := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	live := bridges.IngestDedupRecord{
		IdempotencyKey:   "idem-live",
		BridgeInstanceID: instance.ID,
		ReceivedAt:       base,
		ExpiresAt:        base.Add(5 * time.Minute),
	}
	expired := bridges.IngestDedupRecord{
		IdempotencyKey:   "idem-expired",
		BridgeInstanceID: instance.ID,
		ReceivedAt:       base.Add(-2 * time.Minute),
		ExpiresAt:        base.Add(-time.Minute),
	}
	if err := globalDB.PutBridgeIngestDedup(testutil.Context(t), live); err != nil {
		t.Fatalf("PutBridgeIngestDedup(live) error = %v", err)
	}
	if err := globalDB.PutBridgeIngestDedup(testutil.Context(t), expired); err != nil {
		t.Fatalf("PutBridgeIngestDedup(expired) error = %v", err)
	}

	if _, err := globalDB.GetBridgeIngestDedup(
		testutil.Context(t),
		live.IdempotencyKey,
		base.Add(time.Minute),
	); err != nil {
		t.Fatalf("GetBridgeIngestDedup(live) error = %v", err)
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
}

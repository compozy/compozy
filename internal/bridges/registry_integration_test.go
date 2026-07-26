//go:build integration

package bridges_test

import (
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

func TestRegistryGlobalAndWorkspaceRoutesStayIsolated(t *testing.T) {
	t.Parallel()

	registry, db := newRegistryTestHarness(t)
	workspaceID := registerWorkspaceForBridgesTests(t, db, "ws-route-scope", "route-scope")

	globalInstance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-global-route",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Global Route",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	workspaceInstance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-workspace-route",
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   workspaceID,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Workspace Route",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	globalRoute, created, err := registry.ResolveOrCreateRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: globalInstance.ID,
		PeerID:           "peer-1",
		SessionID:        "sess-global",
		AgentName:        "coder",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreateRoute(global) error = %v", err)
	}
	if !created {
		t.Fatal("ResolveOrCreateRoute(global) created = false, want true")
	}

	workspaceRoute, created, err := registry.ResolveOrCreateRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: workspaceInstance.ID,
		PeerID:           "peer-1",
		SessionID:        "sess-workspace",
		AgentName:        "coder",
	})
	if err != nil {
		t.Fatalf("ResolveOrCreateRoute(workspace) error = %v", err)
	}
	if !created {
		t.Fatal("ResolveOrCreateRoute(workspace) created = false, want true")
	}

	if globalRoute.RoutingKeyHash == workspaceRoute.RoutingKeyHash {
		t.Fatalf("RoutingKeyHash() = %q for both routes, want distinct records", globalRoute.RoutingKeyHash)
	}

	globalKey, err := registry.BuildRoutingKey(testutil.Context(t), bridgepkg.RoutingKey{
		BridgeInstanceID: globalInstance.ID,
		PeerID:           "peer-1",
	})
	if err != nil {
		t.Fatalf("BuildRoutingKey(global) error = %v", err)
	}
	workspaceKey, err := registry.BuildRoutingKey(testutil.Context(t), bridgepkg.RoutingKey{
		BridgeInstanceID: workspaceInstance.ID,
		PeerID:           "peer-1",
	})
	if err != nil {
		t.Fatalf("BuildRoutingKey(workspace) error = %v", err)
	}
	if globalKey.Scope == workspaceKey.Scope {
		t.Fatalf(
			"globalKey.Scope = %q and workspaceKey.Scope = %q, want different scopes",
			globalKey.Scope,
			workspaceKey.Scope,
		)
	}
	if workspaceKey.WorkspaceID != workspaceID {
		t.Fatalf("workspaceKey.WorkspaceID = %q, want %q", workspaceKey.WorkspaceID, workspaceID)
	}
}

func TestRegistryUpsertRouteRebindsWithoutDuplicateRows(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	instance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-rebind",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Route Rebind",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
	})

	first, err := registry.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-1",
		AgentName:        "coder",
	})
	if err != nil {
		t.Fatalf("UpsertRoute(first) error = %v", err)
	}

	second, err := registry.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-2",
		AgentName:        "reviewer",
	})
	if err != nil {
		t.Fatalf("UpsertRoute(second) error = %v", err)
	}
	if first.RoutingKeyHash != second.RoutingKeyHash {
		t.Fatalf(
			"UpsertRoute() hashes = %q and %q, want same canonical route",
			first.RoutingKeyHash,
			second.RoutingKeyHash,
		)
	}
	if second.SessionID != "sess-2" {
		t.Fatalf("UpsertRoute(second).SessionID = %q, want sess-2", second.SessionID)
	}

	routes, err := registry.ListRoutes(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}
	if routes[0].SessionID != "sess-2" {
		t.Fatalf("routes[0].SessionID = %q, want sess-2", routes[0].SessionID)
	}
}

func TestRegistryListRoutesReturnsOnlyTheRequestedInstance(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	first := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-list-a",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "List A",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	second := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-list-b",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "List B",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	if _, err := registry.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: first.ID,
		PeerID:           "peer-a",
		SessionID:        "sess-a",
		AgentName:        "coder",
	}); err != nil {
		t.Fatalf("UpsertRoute(first) error = %v", err)
	}
	if _, err := registry.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		BridgeInstanceID: second.ID,
		PeerID:           "peer-b",
		SessionID:        "sess-b",
		AgentName:        "coder",
	}); err != nil {
		t.Fatalf("UpsertRoute(second) error = %v", err)
	}

	routes, err := registry.ListRoutes(testutil.Context(t), first.ID)
	if err != nil {
		t.Fatalf("ListRoutes(first) error = %v", err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}
	if routes[0].BridgeInstanceID != first.ID {
		t.Fatalf("routes[0].BridgeInstanceID = %q, want %q", routes[0].BridgeInstanceID, first.ID)
	}
}

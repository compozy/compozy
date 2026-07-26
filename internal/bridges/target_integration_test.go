//go:build integration

package bridges_test

import (
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

func TestRegistryResolveDeliveryTargetUsesInstanceDefaults(t *testing.T) {
	t.Parallel()

	registry, _ := newRegistryTestHarness(t)
	instance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            "brg-target-defaults",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Target Defaults",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		DeliveryDefaults: []byte(
			`{"peer_id":"peer-default","thread_id":"thread-default","mode":"reply","parse_mode":"markdown"}`,
		),
	})

	target, err := registry.ResolveDeliveryTarget(testutil.Context(t), bridgepkg.ResolveDeliveryTargetRequest{
		BridgeInstanceID: instance.ID,
	})
	if err != nil {
		t.Fatalf("ResolveDeliveryTarget() error = %v", err)
	}
	if target.BridgeInstanceID != instance.ID {
		t.Fatalf("ResolveDeliveryTarget().BridgeInstanceID = %q, want %q", target.BridgeInstanceID, instance.ID)
	}
	if target.PeerID != "peer-default" {
		t.Fatalf("ResolveDeliveryTarget().PeerID = %q, want peer-default", target.PeerID)
	}
	if target.ThreadID != "thread-default" {
		t.Fatalf("ResolveDeliveryTarget().ThreadID = %q, want thread-default", target.ThreadID)
	}
	if target.Mode != bridgepkg.DeliveryModeReply {
		t.Fatalf("ResolveDeliveryTarget().Mode = %q, want %q", target.Mode, bridgepkg.DeliveryModeReply)
	}
}

func TestRegistryResolveDeliveryTargetKeepsWorkspaceScopeIsolated(t *testing.T) {
	t.Parallel()

	registry, db := newRegistryTestHarness(t)
	workspaceID := registerWorkspaceForBridgesTests(t, db, "ws-target-scope", "target-scope")

	globalInstance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:               "brg-target-global",
		Scope:            bridgepkg.ScopeGlobal,
		Platform:         "telegram",
		ExtensionName:    "telegram-adapter",
		DisplayName:      "Global Targets",
		Enabled:          true,
		Status:           bridgepkg.BridgeStatusReady,
		RoutingPolicy:    bridgepkg.RoutingPolicy{IncludeGroup: true},
		DeliveryDefaults: []byte(`{"group_id":"global-group","mode":"direct-send"}`),
	})
	workspaceInstance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
		ID:               "brg-target-workspace",
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		Platform:         "telegram",
		ExtensionName:    "telegram-adapter",
		DisplayName:      "Workspace Targets",
		Enabled:          true,
		Status:           bridgepkg.BridgeStatusReady,
		RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		DeliveryDefaults: []byte(`{"peer_id":"workspace-peer","thread_id":"workspace-thread","mode":"reply"}`),
	})

	globalTarget, err := registry.ResolveDeliveryTarget(testutil.Context(t), bridgepkg.ResolveDeliveryTargetRequest{
		BridgeInstanceID: globalInstance.ID,
	})
	if err != nil {
		t.Fatalf("ResolveDeliveryTarget(global) error = %v", err)
	}

	workspaceTarget, err := registry.ResolveDeliveryTarget(testutil.Context(t), bridgepkg.ResolveDeliveryTargetRequest{
		BridgeInstanceID: workspaceInstance.ID,
	})
	if err != nil {
		t.Fatalf("ResolveDeliveryTarget(workspace) error = %v", err)
	}

	if globalTarget.BridgeInstanceID != globalInstance.ID {
		t.Fatalf("globalTarget.BridgeInstanceID = %q, want %q", globalTarget.BridgeInstanceID, globalInstance.ID)
	}
	if globalTarget.GroupID != "global-group" {
		t.Fatalf("globalTarget.GroupID = %q, want global-group", globalTarget.GroupID)
	}
	if workspaceTarget.BridgeInstanceID != workspaceInstance.ID {
		t.Fatalf(
			"workspaceTarget.BridgeInstanceID = %q, want %q",
			workspaceTarget.BridgeInstanceID,
			workspaceInstance.ID,
		)
	}
	if workspaceTarget.PeerID != "workspace-peer" {
		t.Fatalf("workspaceTarget.PeerID = %q, want workspace-peer", workspaceTarget.PeerID)
	}
	if workspaceTarget.ThreadID != "workspace-thread" {
		t.Fatalf("workspaceTarget.ThreadID = %q, want workspace-thread", workspaceTarget.ThreadID)
	}
	if workspaceTarget.GroupID != "" {
		t.Fatalf("workspaceTarget.GroupID = %q, want empty", workspaceTarget.GroupID)
	}
	if workspaceTarget.Mode != bridgepkg.DeliveryModeReply {
		t.Fatalf("workspaceTarget.Mode = %q, want %q", workspaceTarget.Mode, bridgepkg.DeliveryModeReply)
	}
}

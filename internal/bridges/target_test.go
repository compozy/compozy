package bridges_test

import (
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

func TestBuildDeliveryTargetDefaultsToDirectSend(t *testing.T) {
	t.Parallel()

	t.Run("Should default a peer target to direct-send", func(t *testing.T) {
		t.Parallel()

		instance := testBridgeInstanceForTargets()

		target, err := bridgepkg.BuildDeliveryTarget(instance, bridgepkg.ResolveDeliveryTargetRequest{
			BridgeInstanceID: instance.ID,
			PeerID:           "peer-1",
		})
		if err != nil {
			t.Fatalf("BuildDeliveryTarget() error = %v", err)
		}
		if target.BridgeInstanceID != instance.ID {
			t.Fatalf("BuildDeliveryTarget().BridgeInstanceID = %q, want %q", target.BridgeInstanceID, instance.ID)
		}
		if target.PeerID != "peer-1" {
			t.Fatalf("BuildDeliveryTarget().PeerID = %q, want peer-1", target.PeerID)
		}
		if target.Mode != bridgepkg.DeliveryModeDirectSend {
			t.Fatalf("BuildDeliveryTarget().Mode = %q, want %q", target.Mode, bridgepkg.DeliveryModeDirectSend)
		}
	})
}

func TestBuildDeliveryTargetExplicitOverridesWinOverDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer explicit fields while retaining unspecified defaults", func(t *testing.T) {
		t.Parallel()

		instance := testBridgeInstanceForTargets()
		instance.DeliveryDefaults = []byte(
			`{"peer_id":"peer-default","thread_id":"thread-default","group_id":"group-default","mode":"reply"}`,
		)

		target, err := bridgepkg.BuildDeliveryTarget(instance, bridgepkg.ResolveDeliveryTargetRequest{
			BridgeInstanceID: instance.ID,
			PeerID:           "peer-explicit",
			ThreadID:         "thread-explicit",
			Mode:             bridgepkg.DeliveryModeDirectSend,
		})
		if err != nil {
			t.Fatalf("BuildDeliveryTarget() error = %v", err)
		}
		if target.PeerID != "peer-explicit" {
			t.Fatalf("BuildDeliveryTarget().PeerID = %q, want peer-explicit", target.PeerID)
		}
		if target.ThreadID != "thread-explicit" {
			t.Fatalf("BuildDeliveryTarget().ThreadID = %q, want thread-explicit", target.ThreadID)
		}
		if target.GroupID != "group-default" {
			t.Fatalf("BuildDeliveryTarget().GroupID = %q, want group-default", target.GroupID)
		}
		if target.Mode != bridgepkg.DeliveryModeDirectSend {
			t.Fatalf("BuildDeliveryTarget().Mode = %q, want %q", target.Mode, bridgepkg.DeliveryModeDirectSend)
		}
	})
}

func TestRegistryResolveDeliveryTargetUsesServiceSeam(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve provider defaults through the registry service seam", func(t *testing.T) {
		t.Parallel()

		registry, _ := newRegistryTestHarness(t)
		instance := createTestBridgeInstance(t, registry, bridgepkg.CreateInstanceRequest{
			ID:               "brg-target-service",
			Scope:            bridgepkg.ScopeGlobal,
			Platform:         "telegram",
			ExtensionName:    "telegram-adapter",
			DisplayName:      "Target Service",
			Enabled:          true,
			Status:           bridgepkg.BridgeStatusReady,
			RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
			DeliveryDefaults: []byte(`{"peer_id":"peer-service","thread_id":"thread-service","mode":"reply"}`),
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
		if target.PeerID != "peer-service" {
			t.Fatalf("ResolveDeliveryTarget().PeerID = %q, want peer-service", target.PeerID)
		}
		if target.ThreadID != "thread-service" {
			t.Fatalf("ResolveDeliveryTarget().ThreadID = %q, want thread-service", target.ThreadID)
		}
		if target.Mode != bridgepkg.DeliveryModeReply {
			t.Fatalf("ResolveDeliveryTarget().Mode = %q, want %q", target.Mode, bridgepkg.DeliveryModeReply)
		}
	})
}

func testBridgeInstanceForTargets() bridgepkg.BridgeInstance {
	return bridgepkg.BridgeInstance{
		ID:            "brg-targets",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Targets",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	}
}

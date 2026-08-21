package bridges_test

import (
	"testing"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should keep the configured default when an override carries no identity", func(t *testing.T) {
		t.Parallel()

		instance := testBridgeInstanceForTargets()
		instance.DeliveryDefaults = []byte(`{"peer_id":" peer-default "}`)

		target, err := bridgepkg.BuildDeliveryTarget(instance, bridgepkg.ResolveDeliveryTargetRequest{
			BridgeInstanceID: instance.ID,
			PeerID:           "   ",
		})
		if err != nil {
			t.Fatalf("BuildDeliveryTarget() error = %v", err)
		}
		if target.PeerID != " peer-default " {
			t.Fatalf("BuildDeliveryTarget().PeerID = %q, want the configured default verbatim", target.PeerID)
		}
	})

	t.Run("Should reject a target that resolves to no identity", func(t *testing.T) {
		t.Parallel()

		instance := testBridgeInstanceForTargets()
		instance.DeliveryDefaults = []byte(`{"peer_id":"   "}`)

		if _, err := bridgepkg.BuildDeliveryTarget(instance, bridgepkg.ResolveDeliveryTargetRequest{
			BridgeInstanceID: instance.ID,
			PeerID:           "   ",
		}); err == nil {
			t.Fatal("BuildDeliveryTarget(blank peer and blank default) error = nil, want rejection")
		}
	})
}

func TestResolveDeliveryTargetRequestValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve opaque identities and accept an exact explicit mode", func(t *testing.T) {
		t.Parallel()

		request := bridgepkg.ResolveDeliveryTargetRequest{
			BridgeInstanceID: " bridge ",
			PeerID:           " peer ",
			ThreadID:         " thread ",
			GroupID:          " group ",
			Mode:             bridgepkg.DeliveryModeReply,
		}
		if err := request.Validate(); err != nil {
			t.Fatalf("ResolveDeliveryTargetRequest.Validate() error = %v", err)
		}
	})

	t.Run("Should reject noncanonical modes and invalid opaque identities", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			request bridgepkg.ResolveDeliveryTargetRequest
		}{
			{
				name: "Should reject a noncanonical mode",
				request: bridgepkg.ResolveDeliveryTargetRequest{
					BridgeInstanceID: "bridge",
					Mode:             " reply ",
				},
			},
			{
				name: "Should reject an invalid peer identity",
				request: bridgepkg.ResolveDeliveryTargetRequest{
					BridgeInstanceID: "bridge",
					PeerID:           string([]byte{0xff}),
				},
			},
			{
				name:    "Should reject a blank bridge instance identity",
				request: bridgepkg.ResolveDeliveryTargetRequest{BridgeInstanceID: " "},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.request.Validate(); err == nil {
					t.Fatal("ResolveDeliveryTargetRequest.Validate() error = nil")
				}
			})
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
		ProfileID:     store.DefaultProfileID,
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

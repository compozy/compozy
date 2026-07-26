package subprocess

import (
	"testing"
	"time"

	bridges "github.com/compozy/agh/internal/bridges/contract"
)

func TestInitializeBridgeBoundSecretValuePreservation(t *testing.T) {
	t.Run("Should preserve opaque values through clone and managed instance helpers", func(t *testing.T) {
		t.Parallel()

		const opaqueValue = "  secret-token\n"
		runtime := testBridgeRuntimeWithBoundSecretValue(opaqueValue)
		if err := runtime.Validate(); err != nil {
			t.Fatalf("runtime.Validate() error = %v", err)
		}

		cloned := CloneInitializeBridgeRuntime(&runtime)
		if cloned == nil {
			t.Fatal("CloneInitializeBridgeRuntime() = nil, want runtime clone")
		}
		assertBridgeBoundSecretValue(t, cloned.ManagedInstances[0], opaqueValue)

		single, err := runtime.SingleManagedInstance()
		if err != nil {
			t.Fatalf("SingleManagedInstance() error = %v", err)
		}
		assertBridgeBoundSecretValue(t, *single, opaqueValue)

		selected, ok := runtime.ManagedInstance("brg-1")
		if !ok {
			t.Fatal("ManagedInstance(brg-1) = false, want true")
		}
		assertBridgeBoundSecretValue(t, *selected, opaqueValue)
	})
}

func testBridgeRuntimeWithBoundSecretValue(value string) InitializeBridgeRuntime {
	now := time.Date(2026, 5, 16, 20, 35, 0, 0, time.UTC)
	return InitializeBridgeRuntime{
		RuntimeVersion: InitializeBridgeRuntimeVersion2,
		Purpose:        BridgeRuntimePurposeService,
		Provider:       "discord-reference",
		Platform:       "discord",
		ManagedInstances: []InitializeBridgeManagedInstance{{
			Instance: bridges.BridgeInstance{
				ID:            "brg-1",
				Scope:         bridges.ScopeWorkspace,
				WorkspaceID:   "ws-1",
				Platform:      "discord",
				ExtensionName: "discord-reference",
				DisplayName:   "Discord",
				Enabled:       true,
				Status:        bridges.BridgeStatusReady,
				RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			BoundSecrets: []InitializeBridgeBoundSecret{
				{BindingName: " bot_token ", Kind: " token ", Value: value},
			},
		}},
	}
}

func assertBridgeBoundSecretValue(
	t *testing.T,
	managed InitializeBridgeManagedInstance,
	wantValue string,
) {
	t.Helper()

	if got, want := len(managed.BoundSecrets), 1; got != want {
		t.Fatalf("len(BoundSecrets) = %d, want %d", got, want)
	}
	secret := managed.BoundSecrets[0]
	if got, want := secret.BindingName, "bot_token"; got != want {
		t.Fatalf("BindingName = %q, want %q", got, want)
	}
	if got, want := secret.Kind, "token"; got != want {
		t.Fatalf("Kind = %q, want %q", got, want)
	}
	if got := secret.Value; got != wantValue {
		t.Fatalf("Value = %q, want %q", got, wantValue)
	}
}

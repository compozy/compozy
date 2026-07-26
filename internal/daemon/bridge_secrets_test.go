package daemon

import (
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

func TestVaultBridgeSecretResolverContract(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve resolved vault secret bytes", func(t *testing.T) {
		t.Parallel()

		resolvedValue := " token-with-significant-space\n"
		store := &recordingBridgeSecretRefStore{
			values: map[string]string{"vault:bridges/brg-vault/bot_token": resolvedValue},
		}
		resolver := vaultBridgeSecretResolver{service: store}
		binding := bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: "brg-vault",
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/brg-vault/bot_token",
			Kind:             "token",
		}

		value, err := resolver.ResolveBridgeSecret(testutil.Context(t), binding)
		if err != nil {
			t.Fatalf("ResolveBridgeSecret() error = %v", err)
		}
		if got, want := value, resolvedValue; got != want {
			t.Fatalf("ResolveBridgeSecret() = %q, want %q", got, want)
		}
	})
}

func TestBridgeRuntimeResolveBoundSecretsContract(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve vault secret bytes during launch binding", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		now := time.Date(2026, 5, 17, 15, 0, 0, 0, time.UTC)
		resolvedValue := " token-with-significant-space\n"
		refStore := &recordingBridgeSecretRefStore{
			values: map[string]string{
				"vault:bridges/brg-secret-vault/bot_token": resolvedValue,
			},
		}
		runtime := newBridgeRuntime(
			db,
			discardLogger(),
			func() time.Time { return now },
			vaultBridgeSecretResolver{service: refStore},
		)
		instance := mustCreateDaemonBridgeInstance(t, runtime, bridgepkg.CreateInstanceRequest{
			ID:            "brg-secret-vault",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "ext-secret-vault",
			DisplayName:   "Secret Vault Bridge",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		if err := db.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
			BridgeInstanceID: instance.ID,
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/brg-secret-vault/bot_token",
			Kind:             "bot_token",
			CreatedAt:        now,
			UpdatedAt:        now,
		}); err != nil {
			t.Fatalf("PutBridgeSecretBinding() error = %v", err)
		}

		launch, err := runtime.ResolveBridgeRuntime(testutil.Context(t), instance.ExtensionName)
		if err != nil {
			t.Fatalf("ResolveBridgeRuntime() error = %v", err)
		}
		managed, ok := launch.ManagedInstance(instance.ID)
		if !ok {
			t.Fatalf("ResolveBridgeRuntime() missing managed instance %q", instance.ID)
		}
		if got, want := len(managed.BoundSecrets), 1; got != want {
			t.Fatalf("ManagedInstance(%q).BoundSecrets count = %d, want %d", instance.ID, got, want)
		}
		if got, want := managed.BoundSecrets[0].Value, resolvedValue; got != want {
			t.Fatalf("ManagedInstance(%q).BoundSecrets[0].Value = %q, want %q", instance.ID, got, want)
		}
	})
}

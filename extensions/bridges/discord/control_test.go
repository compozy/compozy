package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func TestDiscordIdentityChecksValidateTheConfiguredApplication(t *testing.T) {
	t.Run("Should pass for the authenticated configured bot", func(t *testing.T) {
		t.Parallel()

		checks := discordIdentityChecks(&discordBotIdentity{ID: "app-1", Username: "agh"}, "app-1", nil)
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("discordIdentityChecks() = %#v, want one pass", checks)
		}
	})

	t.Run("Should fail an application identity mismatch with config remediation", func(t *testing.T) {
		t.Parallel()

		checks := discordIdentityChecks(&discordBotIdentity{ID: "app-2"}, "app-1", nil)
		if len(checks) != 1 || checks[0].Status != bridgepkg.BridgeCheckStatusFail {
			t.Fatalf("discordIdentityChecks() = %#v, want one fail", checks)
		}
		if !strings.Contains(checks[0].Remediation, "application_id") {
			t.Fatalf("remediation = %q, want application_id", checks[0].Remediation)
		}
	})

	t.Run("Should map authentication failure to the bot token binding", func(t *testing.T) {
		t.Parallel()

		checks := discordIdentityChecks(nil, "app-1", errors.New("unauthorized"))
		if len(checks) != 1 || !strings.Contains(checks[0].Remediation, "bot_token") {
			t.Fatalf("discordIdentityChecks() = %#v, want bot_token remediation", checks)
		}
	})
}

func TestDiscordControlRuntimeCheck(t *testing.T) {
	t.Run("Should verify one managed Discord instance through the control session", func(t *testing.T) {
		t.Parallel()

		managed := subprocess.InitializeBridgeManagedInstance{
			Instance: bridgepkg.BridgeInstance{
				ID:             "brg-discord-control",
				ProviderConfig: json.RawMessage(`{"application_id":"bot-1"}`),
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Value: "discord-token"},
				{BindingName: "public_key", Value: strings.Repeat("a", 64)},
			},
		}
		cache := bridgesdk.NewInstanceCache(&subprocess.InitializeBridgeRuntime{
			ManagedInstances: []subprocess.InitializeBridgeManagedInstance{managed},
		})
		session := injectedDiscordSession(t, nil, cache)
		provider, err := newDiscordProvider(nil)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return &discordAPIFake{} }

		response, err := provider.handleBridgeCheck(context.Background(), session, bridgepkg.BridgeCheckRequest{
			BridgeInstanceID: managed.Instance.ID,
		})
		if err != nil {
			t.Fatalf("handleBridgeCheck() error = %v", err)
		}
		if len(response.Checks) < 2 || response.Checks[0].Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("handleBridgeCheck() checks = %#v, want identity and public-key passes", response.Checks)
		}
	})
}

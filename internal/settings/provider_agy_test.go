package settings

import (
	"context"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
)

// Invariant: builtin agy survives the Settings collection round-trip with its
// method field and safe descriptor, while explicit overrides and independent
// custom names keep their boundaries. Owner: settings provider collection
// with config load/overlay (IT-006).
func TestAgyProviderSettingsUpgradeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should expose builtin agy without a custom provider block", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{
			ProviderAuthCommandResolver: func(
				_ context.Context,
				command string,
				_ []string,
				_ string,
			) (string, error) {
				return "/test/bin/" + command, nil
			},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		agy := mustFindProviderItem(t, envelope.Providers, "agy")
		if got, want := agy.Settings.DisplayName, "Google Antigravity"; got != want {
			t.Fatalf("Settings.DisplayName = %q, want %q", got, want)
		}
		if got, want := agy.Settings.AuthACPMethod, "oauth-personal"; got != want {
			t.Fatalf("Settings.AuthACPMethod = %q, want %q", got, want)
		}
		if got, want := agy.Settings.HomePolicy, compozyconfig.ProviderHomePolicyOperator; got != want {
			t.Fatalf("Settings.HomePolicy = %q, want %q", got, want)
		}
		login := agy.AuthStatus.Login
		if !login.Configured || login.Source != "auth_acp_method" {
			t.Fatalf("AuthStatus.Login = %#v, want configured auth_acp_method source", login)
		}
		if got, want := login.Executable, "agy_acp_server.par"; got != want {
			t.Fatalf("AuthStatus.Login.Executable = %q, want %q", got, want)
		}
		if got, want := login.Presence, authproviders.ProviderLoginPresencePresent; got != want {
			t.Fatalf("AuthStatus.Login.Presence = %q, want %q", got, want)
		}
		for _, item := range envelope.Providers {
			if item.Name == "antigravity" {
				t.Fatal("provider antigravity listed without a custom block")
			}
		}
	})

	t.Run("Should preserve an explicit agy command override", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+
			"\n[providers.agy]\n"+
			"command = \"/opt/agy/test-server --uid=\"\n")
		service := testService(t, homePaths, Dependencies{
			ProviderAuthCommandResolver: func(
				_ context.Context,
				command string,
				_ []string,
				_ string,
			) (string, error) {
				return "/test/bin/" + command, nil
			},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		agy := mustFindProviderItem(t, envelope.Providers, "agy")
		if got, want := agy.Settings.Command, "/opt/agy/test-server --uid="; got != want {
			t.Fatalf("Settings.Command = %q, want %q", got, want)
		}
		if got, want := agy.Settings.AuthACPMethod, "oauth-personal"; got != want {
			t.Fatalf("Settings.AuthACPMethod = %q, want inherited %q", got, want)
		}
	})
}

package settings

import (
	"context"
	"runtime"
	"strings"
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
		executable := "agy_acp_server.par"
		if runtime.GOOS == "windows" {
			executable = "agy_acp_server.exe"
		}
		if got, want := login.Executable, executable; got != want {
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

	t.Run("Should save a pi_acp harness switch while keeping native CLI authentication", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		settings := mustFindProviderItem(t, envelope.Providers, "agy").Settings
		settings.Harness = compozyconfig.ProviderHarnessPiACP
		settings.AuthACPMethod = ""
		settings.AuthACPMethodSet = true
		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			Collection: CollectionProviders,
			Name:       "agy",
			Provider:   &settings,
		}); err != nil {
			t.Fatalf("PutCollectionItem(pi_acp switch) error = %v", err)
		}

		if stored := readFile(t, homePaths.ConfigFile); !strings.Contains(stored, `auth_acp_method = ""`) {
			t.Fatalf("stored config missing explicit empty auth_acp_method:\n%s", stored)
		}
		cfg, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after harness switch) error = %v", err)
		}
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		if got := provider.EffectiveHarness(); got != compozyconfig.ProviderHarnessPiACP {
			t.Fatalf("Harness = %q, want pi_acp", got)
		}
		if got := provider.EffectiveAuthMode(); got != compozyconfig.ProviderAuthModeNativeCLI {
			t.Fatalf("AuthMode = %q, want native_cli", got)
		}
		if got := provider.AuthACPMethod; got != "" {
			t.Fatalf("AuthACPMethod = %q, want no inherited method", got)
		}
	})
}

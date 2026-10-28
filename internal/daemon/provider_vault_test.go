package daemon

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/vault"
	"github.com/compozy/compozy/internal/workspace"
)

func TestBuildProviderVault(t *testing.T) {
	t.Parallel()
	t.Run("Should allow declaration-free human terminals without agent provider or Vault", func(t *testing.T) {
		t.Parallel()
		database := openDaemonTestGlobalDB(t)
		home := testHomePaths(t)
		profiles, err := profile.NewManager(profile.WithStore(database), profile.WithHomePaths(home))
		if err != nil {
			t.Fatal(err)
		}
		cfg := testConfig(t, home)
		cfg.Defaults.Provider = ""
		resolver, err := workspace.NewResolver(database, workspace.WithHomePaths(home),
			workspace.WithProfileAvailabilityChecker(profiles),
			workspace.WithProfileConfigLoader(func(string, string) (config.Config, error) { return cfg, nil }))
		if err != nil {
			t.Fatal(err)
		}
		registered, err := resolver.Register(t.Context(), workspace.RegisterOptions{RootDir: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		provider := terminalEnvironmentProvider(&bootState{profiles: profiles, workspaceResolver: resolver})
		for _, selector := range []string{registered.ID, "", "@user"} {
			t.Run("Should resolve terminal environment for "+selector, func(t *testing.T) {
				environment, err := provider(t.Context(), selector, terminal.Actor{
					Kind: terminal.ActorKindHuman, ProfileID: "00000000000000000000000000",
				})
				if err != nil || len(environment.Values) != 0 || len(environment.Unset) != 0 {
					t.Fatalf("declaration-free launch = %#v, %v", environment, err)
				}
			})
		}
	})

	t.Run("Should warn when registry does not implement vault store", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		homePaths := testHomePaths(t)
		daemonInstance := &Daemon{
			homePaths: homePaths,
			getenv:    func(string) string { return "" },
			now:       func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) },
		}
		service, err := daemonInstance.buildProviderVault(&bootState{
			logger:   slog.New(slog.NewTextHandler(&logs, nil)),
			registry: &recordingRegistry{path: homePaths.DatabaseFile},
		})
		if err != nil {
			t.Fatalf("buildProviderVault() error = %v", err)
		}
		if service != nil {
			t.Fatalf("buildProviderVault() = %#v, want nil service for non-vault registry", service)
		}
		if !strings.Contains(logs.String(), "daemon.provider_vault.disabled") ||
			!strings.Contains(logs.String(), "registry_missing_vault_store") ||
			!strings.Contains(logs.String(), "registry_type=") {
			t.Fatalf("provider vault warning log = %q, want disabled warning", logs.String())
		}
	})

	t.Run("Should create service when registry implements vault store", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		daemonInstance := &Daemon{
			homePaths: homePaths,
			getenv:    func(string) string { return "" },
			now:       func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) },
		}
		service, err := daemonInstance.buildProviderVault(&bootState{
			logger:   discardLogger(),
			registry: openDaemonTestGlobalDB(t),
		})
		if err != nil {
			t.Fatalf("buildProviderVault() error = %v", err)
		}
		if service == nil {
			t.Fatal("buildProviderVault() = nil, want provider vault service")
		}
	})

	t.Run("Should expose true nil interfaces when provider vault is disabled", func(t *testing.T) {
		t.Parallel()

		var service *vault.Service
		if got := sessionProviderVaultDependency(service); got != nil {
			t.Fatalf("sessionProviderVaultDependency(nil) = %#v, want nil interface", got)
		}
		if got := settingsProviderVaultDependency(service); got != nil {
			t.Fatalf("settingsProviderVaultDependency(nil) = %#v, want nil interface", got)
		}
	})
}

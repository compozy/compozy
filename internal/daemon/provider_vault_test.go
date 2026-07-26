package daemon

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/vault"
)

func TestBuildProviderVault(t *testing.T) {
	t.Parallel()

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

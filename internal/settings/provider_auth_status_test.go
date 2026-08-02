package settings

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	authproviders "github.com/compozy/compozy/internal/providers"
)

func TestProviderAuthStatusDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should expose missing native CLI diagnostics through provider settings", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+
			"\n[providers.local]\n"+
			"command = \"missing-agent acp\"\n"+
			"auth_mode = \"native_cli\"\n"+
			"auth_status_command = \"missing-agent auth status\"\n"+
			"auth_login_command = \"missing-agent login\"\n")
		resolveCalls := 0
		service := testService(t, homePaths, Dependencies{
			CommandLookPath: func(string) (string, error) {
				return "", exec.ErrNotFound
			},
			ProviderAuthCommandResolver: func(
				_ context.Context,
				command string,
				_ []string,
				_ string,
			) (string, error) {
				if command != "missing-agent" {
					return "/test/bin/" + command, nil
				}
				resolveCalls++
				if resolveCalls == 1 {
					return "", exec.ErrNotFound
				}
				return "/later/bin/missing-agent", nil
			},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		local := mustFindProviderItem(t, envelope.Providers, "local")

		if got, want := local.AuthStatus.State, "missing_cli"; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := local.AuthStatus.Code, "provider_cli_missing"; got != want {
			t.Fatalf("AuthStatus.Code = %q, want %q", got, want)
		}
		login := local.AuthStatus.Login
		if !login.Configured {
			t.Fatal("AuthStatus.Login.Configured = false, want true")
		}
		if got, want := login.Source, "auth_login_command"; got != want {
			t.Fatalf("AuthStatus.Login.Source = %q, want %q", got, want)
		}
		if got, want := login.Executable, "missing-agent"; got != want {
			t.Fatalf("AuthStatus.Login.Executable = %q, want %q", got, want)
		}
		if got, want := login.Presence, authproviders.ProviderLoginPresenceMissing; got != want {
			t.Fatalf("AuthStatus.Login.Presence = %q, want %q", got, want)
		}
		if got, want := login.RecommendedAction, authproviders.ProviderFailureActionInstallCLI; got != want {
			t.Fatalf("AuthStatus.Login.RecommendedAction = %q, want %q", got, want)
		}
		if !strings.Contains(local.AuthStatus.Message, "missing-agent") {
			t.Fatalf("AuthStatus.Message = %q, want missing-agent guidance", local.AuthStatus.Message)
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}
	})

	t.Run("Should expose a safe login descriptor without creating isolated provider state", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+
			"\n[providers.pi]\n"+
			"env_policy = \"isolated\"\n"+
			"home_policy = \"isolated\"\n")
		providerHome := filepath.Join(homePaths.HomeDir, "providers", "pi")
		assertPathMissing(t, providerHome)
		service := testService(t, homePaths, Dependencies{
			CommandLookPath: func(command string) (string, error) {
				if command == "npx" {
					return "/usr/local/bin/npx", nil
				}
				return "", errors.New("unexpected command lookup: " + command)
			},
			ProviderAuthCommandResolver: func(
				_ context.Context,
				command string,
				_ []string,
				_ string,
			) (string, error) {
				if command == "npx" {
					return "/usr/local/bin/npx", nil
				}
				return "", errors.New("unexpected command lookup: " + command)
			},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		pi := mustFindProviderItem(t, envelope.Providers, "pi")

		if got, want := pi.AuthStatus.State, "unknown"; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		login := pi.AuthStatus.Login
		if !login.Configured {
			t.Fatal("AuthStatus.Login.Configured = false, want true")
		}
		if got, want := login.Source, "auth_login_command"; got != want {
			t.Fatalf("AuthStatus.Login.Source = %q, want %q", got, want)
		}
		if got, want := login.Executable, "npx"; got != want {
			t.Fatalf("AuthStatus.Login.Executable = %q, want %q", got, want)
		}
		if got, want := login.Presence, authproviders.ProviderLoginPresencePresent; got != want {
			t.Fatalf("AuthStatus.Login.Presence = %q, want %q", got, want)
		}
		assertPathMissing(t, providerHome)
		if got, want := pi.AuthStatus.HomePolicy, compozyconfig.ProviderHomePolicyIsolated; got != want {
			t.Fatalf("AuthStatus.HomePolicy = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve the declared status command once for the returned native CLI", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+
			"\n[providers.local]\n"+
			"command = \"local-agent acp\"\n"+
			"auth_mode = \"native_cli\"\n"+
			"auth_status_command = \"status-helper auth status\"\n")

		var localResolveCalls int
		var localResolvedEnv []string
		service := testService(t, homePaths, Dependencies{
			CommandLookPath: func(string) (string, error) {
				return "/final/bin/launch", nil
			},
			ProviderAuthCommandResolver: func(
				_ context.Context,
				command string,
				env []string,
				dir string,
			) (string, error) {
				if command != "status-helper" {
					return "/final/bin/" + command, nil
				}
				localResolveCalls++
				if dir != "" {
					t.Fatalf("resolver dir = %q, want empty", dir)
				}
				localResolvedEnv = append([]string(nil), env...)
				if localResolveCalls > 1 {
					return "", errors.New("second local auth status resolution must not occur")
				}
				return "/final/bin/status-helper", nil
			},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		if got, want := localResolveCalls, 1; got != want {
			t.Fatalf("local auth status resolver calls = %d, want %d", got, want)
		}
		local := mustFindProviderItem(t, envelope.Providers, "local")
		if local.AuthStatus.Login.Configured {
			t.Fatal("AuthStatus.Login.Configured = true, want no configured login command")
		}
		if !slices.ContainsFunc(localResolvedEnv, func(entry string) bool {
			return entry == "COMPOZY_PROVIDER_AUTH_MODE=native_cli"
		}) {
			t.Fatalf("resolver environment = %#v, want final native auth mode", localResolvedEnv)
		}
	})

	t.Run("Should keep an operational status resolution failure unknown", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+
			"\n[providers.local]\n"+
			"command = \"local-agent acp\"\n"+
			"auth_mode = \"native_cli\"\n"+
			"auth_status_command = \"status-helper auth status\"\n")

		resolveCalls := 0
		service := testService(t, homePaths, Dependencies{
			CommandLookPath: func(string) (string, error) {
				return "/final/bin/local-agent", nil
			},
			ProviderAuthCommandResolver: func(
				_ context.Context,
				command string,
				_ []string,
				_ string,
			) (string, error) {
				if command != "status-helper" {
					return "/test/bin/" + command, nil
				}
				resolveCalls++
				if resolveCalls == 1 {
					return "", errors.New("resolver failed token=settings-resolution-secret")
				}
				return "/later/bin/status-helper", nil
			},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		local := mustFindProviderItem(t, envelope.Providers, "local")
		if got, want := local.AuthStatus.State, "unknown"; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := local.AuthStatus.Code, diagcontract.CodeProviderClassificationUnknown; got != want {
			t.Fatalf("AuthStatus.Code = %q, want %q", got, want)
		}
		if strings.Contains(local.AuthStatus.Message, "settings-resolution-secret") {
			t.Fatalf("message = %q, leaked resolver secret", local.AuthStatus.Message)
		}
		if !strings.Contains(local.AuthStatus.Message, "[REDACTED]") {
			t.Fatalf("message = %q, want redaction marker", local.AuthStatus.Message)
		}
		if got := local.AuthStatus.Login.Executable; got != "" {
			t.Fatalf("login executable = %q, want no configured login descriptor", got)
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}
	})
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("os.Stat(%q) error = nil, want missing path", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", path, err)
	}
}

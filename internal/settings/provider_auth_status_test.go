package settings

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/providerauth"
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
			"auth_login_command = \"missing-agent login\"\n")
		service := testService(t, homePaths, Dependencies{
			CommandLookPath: func(string) (string, error) {
				return "", exec.ErrNotFound
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
		nativeCLI := local.AuthStatus.NativeCLI
		if nativeCLI == nil {
			t.Fatal("AuthStatus.NativeCLI = nil, want missing CLI diagnostic")
		}
		if got, want := nativeCLI.Command, "missing-agent"; got != want {
			t.Fatalf("NativeCLI.Command = %q, want %q", got, want)
		}
		if nativeCLI.Present {
			t.Fatal("NativeCLI.Present = true, want false")
		}
		if got, want := nativeCLI.Source, providerauth.NativeCLISourceAuthLogin; got != want {
			t.Fatalf("NativeCLI.Source = %q, want %q", got, want)
		}
		if !strings.Contains(local.AuthStatus.Message, "missing-agent") {
			t.Fatalf("AuthStatus.Message = %q, want missing-agent guidance", local.AuthStatus.Message)
		}
	})

	t.Run("Should expose isolated native CLI login environment through provider settings", func(t *testing.T) {
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
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		pi := mustFindProviderItem(t, envelope.Providers, "pi")

		if got, want := pi.AuthStatus.State, "unknown"; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		nativeCLI := pi.AuthStatus.NativeCLI
		if nativeCLI == nil || !nativeCLI.Present {
			t.Fatalf("AuthStatus.NativeCLI = %#v, want present npx diagnostic", nativeCLI)
		}
		if got, want := nativeCLI.Command, "npx"; got != want {
			t.Fatalf("NativeCLI.Command = %q, want %q", got, want)
		}
		if got, want := nativeCLI.Source, providerauth.NativeCLISourceAuthLogin; got != want {
			t.Fatalf("NativeCLI.Source = %q, want %q", got, want)
		}
		assertPathMissing(t, providerHome)

		assertProviderAuthEnv(t, pi.AuthStatus.LoginEnv, "PROVIDER_HOME", providerHome)
		assertProviderAuthEnv(t, pi.AuthStatus.LoginEnv, "HOME", providerHome)
		assertProviderAuthEnv(
			t,
			pi.AuthStatus.LoginEnv,
			"PI_CODING_AGENT_DIR",
			filepath.Join(providerHome, ".pi", "agent"),
		)
		if got, want := pi.AuthStatus.HomePolicy, aghconfig.ProviderHomePolicyIsolated; got != want {
			t.Fatalf("AuthStatus.HomePolicy = %q, want %q", got, want)
		}
	})

	t.Run("Should deep copy mutable auth status fields when cloning provider items", func(t *testing.T) {
		t.Parallel()

		source := &ProviderItem{
			Name: "codex",
			AuthStatus: ProviderAuthStatus{
				LoginEnv: []string{"HOME=/tmp/original"},
				NativeCLI: &ProviderNativeCLIStatus{
					Command: "codex",
					Present: true,
					Source:  providerauth.NativeCLISourceAuthLogin,
				},
			},
		}

		cloned := cloneProviderItem(source)
		cloned.AuthStatus.LoginEnv[0] = "HOME=/tmp/cloned"
		cloned.AuthStatus.NativeCLI.Command = "changed"

		if got, want := source.AuthStatus.LoginEnv[0], "HOME=/tmp/original"; got != want {
			t.Fatalf("source AuthStatus.LoginEnv[0] = %q, want %q", got, want)
		}
		if got, want := source.AuthStatus.NativeCLI.Command, "codex"; got != want {
			t.Fatalf("source AuthStatus.NativeCLI.Command = %q, want %q", got, want)
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

func assertProviderAuthEnv(t *testing.T, env []string, key string, want string) {
	t.Helper()

	prefix := key + "="
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			if value != want {
				t.Fatalf("%s = %q, want %q in %#v", key, value, want, env)
			}
			return
		}
	}
	t.Fatalf("%s missing from %#v", key, env)
}

package providers

import (
	"context"
	"os/exec"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	diagcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/testutil"
)

func TestPreStart(t *testing.T) {
	// Pre-start cache is package-global, so these cases run sequentially.
	t.Run("Should return nil item when auth mode is none", func(t *testing.T) {
		InvalidatePreStartCache()

		calls := 0
		report := PreStart(testutil.Context(t), aghconfig.ProviderConfig{
			Command:  "missing-provider acp",
			AuthMode: aghconfig.ProviderAuthModeNone,
		}, &ProbeEnv{
			ProviderName: "none-provider",
			LookPath: func(string) (string, error) {
				calls++
				return "", exec.ErrNotFound
			},
		})
		if report.Item != nil {
			t.Fatalf("PreStart(none).Item = %#v, want nil", report.Item)
		}
		if calls != 0 {
			t.Fatalf("LookPath calls = %d, want 0", calls)
		}
	})

	t.Run("Should return provider CLI missing item when launch command is unavailable", func(t *testing.T) {
		InvalidatePreStartCache()

		report := PreStart(testutil.Context(t), aghconfig.ProviderConfig{
			Command:  "missing-provider acp",
			AuthMode: aghconfig.ProviderAuthModeNativeCLI,
		}, &ProbeEnv{
			ProviderName: "missing",
			LookPath: func(string) (string, error) {
				return "", exec.ErrNotFound
			},
		})
		if report.Item == nil {
			t.Fatal("PreStart(missing CLI).Item = nil, want diagnostic")
		}
		if report.Item.Code != diagcontract.CodeProviderCLIMissing {
			t.Fatalf("Code = %q, want %q", report.Item.Code, diagcontract.CodeProviderCLIMissing)
		}
	})

	t.Run("Should return credential unresolved item when required env ref is absent", func(t *testing.T) {
		InvalidatePreStartCache()

		report := PreStart(testutil.Context(t), aghconfig.ProviderConfig{
			Command:  "provider acp",
			AuthMode: aghconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []aghconfig.ProviderCredentialSlot{
				{Name: "api_key", TargetEnv: "TEST_API_KEY", SecretRef: "env:TEST_API_KEY", Required: true},
			},
		}, &ProbeEnv{
			ProviderName: "bound",
			LookPath: func(string) (string, error) {
				return "/bin/provider", nil
			},
			LookupEnv: func(string) (string, bool) {
				return "", false
			},
		})
		if report.Item == nil {
			t.Fatal("PreStart(missing credential).Item = nil, want diagnostic")
		}
		if report.Item.Code != diagcontract.CodeProviderCredentialUnresolved {
			t.Fatalf("Code = %q, want %q", report.Item.Code, diagcontract.CodeProviderCredentialUnresolved)
		}
	})

	t.Run("Should run status command without TTY", func(t *testing.T) {
		InvalidatePreStartCache()

		report := PreStart(testutil.Context(t), aghconfig.ProviderConfig{
			Command:       "provider acp",
			AuthMode:      aghconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: "provider auth status",
		}, &ProbeEnv{
			ProviderName: "native",
			LookPath: func(string) (string, error) {
				return "/bin/provider", nil
			},
			RunCommand: func(_ context.Context, spec ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				if !spec.NoTTY {
					t.Fatal("NoTTY = false, want pre-start probe to be non-interactive")
				}
				return ProviderAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
			},
		})
		if report.Item == nil {
			t.Fatal("PreStart(needs login).Item = nil, want diagnostic")
		}
		if report.Item.Code != diagcontract.CodeProviderNotAuthenticated {
			t.Fatalf("Code = %q, want %q", report.Item.Code, diagcontract.CodeProviderNotAuthenticated)
		}
	})

	t.Run("Should cache report within TTL", func(t *testing.T) {
		InvalidatePreStartCache()

		calls := 0
		env := &ProbeEnv{
			ProviderName: "cached",
			LookPath: func(string) (string, error) {
				calls++
				return "", exec.ErrNotFound
			},
		}
		provider := aghconfig.ProviderConfig{
			Command:  "cached-provider acp",
			AuthMode: aghconfig.ProviderAuthModeNativeCLI,
		}
		first := PreStart(context.Background(), provider, env)
		second := PreStart(context.Background(), provider, env)
		if first.Item == nil || second.Item == nil {
			t.Fatalf("PreStart cache reports = %#v %#v, want diagnostics", first.Item, second.Item)
		}
		if calls != 1 {
			t.Fatalf("LookPath calls = %d, want 1", calls)
		}
	})

	t.Run("Should recompute when config hash changes", func(t *testing.T) {
		InvalidatePreStartCache()

		calls := 0
		env := &ProbeEnv{
			ProviderName: "changed",
			LookPath: func(string) (string, error) {
				calls++
				return "", exec.ErrNotFound
			},
		}
		_ = PreStart(context.Background(), aghconfig.ProviderConfig{
			Command:  "changed-a acp",
			AuthMode: aghconfig.ProviderAuthModeNativeCLI,
		}, env)
		_ = PreStart(context.Background(), aghconfig.ProviderConfig{
			Command:  "changed-b acp",
			AuthMode: aghconfig.ProviderAuthModeNativeCLI,
		}, env)
		if calls != 2 {
			t.Fatalf("LookPath calls = %d, want 2", calls)
		}
	})

	t.Run("Should recompute after cache invalidation", func(t *testing.T) {
		InvalidatePreStartCache()

		calls := 0
		env := &ProbeEnv{
			ProviderName: "invalidated",
			LookPath: func(string) (string, error) {
				calls++
				return "", exec.ErrNotFound
			},
		}
		provider := aghconfig.ProviderConfig{
			Command:  "invalidated-provider acp",
			AuthMode: aghconfig.ProviderAuthModeNativeCLI,
		}
		_ = PreStart(context.Background(), provider, env)
		InvalidatePreStartCache()
		_ = PreStart(context.Background(), provider, env)
		if calls != 2 {
			t.Fatalf("LookPath calls = %d, want 2", calls)
		}
	})
}

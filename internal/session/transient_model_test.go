package session

import (
	"errors"
	"slices"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
)

func TestInvokeTransientModel(t *testing.T) {
	t.Parallel()

	t.Run("Should execute one bounded turn without publishing a durable session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		provider := "claude"
		result, err := h.manager.InvokeTransientModel(testutil.Context(t), TransientModelCall{
			Config:         &h.cfg,
			Provider:       provider,
			Model:          h.cfg.Providers[provider].Models.Default,
			CWD:            h.workspace,
			Prompt:         "Choose the memory operation.",
			MaxOutputBytes: 64,
		})
		if err != nil {
			t.Fatalf("InvokeTransientModel() error = %v", err)
		}
		if !result.Accepted || result.Output != "reply" {
			t.Fatalf("InvokeTransientModel() = %#v, want accepted reply", result)
		}
		if sessions := h.manager.List(); len(sessions) != 0 {
			t.Fatalf("List() = %#v, want no durable transient session", sessions)
		}
		if len(h.driver.startCalls) != 1 || len(h.driver.promptCalls) != 1 || h.driver.stopCalls != 1 {
			t.Fatalf(
				"driver start/prompt/stop calls = %d/%d/%d, want 1/1/1",
				len(h.driver.startCalls),
				len(h.driver.promptCalls),
				h.driver.stopCalls,
			)
		}
		if h.driver.startCalls[0].Permissions != compozyconfig.PermissionModeDenyAll {
			t.Fatalf(
				"Start().Permissions = %q, want %q",
				h.driver.startCalls[0].Permissions,
				compozyconfig.PermissionModeDenyAll,
			)
		}
	})

	t.Run("Should propagate turn ID entropy failure without prompting", func(t *testing.T) {
		t.Parallel()

		entropyErr := errors.New("turn entropy unavailable")
		h := newHarness(t, WithTurnIDGenerator(func() (string, error) {
			return "", entropyErr
		}))
		provider := "claude"
		result, err := h.manager.InvokeTransientModel(testutil.Context(t), TransientModelCall{
			Config:         &h.cfg,
			Provider:       provider,
			Model:          h.cfg.Providers[provider].Models.Default,
			CWD:            h.workspace,
			Prompt:         "Choose the memory operation.",
			MaxOutputBytes: 64,
		})
		if !errors.Is(err, entropyErr) {
			t.Fatalf("InvokeTransientModel() error = %v, want entropy failure", err)
		}
		if result.Accepted {
			t.Fatalf("InvokeTransientModel() result = %#v, want unaccepted", result)
		}
		if got, want := len(h.driver.startCalls), 1; got != want {
			t.Fatalf("driver start calls = %d, want %d", got, want)
		}
		if got := len(h.driver.promptCalls); got != 0 {
			t.Fatalf("driver prompt calls = %d, want 0", got)
		}
		if got, want := h.driver.stopCalls, 1; got != want {
			t.Fatalf("driver stop calls = %d, want %d", got, want)
		}
	})

	t.Run("Should forward the final Pi provider probe environment to ACP", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		provider := "pi"
		h.cfg.Providers[provider] = compozyconfig.ProviderConfig{
			EnvPolicy:  compozyconfig.ProviderEnvPolicyIsolated,
			HomePolicy: compozyconfig.ProviderHomePolicyIsolated,
		}
		resolved, err := h.cfg.ResolveProvider(provider)
		if err != nil {
			t.Fatalf("ResolveProvider(%q) error = %v", provider, err)
		}
		_, err = h.manager.InvokeTransientModel(testutil.Context(t), TransientModelCall{
			Config:         &h.cfg,
			Provider:       provider,
			Model:          resolved.Models.Default,
			CWD:            h.workspace,
			Prompt:         "Choose the memory operation.",
			MaxOutputBytes: 64,
		})
		if err != nil {
			t.Fatalf("InvokeTransientModel() error = %v", err)
		}
		if len(h.driver.startCalls) != 1 {
			t.Fatalf("Start() calls = %d, want 1", len(h.driver.startCalls))
		}
		start := h.driver.startCalls[0]
		if start.ProviderAuthEnv == nil {
			t.Fatal("StartOpts.ProviderAuthEnv = nil, want final provider probe environment")
		}
		if !slices.Equal(start.ProviderAuthEnv.CommandEnv, start.Env) {
			t.Fatalf(
				"ProviderAuthEnv.CommandEnv = %#v, want final env %#v",
				start.ProviderAuthEnv.CommandEnv,
				start.Env,
			)
		}
		if start.ProviderAuthEnv.CommandDir != start.Cwd {
			t.Fatalf("ProviderAuthEnv.CommandDir = %q, want final cwd %q", start.ProviderAuthEnv.CommandDir, start.Cwd)
		}
		if start.ProviderAuthEnv.RunCommand == nil {
			t.Fatal("ProviderAuthEnv.RunCommand = nil, want validated command runner")
		}
		if piDir, ok := subprocess.LookupEnv(start.Env, "PI_CODING_AGENT_DIR"); !ok || piDir == "" {
			t.Fatalf("PI_CODING_AGENT_DIR = %q, %t; want final Pi runtime directory", piDir, ok)
		}
	})
}

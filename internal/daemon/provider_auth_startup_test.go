package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
)

type startupProbeFixture struct {
	check     providerAuthStartup
	verdicts  *providers.AuthVerdicts
	installed map[string]bool
	run       providers.ProviderAuthCommandRunner
}

func newStartupProbeFixture(run providers.ProviderAuthCommandRunner, installed ...string) *startupProbeFixture {
	f := &startupProbeFixture{verdicts: providers.NewAuthVerdicts(), installed: map[string]bool{}, run: run}
	for _, name := range installed {
		f.installed[name] = true
	}
	f.check = providerAuthStartup{
		verdicts: f.verdicts,
		envFor: func(name string, _ compozyconfig.ProviderConfig) (providers.ProbeEnv, error) {
			return providers.ProbeEnv{
				ProviderName: name,
				LookPath: func(command string) (string, error) {
					if f.installed[name] {
						return "/bin/" + command, nil
					}
					return "", exec.ErrNotFound
				},
				LookupEnv:  func(string) (string, bool) { return "", false },
				RunCommand: f.run,
			}, nil
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    time.Now,
	}
	return f
}

func nativeStatusProvider(name string) compozyconfig.ProviderConfig {
	return compozyconfig.ProviderConfig{
		Command:       name + " acp",
		AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
		AuthStatusCmd: name + " auth status",
	}
}

func startupConfig(providerConfigs map[string]compozyconfig.ProviderConfig) *compozyconfig.Config {
	return &compozyconfig.Config{Providers: providerConfigs}
}

func TestProviderAuthStartup(t *testing.T) {
	t.Run("Should record each installed native provider's live verdict", func(t *testing.T) {
		t.Parallel()
		f := newStartupProbeFixture(func(_ context.Context, spec providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			if spec.Executable == "/bin/acmeout" {
				return providers.ProviderAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
			}
			return providers.ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in"}, nil
		}, "acmein", "acmeout")
		cfg := startupConfig(map[string]compozyconfig.ProviderConfig{
			"acmein":  nativeStatusProvider("acmein"),
			"acmeout": nativeStatusProvider("acmeout"),
		})
		f.check.run(t.Context(), cfg)
		in, ok := f.verdicts.Lookup("acmein", cfg.Providers["acmein"])
		if !ok || in.Classification.State != providers.ProviderAuthStateAuthenticated {
			t.Fatalf("acmein verdict = %#v, %v; want authenticated", in, ok)
		}
		out, ok := f.verdicts.Lookup("acmeout", cfg.Providers["acmeout"])
		if !ok || out.Classification.State != providers.ProviderAuthStateNeedsLogin {
			t.Fatalf("acmeout verdict = %#v, %v; want needs_login", out, ok)
		}
	})

	t.Run("Should run at most two status commands at a time", func(t *testing.T) {
		t.Parallel()
		var active, peak atomic.Int32
		started := make(chan struct{}, 8)
		gate := make(chan struct{})
		f := newStartupProbeFixture(func(context.Context, providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			now := active.Add(1)
			for {
				prev := peak.Load()
				if now <= prev || peak.CompareAndSwap(prev, now) {
					break
				}
			}
			started <- struct{}{}
			<-gate
			active.Add(-1)
			return providers.ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in"}, nil
		}, "p1", "p2", "p3", "p4", "p5")
		configs := map[string]compozyconfig.ProviderConfig{}
		for name := range f.installed {
			configs[name] = nativeStatusProvider(name)
		}
		cfg := startupConfig(configs)
		done := make(chan struct{})
		go func() { f.check.run(t.Context(), cfg); close(done) }()
		<-started
		<-started
		close(gate)
		<-done
		if got := peak.Load(); got != providerAuthStartupConcurrency {
			t.Fatalf("peak concurrent status commands = %d, want %d", got, providerAuthStartupConcurrency)
		}
		for name, provider := range configs {
			if _, ok := f.verdicts.Lookup(name, provider); !ok {
				t.Fatalf("%s verdict missing", name)
			}
		}
	})

	t.Run("Should skip providers the startup check does not own", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		f := newStartupProbeFixture(func(context.Context, providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			calls.Add(1)
			return providers.ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in"}, nil
		}, "acponly", "secret", "known")
		acpOnly := nativeStatusProvider("acponly")
		acpOnly.AuthStatusCmd = ""
		secret := nativeStatusProvider("secret")
		secret.AuthMode = compozyconfig.ProviderAuthModeBoundSecret
		known := nativeStatusProvider("known")
		f.verdicts.Record(
			"known",
			known,
			providers.Classification{State: providers.ProviderAuthStateAuthenticated},
			time.Now(),
		)
		f.check.run(t.Context(), startupConfig(map[string]compozyconfig.ProviderConfig{
			"missing": nativeStatusProvider("missing"),
			"acponly": acpOnly,
			"secret":  secret,
			"known":   known,
		}))
		if got := calls.Load(); got != 0 {
			t.Fatalf("status commands run = %d, want 0", got)
		}
	})

	t.Run("Should stop on cancellation without recording a verdict", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		var once sync.Once
		f := newStartupProbeFixture(func(ctx context.Context, _ providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			once.Do(func() { close(entered) })
			<-ctx.Done()
			return providers.ProviderAuthCommandResult{ExitCode: -1}, errors.Join(ctx.Err())
		}, "slow")
		cfg := startupConfig(map[string]compozyconfig.ProviderConfig{"slow": nativeStatusProvider("slow")})
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { f.check.run(ctx, cfg); close(done) }()
		<-entered
		cancel()
		<-done
		if _, ok := f.verdicts.Lookup("slow", cfg.Providers["slow"]); ok {
			t.Fatal("canceled startup check recorded a verdict")
		}
	})

	t.Run("Should serve the startup verdict to provider reads", func(t *testing.T) {
		t.Parallel()
		f := newStartupProbeFixture(func(context.Context, providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			return providers.ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in"}, nil
		}, "acme")
		provider := nativeStatusProvider("acme")
		f.check.run(t.Context(), startupConfig(map[string]compozyconfig.ProviderConfig{"acme": provider}))
		env, err := f.check.envFor("acme", provider)
		if err != nil {
			t.Fatal(err)
		}
		declared, err := providers.ClassifyDeclared(t.Context(), provider, &env)
		if err != nil {
			t.Fatal(err)
		}
		applied, checkedAt := f.verdicts.Apply("acme", provider, declared)
		if applied.State != providers.ProviderAuthStateAuthenticated || checkedAt.IsZero() {
			t.Fatalf("Apply = %q at %v, want authenticated with a check time", applied.State, checkedAt)
		}
	})
}

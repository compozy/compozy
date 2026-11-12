package daemon

import (
	"context"
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

// runWith serves cfg as the active configuration for one startup check.
func (f *startupProbeFixture) runWith(ctx context.Context, cfg *compozyconfig.Config) {
	f.check.config = func(context.Context) (compozyconfig.Config, error) { return *cfg, nil }
	f.check.run(ctx)
}

// liveConfig is an active configuration a test edits while the startup check runs.
type liveConfig struct {
	mu  sync.Mutex
	cfg compozyconfig.Config
}

func (l *liveConfig) read(context.Context) (compozyconfig.Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return compozyconfig.CloneConfig(&l.cfg), nil
}

func (l *liveConfig) setProvider(name string, provider compozyconfig.ProviderConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cfg.Providers[name] = provider
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
	t.Parallel()

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
		f.runWith(t.Context(), cfg)
		in, ok := f.verdicts.Lookup("acmein", cfg.Providers["acmein"], nil)
		if !ok || in.Classification.State != providers.ProviderAuthStateAuthenticated {
			t.Fatalf("acmein verdict = %#v, %v; want authenticated", in, ok)
		}
		out, ok := f.verdicts.Lookup("acmeout", cfg.Providers["acmeout"], nil)
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
		go func() { f.runWith(t.Context(), cfg); close(done) }()
		<-started
		<-started
		close(gate)
		<-done
		if got := peak.Load(); got > 2 {
			t.Fatalf("peak concurrent status commands = %d, want at most 2", got)
		}
		for name, provider := range configs {
			if _, ok := f.verdicts.Lookup(name, provider, nil); !ok {
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
			nil,
			providers.Classification{State: providers.ProviderAuthStateAuthenticated},
			time.Now(),
		)
		f.runWith(t.Context(), startupConfig(map[string]compozyconfig.ProviderConfig{
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
			return providers.ProviderAuthCommandResult{ExitCode: -1}, ctx.Err()
		}, "slow")
		cfg := startupConfig(map[string]compozyconfig.ProviderConfig{"slow": nativeStatusProvider("slow")})
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { f.runWith(ctx, cfg); close(done) }()
		<-entered
		cancel()
		<-done
		if _, ok := f.verdicts.Lookup("slow", cfg.Providers["slow"], nil); ok {
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
		f.runWith(t.Context(), startupConfig(map[string]compozyconfig.ProviderConfig{"acme": provider}))
		env, err := f.check.envFor("acme", provider)
		if err != nil {
			t.Fatal(err)
		}
		declared, err := providers.ClassifyDeclared(t.Context(), provider, &env)
		if err != nil {
			t.Fatal(err)
		}
		applied, checkedAt := f.verdicts.Apply("acme", provider, env.CommandEnv, declared)
		if applied.State != providers.ProviderAuthStateAuthenticated || checkedAt.IsZero() {
			t.Fatalf("Apply = %q at %v, want authenticated with a check time", applied.State, checkedAt)
		}
	})

	t.Run("Should keep a newer verdict for a provider edited while it waited in the queue", func(t *testing.T) {
		t.Parallel()
		started := make(chan struct{}, 4)
		gate := make(chan struct{})
		f := newStartupProbeFixture(func(context.Context, providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			started <- struct{}{}
			<-gate
			return providers.ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in"}, nil
		}, "p1", "p2", "p3")
		live := &liveConfig{cfg: *startupConfig(map[string]compozyconfig.ProviderConfig{
			"p1": nativeStatusProvider("p1"),
			"p2": nativeStatusProvider("p2"),
			"p3": nativeStatusProvider("p3"),
		})}
		f.check.config = live.read
		done := make(chan struct{})
		go func() { f.check.run(t.Context()); close(done) }()
		<-started
		<-started
		edited := nativeStatusProvider("p3")
		edited.AuthStatusCmd = "p3 auth status --json"
		live.setProvider("p3", edited)
		f.verdicts.Record(
			"p3",
			edited,
			nil,
			providers.Classification{State: providers.ProviderAuthStateNeedsLogin},
			time.Now(),
		)
		close(gate)
		<-done
		verdict, ok := f.verdicts.Lookup("p3", edited, nil)
		if !ok || verdict.Classification.State != providers.ProviderAuthStateNeedsLogin {
			t.Fatalf("p3 verdict = %#v, %v; want the newer needs_login verdict kept", verdict, ok)
		}
	})

	t.Run("Should not record a probe whose provider changed while it ran", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		gate := make(chan struct{})
		var once sync.Once
		f := newStartupProbeFixture(func(context.Context, providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			once.Do(func() { close(entered) })
			<-gate
			return providers.ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in"}, nil
		}, "acme")
		original := nativeStatusProvider("acme")
		live := &liveConfig{cfg: *startupConfig(map[string]compozyconfig.ProviderConfig{"acme": original})}
		f.check.config = live.read
		done := make(chan struct{})
		go func() { f.check.run(t.Context()); close(done) }()
		<-entered
		edited := original
		edited.AuthStatusCmd = "acme auth status --json"
		live.setProvider("acme", edited)
		close(gate)
		<-done
		if _, ok := f.verdicts.Lookup("acme", original, nil); ok {
			t.Fatal("startup check recorded a verdict for the replaced provider settings")
		}
	})

	t.Run("Should keep a verdict recorded for new settings while the check held stale ones", func(t *testing.T) {
		t.Parallel()
		var runs atomic.Int32
		f := newStartupProbeFixture(func(context.Context, providers.ProviderAuthCommandSpec) (
			providers.ProviderAuthCommandResult, error,
		) {
			runs.Add(1)
			return providers.ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in"}, nil
		}, "acme")
		stale := nativeStatusProvider("acme")
		applied := stale
		applied.AuthStatusCmd = "acme auth status --json"
		live := &liveConfig{cfg: *startupConfig(map[string]compozyconfig.ProviderConfig{"acme": stale})}
		f.check.config = live.read
		envFor := f.check.envFor
		f.check.envFor = func(name string, provider compozyconfig.ProviderConfig) (providers.ProbeEnv, error) {
			// Apply only after acme captured its settings; other providers may read config concurrently.
			if name == "acme" {
				live.setProvider(name, applied)
				f.verdicts.Record(
					name,
					applied,
					nil,
					providers.Classification{State: providers.ProviderAuthStateNeedsLogin},
					time.Now(),
				)
			}
			return envFor(name, provider)
		}
		f.check.run(t.Context())
		verdict, ok := f.verdicts.Lookup("acme", applied, nil)
		if !ok || verdict.Classification.State != providers.ProviderAuthStateNeedsLogin {
			t.Fatalf("acme verdict = %#v, %v; want the verdict for the applied settings kept", verdict, ok)
		}
		if got := runs.Load(); got != 1 {
			t.Fatalf("status commands run = %d, want 1 (stale probe runs but is not recorded)", got)
		}
	})
}

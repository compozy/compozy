package daemon

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	config "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
)

const (
	providerAuthStartupConcurrency = 2
	providerAuthStartupLabel       = "provider sign-in check"
)

// providerAuthStartup verifies native-CLI sign-in once after the daemon starts.
type providerAuthStartup struct {
	verdicts *providers.AuthVerdicts
	envFor   func(string, config.ProviderConfig) (providers.ProbeEnv, error)
	logger   *slog.Logger
	now      func() time.Time
	// config reads the active configuration, so a provider edited during the check is not probed
	// or recorded with its boot-time settings.
	config func(context.Context) (config.Config, error)
}

// bootProviderAuthStartup runs the startup sign-in check as an owned worker that shutdown joins.
func (d *Daemon) bootProviderAuthStartup(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if d.providerAuthVerdicts == nil {
		return nil
	}
	options := providers.DaemonProbeEnvOptions{
		HomePaths:      d.homePaths,
		ResolveCommand: providers.DefaultProviderAuthCommandResolver,
		RunCommand:     providers.DefaultProviderAuthCommandRunner,
	}
	if state.providerVault != nil {
		options.Vault = state.providerVault
	}
	check := providerAuthStartup{
		verdicts: d.providerAuthVerdicts,
		envFor: func(name string, provider config.ProviderConfig) (providers.ProbeEnv, error) {
			return providers.DaemonProbeEnv(options, name, provider)
		},
		logger: state.logger,
		now:    func() time.Time { return d.now().UTC() },
		config: activeConfigSource(state),
	}
	workerCtx, cancel := context.WithCancel(ctx)
	worker := newOwnedWorkerGroup(cancel)
	complete, ok := worker.Begin()
	if !ok {
		cancel()
		return nil
	}
	state.runtimeWorkers.providerAuthStartup = worker
	cleanup.add(func(cleanupCtx context.Context) error {
		return stopOwnedWorker(cleanupCtx, providerAuthStartupLabel, worker)
	})
	go func() {
		defer complete()
		defer cancel()
		check.run(workerCtx)
	}()
	return nil
}

// activeConfigSource reads settings' active configuration, falling back to the boot snapshot.
func activeConfigSource(state *bootState) func(context.Context) (config.Config, error) {
	if source, ok := state.deps.Settings.(interface {
		ActiveConfig(context.Context) (config.Config, error)
	}); ok {
		return source.ActiveConfig
	}
	snapshot := config.CloneConfig(&state.cfg)
	return func(context.Context) (config.Config, error) { return config.CloneConfig(&snapshot), nil }
}

func (p providerAuthStartup) run(ctx context.Context) {
	cfg, err := p.config(ctx)
	if err != nil {
		p.logger.Warn("provider_auth.startup_probe", "error", err)
		return
	}
	limit := make(chan struct{}, providerAuthStartupConcurrency)
	var wg sync.WaitGroup
	var probed, skipped atomic.Int32
	for _, name := range config.ProviderInventoryNames(&cfg) {
		if provider, err := cfg.ResolveProvider(name); err != nil || !statusCommandProvider(provider) {
			skipped.Add(1)
			continue
		}
		select {
		case limit <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Go(func() {
			defer func() { <-limit }()
			if p.probe(ctx, name) {
				probed.Add(1)
				return
			}
			skipped.Add(1)
		})
	}
	wg.Wait()
	if ctx.Err() == nil {
		p.logger.Info("provider_auth.startup_probe_done", "probed", probed.Load(), "skipped", skipped.Load())
	}
}

// statusCommandProvider skips ACP-only providers, which would start a full agent.
func statusCommandProvider(provider config.ProviderConfig) bool {
	return provider.EffectiveAuthMode() == config.ProviderAuthModeNativeCLI &&
		strings.TrimSpace(provider.AuthStatusCmd) != ""
}

// currentProvider resolves the provider from the active configuration at this moment.
func (p providerAuthStartup) currentProvider(ctx context.Context, name string) (config.ProviderConfig, bool) {
	cfg, err := p.config(ctx)
	if err != nil {
		return config.ProviderConfig{}, false
	}
	provider, err := cfg.ResolveProvider(name)
	return provider, err == nil && statusCommandProvider(provider)
}

// probe runs one status command against the active configuration and records its verdict unless
// the check was canceled or the provider changed meanwhile.
func (p providerAuthStartup) probe(ctx context.Context, name string) bool {
	provider, ok := p.currentProvider(ctx, name)
	if !ok {
		return false
	}
	// Known never drops a verdict, so settings read just before an apply cannot erase a newer one.
	if p.verdicts.Known(name, provider) {
		return false
	}
	env, err := p.envFor(name, provider)
	if err != nil {
		p.logger.Warn("provider_auth.startup_probe", "provider", name, "error", err)
		return false
	}
	declared, err := providers.ClassifyDeclared(ctx, provider, &env)
	if err != nil {
		p.logger.Warn("provider_auth.startup_probe", "provider", name, "error", err)
		return false
	}
	if declared.State != providers.ProviderAuthStateUnknown {
		return false
	}
	result, err := providers.RunAuthStatusProbe(ctx, provider, &env, p.now)
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		p.logger.Warn("provider_auth.startup_probe", "provider", name, "error", err)
		return false
	}
	if !result.Ran {
		p.logger.Info("provider_auth.startup_probe", "provider", name, "state", result.Classification.State,
			"error_code", result.Classification.Code)
		return false
	}
	if current, ok := p.currentProvider(ctx, name); !ok || !reflect.DeepEqual(current, provider) {
		return false
	}
	p.verdicts.Record(name, provider, result.Classification, result.StartedAt)
	p.logger.Info(
		"provider_auth.startup_probe",
		"provider", name,
		"state", result.Classification.State,
		"duration_ms", result.Result.DurationMs,
		"error_code", result.Classification.Code,
	)
	return true
}

package daemon

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	config "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
)

// providerAuthStartupConcurrency bounds how many provider CLIs the startup sign-in check runs at once.
const providerAuthStartupConcurrency = 2

// providerAuthStartup verifies native-CLI sign-in once after the daemon starts, so provider reads
// do not report every provider as unverified until a session or a manual check probes it.
type providerAuthStartup struct {
	verdicts *providers.AuthVerdicts
	envFor   func(string, config.ProviderConfig) (providers.ProbeEnv, error)
	logger   *slog.Logger
	now      func() time.Time
}

// bootProviderAuthStartup starts the startup sign-in check as an owned worker that shutdown cancels
// and joins.
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
		now:    func() time.Time { return time.Now().UTC() },
	}
	cfg := config.CloneConfig(&state.cfg)
	workerCtx, cancel := context.WithCancel(ctx)
	worker := newOwnedWorkerGroup(cancel)
	complete, ok := worker.Begin()
	if !ok {
		cancel()
		return nil
	}
	state.runtimeWorkers.providerAuthStartup = worker
	cleanup.add(func(cleanupCtx context.Context) error {
		return stopOwnedWorker(cleanupCtx, "provider sign-in check", worker)
	})
	go func() {
		defer complete()
		defer cancel()
		check.run(workerCtx, &cfg)
	}()
	return nil
}

func (p providerAuthStartup) run(ctx context.Context, cfg *config.Config) {
	limit := make(chan struct{}, providerAuthStartupConcurrency)
	var wg sync.WaitGroup
	var probed, skipped atomic.Int32
	for _, name := range core.ProviderInventoryNames(cfg) {
		provider, err := cfg.ResolveProvider(name)
		if err != nil || !p.eligible(name, provider) {
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
			if p.probe(ctx, name, provider) {
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

// eligible keeps the check to native-CLI providers with a status command and no verdict yet; ACP-only
// providers would start a full agent and stay on the manual check.
func (p providerAuthStartup) eligible(name string, provider config.ProviderConfig) bool {
	if provider.EffectiveAuthMode() != config.ProviderAuthModeNativeCLI ||
		strings.TrimSpace(provider.AuthStatusCmd) == "" {
		return false
	}
	_, known := p.verdicts.Lookup(name, provider)
	return !known
}

// probe runs one status command and records its verdict unless the check was canceled.
func (p providerAuthStartup) probe(ctx context.Context, name string, provider config.ProviderConfig) bool {
	env, err := p.envFor(name, provider)
	if err != nil {
		p.logger.Warn("provider_auth.startup_probe", "provider", name, "error", err)
		return false
	}
	declared, err := providers.ClassifyDeclared(ctx, provider, &env)
	if err != nil || declared.State != providers.ProviderAuthStateUnknown {
		return false
	}
	started := p.now()
	result, err := providers.RunAuthStatusProbe(ctx, provider, &env, p.now)
	if ctx.Err() != nil || err != nil || !result.Ran {
		return false
	}
	p.verdicts.Record(name, provider, result.Classification, result.StartedAt)
	p.logger.Info(
		"provider_auth.startup_probe",
		"provider", name,
		"state", result.Classification.State,
		"duration_ms", p.now().Sub(started).Milliseconds(),
		"error_code", result.Classification.Code,
	)
	return true
}

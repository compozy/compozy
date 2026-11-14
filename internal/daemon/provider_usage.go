package daemon

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	config "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/providerusage"
	"github.com/compozy/compozy/internal/store"
)

type providerUsageRuntime struct {
	*providerusage.Manager
	mu            sync.Mutex
	resolver      *providerUsageResolver
	metrics       *providerUsageMetrics
	ctx           context.Context
	profileEvents chan profile.Event
	profileCancel context.CancelFunc
	profileDone   chan struct{}
	authMu        sync.RWMutex
	recoverAuth   func(string, string, string) bool
	// recheckSignIn re-checks a provider whose stored needs_login verdict a verified collection
	// contradicts.
	recheckSignIn func(string)
}

func (d *Daemon) bootProviderUsage(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	registry := providerUsageCollectors(d.now)
	resolver := &providerUsageResolver{
		cfg: config.CloneConfig(&state.cfg), home: d.homePaths, baseEnv: os.Environ(),
		profiles: state.profiles, secrets: d.modelCatalogSecretResolver(state), collectors: registry,
		logger: state.logger,
	}
	if state.providerVault != nil {
		resolver.metadata = state.providerVault
	}
	metrics := &providerUsageMetrics{}
	runtime := &providerUsageRuntime{resolver: resolver, metrics: metrics, ctx: ctx}
	runtime.Manager = providerusage.NewService(providerusage.Options{
		Resolver:        resolver,
		Collectors:      registry,
		RefreshInterval: state.cfg.ProviderUsage.RefreshInterval,
		Logger:          state.logger,
		Metrics:         metrics,
		OnAuthenticated: runtime.onAuthenticated,
		Disabled:        !state.cfg.ProviderUsage.Enabled,
	})
	cleanup.add(runtime.Shutdown)
	if err := runtime.syncProfiles(ctx); err != nil {
		return fmt.Errorf("daemon: start provider usage: %w", err)
	}
	runtime.startProfileWorker(state.logger)
	state.providerUsage = runtime
	return nil
}

func providerUsageCollectors(now func() time.Time) providerusage.Registry {
	return providerusage.DefaultRegistry(providerusage.RegistryOptions{Now: now})
}

func (r *providerUsageRuntime) syncProfiles(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, profileLifecycleEventWriteTimeout)
	defer cancel()
	ids := []string{store.DefaultProfileID}
	if r.resolver.profiles != nil {
		profiles, err := r.resolver.profiles.List(ctx)
		if err != nil {
			return err
		}
		ids = ids[:0]
		for _, item := range profiles {
			ids = append(ids, item.ID)
		}
	}
	r.resolver.pruneSelectorWarnings(ids)
	return r.Start(ctx, ids)
}

func (r *providerUsageRuntime) ReconcileConfig(ctx context.Context, next *config.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolver.mu.Lock()
	previous := r.resolver.cfg.ProviderUsage
	r.resolver.cfg = config.CloneConfig(next)
	r.resolver.mu.Unlock()
	if previous.ClaudeKeychainAccess() && !next.ProviderUsage.ClaudeKeychainAccess() {
		r.ClearCredentials("", "claude")
	}
	if err := r.SetEnabled(ctx, next.ProviderUsage.Enabled); err != nil {
		return err
	}
	if previous.RefreshInterval != next.ProviderUsage.RefreshInterval {
		r.SetRefreshInterval(next.ProviderUsage.RefreshInterval)
	}
	if err := r.syncProfiles(ctx); err != nil {
		return err
	}
	return nil
}

// ApplyConsent runs only after the full settings transaction became active.
func (r *providerUsageRuntime) ApplyConsent(ctx context.Context, previous, next config.ProviderUsageConfig) {
	if profile, local := core.ProviderUsageConsentProfile(
		ctx,
	); local && !previous.ClaudeKeychainAccess() && next.ClaudeKeychainAccess() &&
		next.ProviderEnabled("claude") {
		// Wait zero admits a daemon-owned flight; caller cancellation cannot own it.
		_, err := r.Refresh(
			ctx,
			providerusage.RefreshRequest{
				ProfileID:   profile,
				ProviderIDs: []string{"claude"},
				Intent:      providerusage.IntentExplicit,
			},
		)
		if err != nil {
			r.ClearCredentials(profile, "claude")
		}
	}
}

func providerUsageConfigChanged(previous, next *config.Config) bool {
	return !reflect.DeepEqual(previous.ProviderUsage, next.ProviderUsage) ||
		!reflect.DeepEqual(previous.Providers, next.Providers)
}

// onAuthenticated forwards published native-auth proofs without holding the config reconciliation lock.
func (r *providerUsageRuntime) onAuthenticated(_ context.Context, target providerusage.Target) {
	r.authMu.RLock()
	recoverAuth, recheckSignIn := r.recoverAuth, r.recheckSignIn
	r.authMu.RUnlock()
	if target.AuthRouteFingerprint == "" {
		return
	}
	if recoverAuth != nil {
		recoverAuth(target.ProfileID, target.ProviderID, target.AuthRouteFingerprint)
	}
	if recheckSignIn != nil {
		recheckSignIn(target.ProviderID)
	}
}

// setSignInRecheck installs the native-CLI sign-in re-check that verified collections trigger.
func (r *providerUsageRuntime) setSignInRecheck(recheck func(string)) {
	r.authMu.Lock()
	r.recheckSignIn = recheck
	r.authMu.Unlock()
}

// setAuthRecovery reports whether the session manager exposes verified-auth recovery.
func (r *providerUsageRuntime) setAuthRecovery(sessions core.SessionManager) bool {
	recovery, ok := sessions.(core.ProviderAuthRecoverer)
	if !ok {
		return false
	}
	r.authMu.Lock()
	r.recoverAuth = recovery.ProviderAuthenticationReady
	r.authMu.Unlock()
	return true
}

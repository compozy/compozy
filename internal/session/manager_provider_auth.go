package session

import (
	"context"
	"errors"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

// RecoverProviderAuthentication reloads only idle auth-failed sessions on the verified account route.
func (m *Manager) RecoverProviderAuthentication(
	ctx context.Context,
	profileID, provider, commandFingerprint string,
) error {
	m.mu.RLock()
	targets := make([]*Session, 0, len(m.sessions))
	for _, target := range m.sessions {
		targets = append(targets, target)
	}
	m.mu.RUnlock()
	var result error
	for _, target := range targets {
		if ctx.Err() != nil || m.daemonShuttingDown() {
			return errors.Join(result, ctx.Err())
		}
		info := target.Info()
		if info.ProfileID != profileID || info.Provider != provider {
			continue
		}
		snapshot := target.runtimeBindingSnapshot()
		if providerCommandFingerprint(target.providerRoutingSnapshot().Command) != commandFingerprint ||
			len(snapshot.acceptedSessionEnv) > 0 {
			continue
		}
		if info.RuntimeRecovery != nil && info.RuntimeRecovery.Reason == store.SessionRuntimeRecoveryProviderAuth {
			select {
			case target.authRecoverySignal(store.SessionRuntimeRecoveryProviderAuth) <- struct{}{}:
			default:
			}
			continue
		}
		if info.Failure == nil || info.Failure.Kind != store.FailureProviderAuth {
			continue
		}
		proc, err := target.beginExclusivePromptSetup()
		if errors.Is(err, ErrPromptInProgress) || errors.Is(err, ErrSessionNotActive) {
			continue
		}
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		recoveryCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
		_, err = m.ensurePromptRuntime(recoveryCtx, target, nil, proc)
		cancel()
		target.finishPromptSetup()
		result = errors.Join(result, err)
	}
	return result
}

func (s *Session) authRecoverySignal(reason store.SessionRuntimeRecoveryReason) chan struct{} {
	if reason != store.SessionRuntimeRecoveryProviderAuth {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentPromptRecovery == nil {
		return nil
	}
	return s.currentPromptRecovery.authReady
}

type providerAuthRecoveryLifecycle struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	closing bool
	wg      sync.WaitGroup
}

// ProviderAuthenticationReady admits recovery without holding a probe or collection flight open.
func (m *Manager) ProviderAuthenticationReady(profileID, provider, fingerprint string) bool {
	lifecycle := &m.providerAuthRecovery
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.closing || m.daemonShuttingDown() {
		return false
	}
	if lifecycle.ctx == nil {
		lifecycle.ctx, lifecycle.cancel = context.WithCancel(m.fallbackLifecycleContext())
	}
	lifecycle.wg.Go(func() {
		if err := m.RecoverProviderAuthentication(lifecycle.ctx, profileID, provider, fingerprint); err != nil {
			m.logger.WarnContext(
				lifecycle.ctx,
				"session: provider authentication recovery deferred",
				"provider",
				provider,
				"error",
				err,
			)
		}
	})
	return true
}

func (m *Manager) stopProviderAuthRecovery() {
	lifecycle := &m.providerAuthRecovery
	lifecycle.mu.Lock()
	lifecycle.closing = true
	if lifecycle.cancel != nil {
		lifecycle.cancel()
	}
	lifecycle.mu.Unlock()
	lifecycle.wg.Wait()
}

func (m *Manager) releaseAuthFailedRuntime(
	ctx context.Context,
	target *Session,
	snapshot *runtimeBindingSnapshot,
) error {
	if failure := target.Info().Failure; failure != nil && failure.Kind == store.FailureProviderAuth &&
		snapshot.process != nil {
		if err := m.releaseBoundRuntime(ctx, target); err != nil {
			return err
		}
		*snapshot = target.runtimeBindingSnapshot()
	}
	return nil
}

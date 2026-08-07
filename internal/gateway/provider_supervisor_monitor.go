package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

func (s *ProviderSupervisor) monitor(ctx context.Context, session *supervisedProvider) {
	var notification *providerDegradationNotification
	defer func() {
		close(session.done)
		if notification != nil {
			s.notifyDegradation(*notification)
		}
	}()
	ticker := time.NewTicker(s.healthEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.checkHealth(ctx, session); err != nil {
				notification = s.prepareDegradation(session, err)
				return
			}
		}
	}
}

type providerDegradationNotification struct {
	activation ProviderActivation
	cause      error
	handler    ProviderDegradationHandler
}

func (s *ProviderSupervisor) checkHealth(parent context.Context, session *supervisedProvider) error {
	ctx, cancel := context.WithTimeout(parent, s.healthTimeout)
	defer cancel()
	reachability, err := session.source.Status(ctx, session.activation.Tier)
	if err != nil {
		return fmt.Errorf("%w: health call failed", ErrProviderDegraded)
	}
	if err := validateProviderReachability(session.activation.Tier, reachability); err != nil {
		return errors.Join(ErrProviderDegraded, err)
	}
	if !sameEndpointSet(session.reachable.Endpoints, reachability.Endpoints) {
		return fmt.Errorf("%w: endpoint set changed without verification", ErrProviderDegraded)
	}
	return nil
}

func (s *ProviderSupervisor) prepareDegradation(
	session *supervisedProvider,
	cause error,
) *providerDegradationNotification {
	s.mu.Lock()
	if current := s.sessions[session.activation.Tier]; current != session {
		s.mu.Unlock()
		return nil
	}
	delete(s.sessions, session.activation.Tier)
	handler := s.onDegrade
	s.mu.Unlock()
	session.cleanup()
	teardownCtx, teardownCancel := context.WithTimeout(context.Background(), s.teardownAfter)
	teardownErr := s.teardownSource(teardownCtx, session.source, session.activation.Tier)
	teardownCancel()
	joined := errors.Join(cause, teardownErr)
	return &providerDegradationNotification{activation: session.activation, cause: joined, handler: handler}
}

func (s *ProviderSupervisor) notifyDegradation(notification providerDegradationNotification) {
	if notification.handler != nil {
		handlerCtx, handlerCancel := context.WithTimeout(context.Background(), s.teardownAfter)
		defer handlerCancel()
		if err := notification.handler(handlerCtx, notification.activation, notification.cause); err != nil {
			s.logger.Error(
				"gateway provider degradation handler failed",
				"tier", notification.activation.Tier,
				"error", diagnostics.RedactAndBound(err.Error(), maxPersistedGatewayDiagnosticBytes),
			)
		}
		return
	}
	s.logger.Error(
		"gateway provider degraded",
		"tier", notification.activation.Tier,
		"error", diagnostics.RedactAndBound(notification.cause.Error(), maxPersistedGatewayDiagnosticBytes),
	)
}

func (s *ProviderSupervisor) teardownSource(
	ctx context.Context,
	source ConnectivitySource,
	tier Tier,
) error {
	if source == nil {
		return nil
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.teardownAfter)
	defer cancel()
	deadline, ok := stopCtx.Deadline()
	if !ok {
		return errors.New("gateway: provider teardown deadline is unavailable")
	}
	if err := source.Teardown(stopCtx, tier, deadline); err != nil {
		return fmt.Errorf("gateway: teardown connectivity provider: %w", err)
	}
	return nil
}

func (s *ProviderSupervisor) stopMonitor(ctx context.Context, session *supervisedProvider) error {
	if session == nil || session.cancel == nil || session.done == nil {
		return nil
	}
	session.cancel()
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.healthTimeout)
	defer cancel()
	select {
	case <-session.done:
		return nil
	case <-waitCtx.Done():
		return fmt.Errorf("gateway: wait for provider health monitor: %w", waitCtx.Err())
	}
}

func sameEndpointSet(left []AdvertisedEndpoint, right []AdvertisedEndpoint) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[AdvertisedEndpoint]int, len(left))
	for _, endpoint := range left {
		seen[endpoint]++
	}
	for _, endpoint := range right {
		count := seen[endpoint]
		if count == 0 {
			return false
		}
		seen[endpoint] = count - 1
	}
	return true
}

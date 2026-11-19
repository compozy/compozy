package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/vault"
)

var ErrRefreshBackoff = errors.New("mcp auth: refresh retry is backed off")

var errRefreshRegistrationInvalid = errors.New("mcp auth: stored registration is no longer valid; run login again")

const maxEarlyRenewalWindow = 30 * time.Second

func tokenRenewalDue(token TokenRecord, now time.Time) bool {
	if token.ExpiresAt.IsZero() {
		return false
	}
	issuedAt := token.UpdatedAt
	if issuedAt.IsZero() {
		issuedAt = token.ObtainedAt
	}
	window := maxEarlyRenewalWindow
	if lifetime := token.ExpiresAt.Sub(issuedAt); !issuedAt.IsZero() && lifetime > 0 {
		window = min(window, lifetime/10)
	}
	return !now.Before(token.ExpiresAt.Add(-window))
}

func registrationSecretUnavailable(err error) bool {
	return errors.Is(err, vault.ErrSecretNotFound) || errors.Is(err, vault.ErrMissingSecret) ||
		errors.Is(err, vault.ErrUnsupportedSecretRef)
}

type tokenEndpointError struct {
	status int
	cause  string
}

func (e *tokenEndpointError) Error() string {
	return fmt.Sprintf("mcp auth: token endpoint rejected request with HTTP %d (%s)", e.status, e.cause)
}

func newTokenEndpointError(status int, code string) error {
	cause := "token_endpoint"
	switch code {
	case "":
		cause = "invalid_response"
	case "invalid_grant", "revoked", "invalid_client", "unauthorized_client", "invalid_request", "invalid_scope",
		"unsupported_grant_type", "temporarily_unavailable", "server_error":
		cause = code
	}
	return &tokenEndpointError{status: status, cause: cause}
}

func definitiveRefreshFailure(err error) bool {
	endpoint, ok := errors.AsType[*tokenEndpointError](err)
	if !ok || endpoint.status >= 500 || endpoint.status == 429 {
		return false
	}
	switch endpoint.cause {
	case "invalid_grant", "revoked", "invalid_client", "unauthorized_client":
		return true
	default:
		return false
	}
}

func refreshFailureClass(err error) string {
	if endpoint, ok := errors.AsType[*tokenEndpointError](err); ok {
		return endpoint.cause
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrTokenMutationStale):
		return "superseded"
	case errors.Is(err, ErrTokenBindingInvalid):
		return "binding_invalid"
	default:
		if _, ok := errors.AsType[net.Error](err); ok {
			return "network"
		}
		return "refresh_unavailable"
	}
}

type refreshFailure struct{ err error }

func (e *refreshFailure) Error() string {
	return "mcp auth: refresh failed (" + refreshFailureClass(e.err) + ")"
}
func (e *refreshFailure) Unwrap() error { return e.err }

// WithRefreshBackoff configures the retry interval and cap; nonpositive values keep the defaults.
func WithRefreshBackoff(base, maximum time.Duration) ServiceOption {
	return func(s *Service) {
		if base > 0 && maximum >= base {
			s.refreshBackoff, s.refreshBackoffMax = base, maximum
		}
	}
}

// WithRefreshTimeout bounds a started renewal; nonpositive values keep the default.
func WithRefreshTimeout(timeout time.Duration) ServiceOption {
	return func(s *Service) {
		if timeout > 0 {
			s.refreshTimeout = timeout
		}
	}
}

// WithLogger supplies secret-free refresh diagnostics.
func WithLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func (s *Service) storedTokenStatus(
	ctx context.Context, cfg ServerConfig, token TokenRecord, registration *ClientRegistration,
) Status {
	now := s.now()
	status := statusFromToken(cfg, &token, now)
	if status.Refreshable && cfg.registrationStrategy() == RegistrationAuto {
		if registration == nil || !s.hostedRegistrationRefreshable(cfg, *registration, token.DefinitionFingerprint) {
			return localRefreshRepairStatus(status)
		}
		if tokenRenewalDue(token, now) && registration.TokenEndpointAuthMethod != tokenEndpointAuthMethodNone {
			secret, err := s.resolveRegistrationSecret(ctx, registration.ClientSecretRef)
			if registrationSecretUnavailable(err) || (err == nil && strings.TrimSpace(secret) == "") {
				return localRefreshRepairStatus(status)
			}
		}
	}
	status.renewalDue = status.Status == StatusAuthenticated && status.Refreshable && tokenRenewalDue(token, now)
	return status
}

func localRefreshRepairStatus(status Status) Status {
	status.Refreshable = false
	status.renewalDue = false
	status.Diagnostic = "stored registration is no longer valid; run login again"
	return status
}

// Refresh serializes renewals across services sharing the credential mutation coordinator.
func (s *Service) Refresh(ctx context.Context, cfg ServerConfig) (Status, error) {
	if err := cfg.Validate(); err != nil {
		return Status{}, err
	}
	observed, err := s.store.GetMCPAuthToken(ctx, cfg.Target)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return s.Status(ctx, cfg)
		}
		return Status{}, err
	}
	target, err := s.generation.forTarget(cfg.Target)
	if err != nil {
		return Status{}, err
	}
	select {
	case target.refreshGate <- struct{}{}:
		defer func() { <-target.refreshGate }()
	case <-ctx.Done():
		return Status{}, ctx.Err()
	}
	return s.refreshWithBackoff(ctx, cfg, observed, target)
}

func (s *Service) refreshWithBackoff(
	ctx context.Context, cfg ServerConfig, observed TokenRecord, target *targetMutationGeneration,
) (Status, error) {
	// Once renewal starts, persist a rotated refresh token even if the requesting discovery times out.
	refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.refreshTimeout)
	defer cancel()
	generation, err := s.generation.Snapshot(cfg.Target)
	if err != nil {
		return Status{}, err
	}
	if target.refreshGeneration != generation {
		target.refreshGeneration = generation
		target.refreshAttempts, target.refreshRetryAt = 0, time.Time{}
	}
	if s.now().Before(target.refreshRetryAt) {
		if target.refreshCanUseAccessToken {
			return s.keepValidAccessToken(ctx, cfg, observed, ErrRefreshBackoff)
		}
		return Status{}, ErrRefreshBackoff
	}
	target.refreshAttempts++
	status, err := s.refresh(refreshCtx, cfg, observed, generation)
	if err == nil || (status.TokenPresent && errors.Is(err, ErrTokenBindingInvalid)) {
		target.refreshAttempts, target.refreshRetryAt = 0, time.Time{}
		return status, err
	}
	s.logger.WarnContext(ctx, "mcp.auth.refresh_failed",
		"server", cfg.Target.ServerName, "owner", cfg.Target.Normalize().Owner,
		"scope", string(cfg.Target.Scope), "workspace_id", cfg.Target.WorkspaceID,
		"cause_class", refreshFailureClass(err), "attempt", target.refreshAttempts)
	if definitiveRefreshFailure(err) && status.Status == StatusNeedsLogin {
		return status, nil
	}
	if !errors.Is(err, ErrTokenMutationStale) {
		delay := s.refreshBackoff
		for attempt := 1; attempt < target.refreshAttempts && delay < s.refreshBackoffMax; attempt++ {
			delay = min(delay*2, s.refreshBackoffMax)
		}
		// Jitter prevents independent servers from synchronizing retries after an outage.
		delay += time.Duration(rand.Int64N(max(int64(delay/5), 1))) // #nosec G404 -- retry jitter has no security role.
		delay = min(delay, s.refreshBackoffMax)
		target.refreshRetryAt = s.now().Add(delay)
	}
	target.refreshCanUseAccessToken = !definitiveRefreshFailure(err) && !errors.Is(err, ErrTokenBindingInvalid) &&
		!errors.Is(err, ErrTokenMutationStale)
	if target.refreshCanUseAccessToken {
		return s.keepValidAccessToken(ctx, cfg, observed, &refreshFailure{err: err})
	}
	return Status{}, &refreshFailure{err: err}
}

func (s *Service) keepValidAccessToken(
	ctx context.Context, cfg ServerConfig, observed TokenRecord, refreshErr error,
) (Status, error) {
	if observed.ExpiresAt.After(s.now()) {
		status, err := s.Status(ctx, cfg)
		if err == nil && status.Status == StatusAuthenticated {
			return status, nil
		}
	}
	return Status{}, refreshErr
}

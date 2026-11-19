package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Exchange validates the OAuth callback and stores the token response.
func (s *Service) Exchange(ctx context.Context, state *LoginState, callbackURL string) (Status, error) {
	if state == nil {
		return Status{}, errors.New("mcp auth: login state is required")
	}
	generation, err := s.generation.Advance(state.Config.Target)
	if err != nil {
		return Status{}, err
	}
	token, err := s.exchangeToken(ctx, state, callbackURL)
	if err != nil {
		return Status{}, err
	}
	return s.persistReplacementTokenAtGeneration(ctx, state.Config, token, generation)
}

func (s *Service) persistReplacementTokenAtGeneration(
	ctx context.Context,
	cfg ServerConfig,
	token TokenRecord,
	generation uint64,
) (Status, error) {
	committed, err := s.generation.CommitAndAdvanceIfCurrent(cfg.Target, generation, func() error {
		return s.store.SaveMCPAuthToken(ctx, token)
	})
	if err != nil {
		return Status{}, fmt.Errorf("mcp auth: persist replacement token for %q: %w", cfg.Target.ServerName, err)
	}
	if !committed {
		return Status{}, ErrTokenMutationStale
	}
	return statusFromToken(cfg, &token, s.now()), nil
}

func (s *Service) exchangeToken(ctx context.Context, state *LoginState, callbackURL string) (TokenRecord, error) {
	code, err := authorizationCodeFromCallbackMetadata(callbackURL, state.State, state.Metadata)
	if err != nil {
		return TokenRecord{}, fmt.Errorf("%w: %v", ErrInvalidExchange, err)
	}
	if err := validateVerifier(state.Verifier); err != nil {
		return TokenRecord{}, err
	}

	token, err := s.exchangeCode(ctx, state, code)
	if err != nil {
		return TokenRecord{}, err
	}
	return token, nil
}

func (s *Service) persistTokenAtGeneration(
	ctx context.Context,
	cfg ServerConfig,
	token TokenRecord,
	generation uint64,
	replacesAccess bool,
) (Status, error) {
	committed, err := s.generation.commitIfCurrent(cfg.Target, generation, replacesAccess, func() error {
		return s.store.SaveMCPAuthToken(ctx, token)
	})
	if err != nil {
		return Status{}, fmt.Errorf("mcp auth: persist token for %q: %w", cfg.Target.ServerName, err)
	}
	if !committed {
		return Status{}, ErrTokenMutationStale
	}
	return statusFromToken(cfg, &token, s.now()), nil
}

func (s *Service) refresh(
	ctx context.Context,
	cfg ServerConfig,
	observed TokenRecord,
	generation uint64,
) (Status, error) {
	current, err := s.store.GetMCPAuthToken(ctx, cfg.Target)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return statusFromToken(cfg, nil, s.now()), nil
		}
		return Status{}, err
	}
	matches, registration, err := s.tokenBinding(ctx, current, cfg)
	if err != nil {
		return Status{}, err
	}
	if !matches {
		return statusFromTokenWithDiagnostic(
			cfg,
			nil,
			s.now(),
			"stored token belongs to a different server definition; run login again",
		), nil
	}
	if current.AccessToken != observed.AccessToken || current.RefreshToken != observed.RefreshToken ||
		!current.UpdatedAt.Equal(observed.UpdatedAt) || !current.ExpiresAt.Equal(observed.ExpiresAt) {
		return s.Status(ctx, cfg)
	}
	status, err := s.statusWithRemoteRejection(cfg, s.storedTokenStatus(ctx, cfg, current, registration))
	if err != nil || status.Status == StatusNeedsLogin {
		return status, err
	}
	if !current.ExpiresAt.IsZero() && status.Status == StatusAuthenticated && !status.RenewalDue() {
		return status, nil
	}
	if strings.TrimSpace(current.RefreshToken) == "" {
		return statusFromTokenWithDiagnostic(
			cfg,
			&current,
			s.now(),
			"refresh token is unavailable; run login again",
		), nil
	}
	if !status.Refreshable {
		return status, nil
	}
	return s.renewToken(ctx, cfg, current, generation)
}

func (s *Service) renewToken(
	ctx context.Context,
	cfg ServerConfig,
	current TokenRecord,
	generation uint64,
) (Status, error) {
	refreshConfig, metadata, err := s.refreshConfiguration(ctx, cfg)
	if errors.Is(err, errRefreshRegistrationInvalid) {
		return localRefreshRepairStatus(statusFromToken(cfg, &current, s.now())), nil
	}
	// Hosted discovery can return a validated replacement issuer with a binding error.
	if err != nil && metadata.Issuer == "" {
		return Status{}, err
	}
	if !issuersEqual(current.Issuer, metadata.Issuer) {
		// Keep access material bound to its original issuer, but stop advertising an unusable renewal path.
		current.RefreshToken = ""
		if _, err := s.persistTokenAtGeneration(ctx, cfg, current, generation, false); err != nil {
			return Status{}, err
		}
		return statusFromTokenWithDiagnostic(cfg, &current, s.now(), "OAuth issuer changed; run login again"),
			ErrTokenBindingInvalid
	}
	if err != nil {
		return Status{}, err
	}
	if strings.TrimSpace(refreshConfig.ClientID) != strings.TrimSpace(current.ClientID) {
		return Status{}, ErrTokenBindingInvalid
	}
	refreshed, err := s.refreshToken(ctx, refreshConfig, metadata, current)
	if err != nil {
		if definitiveRefreshFailure(err) {
			return s.rejectRefresh(ctx, cfg, generation, err)
		}
		return Status{}, err
	}
	return s.persistTokenAtGeneration(ctx, cfg, refreshed, generation, true)
}

func (s *Service) rejectRefresh(ctx context.Context, cfg ServerConfig, generation uint64, cause error) (Status, error) {
	committed, err := s.generation.commitIfCurrent(cfg.Target, generation, true, func() error {
		return s.store.DeleteMCPAuthToken(ctx, cfg.Target)
	})
	if err != nil {
		return Status{}, errors.Join(cause, err)
	}
	if !committed {
		return Status{}, ErrTokenMutationStale
	}
	return statusFromTokenWithDiagnostic(cfg, nil, s.now(), "refresh was rejected; run login again"), cause
}

// Status returns redacted durable auth state for one server.
func (s *Service) Status(ctx context.Context, cfg ServerConfig) (Status, error) {
	cfg.Target = cfg.Target.Normalize()
	if err := cfg.Target.Validate(); err != nil {
		return Status{}, err
	}
	if strings.TrimSpace(cfg.Type) == "" {
		return Status{
			Owner:       cfg.Target.Owner,
			ServerName:  cfg.Target.ServerName,
			Scope:       cfg.Target.Scope,
			WorkspaceID: cfg.Target.WorkspaceID,
			Status:      StatusUnconfigured,
			RemoteURL:   cfg.publicRemoteURL(),
		}, nil
	}
	if err := cfg.Validate(); err != nil {
		return Status{}, err
	}
	status, _, err := s.generation.ReadAtRevision(ctx, cfg.Target, func() (Status, error) {
		return s.readStoredStatus(ctx, cfg)
	})
	return status, err
}

func (s *Service) readStoredStatus(ctx context.Context, cfg ServerConfig) (Status, error) {
	token, err := s.store.GetMCPAuthToken(ctx, cfg.Target)
	if err != nil {
		if errors.Is(err, ErrTokenResolution) {
			return s.unresolvedTokenStatus(cfg, err)
		}
		if errors.Is(err, ErrTokenNotFound) {
			if cfg.Implicit {
				cfg.Type = ""
			}
			return statusFromToken(cfg, nil, s.now()), nil
		}
		return Status{}, err
	}
	matches, registration, err := s.tokenBinding(ctx, token, cfg)
	if err != nil {
		return s.unresolvedTokenStatus(cfg, err)
	}
	if !matches {
		const diagnostic = "stored token belongs to a different server definition; run login again"
		if !cfg.Implicit {
			return statusFromTokenWithDiagnostic(cfg, nil, s.now(), diagnostic), nil
		}
		if !tokenMatchesFingerprint(token, cfg) {
			cfg.Type = ""
			return statusFromToken(cfg, nil, s.now()), nil
		}
		return s.invalidTokenStatus(cfg, diagnostic), nil
	}
	return s.statusWithRemoteRejection(cfg, s.storedTokenStatus(ctx, cfg, token, registration))
}

func (s *Service) statusWithRemoteRejection(cfg ServerConfig, status Status) (Status, error) {
	rejected, err := s.generation.remotelyRejected(cfg.Target)
	if err != nil {
		return Status{}, err
	}
	if !rejected {
		return status, nil
	}
	if !status.Refreshable {
		return statusFromTokenWithDiagnostic(cfg, nil, s.now(), ErrTokenRejected.Error()+"; sign in again"), nil
	}
	status.Status = StatusExpired
	status.renewalDue = false
	status.Diagnostic = ErrTokenRejected.Error() + "; refresh required"
	return status, nil
}

func (s *Service) invalidTokenStatus(cfg ServerConfig, diagnostic string) Status {
	status := statusFromTokenWithDiagnostic(cfg, nil, s.now(), diagnostic)
	status.Status = StatusInvalid
	return status
}

func (s *Service) unresolvedTokenStatus(cfg ServerConfig, err error) (Status, error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Status{}, err
	}
	return s.invalidTokenStatus(cfg, ErrTokenResolution.Error()+"; check the credential store"), nil
}

// AuthorizationToken returns bearer material only after full definition, registration, and scope binding validation.
func (s *Service) AuthorizationToken(ctx context.Context, cfg ServerConfig) (TokenRecord, error) {
	if err := cfg.Validate(); err != nil {
		return TokenRecord{}, err
	}
	token, err := s.store.GetMCPAuthToken(ctx, cfg.Target)
	if err != nil {
		return TokenRecord{}, err
	}
	matches, err := s.tokenMatchesServerDefinition(ctx, token, cfg)
	if err != nil {
		return TokenRecord{}, errors.Join(ErrTokenResolution, err)
	}
	if !matches || strings.TrimSpace(token.AccessToken) == "" {
		if cfg.Implicit && !tokenMatchesFingerprint(token, cfg) {
			return TokenRecord{}, ErrTokenDefinitionChanged
		}
		return TokenRecord{}, ErrTokenBindingInvalid
	}
	rejected, err := s.generation.remotelyRejected(cfg.Target)
	if err != nil {
		return TokenRecord{}, err
	}
	if rejected {
		return TokenRecord{}, ErrTokenRejected
	}
	if !token.ExpiresAt.IsZero() && !token.ExpiresAt.After(s.now().UTC()) {
		return TokenRecord{}, ErrTokenExpired
	}
	return token, nil
}

// Logout revokes the refresh token when revocation metadata is configured,
// then deletes local durable token state.
func (s *Service) Logout(ctx context.Context, cfg ServerConfig) (Status, error) {
	if err := cfg.Validate(); err != nil {
		return Status{}, err
	}
	token, err := s.store.GetMCPAuthToken(ctx, cfg.Target)
	if err != nil && !errors.Is(err, ErrTokenNotFound) {
		return Status{}, err
	}
	var remoteErr error
	definitionMatches := false
	if err == nil {
		var bindingErr error
		definitionMatches, bindingErr = s.tokenMatchesServerDefinition(ctx, token, cfg)
		if bindingErr != nil {
			remoteErr = fmt.Errorf("mcp auth: validate revocation binding: %w", bindingErr)
		}
	}
	if definitionMatches {
		metadata, metaErr := s.metadataForConfig(ctx, cfg)
		switch {
		case metaErr != nil:
			remoteErr = fmt.Errorf("mcp auth: discover revocation metadata: %w", metaErr)
		case !issuersEqual(token.Issuer, metadata.Issuer):
			remoteErr = ErrTokenBindingInvalid
		case strings.TrimSpace(metadata.RevocationEndpoint) != "":
			if revokeErr := s.revoke(ctx, cfg, metadata, token); revokeErr != nil {
				remoteErr = fmt.Errorf("mcp auth: revoke remote token: %w", revokeErr)
			}
		}
	}
	if err := s.deleteAuthorizationState(ctx, cfg.Target); err != nil {
		return Status{}, err
	}
	if remoteErr != nil {
		return statusFromTokenWithDiagnostic(
			cfg,
			nil,
			s.now(),
			"local logout completed; remote revocation failed: "+remoteErr.Error(),
		), nil
	}
	if err == nil && !definitionMatches {
		return statusFromTokenWithDiagnostic(
			cfg,
			nil,
			s.now(),
			"local logout completed; stored token belonged to a different server definition",
		), nil
	}
	return statusFromToken(cfg, nil, s.now()), nil
}

// DeleteAuthorizationState atomically removes token, registration, and Vault state for one target.
func (s *Service) DeleteAuthorizationState(ctx context.Context, target Target) error {
	if err := target.Validate(); err != nil {
		return err
	}
	return s.deleteAuthorizationState(ctx, target)
}

func (s *Service) deleteAuthorizationState(ctx context.Context, target Target) error {
	if _, err := s.generation.Advance(target); err != nil {
		return err
	}
	if err := s.store.DeleteMCPAuthorizationState(ctx, target); err != nil {
		return fmt.Errorf("mcp auth: delete authorization state: %w", err)
	}
	return nil
}

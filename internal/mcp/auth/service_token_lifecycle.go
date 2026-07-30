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
) (Status, error) {
	committed, err := s.generation.CommitIfCurrent(cfg.Target, generation, func() error {
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

// Refresh refreshes a persisted token and updates durable storage.
func (s *Service) Refresh(ctx context.Context, cfg ServerConfig) (Status, error) {
	if err := cfg.Validate(); err != nil {
		return Status{}, err
	}
	generation, err := s.generation.Snapshot(cfg.Target)
	if err != nil {
		return Status{}, err
	}
	current, err := s.store.GetMCPAuthToken(ctx, cfg.Target)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return statusFromToken(cfg, nil, s.now()), nil
		}
		return Status{}, err
	}
	matches, err := s.tokenMatchesServerDefinition(ctx, current, cfg)
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
	if strings.TrimSpace(current.RefreshToken) == "" {
		return statusFromTokenWithDiagnostic(
			cfg,
			&current,
			s.now(),
			"refresh token is unavailable; run login again",
		), nil
	}

	refreshConfig, metadata, err := s.refreshConfiguration(ctx, cfg)
	if err != nil {
		return Status{}, err
	}
	refreshed, err := s.refreshToken(ctx, refreshConfig, metadata, current)
	if err != nil {
		return Status{}, err
	}
	return s.persistTokenAtGeneration(ctx, cfg, refreshed, generation)
}

// Status returns redacted durable auth state for one server.
func (s *Service) Status(ctx context.Context, cfg ServerConfig) (Status, error) {
	cfg.Target = cfg.Target.Normalize()
	if err := cfg.Target.Validate(); err != nil {
		return Status{}, err
	}
	if strings.TrimSpace(cfg.Type) == "" {
		return Status{
			ServerName:  cfg.Target.ServerName,
			Scope:       cfg.Target.Scope,
			WorkspaceID: cfg.Target.WorkspaceID,
			Status:      StatusUnconfigured,
			RemoteURL:   strings.TrimSpace(cfg.RemoteURL),
		}, nil
	}
	if err := cfg.Validate(); err != nil {
		return Status{}, err
	}
	token, err := s.store.GetMCPAuthToken(ctx, cfg.Target)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return statusFromToken(cfg, nil, s.now()), nil
		}
		return Status{}, err
	}
	matches, err := s.tokenMatchesServerDefinition(ctx, token, cfg)
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
	return statusFromToken(cfg, &token, s.now()), nil
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
		return TokenRecord{}, err
	}
	if !matches || strings.TrimSpace(token.AccessToken) == "" {
		return TokenRecord{}, ErrTokenBindingInvalid
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
		definitionMatches, err = s.tokenMatchesServerDefinition(ctx, token, cfg)
		if err != nil {
			return Status{}, err
		}
	}
	if definitionMatches {
		metadata, metaErr := s.metadataForConfig(ctx, cfg)
		if metaErr != nil {
			remoteErr = fmt.Errorf("mcp auth: discover revocation metadata: %w", metaErr)
		} else if strings.TrimSpace(metadata.RevocationEndpoint) != "" {
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

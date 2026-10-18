package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

type appTokenService struct {
	store     AppTokenStore
	mutations sync.Mutex
	mu        sync.RWMutex
	origins   map[string]struct{}
}

func WithAppTokenStore(store AppTokenStore) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		if store == nil {
			return errAppTokenStoreRequired
		}
		config.appTokens = store
		return nil
	}
}

func (s *DeviceService) LoadAppTokenOrigins(ctx context.Context) error {
	if s.appTokens.store == nil {
		return errAppTokenStoreRequired
	}
	s.appTokens.mutations.Lock()
	defer s.appTokens.mutations.Unlock()
	origins, err := s.appTokens.store.ListActiveAppTokenOrigins(ctx)
	if err != nil {
		return fmt.Errorf("gateway: load app token origins: %w", err)
	}
	s.appTokens.setOrigins(origins)
	return nil
}

func (s *DeviceService) AppTokenOriginAllowed(origin string) bool {
	s.appTokens.mu.RLock()
	defer s.appTokens.mu.RUnlock()
	_, allowed := s.appTokens.origins[origin]
	return allowed
}

func (s *appTokenService) setOrigins(origins []string) {
	union := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		union[origin] = struct{}{}
	}
	s.mu.Lock()
	s.origins = union
	s.mu.Unlock()
}

func (s *DeviceService) CreateAppToken(ctx context.Context, name string, origins []string) (AppTokenIssued, error) {
	name, err := normalizeAppTokenName(name)
	if err != nil {
		return AppTokenIssued{}, err
	}
	origins, err = normalizeAppTokenOrigins(origins)
	if err != nil {
		return AppTokenIssued{}, err
	}
	if s.appTokens.store == nil {
		return AppTokenIssued{}, errAppTokenStoreRequired
	}
	s.appTokens.mutations.Lock()
	defer s.appTokens.mutations.Unlock()
	secret, err := generateOpaqueSecret(s.entropy, appTokenPrefix)
	if err != nil {
		return AppTokenIssued{}, err
	}
	random := make([]byte, 16)
	if _, err := io.ReadFull(s.entropy, random); err != nil {
		return AppTokenIssued{}, fmt.Errorf("gateway: generate app token ID: %w", err)
	}
	token := AppToken{
		Session: DeviceSession{
			ID: "apt_" + base64.RawURLEncoding.EncodeToString(random), Name: name,
			ActorKind: ActorKindAppToken, PairingOrigin: string(PairingSourceLocal), CreatedAt: s.now(),
		},
		Origins: origins,
	}
	union, err := s.appTokens.store.CreateAppToken(ctx, StoredAppToken{
		Token: token, TokenHash: hashCredential(secret),
	})
	if err != nil {
		return AppTokenIssued{}, fmt.Errorf("gateway: create app token: %w", err)
	}
	s.appTokens.setOrigins(union)
	slog.InfoContext(ctx, "app_token.created", "app_token_id", token.Session.ID, "name", token.Session.Name)
	return AppTokenIssued{Token: token, Secret: secret}, nil
}

func (s *DeviceService) ListAppTokens(ctx context.Context, state AppTokenState) ([]AppToken, error) {
	if state == "" {
		state = AppTokenStateActive
	}
	if state != AppTokenStateActive && state != AppTokenStateRevoked && state != AppTokenStateAll {
		return nil, ErrAppTokenStateInvalid
	}
	if s.appTokens.store == nil {
		return nil, errAppTokenStoreRequired
	}
	tokens, err := s.appTokens.store.ListAppTokens(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("gateway: list app tokens: %w", err)
	}
	return tokens, nil
}

func (s *DeviceService) UpdateAppToken(ctx context.Context, id string, patch AppTokenPatch) (AppToken, error) {
	if patch.Name != nil {
		name, err := normalizeAppTokenName(*patch.Name)
		if err != nil {
			return AppToken{}, err
		}
		patch.Name = &name
	}
	if patch.Origins != nil {
		origins, err := normalizeAppTokenOrigins(*patch.Origins)
		if err != nil {
			return AppToken{}, err
		}
		patch.Origins = &origins
	}
	if s.appTokens.store == nil {
		return AppToken{}, errAppTokenStoreRequired
	}
	s.appTokens.mutations.Lock()
	defer s.appTokens.mutations.Unlock()
	token, union, err := s.appTokens.store.UpdateAppToken(ctx, id, patch, s.now())
	if err != nil {
		return AppToken{}, fmt.Errorf("gateway: update app token: %w", err)
	}
	s.appTokens.setOrigins(union)
	slog.InfoContext(ctx, "app_token.updated", "app_token_id", id, "name", token.Session.Name)
	return token, nil
}

func (s *DeviceService) RevokeAppToken(ctx context.Context, id string) (AppTokenRevokeResult, error) {
	if s.appTokens.store == nil {
		return AppTokenRevokeResult{}, errAppTokenStoreRequired
	}
	var token AppToken
	result, err := s.revokeSession(ctx, id, func(operationCtx context.Context) (DeviceSession, bool, error) {
		s.appTokens.mutations.Lock()
		defer s.appTokens.mutations.Unlock()
		revoked, changed, union, revokeErr := s.appTokens.store.RevokeAppToken(operationCtx, id, s.now())
		if revokeErr != nil {
			return DeviceSession{}, false, revokeErr
		}
		token = revoked
		s.appTokens.setOrigins(union)
		return token.Session, changed, nil
	})
	if err == nil {
		slog.InfoContext(ctx, "app_token.revoked",
			"app_token_id", id, "name", token.Session.Name, "canceled", result.Canceled,
		)
	}
	return AppTokenRevokeResult{Token: token, Changed: result.Changed, Canceled: result.Canceled}, err
}

func (s *DeviceService) AuthenticateAppToken(ctx context.Context, secret string) (AppToken, error) {
	if !validOpaqueSecret(secret, appTokenPrefix) {
		constantTimeCredentialMatch(secret, "")
		return AppToken{}, ErrAppTokenUnauthenticated
	}
	if s.appTokens.store == nil {
		return AppToken{}, errAppTokenStoreRequired
	}
	stored, err := s.appTokens.store.FindAppTokenByTokenHash(ctx, hashCredential(secret))
	if err != nil {
		if errors.Is(err, ErrAppTokenNotFound) {
			constantTimeCredentialMatch(secret, "")
			return AppToken{}, ErrAppTokenUnauthenticated
		}
		return AppToken{}, fmt.Errorf("gateway: find app token credential: %w", err)
	}
	if !constantTimeCredentialMatch(secret, stored.TokenHash) || stored.Token.Session.ActorKind != ActorKindAppToken {
		return AppToken{}, ErrAppTokenUnauthenticated
	}
	if !stored.Token.Session.RevokedAt.IsZero() {
		return AppToken{}, ErrAppTokenRevoked
	}
	return stored.Token, nil
}

func (s *DeviceService) TouchAppTokenLastSeen(ctx context.Context, id string, epoch uint64) error {
	if s.appTokens.store == nil {
		return errAppTokenStoreRequired
	}
	if err := s.appTokens.store.TouchAppTokenLastSeen(ctx, id, epoch, s.now()); err != nil {
		return fmt.Errorf("gateway: touch app token last use: %w", err)
	}
	return nil
}

var (
	_ AppTokenManager       = (*DeviceService)(nil)
	_ AppTokenAuthenticator = (*DeviceService)(nil)
)

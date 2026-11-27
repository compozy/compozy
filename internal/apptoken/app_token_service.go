package apptoken

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/remotehost"
)

type Service struct {
	store       Store
	gate        *remotehost.MutationGate
	connections remotehost.ConnectionRegistry
	now         func() time.Time
	mutations   sync.Mutex
	mu          sync.RWMutex
	origins     map[string]struct{}
}

func NewService(
	store Store,
	gate *remotehost.MutationGate,
	connections remotehost.ConnectionRegistry,
	now func() time.Time,
) (*Service, error) {
	if store == nil {
		return nil, errAppTokenStoreRequired
	}
	if gate == nil || connections == nil {
		return nil, errors.New("app token service requires mutation and connection tracking")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{store: store, gate: gate, connections: connections, now: now}, nil
}

func (s *Service) LoadAppTokenOrigins(ctx context.Context) error {
	s.mutations.Lock()
	defer s.mutations.Unlock()
	origins, err := s.store.ListActiveAppTokenOrigins(ctx)
	if err != nil {
		return fmt.Errorf("app token: load app token origins: %w", err)
	}
	s.setOrigins(origins)
	return nil
}

func (s *Service) AppTokenOriginAllowed(origin string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, allowed := s.origins[origin]
	return allowed
}

func (s *Service) setOrigins(origins []string) {
	union := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		union[origin] = struct{}{}
	}
	s.mu.Lock()
	s.origins = union
	s.mu.Unlock()
}

func (s *Service) CreateAppToken(ctx context.Context, name string, origins []string) (Issued, error) {
	name, err := normalizeAppTokenName(name)
	if err != nil {
		return Issued{}, err
	}
	origins, err = normalizeAppTokenOrigins(origins)
	if err != nil {
		return Issued{}, err
	}
	s.mutations.Lock()
	defer s.mutations.Unlock()
	secret, err := remotehost.GenerateOpaqueSecret(rand.Reader, appTokenPrefix)
	if err != nil {
		return Issued{}, err
	}
	random := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return Issued{}, fmt.Errorf("app token: generate app token ID: %w", err)
	}
	token := Token{
		ID: "apt_" + base64.RawURLEncoding.EncodeToString(random), Name: name,
		ActorKind: ActorKind, CreatedAt: s.now(), Origins: origins,
	}
	union, err := s.store.CreateAppToken(ctx, StoredToken{
		Token: token, TokenHash: remotehost.HashCredential(secret),
	})
	if err != nil {
		return Issued{}, fmt.Errorf("app token: create app token: %w", err)
	}
	s.setOrigins(union)
	return Issued{Token: token, Secret: secret}, nil
}

func (s *Service) ListAppTokens(ctx context.Context, state State) ([]Token, error) {
	if state == "" {
		state = StateActive
	}
	if state != StateActive && state != StateRevoked && state != StateAll {
		return nil, ErrStateInvalid
	}
	tokens, err := s.store.ListAppTokens(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("app token: list app tokens: %w", err)
	}
	return tokens, nil
}

func (s *Service) UpdateAppToken(ctx context.Context, id string, patch Patch) (Token, error) {
	if patch.Name != nil {
		name, err := normalizeAppTokenName(*patch.Name)
		if err != nil {
			return Token{}, err
		}
		patch.Name = &name
	}
	if patch.Origins != nil {
		origins, err := normalizeAppTokenOrigins(*patch.Origins)
		if err != nil {
			return Token{}, err
		}
		patch.Origins = &origins
	}
	s.mutations.Lock()
	defer s.mutations.Unlock()
	token, union, err := s.store.UpdateAppToken(ctx, id, patch, s.now())
	if err != nil {
		return Token{}, fmt.Errorf("app token: update app token: %w", err)
	}
	s.setOrigins(union)
	return token, nil
}

func (s *Service) RevokeAppToken(ctx context.Context, id string) (RevokeResult, error) {
	var result RevokeResult
	canceled, err := s.gate.Revoke(ctx, id, s.connections, func(operationCtx context.Context) error {
		s.mutations.Lock()
		defer s.mutations.Unlock()
		revoked, changed, union, err := s.store.RevokeAppToken(operationCtx, id, s.now())
		if err != nil {
			return err
		}
		s.setOrigins(union)
		result.Token, result.Changed = revoked, changed
		return nil
	})
	result.Canceled = canceled
	return result, err
}

func (s *Service) AuthenticateAppToken(ctx context.Context, secret string) (Token, error) {
	if !remotehost.ValidOpaqueSecret(secret, appTokenPrefix) {
		remotehost.ConstantTimeCredentialMatch(secret, "")
		return Token{}, ErrUnauthenticated
	}
	stored, err := s.store.FindAppTokenByTokenHash(ctx, remotehost.HashCredential(secret))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			remotehost.ConstantTimeCredentialMatch(secret, "")
			return Token{}, ErrUnauthenticated
		}
		return Token{}, fmt.Errorf("app token: find app token credential: %w", err)
	}
	if !remotehost.ConstantTimeCredentialMatch(secret, stored.TokenHash) || stored.Token.ActorKind != ActorKind {
		return Token{}, ErrUnauthenticated
	}
	if !stored.Token.RevokedAt.IsZero() {
		return Token{}, ErrRevoked
	}
	return stored.Token, nil
}

func (s *Service) TouchAppTokenLastSeen(ctx context.Context, id string, epoch uint64) error {
	if err := s.store.TouchAppTokenLastSeen(ctx, id, epoch, s.now()); err != nil {
		return fmt.Errorf("app token: touch app token last use: %w", err)
	}
	return nil
}

func (s *Service) AcquireMutation(ctx context.Context, id string, epoch uint64) (context.Context, error) {
	return s.gate.AcquireMutation(ctx, id, epoch)
}

func (s *Service) RevalidateForCommit(ctx context.Context, id string, epoch uint64) error {
	return s.gate.RevalidateForCommit(ctx, id, epoch)
}

func (s *Service) Connections() remotehost.ConnectionRegistry {
	return s.connections
}

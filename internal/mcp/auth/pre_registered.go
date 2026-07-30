package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func (s *Service) beginPreRegisteredOAuth(
	ctx context.Context,
	cfg ServerConfig,
	redirectURL string,
) (LoginState, error) {
	metadata, err := s.preRegisteredMetadata(ctx, cfg)
	if err != nil {
		return LoginState{}, err
	}
	if !supportsS256(metadata.CodeChallengeMethodsSupported) {
		return LoginState{}, errors.New("mcp auth: OAuth server must support S256 PKCE")
	}
	pkce, err := newPKCEPair(s.random)
	if err != nil {
		return LoginState{}, err
	}
	state, err := newState(s.random)
	if err != nil {
		return LoginState{}, err
	}
	authorizationURL, err := authorizationURL(
		metadata.AuthorizationEndpoint,
		cfg,
		redirectURL,
		state,
		pkce,
	)
	if err != nil {
		return LoginState{}, err
	}
	return LoginState{
		Target:           cfg.Target.Normalize(),
		RedirectURL:      strings.TrimSpace(redirectURL),
		State:            state,
		Verifier:         pkce.Verifier,
		AuthorizationURL: authorizationURL,
		Metadata:         metadata,
		Config:           cfg,
		ResourceURL:      firstNonEmpty(cfg.ResourceURL, cfg.RemoteURL),
	}, nil
}

func (s *Service) preRegisteredMetadata(ctx context.Context, cfg ServerConfig) (Metadata, error) {
	issuer := strings.TrimSpace(cfg.IssuerURL)
	if issuer == "" {
		return Metadata{}, errors.New("mcp auth: pre-registered issuer is required")
	}
	asm, err := sdkauth.GetAuthServerMetadata(ctx, issuer, s.clientFor(cfg))
	if err != nil {
		return Metadata{}, fmt.Errorf("mcp auth: discover authorization server metadata: %w", err)
	}
	if asm == nil {
		return Metadata{}, errors.New("mcp auth: authorization server metadata is required")
	}
	if !issuersEqual(issuer, asm.Issuer) {
		return Metadata{}, errors.New(
			"mcp auth: pre-registered issuer does not match authorization server",
		)
	}
	return metadataFromAuthServer(asm), nil
}

func (s *Service) refreshConfiguration(
	ctx context.Context,
	cfg ServerConfig,
) (ServerConfig, Metadata, error) {
	if cfg.registrationStrategy() == RegistrationAuto {
		return s.hostedRefreshConfig(ctx, cfg)
	}
	metadata, err := s.preRegisteredMetadata(ctx, cfg)
	if err != nil {
		return ServerConfig{}, Metadata{}, err
	}
	resolved := cfg
	resolved.ResourceURL = firstNonEmpty(cfg.ResourceURL, cfg.RemoteURL)
	return resolved, metadata, nil
}

func (s *Service) metadataForConfig(ctx context.Context, cfg ServerConfig) (Metadata, error) {
	if cfg.registrationStrategy() == RegistrationPreRegistered {
		return s.preRegisteredMetadata(ctx, cfg)
	}
	resourceURL := firstNonEmpty(cfg.ResourceURL, cfg.RemoteURL)
	prm, err := s.protectedResourceMetadata(ctx, cfg, resourceURL)
	if err != nil {
		return Metadata{}, err
	}
	if len(prm.AuthorizationServers) == 0 {
		return Metadata{}, errors.New(
			"mcp auth: protected resource metadata has no authorization servers",
		)
	}
	asm, err := sdkauth.GetAuthServerMetadata(ctx, prm.AuthorizationServers[0], s.clientFor(cfg))
	if err != nil {
		return Metadata{}, fmt.Errorf("mcp auth: discover authorization server metadata: %w", err)
	}
	if asm == nil {
		return Metadata{}, errors.New("mcp auth: authorization server metadata is required")
	}
	return metadataFromAuthServer(asm), nil
}

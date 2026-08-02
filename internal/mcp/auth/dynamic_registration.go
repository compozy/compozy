package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/mcp/securehttp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const maxDynamicRegistrationResponseBytes = int64(1 << 20)

func (s *Service) dynamicClientRegistration(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
	redirectURL string,
	asm *oauthex.AuthServerMeta,
) (string, string, string, error) {
	if s.registrations == nil {
		return "", "", "", errors.New(
			"mcp auth: registration store is required for dynamic registration",
		)
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return "", "", "", err
	}
	existing, err := s.registrations.GetMCPAuthRegistration(ctx, cfg.Target)
	if err == nil && s.dynamicRegistrationReusable(
		existing,
		cfg,
		fingerprint,
		resourceURL,
		redirectURL,
		asm.Issuer,
	) {
		secret, resolveErr := s.resolveRegistrationSecret(ctx, existing.ClientSecretRef)
		if resolveErr != nil {
			return "", "", "", resolveErr
		}
		return existing.ClientID, secret, existing.TokenEndpointAuthMethod, nil
	}
	if err != nil && !errors.Is(err, ErrRegistrationNotFound) {
		return "", "", "", fmt.Errorf("mcp auth: load client registration: %w", err)
	}
	if strings.TrimSpace(asm.RegistrationEndpoint) == "" {
		return "", "", "", errors.New(
			"mcp auth: authorization server does not support CIMD or dynamic registration",
		)
	}
	response, err := s.registerClient(
		ctx,
		cfg,
		asm.RegistrationEndpoint,
		&oauthex.ClientRegistrationMetadata{
			RedirectURIs:  []string{redirectURL},
			GrantTypes:    []string{"authorization_code", "refresh_token"},
			ResponseTypes: []string{"code"},
			ClientName:    "Compozy",
			Scope:         strings.Join(trimStrings(cfg.Scopes), " "),
		},
	)
	if err != nil {
		return "", "", "", fmt.Errorf("mcp auth: dynamic client registration: %w", err)
	}
	registration, secret, err := s.persistDynamicRegistration(
		ctx,
		cfg,
		resourceURL,
		redirectURL,
		asm.Issuer,
		fingerprint,
		response,
	)
	if err != nil {
		return "", "", "", err
	}
	return registration.ClientID, secret, registration.TokenEndpointAuthMethod, nil
}

func (s *Service) dynamicRegistrationReusable(
	existing ClientRegistration,
	cfg ServerConfig,
	fingerprint string,
	resourceURL string,
	redirectURL string,
	issuer string,
) bool {
	if existing.DefinitionFingerprint != fingerprint || existing.ResourceURL != resourceURL ||
		!issuersEqual(existing.Issuer, issuer) ||
		strings.TrimSpace(existing.ClientID) == "" ||
		strings.TrimSpace(existing.RedirectURL) != strings.TrimSpace(redirectURL) ||
		!scopesCover(existing.Scopes, cfg.Scopes) ||
		(!existing.ClientSecretExpiresAt.IsZero() &&
			!existing.ClientSecretExpiresAt.After(s.now().UTC())) {
		return false
	}
	method := strings.TrimSpace(existing.TokenEndpointAuthMethod)
	switch method {
	case tokenEndpointAuthMethodNone:
		return true
	case tokenEndpointAuthMethodBasic, tokenEndpointAuthMethodPost:
		return strings.TrimSpace(existing.ClientSecretRef) != ""
	default:
		return false
	}
}

func (s *Service) persistDynamicRegistration(
	ctx context.Context,
	cfg ServerConfig,
	resourceURL string,
	redirectURL string,
	issuer string,
	fingerprint string,
	response dynamicRegistrationResponse,
) (ClientRegistration, string, error) {
	if strings.TrimSpace(response.Registration.ClientID) == "" {
		return ClientRegistration{}, "", errors.New("mcp auth: DCR response client id is required")
	}
	secret := strings.TrimSpace(response.Registration.ClientSecret)
	authMethod, err := registrationTokenEndpointAuthMethod(
		response.Registration.TokenEndpointAuthMethod,
		secret,
	)
	if err != nil {
		return ClientRegistration{}, "", err
	}
	if authMethod == tokenEndpointAuthMethodNone {
		secret = ""
	}
	registration := ClientRegistration{
		Target:                  cfg.Target.Normalize(),
		DefinitionFingerprint:   fingerprint,
		ResourceURL:             resourceURL,
		Issuer:                  strings.TrimSpace(issuer),
		ClientID:                strings.TrimSpace(response.Registration.ClientID),
		RegistrationClientURI:   response.RegistrationClientURI,
		ClientIDIssuedAt:        response.Registration.ClientIDIssuedAt.UTC(),
		ClientSecretExpiresAt:   response.Registration.ClientSecretExpiresAt.UTC(),
		TokenEndpointAuthMethod: authMethod,
		RedirectURL:             redirectURL,
		Scopes:                  trimStrings(cfg.Scopes),
		UpdatedAt:               s.now().UTC(),
	}
	persisted, err := s.registrations.SaveMCPAuthRegistration(
		ctx,
		registration,
		RegistrationSecrets{
			ClientSecret:            secret,
			RegistrationAccessToken: response.RegistrationAccessToken,
		},
	)
	if err != nil {
		return ClientRegistration{}, "", fmt.Errorf(
			"mcp auth: persist dynamic registration: %w",
			err,
		)
	}
	return persisted, secret, nil
}

type dynamicRegistrationResponse struct {
	Registration            oauthex.ClientRegistrationResponse
	RegistrationAccessToken string `json:"registration_access_token"`
	RegistrationClientURI   string `json:"registration_client_uri"`
}

func (s *Service) registerClient(
	ctx context.Context,
	cfg ServerConfig,
	endpoint string,
	metadata *oauthex.ClientRegistrationMetadata,
) (response dynamicRegistrationResponse, err error) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: encode dynamic registration: %w",
			err,
		)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: build dynamic registration request: %w",
			err,
		)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := s.clientFor(cfg).Do(req)
	if err != nil {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: dynamic registration request: %w",
			err,
		)
	}
	defer func() {
		err = errors.Join(err, drainAndCloseResponseBody(resp.Body))
	}()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: dynamic registration rejected with HTTP %d",
			resp.StatusCode,
		)
	}
	return decodeDynamicRegistrationResponse(resp.Body, endpoint)
}

func decodeDynamicRegistrationResponse(body io.Reader, endpoint string) (dynamicRegistrationResponse, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxDynamicRegistrationResponseBytes+1))
	if err != nil {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: read dynamic registration response: %w",
			err,
		)
	}
	if int64(len(payload)) > maxDynamicRegistrationResponseBytes {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: read dynamic registration response: %w: limit=%d",
			securehttp.ErrResponseBodyTooLarge,
			maxDynamicRegistrationResponseBytes,
		)
	}
	var registration oauthex.ClientRegistrationResponse
	if err := json.Unmarshal(payload, &registration); err != nil {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: decode dynamic registration response: %w",
			err,
		)
	}
	var extension struct {
		RegistrationAccessToken string `json:"registration_access_token"`
		RegistrationClientURI   string `json:"registration_client_uri"`
	}
	if err := json.Unmarshal(payload, &extension); err != nil {
		return dynamicRegistrationResponse{}, fmt.Errorf(
			"mcp auth: decode dynamic registration extension: %w",
			err,
		)
	}
	if strings.TrimSpace(registration.ClientID) == "" {
		return dynamicRegistrationResponse{}, errors.New(
			"mcp auth: dynamic registration response client id is required",
		)
	}
	registrationAccessToken, registrationClientURI, err := normalizeRegistrationManagementCredentials(
		endpoint,
		extension.RegistrationAccessToken,
		extension.RegistrationClientURI,
	)
	if err != nil {
		return dynamicRegistrationResponse{}, err
	}
	return dynamicRegistrationResponse{
		Registration:            registration,
		RegistrationAccessToken: registrationAccessToken,
		RegistrationClientURI:   registrationClientURI,
	}, nil
}

func (s *Service) resolveRegistrationSecret(ctx context.Context, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", nil
	}
	if s.secretResolver == nil {
		return "", errors.New("mcp auth: registration secret resolver is required")
	}
	secret, err := s.secretResolver(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("mcp auth: resolve registration client secret: %w", err)
	}
	return secret, nil
}

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
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
	loginScopes hostedLoginScopes,
) (hostedOAuthClient, error) {
	if s.registrations == nil {
		return hostedOAuthClient{}, errors.New(
			"mcp auth: registration store is required for dynamic registration",
		)
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return hostedOAuthClient{}, err
	}
	existing, err := s.registrations.GetMCPAuthRegistration(ctx, cfg.Target)
	reuseCfg := cfg
	reuseCfg.Scopes = loginScopes.values
	if loginScopes.metadataOnly && len(existing.Scopes) == 0 {
		// A scope-less DCR client uses the issuer's defaults until the resource demands specific scopes.
		reuseCfg.Scopes = nil
	}
	if err == nil && s.dynamicRegistrationReusable(
		existing,
		reuseCfg,
		fingerprint,
		resourceURL,
		redirectURL,
		asm.Issuer,
	) {
		secret, resolveErr := s.resolveRegistrationSecret(ctx, existing.ClientSecretRef)
		if resolveErr != nil {
			return hostedOAuthClient{}, resolveErr
		}
		return registeredOAuthClient(existing, secret, reuseCfg.Scopes), nil
	}
	if err != nil && !errors.Is(err, ErrRegistrationNotFound) {
		return hostedOAuthClient{}, fmt.Errorf("mcp auth: load client registration: %w", err)
	}
	if strings.TrimSpace(asm.RegistrationEndpoint) == "" {
		return hostedOAuthClient{}, errors.New(
			"mcp auth: authorization server does not support CIMD or dynamic registration",
		)
	}
	requestedScopes := withOfflineAccess(loginScopes.values, metadataFromAuthServer(asm), true)
	response, err := s.registerClient(
		ctx,
		cfg,
		asm.RegistrationEndpoint,
		&oauthex.ClientRegistrationMetadata{
			RedirectURIs:  []string{redirectURL},
			GrantTypes:    []string{"authorization_code", "refresh_token"},
			ResponseTypes: []string{"code"},
			ClientName:    "Compozy",
			Scope:         strings.Join(requestedScopes, " "),
		},
	)
	if err != nil {
		return hostedOAuthClient{}, fmt.Errorf("mcp auth: dynamic client registration: %w", err)
	}
	cfg.Scopes, err = response.grantedScopes(requestedScopes, loginScopes.values)
	if err != nil {
		return hostedOAuthClient{}, err
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
		return hostedOAuthClient{}, err
	}
	return registeredOAuthClient(registration, secret, loginScopes.values), nil
}

func registeredOAuthClient(registration ClientRegistration, secret string, scopes []string) hostedOAuthClient {
	return hostedOAuthClient{
		id: registration.ClientID, secret: secret, authMethod: registration.TokenEndpointAuthMethod,
		scopes: scopes, allowOfflineAccess: slices.Contains(registration.Scopes, offlineAccessScope),
	}
}

func (s *Service) loadTokenRegistration(ctx context.Context, target Target) (ClientRegistration, error) {
	registration, err := s.registrations.GetMCPAuthRegistration(ctx, target)
	// CIMD has no server-issued auth method, while DCR persists one even for public clients.
	if err == nil && registration.TokenEndpointAuthMethod == "" && registration.hasCIMDIdentity() &&
		(s.redirectURI == "" || s.redirectURI == strings.TrimSpace(registration.RedirectURL)) {
		registration.TokenEndpointAuthMethod = tokenEndpointAuthMethodNone
	}
	return registration, err
}

func (r ClientRegistration) hasCIMDIdentity() bool {
	if r.ClientSecretRef != "" || r.RegistrationAccessTokenRef != "" || r.RegistrationClientURI != "" ||
		!r.ClientIDIssuedAt.IsZero() || !r.ClientSecretExpiresAt.IsZero() {
		return false
	}
	return validateClientMetadataURL(r.ClientID) == nil
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
	scopePresent            bool
}

func (r dynamicRegistrationResponse) grantedScopes(requested, required []string) ([]string, error) {
	scopes := requested
	if r.scopePresent {
		scopes = strings.Fields(r.Registration.Scope)
	}
	if !scopesCover(scopes, required) {
		return nil, errors.New(
			"mcp auth: issuer registration omitted required resource scopes; check the issuer's client scope settings",
		)
	}
	if grants := r.Registration.GrantTypes; len(grants) != 0 && !slices.Contains(grants, serviceRefreshTokenKey) {
		scopes = resourceOAuthScopes(scopes)
	}
	return scopes, nil
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
		RegistrationAccessToken string  `json:"registration_access_token"`
		RegistrationClientURI   string  `json:"registration_client_uri"`
		Scope                   *string `json:"scope"`
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
		scopePresent:            extension.Scope != nil,
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
